package pamone

import (
	"log"
	"os"
	"sync"
)

var logFileMu sync.Mutex

// startLogs mirrors stdout and stderr into <SINK_DIR>/app.log without touching
// the original fds (container logs are unaffected). Child processes are not
// captured. Best effort.
func startLogs() {
	tee(&os.Stdout)
	tee(&os.Stderr)
	// The standard logger caches os.Stderr at init; point it at the new (teed)
	// stderr so log.Printf output is captured too.
	log.SetOutput(os.Stderr)
}

func tee(target **os.File) {
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	orig := *target
	*target = w
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				_, _ = orig.Write(buf[:n])
				writeLog(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
}

func writeLog(b []byte) {
	path := sinkPath("app.log")
	logFileMu.Lock()
	defer logFileMu.Unlock()
	if fi, err := os.Stat(path); err == nil && fi.Size() > cfg.maxFileBytes {
		_ = os.Truncate(path, 0)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(b)
}
