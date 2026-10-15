"""pamone-obs: reusable helper for the pamone obs-agent sidecar.

Features are opt-in per app:
  - logs    : mirror stdout/stderr into <SINK_DIR>/app.log (stdout-only apps)
  - metrics : app-cgroup CPU/memory/IO/network -> <SINK_DIR>/metrics.jsonl
  - traces  : mirror spans from the app's tracer  -> <SINK_DIR>/spans.jsonl

Enable via env (defaults on; disable what you do not need):
  PAMONE_ENABLED=true
  PAMONE_LOGS_ENABLED=true          # logs only (with resource) -> omit for traces
  PAMONE_METRICS_ENABLED=true
  PAMONE_TRACES_ENABLED=false       # metrics-only apps set this false
  PAMONE_SINK_DIR=/var/log/pamone   # shared emptyDir mounted into the sidecar

Usage (explicit):
    import pamone_obs; pamone_obs.init()

Usage (zero-code, install once in the base image):
    pamone-obs install-pth           # adds a .pth that calls autostart()
"""
import logging
import os

from . import contract, metrics, trace  # noqa: F401 (re-export)
from .contract import install_log_tee

logger = logging.getLogger("pamone_obs")

__all__ = ["init", "autostart", "contract", "metrics", "trace"]
__version__ = "0.1.1"


def _bool(env, default):
    value = os.getenv(env)
    if value is None:
        return default
    return value.strip().lower() not in ("0", "false", "no", "off", "")


def init(metrics_enabled=None, logs_enabled=None, traces_enabled=None):
    """Start the selected components. Never raises (best effort)."""
    if not _bool("PAMONE_ENABLED", True):
        return
    if logs_enabled is None:
        logs_enabled = _bool("PAMONE_LOGS_ENABLED", True)
    if metrics_enabled is None:
        metrics_enabled = _bool("PAMONE_METRICS_ENABLED", True)
    if traces_enabled is None:
        traces_enabled = _bool("PAMONE_TRACES_ENABLED", True)
    try:
        contract.ensure_dir()
        if logs_enabled:
            install_log_tee()
        if metrics_enabled:
            metrics.start()
        if traces_enabled:
            trace.start()
    except Exception as exc:
        logger.warning(f"pamone-obs: init failed ({exc})")


def autostart():
    """Entry point for the .pth import hook (zero-code). Honours PAMONE_AUTOSTART."""
    if not _bool("PAMONE_AUTOSTART", True):
        return
    try:
        init()
    except Exception:
        pass
