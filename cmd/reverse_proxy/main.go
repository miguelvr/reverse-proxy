package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/miguelvr/reverse-proxy/pkg/middleware/cache"
	"github.com/miguelvr/reverse-proxy/pkg/middleware/tracing"
	"github.com/miguelvr/reverse-proxy/pkg/proxy"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func tracerProvider(jaegerURL, targetURL string) (*tracesdk.TracerProvider, error) {
	// Create the Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(jaegerURL)))
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// Record information about this application in a Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("reverse_proxy"),
			attribute.String("service.target_url", targetURL),
		)),
	)
	return tp, nil
}

func main() {
	var (
		target string
		port   string
	)
	flag.StringVar(&target, "target-url", "", "target url where the traffic will be forwarded to")
	flag.StringVar(&port, "port", "8000", "server port")
	flag.Parse()

	if target == "" {
		log.Fatal("--target-url flag is required")
	}

	targetURL, err := url.Parse(target)
	if err != nil {
		log.Fatal(err)
	}

	tp, err := tracerProvider("http://localhost:14268/api/traces", target)
	if err != nil {
		log.Fatal(err)
	}
	otel.SetTracerProvider(tp)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}(ctx)

	reverseProxy := proxy.New(targetURL)
	reverseProxy = cache.WithCache(reverseProxy)
	reverseProxy = tracing.WithTracing(reverseProxy)

	log.Printf("Running on port :%s\n", port)
	if err = http.ListenAndServe(":"+port, reverseProxy); err != nil {
		log.Fatal(err)
	}
}
