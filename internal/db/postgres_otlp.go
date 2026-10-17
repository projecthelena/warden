package db

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func postgresTraceProvider(endpoint string) (*sdktrace.TracerProvider, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return nil, fmt.Errorf("invalid PostgreSQL OTLP endpoint: expected HTTP(S) URL without credentials")
	}
	exporter, err := otlptracehttp.New(context.Background(), otlptracehttp.WithEndpointURL(endpoint), otlptracehttp.WithTimeout(5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("cannot initialize PostgreSQL OTLP exporter")
	}
	res, err := resource.New(context.Background(), resource.WithAttributes(attribute.String("service.name", "warden")), resource.WithFromEnv())
	if err != nil {
		_ = exporter.Shutdown(context.Background())
		return nil, err
	}
	return sdktrace.NewTracerProvider(sdktrace.WithResource(res), sdktrace.WithBatcher(exporter,
		sdktrace.WithMaxQueueSize(2048), sdktrace.WithMaxExportBatchSize(256), sdktrace.WithExportTimeout(5*time.Second))), nil
}
