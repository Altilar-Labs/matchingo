package otel

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"
)

// NewGRPCStatsHandler creates a stats handler for gRPC telemetry using OpenTelemetry.
// This is the preferred method for instrumenting gRPC servers and clients.
func NewGRPCStatsHandler() stats.Handler {
	return otelgrpc.NewServerHandler(
		otelgrpc.WithMeterProvider(otel.GetMeterProvider()),
		otelgrpc.WithTracerProvider(otel.GetTracerProvider()),
	)
}

// MetricsServerInterceptor returns a unary server interceptor that records standard OpenTelemetry gRPC metrics.
// Use this for backwards compatibility. For new code, prefer NewGRPCStatsHandler instead.
func MetricsServerInterceptor() (grpc.UnaryServerInterceptor, error) {
	return otelgrpc.UnaryServerInterceptor(
		otelgrpc.WithMeterProvider(otel.GetMeterProvider()),
		otelgrpc.WithTracerProvider(otel.GetTracerProvider()),
	), nil
}

// MetricsStreamServerInterceptor returns a stream server interceptor that records standard OpenTelemetry gRPC metrics.
// Use this for backwards compatibility. For new code, prefer NewGRPCStatsHandler instead.
func MetricsStreamServerInterceptor() (grpc.StreamServerInterceptor, error) {
	return otelgrpc.StreamServerInterceptor(
		otelgrpc.WithMeterProvider(otel.GetMeterProvider()),
		otelgrpc.WithTracerProvider(otel.GetTracerProvider()),
	), nil
}
