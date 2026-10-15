# Plan — pamone SDK for Go

Goal: a Go module that implements the **same file contract** as the Python SDK
(`../python/CONTRACT.md`), so any Go app can ship **logs + resource metrics +
traces** to the pamone obs-agent sidecar with no hostPath (Pod Security
`restricted` safe) and without changing Datadog/its existing tracing.

Module path (VCS-distributed, no registry needed):
```
github.com/bennnr/pamone-observability/sdk/go
```
Import as `go get github.com/bennnr/pamone-observability/sdk/go@v0.1.0`.

## 1. Contract parity (reuse the same files)
`<SINK_DIR>/app.log`, `<SINK_DIR>/metrics.jsonl`, `<SINK_DIR>/spans.jsonl` with
the exact schemas in `sdk/python/CONTRACT.md`. The agent already reads them
(`AGENT_LOG_PATH` / `AGENT_METRICS_PATH` / `AGENT_SPANS_PATH`).

## 2. Package layout
```
sdk/go/
  go.mod                      # module github.com/bennnr/pamone-observability/sdk/go
  pamone/
    config.go                 # env config (PAMONE_*)
    contract.go               # paths + JSONL writer (rotation) + helpers
    logs.go                   # stdout/stderr tee -> app.log
    metrics.go                # cgroup v2 + /proc/net/dev -> metrics.jsonl
    trace.go                  # OTel SpanProcessor (+ dd-trace-go hook if feasible)
    pamone.go                 # Init(ctx, Options) / Start()
  autostart/                  # blank-import package: func init(){ pamone.Start() }
  README.md
  example/main.go
  *_test.go
```

## 3. API
```go
import pamone "github.com/bennnr/pamone-observability/sdk/go/pamone"

func main() {
    pamone.Init(pamone.Options{ /* optional overrides */ }) // starts goroutines
    // ... app ...
}
```
Zero-touch alternative (add one blank import):
```go
import _ "github.com/bennnr/pamone-observability/sdk/go/autostart"
```
Env-driven (same names as Python): `PAMONE_SINK_DIR`, `PAMONE_ENABLED`,
`PAMONE_LOGS_ENABLED`, `PAMONE_METRICS_ENABLED`, `PAMONE_TRACES_ENABLED`,
`PAMONE_TRACES_BACKEND`, `PAMONE_METRICS_INTERVAL`, `PAMONE_CGROUP_ROOT`,
`PAMONE_NET_DEV`, `PAMONE_MAX_FILE_BYTES`.

## 4. Feature details
- **logs**: `os.Stdout`/`os.Stderr` are `*os.File`; capture by `os.Pipe()` —
  replace `os.Stdout` with the pipe writer, and a goroutine copies each write to
  the original fd **and** `app.log` (also `log.SetOutput(MultiWriter(...))`).
  Must be careful with `log` default flags; document that libraries writing
  directly to fd 1 via syscall are not captured (same caveat as Python).
- **metrics**: read `/sys/fs/cgroup/{cpu.stat,cpu.max,memory.current,memory.max,
  memory.stat,io.stat}` + `/proc/net/dev`; emit the same metric names/units.
  (Optionally also Go `runtime/metrics`, but keep container metrics as the
  contract; runtime metrics are a later addition.)
- **traces**: primary target **OpenTelemetry Go** (`go.opentelemetry.io/otel/sdk/trace`):
  add a `SpanProcessor` that serializes `ReadOnlySpan` → `spans.jsonl`
  (trace/span/parent hex, name, service from resource, duration, status).
  Secondary: **dd-trace-go** — evaluate hooks (`tracer.Start` with a custom
  writer is not public); if not feasible, document OTel as the Go path and keep
  dd-trace-go untouched (traces simply not mirrored there).

## 5. Distribution (Go-native, simpler than PyPI)
- Go modules resolve from the git repo by **tag** — no upload step. Releasing =
  `git tag sdk/go/v0.1.0 && git push --tags`.
- Consumers: `go get github.com/bennnr/pamone-observability/sdk/go@v0.1.0`.
- Optional internal `GOPROXY` (Athens/JFrog) for air-gapped clusters; otherwise
  direct VCS via `GOPROXY=direct` / `GONOSUMDB`/`GOPRIVATE` as needed.
- Tag naming: use the Go module subdir tag (`sdk/go/vX.Y.Z`) so `go get` works
  when the module lives in a subdirectory of the repo.

## 6. CI (GitHub Actions)
- `go vet ./sdk/go/...`, `go test ./sdk/go/...`, `gofmt -l`.
- Build example. No publish job (tags publish implicitly for Go).
- Add a job that verifies the JSONL output against a synthetic cgroup (mirrors
  the Python tests).

## 7. Tests / verification
- Unit: cgroup parsers, JSONL writer (rotation), config.
- Contract test: write sample files, run the real `obs-agent` binary against
  them (like the Python E2E) and assert the payloads.
- Example app prints logs + writes a span, then check `<SINK_DIR>/*.jsonl`.

## 8. Milestones / checklist
- [ ] `go.mod` + module structure.
- [ ] `contract.go` (paths, writer, rotation).
- [ ] `logs.go` (pipe tee) + tests.
- [ ] `metrics.go` (cgroup + netdev) + tests.
- [ ] `trace.go` (OTel SpanProcessor) + tests.
- [ ] `autostart` package + `example/`.
- [ ] CI (vet/test/fmt).
- [ ] Tag `sdk/go/v0.1.0`, README with `go get` + manifest snippet.
- [ ] (later) dd-trace-go adapter if a viable hook exists.
- [ ] (later) runtime/metrics addition (opt-in).

## 9. Manifest / sidecar (unchanged)
Same as Python: one shared `emptyDir` mounted at `PAMONE_SINK_DIR` into the app
and the sidecar; use the Helm library chart `deploy/helm/pamone-sidecar`
(`pamone-sidecar.appMounts`/`.appEnv`/`.container`/`.volume`). The Go app only
needs the blank import (or `pamone.Init`) + env.

## 10. Language rollout after Go
Same contract → Node/Java/Rust shims later; they only need to write the three
files. Keep `CONTRACT.md` as the single source of truth.