package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bennnr/pamone-observability/sdk/go/pamone"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	// logs + resource metrics -> PAMONE_SINK_DIR (default /var/log/pamone)
	pamone.Init()

	res, _ := resource.New(context.Background(),
		resource.WithAttributes(attribute.String("service.name", "pamone-go-example")))
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(pamone.NewSpanProcessor()), // traces -> spans.jsonl
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/work", func(w http.ResponseWriter, r *http.Request) {
		_, span := otel.Tracer("example").Start(r.Context(), "work")
		time.Sleep(50 * time.Millisecond)
		span.End()
		fmt.Fprintln(w, "done")
	})

	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}
	log.Printf("pamone-go-example listening on %s (sink=%s)", addr, os.Getenv("PAMONE_SINK_DIR"))
	if err := http.ListenAndServe(addr, otelhttp.NewHandler(mux, "http")); err != nil {
		log.Fatal(err)
	}
}
