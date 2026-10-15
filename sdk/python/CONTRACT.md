# pamone sidecar file contract

The sidecar (`obs-agent`) reads up to three files from **one shared emptyDir**
mounted into both the app and the agent. Any language can implement this
contract; the [Python library](./README.md) is a reference implementation.

| File | Purpose | Agent env |
|------|---------|-----------|
| `<SINK_DIR>/app.log`       | logs    | `AGENT_LOG_PATH` |
| `<SINK_DIR>/metrics.jsonl` | metrics | `AGENT_METRICS_PATH` |
| `<SINK_DIR>/spans.jsonl`   | traces  | `AGENT_SPANS_PATH` |

Default `SINK_DIR = /var/log/pamone`. The agent is authoritative for identity
(`cluster`, `namespace`, `workload_name`, `pod`, `container`, `node`) and, when
omitted, the timestamp — writers only provide payload fields.

## app.log
Plain text, one log line per line (no format required). Write stdout/stderr here
if the app cannot write its own log file.

## metrics.jsonl — one JSON object per line
```json
{"metric": "container_memory_working_set_bytes", "unit": "bytes", "value": 318500000}
{"metric": "container_cpu_usage_usec_total",     "unit": "usec",  "value": 104419619}
```
Optional `"ts"`: epoch **milliseconds**. If absent the agent stamps ingest time.

Recommended metric names (match the agent's own collectors so query/charts work):

| metric | unit | meaning |
|--------|------|---------|
| `container_cpu_usage_usec_total` | usec | cumulative CPU (counter; query derives rate) |
| `container_cpu_limit_cores` | cores | CPU limit (quota/period) |
| `container_memory_working_set_bytes` | bytes | memory.current − memory.stat:file |
| `container_memory_current_bytes` | bytes | memory.current |
| `container_memory_rss_bytes` | bytes | memory.stat: anon |
| `container_memory_limit_bytes` | bytes | memory.max |
| `container_fs_read_bytes_total` / `container_fs_write_bytes_total` | bytes | io.stat (counter) |
| `container_network_receive_bytes_total` / `container_network_transmit_bytes_total` | bytes | /proc/net/dev (counter) |

## spans.jsonl — one JSON object per line
```json
{"ts": 1792037877986, "trace_id": "6acf1b2d3e4f5a6b7c8d9e0f1a2b3c4d",
 "span_id": "0123456789abcdef", "parent_span_id": "",
 "service": "my-service", "operation": "GET /health",
 "span_kind": "server", "duration_ns": 1500000, "status_code": "OK"}
```
- `trace_id` hex 32, `span_id`/`parent_span_id` hex 16.
- `span_kind`: `unspecified|internal|server|client|producer|consumer` (default `internal`).
- `status_code`: `UNSET|OK|ERROR` (default `OK`).
- `ts` optional (epoch millis); `duration_ns` required.

## Behaviour notes
- Writers should truncate their file when it grows past a few MB; the agent
  detects the shrink and restarts from byte 0.
- The agent tails incrementally (no re-send). Restarting the app is fine.
- No hostPath is involved → compatible with Pod Security `restricted`.
