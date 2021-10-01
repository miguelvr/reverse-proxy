package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func jsonHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, err := io.WriteString(w, "{}")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func streamHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Transfer-Encoding", "chunked")
	w.(http.Flusher).Flush()

	for i := 0; i < 5; i++ {
		time.Sleep(time.Second)
		_, err := io.WriteString(w, "hello")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.(http.Flusher).Flush()
	}
}

func trailerHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Trailer", "AtEnd1, AtEnd2")
	w.Header().Add("Trailer", "AtEnd3")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8") // normal header
	w.WriteHeader(http.StatusOK)

	w.Header().Set("AtEnd1", "value 1")
	_, err := io.WriteString(w, "This HTTP response has both headers before this text and trailers at the end.\n")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("AtEnd2", "value 2")
	w.Header().Set("AtEnd3", "value 3") // These will appear as trailers.
}

func setupTracing() func(context.Context) {
	tp, err := tracerProvider("http://localhost:14268/api/traces")
	if err != nil {
		log.Fatal(err)
	}
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Cleanly shutdown and flush telemetry when the application exits.
	return func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}

func tracerProvider(url string) (*tracesdk.TracerProvider, error) {
	// Create the Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// Record information about this application in a Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("demo"),
		)),
	)
	return tp, nil
}

func main() {
	var port string
	flag.StringVar(&port, "port", "8000", "server port")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdown := setupTracing()
	defer shutdown(ctx)
	
	mux := http.NewServeMux()
	mux.Handle("/json", otelhttp.NewHandler(http.HandlerFunc(jsonHandler), "/json"))
	mux.Handle("/stream", otelhttp.NewHandler(http.HandlerFunc(streamHandler), "/stream"))
	mux.Handle("/trailer", otelhttp.NewHandler(http.HandlerFunc(trailerHandler), "/trailer"))

	log.Printf("Running on port :%s\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
