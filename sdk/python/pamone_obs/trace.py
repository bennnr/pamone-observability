"""Optional trace mirroring to <SINK_DIR>/spans.jsonl.

Additive: the app's existing tracer keeps working (Datadog path untouched).
Auto-detects the tracer in use:
  - ddtrace  -> a TraceProcessor registered via tracer.configure(trace_processors=)
  - OpenTelemetry -> a SpanProcessor added to the app's TracerProvider

Enable only where traces are wanted; metrics/logs live in the metrics module.
"""
import logging
import os
import threading

from . import contract

logger = logging.getLogger("pamone_obs")

MODE = os.getenv("PAMONE_TRACES_BACKEND", "auto").lower()  # auto|ddtrace|otel
_started = False
_lock = threading.Lock()


def _hex(value, width):
    if value is None:
        return ""
    try:
        return format(int(value), "0{}x".format(width))
    except (TypeError, ValueError):
        return ""


def _write(rows):
    contract.append_jsonl(contract.SPANS_PATH, rows)


def _start_ddtrace():
    from ddtrace._trace.processor import TraceProcessor
    from ddtrace import tracer

    class PamoneTraceProcessor(TraceProcessor):
        def process_trace(self, trace):
            try:
                rows = []
                for span in trace:
                    rows.append(
                        {
                            "ts": (span.start_ns or 0) // 1_000_000,
                            "trace_id": _hex(span.trace_id, 32),
                            "span_id": _hex(span.span_id, 16),
                            "parent_span_id": _hex(span.parent_id, 16),
                            "service": span.service or "",
                            "operation": span.name or "",
                            "span_kind": "internal",
                            "duration_ns": int(span.duration_ns or 0),
                            "status_code": "ERROR" if span.error else "OK",
                        }
                    )
                if rows:
                    _write(rows)
            except Exception as exc:  # never disturb the app / Datadog
                logger.debug(f"pamone trace: {exc}")
            return trace

    tracer.configure(trace_processors=[PamoneTraceProcessor()])
    return "ddtrace"


def _start_otel():
    from opentelemetry import trace as otel_api
    from opentelemetry.sdk.trace import TracerProvider, SpanProcessor

    provider = otel_api.get_tracer_provider()
    if not isinstance(provider, TracerProvider):
        # OTel SDK not installed/configured by the app; nothing to attach to.
        return None

    class PamoneSpanProcessor(SpanProcessor):
        def on_end(self, span):
            try:
                ctx = span.get_span_context()
                parent = getattr(span, "parent", None)
                service = ""
                try:
                    service = span.resource.attributes.get("service.name", "")
                except Exception:
                    pass
                status = "ERROR" if getattr(span.status, "status_code", 0) == 2 else "OK"
                _write(
                    [
                        {
                            "ts": (span.start_time or 0) // 1_000_000,
                            "trace_id": _hex(ctx.trace_id, 32),
                            "span_id": _hex(ctx.span_id, 16),
                            "parent_span_id": _hex(getattr(parent, "span_id", None), 16),
                            "service": service,
                            "operation": span.name or "",
                            "span_kind": "internal",
                            "duration_ns": int((span.end_time or 0) - (span.start_time or 0)),
                            "status_code": status,
                        }
                    ]
                )
            except Exception as exc:
                logger.debug(f"pamone trace: {exc}")

    provider.add_span_processor(PamoneSpanProcessor())
    return "otel"


def start():
    """Attach the trace mirror. Returns the backend used, or None if none."""
    global _started
    with _lock:
        if _started:
            return None
        _started = True
    order = ["ddtrace", "otel"] if MODE == "auto" else [MODE]
    for backend in order:
        try:
            used = _start_ddtrace() if backend == "ddtrace" else _start_otel()
        except Exception as exc:
            logger.debug(f"pamone trace: {backend} not available ({exc})")
            used = None
        if used:
            logger.info(f"pamone trace: mirroring spans to {contract.SPANS_PATH} via {used}")
            return used
    logger.info("pamone trace: no tracer detected; spans not mirrored")
    return None
