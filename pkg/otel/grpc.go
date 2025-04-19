package otel

import (
	"context"
	"runtime"
	"time"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MetricsServerInterceptor creates a new unary server interceptor for OpenTelemetry metrics
func MetricsServerInterceptor() (grpc.UnaryServerInterceptor, error) {
	meter := otel.GetMeterProvider().Meter(instrumentationName)
	metrics, err := NewGRPCServerMetrics(meter)
	if err != nil {
		return nil, err
	}

	// Start a goroutine to periodically update the goroutine count
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			metrics.SetGoroutines(context.Background(), int64(runtime.NumGoroutine()))
		}
	}()

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()

		// Increment in-flight requests
		metrics.AddInFlightRequests(ctx, 1)
		defer metrics.AddInFlightRequests(ctx, -1)

		// Increment total requests
		metrics.IncRequests(ctx, info.FullMethod)

		// Handle the RPC
		resp, err := handler(ctx, req)

		// Record metrics based on the result
		duration := time.Since(startTime)
		statusCode := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				statusCode = st.Code()
			} else {
				statusCode = codes.Unknown
			}
		}

		// Record latency
		metrics.RecordLatency(ctx, info.FullMethod, duration, statusCode.String())

		// Record error if any
		if statusCode != codes.OK {
			metrics.IncErrors(ctx, info.FullMethod, statusCode.String())
		}

		return resp, err
	}, nil
}

// MetricsStreamServerInterceptor creates a new stream server interceptor for OpenTelemetry metrics
func MetricsStreamServerInterceptor() (grpc.StreamServerInterceptor, error) {
	meter := otel.GetMeterProvider().Meter(instrumentationName)
	metrics, err := NewGRPCServerMetrics(meter)
	if err != nil {
		return nil, err
	}

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()

		// Increment in-flight requests
		metrics.AddInFlightRequests(ss.Context(), 1)
		defer metrics.AddInFlightRequests(ss.Context(), -1)

		// Increment total requests
		metrics.IncRequests(ss.Context(), info.FullMethod)

		// Handle the RPC
		err := handler(srv, ss)

		// Record metrics based on the result
		duration := time.Since(startTime)
		statusCode := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				statusCode = st.Code()
			} else {
				statusCode = codes.Unknown
			}
		}

		// Record latency
		metrics.RecordLatency(ss.Context(), info.FullMethod, duration, statusCode.String())

		// Record error if any
		if statusCode != codes.OK {
			metrics.IncErrors(ss.Context(), info.FullMethod, statusCode.String())
		}

		return err
	}, nil
}
