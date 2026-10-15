package pamone

import "sync"

var startOnce sync.Once

// Init starts the file writers once: logs (stdout/stderr -> app.log) and
// resource metrics (cgroup -> metrics.jsonl). Traces are mirrored by adding
// NewSpanProcessor() to the app's OpenTelemetry TracerProvider. Best effort.
func Init() {
	if !cfg.enabled {
		return
	}
	startOnce.Do(func() {
		ensureDir()
		if cfg.logs {
			startLogs()
		}
		if cfg.metrics {
			startMetrics()
		}
	})
}
