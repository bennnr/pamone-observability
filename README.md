# pamone-observability SDK

Reusable libraries that let any application ship **logs, resource metrics and
traces** to the pamone platform through the `obs-agent` sidecar — with **no
hostPath** (Pod Security `restricted` safe) and **without changing** the app's
existing tracing (e.g. Datadog).

Two implementations share one wire contract:

| Language | Package | Status |
|----------|---------|--------|
| Python   | `pamone-obs` (PyPI) | released `0.1.1` |
| Go       | module `github.com/bennnr/pamone-observability/sdk/go` | source ready (`v0.1.0`) |

## Architecture

```
     pod                                  pamone platform
┌───────────────────────────┐        ┌───────────────────────────────┐
│ app (+ pamone SDK)        │        │ ingest → ClickHouse → query   │
│   logs   → app.log        │ shared │        → gateway → dashboard  │
│   metrics→ metrics.jsonl  │ emptyDir│                              │
│   traces → spans.jsonl    │◄──────►│                              │
│ obs-agent sidecar         │  reads │                              │
│   tails the 3 files  ─────┼────────┼─► https://pamone.owllab.site/ingest
└───────────────────────────┘        └───────────────────────────────┘
```

The SDK writes three files into one shared directory (`PAMONE_SINK_DIR`, default
`/var/log/pamone`); the sidecar reads them and forwards to pamone. The agent is
authoritative for identity (cluster/namespace/workload/pod/container) and, when
omitted, timestamps. See [`sdk/python/CONTRACT.md`](sdk/python/CONTRACT.md) for
the exact file schemas.

## What we add to the application image

Nothing to the application **code** for logs/metrics — only the library plus env.

### Python
```dockerfile
RUN pip install "pamone-obs>=0.1.1"        # or add to requirements/uv
RUN pamone-obs install-pth                  # zero-code auto-start (.pth)
```
Then set env (see below). The library:
- mirrors stdout/stderr into `app.log` (fixes stdout-only apps — **no shell tee needed**),
- reads the app's **own cgroup v2** (`cpu.stat`, `cpu.max`, `memory.*`, `io.stat`)
  and `/proc/net/dev` into `metrics.jsonl`,
- optionally mirrors spans from the tracer already present (dd-trace or OTel)
  into `spans.jsonl`.

Explicit alternative to the `.pth`: call `pamone_obs.init()` once at startup.

### Go
```go
import pamone "github.com/bennnr/pamone-observability/sdk/go/pamone"

func main() {
    pamone.Init()                                   // logs + metrics
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithSpanProcessor(pamone.NewSpanProcessor()), // traces
    )
    ...
}
```
Zero-touch for logs+metrics: `import _ ".../sdk/go/autostart"`.

### Env (identical names in both SDKs)
`PAMONE_SINK_DIR` (default `/var/log/pamone`), `PAMONE_ENABLED`,
`PAMONE_LOGS_ENABLED`, `PAMONE_METRICS_ENABLED`, `PAMONE_TRACES_ENABLED`,
`PAMONE_METRICS_INTERVAL`, `PAMONE_CGROUP_ROOT`, `PAMONE_NET_DEV`,
`PAMONE_MAX_FILE_BYTES`.

Pick features per pod: `PAMONE_TRACES_ENABLED=false` → metrics + logs only (pods
without a tracer / Datadog disabled).

## Sidecar pairing

One shared `emptyDir` is mounted into the app and the `obs-agent` sidecar at
`PAMONE_SINK_DIR`; the sidecar env points at the same files:

```
AGENT_LOG_PATH=<sink>/app.log
AGENT_METRICS_PATH=<sink>/metrics.jsonl
AGENT_SPANS_PATH=<sink>/spans.jsonl
AGENT_OTLP_ADDR=:4318          # serves /healthz (probes); spans come from the file
AGENT_ENABLE_METRICS=false     # cgroup is read by the SDK in-app
```

`fsGroup` on the pod makes the volume writable by the app and readable by the
sidecar (it applies to all containers). No hostPath → PSA `restricted` OK.

## Example Kubernetes templates

Ready-to-copy manifests (generic placeholders) live in
[`examples/k8s/`](examples/k8s/):

- [`examples/k8s/python-app-with-sidecar.yaml`](examples/k8s/python-app-with-sidecar.yaml)
- [`examples/k8s/go-app-with-sidecar.yaml`](examples/k8s/go-app-with-sidecar.yaml)

Minimal skeleton (see the files for the full version):

```yaml
spec:
  template:
    spec:
      securityContext:
        fsGroup: 10001            # app + sidecar share the emptyDir group
      containers:
        - name: app
          env:
            - { name: PAMONE_SINK_DIR, value: /var/log/pamone }
            - { name: PAMONE_TRACES_ENABLED, value: "true" }
          volumeMounts:
            - { name: pamone-sink, mountPath: /var/log/pamone }
        - name: obs-agent
          image: pamone-registry.owllab.site/pamone-release/pamone-agent:latest
          env:
            - { name: AGENT_INGEST_URL, value: https://pamone.owllab.site/ingest }
            - name: AGENT_TOKEN
              valueFrom: { secretKeyRef: { name: obs-ingest-token, key: token } }
            - { name: AGENT_LOG_PATH, value: /var/log/pamone/app.log }
            - { name: AGENT_METRICS_PATH, value: /var/log/pamone/metrics.jsonl }
            - { name: AGENT_SPANS_PATH, value: /var/log/pamone/spans.jsonl }
            - { name: AGENT_OTLP_ADDR, value: ":4318" }
            - { name: AGENT_ENABLE_METRICS, value: "false" }
          volumeMounts:
            - { name: pamone-sink, mountPath: /var/log/pamone, readOnly: true }
      volumes:
        - name: pamone-sink
          emptyDir: {}
```

A Helm **library chart** that renders the sidecar is in the platform repo:
`deploy/helm/pamone-sidecar` (helpers `pamone-sidecar.appMounts` / `.appEnv` /
`.container` / `.volume`).

## Distribution

### Python (pamone-obs)
- Public **PyPI**: `pip install pamone-obs` (published via GitHub Trusted
  Publishing; see [`sdk/python/PUBLISHING.md`](sdk/python/PUBLISHING.md)).
- Internal options: private PyPI (devpi/pypiserver/Nexus), vendor the wheel, or
  bake into a base image (`pip install pamone-obs && pamone-obs install-pth`).

### Go
Go modules resolve from git by tag — no registry:
```bash
git tag sdk/go/v0.1.0 && git push origin sdk/go/v0.1.0
# consumers:
go get github.com/bennnr/pamone-observability/sdk/go@v0.1.0
```
Air-gapped clusters: internal `GOPROXY` (Athens/JFrog) or `go mod vendor`.

## Repository layout
```
sdk/python/            pamone-obs (PyPI)      + CONTRACT.md, PUBLISHING.md
sdk/go/                Go module              + example/ (image), README.md
examples/k8s/          copy-paste Deployment manifests
.github/workflows/     CI: publish (PyPI)
```

## Status / report
- **Python `pamone-obs` 0.1.1** on PyPI. Verified: `pip install pamone-obs==0.1.1`,
  import, `install-pth` auto-start writes `app.log` + `metrics.jsonl` (values
  correct), span mirroring verified with ddtrace 3.17.2 (explicit init and via
  `.pth` + `ddtrace-run`).
- **Go SDK**: `go build ./...` + `go vet` clean; functional test writes
  `app.log`, `metrics.jsonl` and `spans.jsonl` (http server + child span, correct
  parent/child).
- **Example image**: `pamone-registry.owllab.site/pamone-example/pamone-go:0.1.0`
  (built from `sdk/go`, pushed to the pamone registry, pulled and run — sink
  files verified).
- **Pods already integrated**: agent is backward compatible; restarting an
  existing workload with its old manifest keeps working (new features just stay
  off until env is updated).

## Roadmap
- Node/Java SDKs (same contract).
- Go: `runtime/metrics` (opt-in) and a dd-trace-go adapter if a viable hook exists.
- Optional mutating webhook for zero-touch sidecar injection.
