package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"k8s.io/klog/v2"
)

// TelemetryConfig contains OpenTelemetry configuration options.
type TelemetryConfig struct {
	// Enabled controls whether telemetry is enabled.
	// Can also be set via OTEL_SDK_DISABLED=false environment variable.
	Enabled bool `toml:"enabled,omitempty"`

	// Endpoint is the OTLP HTTP endpoint for trace export.
	// Can also be set via OTEL_EXPORTER_OTLP_ENDPOINT environment variable.
	Endpoint string `toml:"endpoint,omitempty"`

	// ServiceName is the name of this service in traces.
	// Can also be set via OTEL_SERVICE_NAME environment variable.
	ServiceName string `toml:"service_name,omitempty"`

	// Headers are additional headers to send with OTLP requests.
	// Can also be set via OTEL_EXPORTER_OTLP_HEADERS environment variable.
	Headers map[string]string `toml:"headers,omitempty"`
}

// InitTelemetry initializes OpenTelemetry based on the configuration.
// It sets up the global tracer provider and propagator.
// Returns a shutdown function that should be called on exit.
func (tc *TelemetryConfig) InitTelemetry(ctx context.Context) (func(context.Context) error, error) {
	// Check if telemetry is disabled via environment variable
	if os.Getenv("OTEL_SDK_DISABLED") == "true" {
		klog.V(2).Info("OpenTelemetry is disabled via OTEL_SDK_DISABLED")
		return func(context.Context) error { return nil }, nil
	}

	// Check if telemetry is enabled in config
	if !tc.Enabled {
		klog.V(2).Info("OpenTelemetry is disabled in configuration")
		return func(context.Context) error { return nil }, nil
	}

	// Determine endpoint (config takes precedence over env var)
	endpoint := tc.Endpoint
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	}
	if endpoint == "" {
		klog.V(2).Info("No OTLP endpoint configured, disabling telemetry")
		return func(context.Context) error { return nil }, nil
	}

	// Determine service name
	serviceName := tc.ServiceName
	if serviceName == "" {
		serviceName = os.Getenv("OTEL_SERVICE_NAME")
	}
	if serviceName == "" {
		serviceName = "must-gather-mcp-server"
	}

	// Parse headers from environment if not in config
	headers := tc.Headers
	if len(headers) == 0 {
		headers = parseOTLPHeaders(os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"))
	}

	klog.V(1).Infof("Initializing OpenTelemetry: endpoint=%s, service=%s", endpoint, serviceName)

	// Create OTLP HTTP exporter options
	exporterOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
	}

	// Add headers if present
	if len(headers) > 0 {
		exporterOpts = append(exporterOpts, otlptracehttp.WithHeaders(headers))
	}

	// Determine if we should use insecure connection
	if strings.HasPrefix(endpoint, "http://") {
		exporterOpts = append(exporterOpts, otlptracehttp.WithInsecure())
	}

	// Create OTLP trace exporter
	exporter, err := otlptracehttp.New(ctx, exporterOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	// Create resource with service name
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator for distributed tracing
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	klog.V(1).Info("OpenTelemetry initialized successfully")

	// Return shutdown function
	return tp.Shutdown, nil
}

// parseOTLPHeaders parses the OTEL_EXPORTER_OTLP_HEADERS environment variable format.
// Format: "key1=value1,key2=value2"
func parseOTLPHeaders(headersStr string) map[string]string {
	if headersStr == "" {
		return nil
	}

	headers := make(map[string]string)
	pairs := strings.Split(headersStr, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 {
			headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return headers
}
