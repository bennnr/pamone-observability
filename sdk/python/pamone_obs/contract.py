"""Shared contract for the pamone sidecar files.

The sidecar (obs-agent) reads three files from one shared emptyDir and forwards
them to pamone:

  <SINK_DIR>/app.log       -> logs      (AGENT_LOG_PATH)
  <SINK_DIR>/metrics.jsonl -> metrics   (AGENT_METRICS_PATH)
  <SINK_DIR>/spans.jsonl   -> traces    (AGENT_SPANS_PATH)

Each metrics/spans line is one JSON object. The agent is authoritative for the
identity (cluster/namespace/workload/pod/container) and the timestamp, so the
library only writes the payload fields. This keeps the contract tiny and
language-agnostic (see CONTRACT.md).
"""
import json
import os
import sys
import threading

# ---------------------------------------------------------------------------
# configuration (env)
# ---------------------------------------------------------------------------
SINK_DIR = os.getenv("PAMONE_SINK_DIR", "/var/log/pamone")
LOG_PATH = os.getenv("PAMONE_LOG_PATH", os.path.join(SINK_DIR, "app.log"))
METRICS_PATH = os.getenv("PAMONE_METRICS_PATH", os.path.join(SINK_DIR, "metrics.jsonl"))
SPANS_PATH = os.getenv("PAMONE_SPANS_PATH", os.path.join(SINK_DIR, "spans.jsonl"))
MAX_FILE_BYTES = int(os.getenv("PAMONE_MAX_FILE_BYTES", "5242880"))

_lock = threading.Lock()


def ensure_dir():
    directory = os.path.dirname(METRICS_PATH) or SINK_DIR
    try:
        os.makedirs(directory, exist_ok=True)
    except OSError:
        pass


def _rotate(path):
    try:
        if os.path.getsize(path) > MAX_FILE_BYTES:
            # Truncate; the agent detects the shrink and restarts from 0.
            open(path, "w").close()
    except OSError:
        pass


def append_jsonl(path, rows):
    """Append each row as one compact JSON line. Never raises."""
    if not rows:
        return
    ensure_dir()
    with _lock:
        try:
            _rotate(path)
            with open(path, "a") as fh:
                for row in rows:
                    fh.write(json.dumps(row, separators=(",", ":")) + "\n")
        except OSError:
            pass


class LogTee:
    """Mirror stdout/stderr into the shared log file (solves stdout-only apps).

    Keeps writing to the original stream so container logs / Datadog are
    unaffected. Child processes are NOT captured (they have their own streams).
    """

    def __init__(self, original, path):
        self._original = original
        self._path = path
        ensure_dir()

    def write(self, data):
        try:
            self._original.write(data)
        except Exception:
            pass
        try:
            with _lock:
                _rotate(self._path)
                with open(self._path, "a") as fh:
                    fh.write(data)
        except OSError:
            pass
        return len(data)

    def flush(self):
        try:
            self._original.flush()
        except Exception:
            pass

    def isatty(self):
        return False

    def fileno(self):
        return self._original.fileno()


def install_log_tee():
    if isinstance(sys.stdout, LogTee) or isinstance(sys.stderr, LogTee):
        return
    sys.stdout = LogTee(sys.stdout, LOG_PATH)
    sys.stderr = LogTee(sys.stderr, LOG_PATH)
