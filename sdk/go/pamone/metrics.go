package pamone

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func startMetrics() {
	go func() {
		for {
			appendJSONL(sinkPath("metrics.jsonl"), collectMetrics())
			time.Sleep(cfg.interval)
		}
	}()
}

func collectMetrics() []any {
	rows := make([]any, 0, 12)
	add := func(metric, unit string, value float64) {
		rows = append(rows, map[string]any{"metric": metric, "unit": unit, "value": value})
	}

	if stat, ok := readKV(filepath.Join(cfg.cgroupRoot, "cpu.stat")); ok {
		if v := stat["usage_usec"]; v > 0 {
			add("container_cpu_usage_usec_total", "usec", float64(v))
		}
	}
	if cores, ok := cpuLimitCores(); ok {
		add("container_cpu_limit_cores", "cores", cores)
	}

	mem, _ := readKV(filepath.Join(cfg.cgroupRoot, "memory.stat"))
	if cur, ok := readInt(filepath.Join(cfg.cgroupRoot, "memory.current")); ok {
		working := cur
		if f, ok := mem["file"]; ok && f > 0 && cur > f {
			working = cur - f
		}
		add("container_memory_current_bytes", "bytes", float64(cur))
		add("container_memory_working_set_bytes", "bytes", float64(working))
	}
	if anon, ok := mem["anon"]; ok {
		add("container_memory_rss_bytes", "bytes", float64(anon))
	}
	if limit, ok := readInt(filepath.Join(cfg.cgroupRoot, "memory.max")); ok {
		add("container_memory_limit_bytes", "bytes", float64(limit))
	}

	if r, w, ok := ioBytes(); ok {
		add("container_fs_read_bytes_total", "bytes", float64(r))
		add("container_fs_write_bytes_total", "bytes", float64(w))
	}
	if rx, tx, ok := netBytes(); ok {
		add("container_network_receive_bytes_total", "bytes", float64(rx))
		add("container_network_transmit_bytes_total", "bytes", float64(tx))
	}
	return rows
}

func readFile(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(b), true
}

func readKV(path string) (map[string]int64, bool) {
	text, ok := readFile(path)
	if !ok {
		return nil, false
	}
	out := map[string]int64{}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			if n, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				out[fields[0]] = n
			}
		}
	}
	return out, true
}

func readInt(path string) (int64, bool) {
	text, ok := readFile(path)
	if !ok {
		return 0, false
	}
	s := strings.TrimSpace(text)
	if s == "" || s == "max" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func cpuLimitCores() (float64, bool) {
	text, ok := readFile(filepath.Join(cfg.cgroupRoot, "cpu.max"))
	if !ok {
		return 0, false
	}
	fields := strings.Fields(text)
	if len(fields) != 2 || fields[0] == "max" {
		return 0, false
	}
	quota, err1 := strconv.ParseInt(fields[0], 10, 64)
	period, err2 := strconv.ParseInt(fields[1], 10, 64)
	if err1 != nil || err2 != nil || period <= 0 {
		return 0, false
	}
	return float64(quota) / float64(period), true
}

func ioBytes() (int64, int64, bool) {
	text, ok := readFile(filepath.Join(cfg.cgroupRoot, "io.stat"))
	if !ok {
		return 0, 0, false
	}
	var r, w int64
	for _, line := range strings.Split(text, "\n") {
		for _, tok := range strings.Fields(line) {
			if strings.HasPrefix(tok, "rbytes=") {
				if n, err := strconv.ParseInt(strings.TrimPrefix(tok, "rbytes="), 10, 64); err == nil {
					r += n
				}
			} else if strings.HasPrefix(tok, "wbytes=") {
				if n, err := strconv.ParseInt(strings.TrimPrefix(tok, "wbytes="), 10, 64); err == nil {
					w += n
				}
			}
		}
	}
	return r, w, true
}

func netBytes() (int64, int64, bool) {
	text, ok := readFile(cfg.netDev)
	if !ok {
		return 0, 0, false
	}
	lines := strings.Split(text, "\n")
	var rx, tx int64
	for _, line := range lines {
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		cols := strings.Fields(parts[1])
		if len(cols) < 9 {
			continue
		}
		if n, err := strconv.ParseInt(cols[0], 10, 64); err == nil {
			rx += n
		}
		if n, err := strconv.ParseInt(cols[8], 10, 64); err == nil {
			tx += n
		}
	}
	return rx, tx, true
}
