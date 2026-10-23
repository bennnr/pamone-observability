// Command example is a minimal HTTP service instrumented with the stock
// OpenTelemetry Go SDK. It writes nothing to disk and needs no sidecar: traces
// and metrics go straight to an OTLP/HTTP endpoint.
//
// Everything is configured with the standard OTEL_* variables, so the same image
// runs with or without pamone:
//
//	OTEL_EXPORTER_OTLP_ENDPOINT   http://pamone-otlp-gateway:4318 (default)
//	OTEL_EXPORTER_OTLP_PROTOCOL   http/protobuf
//	OTEL_SERVICE_NAME             pamone-go-example (default)
//	OTEL_TRACES_EXPORTER          otlp | none
//	OTEL_METRICS_EXPORTER         otlp | none
//
// Pointing OTEL_EXPORTER_OTLP_ENDPOINT at any OTLP collector (or at the pamone
// agent's gateway mode) is the whole integration.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const defaultServiceName = "pamone-go-example"

// exporterOn reports whether an OTel signal exporter is enabled. It follows the
// SDK spec: unset or "otlp" means export, "none" disables the signal.
func exporterOn(env string) bool {
	switch os.Getenv(env) {
	case "none", "NONE":
		return false
	default:
		return true
	}
}

func serviceName() string {
	if v := os.Getenv("OTEL_SERVICE_NAME"); v != "" {
		return v
	}
	return defaultServiceName
}

// newResource builds the resource from the standard OTEL_* variables, with the
// service name falling back to defaultServiceName.
func newResource(ctx context.Context) (*resource.Resource, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(attribute.String("service.name", serviceName())),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// setupTraces installs a tracer provider exporting over OTLP/HTTP. The exporter
// reads OTEL_EXPORTER_OTLP_ENDPOINT, _HEADERS, _TIMEOUT, _COMPRESSION and the
// signal-specific OTEL_EXPORTER_OTLP_TRACES_* overrides itself.
func setupTraces(ctx context.Context, res *resource.Resource) (func(context.Context) error, error) {
	if !exporterOn("OTEL_TRACES_EXPORTER") {
		log.Printf("traces: disabled (OTEL_TRACES_EXPORTER=none)")
		return func(context.Context) error { return nil }, nil
	}
	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("otlp trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exp),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// setupMetrics installs a meter provider with a periodic OTLP/HTTP reader.
func setupMetrics(ctx context.Context, res *resource.Resource) (func(context.Context) error, error) {
	if !exporterOn("OTEL_METRICS_EXPORTER") {
		log.Printf("metrics: disabled (OTEL_METRICS_EXPORTER=none)")
		return func(context.Context) error { return nil }, nil
	}
	exp, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("otlp metric exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp,
			sdkmetric.WithInterval(10*time.Second))),
	)
	otel.SetMeterProvider(mp)
	return mp.Shutdown, nil
}

func main() {
	ctx := context.Background()

	// The SDK exports a fatal error here only for invalid OTEL_* config (for
	// example a malformed endpoint); an unreachable collector is retried in the
	// background and logged, it does not stop the service.
	res, err := newResource(ctx)
	if err != nil {
		log.Fatalf("resource: %v", err)
	}
	shutdownTraces, err := setupTraces(ctx, res)
	if err != nil {
		log.Fatal(err)
	}
	shutdownMetrics, err := setupMetrics(ctx, res)
	if err != nil {
		log.Fatal(err)
	}

	meter := otel.Meter("example")
	requests, err := meter.Int64Counter("pamone.requests",
		metric.WithDescription("Requests served by the example app"),
		metric.WithUnit("{request}"))
	if err != nil {
		log.Fatalf("meter: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/work", func(w http.ResponseWriter, r *http.Request) {
		_, span := otel.Tracer("example").Start(r.Context(), "work")
		span.SetAttributes(attribute.String("work.kind", "sleep"))
		time.Sleep(50 * time.Millisecond)
		span.End()
		requests.Add(r.Context(), 1, metric.WithAttributes(attribute.String("route", "/work")))
		fmt.Fprintln(w, "done")
	})

	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	// Name server spans "<METHOD> <route>": otelhttp falls back to the operation
	// name for every span otherwise, which collapses the whole service into a
	// single operation in pamone's Operations and Endpoints views.
	handler := otelhttp.NewHandler(mux, "http", otelhttp.WithSpanNameFormatter(
		func(_ string, r *http.Request) string { return r.Method + " " + r.URL.Path }))

	srv := &http.Server{Addr: addr, Handler: handler}

	// Flush both pipelines on SIGTERM so a rolling update does not drop the last
	// batch.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
		_ = shutdownTraces(sctx)
		_ = shutdownMetrics(sctx)
	}()

	log.Printf("service=%s listening on %s otlp=%s traces=%t metrics=%t",
		serviceName(), addr,
		os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		exporterOn("OTEL_TRACES_EXPORTER"), exporterOn("OTEL_METRICS_EXPORTER"))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
