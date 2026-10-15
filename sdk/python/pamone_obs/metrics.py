"""Resource metrics from the app container's OWN cgroup (no hostPath).

Writes <SINK_DIR>/metrics.jsonl: {"metric","unit","value"} per line. Works under
Pod Security "restricted" because the container can read its own cgroup v2.
"""
import logging
import os
import threading
import time

from . import contract

logger = logging.getLogger("pamone_obs")

CGROUP_ROOT = os.getenv("PAMONE_CGROUP_ROOT", "/sys/fs/cgroup")
NET_DEV = os.getenv("PAMONE_NET_DEV", "/proc/net/dev")
INTERVAL = float(os.getenv("PAMONE_METRICS_INTERVAL", "10"))

_started = False


def _read(path):
    try:
        with open(path, "r") as fh:
            return fh.read()
    except OSError:
        return None


def _read_kv(path):
    out = {}
    text = _read(path)
    if not text:
        return out
    for line in text.splitlines():
        parts = line.split()
        if len(parts) == 2:
            try:
                out[parts[0]] = int(parts[1])
            except ValueError:
                pass
    return out


def _cpu_limit_cores():
    text = _read(os.path.join(CGROUP_ROOT, "cpu.max"))
    if not text:
        return None
    parts = text.split()
    if len(parts) != 2 or parts[0] == "max":
        return None
    try:
        quota, period = int(parts[0]), int(parts[1])
    except ValueError:
        return None
    return quota / period if period > 0 else None


def _mem_limit_bytes():
    text = _read(os.path.join(CGROUP_ROOT, "memory.max"))
    if not text:
        return None
    value = text.strip()
    if value in ("", "max"):
        return None
    try:
        return int(value)
    except ValueError:
        return None


def _io_bytes():
    text = _read(os.path.join(CGROUP_ROOT, "io.stat"))
    if not text:
        return None
    read_bytes = write_bytes = 0
    for line in text.splitlines():
        for token in line.split():
            if token.startswith("rbytes="):
                try:
                    read_bytes += int(token.split("=", 1)[1])
                except ValueError:
                    pass
            elif token.startswith("wbytes="):
                try:
                    write_bytes += int(token.split("=", 1)[1])
                except ValueError:
                    pass
    return read_bytes, write_bytes


def _net_bytes():
    text = _read(NET_DEV)
    if not text:
        return None
    rx = tx = 0
    for line in text.splitlines()[2:]:
        if ":" not in line:
            continue
        _, rest = line.split(":", 1)
        cols = rest.split()
        if len(cols) < 9:
            continue
        try:
            rx += int(cols[0])
            tx += int(cols[8])
        except ValueError:
            pass
    return rx, tx


def collect():
    """Return the metric rows for the current tick."""
    rows = []

    stat = _read_kv(os.path.join(CGROUP_ROOT, "cpu.stat"))
    if stat.get("usage_usec"):
        rows.append({"metric": "container_cpu_usage_usec_total", "unit": "usec", "value": stat["usage_usec"]})

    cores = _cpu_limit_cores()
    if cores is not None:
        rows.append({"metric": "container_cpu_limit_cores", "unit": "cores", "value": cores})

    mem = _read_kv(os.path.join(CGROUP_ROOT, "memory.stat"))
    current = _read(os.path.join(CGROUP_ROOT, "memory.current"))
    if current:
        try:
            cur = int(current.strip())
            working = cur - mem["file"] if mem.get("file") else cur
            rows.append({"metric": "container_memory_current_bytes", "unit": "bytes", "value": cur})
            rows.append({"metric": "container_memory_working_set_bytes", "unit": "bytes", "value": max(working, 0)})
        except ValueError:
            pass
    if mem.get("anon"):
        rows.append({"metric": "container_memory_rss_bytes", "unit": "bytes", "value": mem["anon"]})

    mem_limit = _mem_limit_bytes()
    if mem_limit is not None:
        rows.append({"metric": "container_memory_limit_bytes", "unit": "bytes", "value": mem_limit})

    io = _io_bytes()
    if io is not None:
        rows.append({"metric": "container_fs_read_bytes_total", "unit": "bytes", "value": io[0]})
        rows.append({"metric": "container_fs_write_bytes_total", "unit": "bytes", "value": io[1]})

    net = _net_bytes()
    if net is not None:
        rows.append({"metric": "container_network_receive_bytes_total", "unit": "bytes", "value": net[0]})
        rows.append({"metric": "container_network_transmit_bytes_total", "unit": "bytes", "value": net[1]})

    return rows


def _loop():
    logger.info(f"pamone metrics: {contract.METRICS_PATH} every {INTERVAL}s (cgroup {CGROUP_ROOT})")
    while True:
        try:
            contract.append_jsonl(contract.METRICS_PATH, collect())
        except Exception as exc:  # never crash the app for metrics
            logger.debug(f"pamone metrics: {exc}")
        time.sleep(INTERVAL)


def start():
    global _started
    if _started:
        return
    _started = True
    threading.Thread(target=_loop, name="pamone-metrics", daemon=True).start()
