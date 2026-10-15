package pamone

import (
	"context"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type pamoneSpanProcessor struct{}

// NewSpanProcessor returns an OpenTelemetry SpanProcessor that mirrors finished
// spans to <SINK_DIR>/spans.jsonl. Add it to the app's TracerProvider; the app's
// existing tracing (Datadog etc.) is untouched:
//
//	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(pamone.NewSpanProcessor()))
func NewSpanProcessor() sdktrace.SpanProcessor { return pamoneSpanProcessor{} }

func (pamoneSpanProcessor) OnStart(context.Context, sdktrace.ReadWriteSpan) {}

func (pamoneSpanProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	if !cfg.enabled || !cfg.traces {
		return
	}
	sc := s.SpanContext()
	row := map[string]any{
		"ts":             s.StartTime().UnixMilli(),
		"trace_id":       sc.TraceID().String(),
		"span_id":        sc.SpanID().String(),
		"parent_span_id": spanID(s.Parent()),
		"service":        serviceName(s),
		"operation":      s.Name(),
		"span_kind":      kindName(s.SpanKind()),
		"duration_ns":    s.EndTime().Sub(s.StartTime()).Nanoseconds(),
		"status_code":    statusName(s.Status().Code),
	}
	appendJSONL(sinkPath("spans.jsonl"), []any{row})
}

func (pamoneSpanProcessor) Shutdown(context.Context) error   { return nil }
func (pamoneSpanProcessor) ForceFlush(context.Context) error { return nil }

func spanID(sc trace.SpanContext) string {
	if sc.HasSpanID() {
		return sc.SpanID().String()
	}
	return ""
}

func serviceName(s sdktrace.ReadOnlySpan) string {
	for _, kv := range s.Resource().Attributes() {
		if string(kv.Key) == "service.name" {
			return kv.Value.AsString()
		}
	}
	return ""
}

func kindName(k trace.SpanKind) string {
	switch k {
	case trace.SpanKindServer:
		return "server"
	case trace.SpanKindClient:
		return "client"
	case trace.SpanKindProducer:
		return "producer"
	case trace.SpanKindConsumer:
		return "consumer"
	case trace.SpanKindInternal:
		return "internal"
	default:
		return "unspecified"
	}
}

func statusName(code codes.Code) string {
	if code == codes.Error {
		return "ERROR"
	}
	return "OK"
}
