package pamone

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

var (
	cfg    = loadConfig()
	fileMu sync.Mutex
)

func sinkPath(name string) string { return filepath.Join(cfg.sinkDir, name) }

func ensureDir() { _ = os.MkdirAll(cfg.sinkDir, 0o755) }

// appendJSONL writes one compact JSON object per line, truncating the file first
// when it grows past PAMONE_MAX_FILE_BYTES (the obs-agent detects the shrink and
// restarts from byte 0). Best effort; never panics.
func appendJSONL(path string, rows []any) {
	if len(rows) == 0 {
		return
	}
	fileMu.Lock()
	defer fileMu.Unlock()
	ensureDir()
	if fi, err := os.Stat(path); err == nil && fi.Size() > cfg.maxFileBytes {
		_ = os.Truncate(path, 0)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, row := range rows {
		_ = enc.Encode(row) // json.Encoder appends a newline
	}
}
