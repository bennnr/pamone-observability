package pamone

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	sinkDir      string
	enabled      bool
	logs         bool
	metrics      bool
	traces       bool
	interval     time.Duration
	cgroupRoot   string
	netDev       string
	maxFileBytes int64
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "0", "false", "no", "off", "":
		return false
	default:
		return true
	}
}

func envInt64(key string, def int64) int64 {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func loadConfig() config {
	return config{
		sinkDir:      getenv("PAMONE_SINK_DIR", "/var/log/pamone"),
		enabled:      envBool("PAMONE_ENABLED", true),
		logs:         envBool("PAMONE_LOGS_ENABLED", true),
		metrics:      envBool("PAMONE_METRICS_ENABLED", true),
		traces:       envBool("PAMONE_TRACES_ENABLED", true),
		interval:     envDuration("PAMONE_METRICS_INTERVAL", 10*time.Second),
		cgroupRoot:   getenv("PAMONE_CGROUP_ROOT", "/sys/fs/cgroup"),
		netDev:       getenv("PAMONE_NET_DEV", "/proc/net/dev"),
		maxFileBytes: envInt64("PAMONE_MAX_FILE_BYTES", 5*1024*1024),
	}
}
