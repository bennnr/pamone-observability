# pamone-obs for Go

Go module implementing the pamone sidecar file contract (see
`../python/CONTRACT.md`): logs, resource metrics and traces shipped to the
pamone `obs-agent` sidecar through one shared directory. No hostPath → safe under
Pod Security `restricted`.

Module:
```
github.com/bennnr/pamone-observability/sdk/go
```

## Install
```bash
go get github.com/bennnr/pamone-observability/sdk/go@v0.1.0
```
(The Go SDK is distributed as a Go module from this git repo — no registry.)

## Use
```go
import (
    pamone "github.com/bennnr/pamone-observability/sdk/go/pamone"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
    pamone.Init() // logs + cgroup metrics -> PAMONE_SINK_DIR

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithSpanProcessor(pamone.NewSpanProcessor()), // traces -> spans.jsonl
    )
    // otel.SetTracerProvider(tp); use otelhttp etc. as usual
    ...
}
```
Zero-touch for logs+metrics (blank import):
```go
import _ "github.com/bennnr/pamone-observability/sdk/go/autostart"
```

## Env (same names as the Python SDK)
`PAMONE_SINK_DIR` (default `/var/log/pamone`), `PAMONE_ENABLED`,
`PAMONE_LOGS_ENABLED`, `PAMONE_METRICS_ENABLED`, `PAMONE_TRACES_ENABLED`,
`PAMONE_METRICS_INTERVAL`, `PAMONE_CGROUP_ROOT`, `PAMONE_NET_DEV`,
`PAMONE_MAX_FILE_BYTES`.

Sidecar env points at the same files:
`AGENT_LOG_PATH=<sink>/app.log`, `AGENT_METRICS_PATH=<sink>/metrics.jsonl`,
`AGENT_SPANS_PATH=<sink>/spans.jsonl`.

## Example image
`example/` is a small HTTP server (otelhttp + pamone span processor). Build from
this module root:
```bash
docker build -f example/Dockerfile -t <registry>/pamone-example/pamone-go:0.1.0 .
```
A ready-to-deploy manifest lives at `deploy/k8s/go-smoketest-prod.yaml`.

## Publish / consume from other servers
Go modules resolve from git by **tag** — no upload step:
```bash
git tag sdk/go/v0.1.0
git push origin sdk/go/v0.1.0
# consumers:
go get github.com/bennnr/pamone-observability/sdk/go@v0.1.0
```
For air-gapped clusters run an internal `GOPROXY` (Athens/JFrog) or vendor the
module. Tag the module subdirectory (`sdk/go/vX.Y.Z`) so `go get` resolves when
the module lives in a subdirectory.
