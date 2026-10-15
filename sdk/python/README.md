# pamone-obs (Python)

Reusable helper for the **pamone obs-agent sidecar**. Apps install this once and
get logs, resource metrics and (optionally) traces shipped to pamone — with **no
hostPath** (works under Pod Security `restricted`) and **no change to Datadog**.

- **logs** — mirror stdout/stderr into the shared file (fixes stdout-only apps).
- **metrics** — read the app's OWN cgroup v2 → CPU/memory/IO + `/proc/net/dev`.
- **traces** — mirror spans from the tracer already in use (dd-trace or OTel),
  additively; the Datadog path is untouched.

## Install
```bash
pip install pamone-obs              # metrics + logs (+ trace auto-detect)
# optional extras carry the tracer libs if the app does not already have them:
pip install "pamone-obs[trace]"     # ddtrace
pip install "pamone-obs[otel]"      # opentelemetry-sdk
```

## Use
Explicit (one line), after the app's logger/tracer setup:
```python
import pamone_obs
pamone_obs.init()
```
Zero-code (recommended for images): install once, then it auto-starts:
```bash
pamone-obs install-pth              # writes zz_pamone_obs.pth into site-packages
```

## Choose features per app (env)
| env | default | effect |
|-----|---------|--------|
| `PAMONE_ENABLED` | `true` | master switch |
| `PAMONE_LOGS_ENABLED` | `true` | stdout/stderr → `app.log` |
| `PAMONE_METRICS_ENABLED` | `true` | cgroup → `metrics.jsonl` |
| `PAMONE_TRACES_ENABLED` | `true` | spans → `spans.jsonl` (auto-detect tracer) |
| `PAMONE_TRACES_BACKEND` | `auto` | `auto` \| `ddtrace` \| `otel` |
| `PAMONE_SINK_DIR` | `/var/log/pamone` | shared emptyDir mount path |
| `PAMONE_METRICS_INTERVAL` | `10` | seconds |
| `PAMONE_CGROUP_ROOT` | `/sys/fs/cgroup` | cgroup v2 root |
| `PAMONE_MAX_FILE_BYTES` | `5242880` | truncate threshold per file |

Examples:
- **metrics + logs only** (no tracer / no DD): `PAMONE_TRACES_ENABLED=false`
- **with traces** (pod uses dd-trace): leave traces enabled (or set
  `PAMONE_TRACES_BACKEND=ddtrace`).

## Sidecar pairing
Set the agent env to the same paths (see the Helm library chart
`deploy/helm/pamone-sidecar` and `CONTRACT.md`):
```
AGENT_LOG_PATH=/var/log/pamone/app.log
AGENT_METRICS_PATH=/var/log/pamone/metrics.jsonl
AGENT_SPANS_PATH=/var/log/pamone/spans.jsonl
```

## Build & distribute to clusters
Build a wheel and publish it so `pip install pamone-obs` works everywhere:
```bash
python -m build            # -> dist/pamone_obs-0.1.0-py3-none-any.whl
```
Options, best → simplest:
1. **Internal PyPI** (devpi / pypiserver / Nexus) reachable by cluster build
   runners: `twine upload` the wheel once; apps do `pip install pamone-obs`.
2. **Bake into an internal base Python image** (e.g. `playcourt/python` fork):
   install the wheel + `pamone-obs install-pth` once; every app inherits it and
   only needs the env + shared volume → truly zero-code.
3. **Vendor the wheel** into each app image when there is no PyPI egress:
   `COPY pamone_obs-0.1.0-py3-none-any.whl . && pip install ./pamone_obs-*.whl`.

The library is stdlib-only; tracer packages are optional extras.

## Notes / limitations
- The log tee captures the current process only (child processes have their own
  streams). For multi-worker servers, each worker runs `init()` — metrics are
  duplicated (harmless) and each worker's output is written.
- Calls are best-effort and never raise; failures are logged at debug level.
