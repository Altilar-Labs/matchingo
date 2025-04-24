package otel

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

const (
	instrumentationName = "github.com/erain9/matchingo/pkg/otel"
)

var (
	grpcMetrics     *GRPCServerMetrics
	grpcMetricsOnce sync.Once
	metricsLock     sync.RWMutex
)

// GRPCServerMetrics holds the metrics instruments for gRPC server monitoring
type GRPCServerMetrics struct {
	// Latency metrics
	serverLatency metric.Float64Histogram

	// Traffic metrics
	requestsTotal    metric.Int64Counter
	requestsInFlight metric.Int64UpDownCounter

	// Error metrics
	errorTotal metric.Int64Counter

	// Saturation metrics
	goroutinesCount metric.Int64UpDownCounter
}

// NewGRPCServerMetrics creates a new GRPCServerMetrics instance
func NewGRPCServerMetrics(meter metric.Meter) (*GRPCServerMetrics, error) {
	serverLatency, err := meter.Float64Histogram(
		"grpc.server.duration",
		metric.WithDescription("Response latency (seconds) of gRPC server"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	requestsTotal, err := meter.Int64Counter(
		"grpc.server.requests.total",
		metric.WithDescription("Total number of gRPC requests started"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	requestsInFlight, err := meter.Int64UpDownCounter(
		"grpc.server.requests.in_flight",
		metric.WithDescription("Number of gRPC requests currently in flight"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	errorTotal, err := meter.Int64Counter(
		"grpc.server.errors.total",
		metric.WithDescription("Total number of gRPC errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	goroutinesCount, err := meter.Int64UpDownCounter(
		"grpc.server.goroutines",
		metric.WithDescription("Number of goroutines currently running"),
		metric.WithUnit("{goroutine}"),
	)
	if err != nil {
		return nil, err
	}

	return &GRPCServerMetrics{
		serverLatency:    serverLatency,
		requestsTotal:    requestsTotal,
		requestsInFlight: requestsInFlight,
		errorTotal:       errorTotal,
		goroutinesCount:  goroutinesCount,
	}, nil
}

// GetGRPCServerMetrics returns a singleton instance of GRPCServerMetrics
func GetGRPCServerMetrics(meter metric.Meter) (*GRPCServerMetrics, error) {
	var err error
	grpcMetricsOnce.Do(func() {
		grpcMetrics, err = NewGRPCServerMetrics(meter)
	})
	if err != nil {
		return nil, err
	}
	return grpcMetrics, nil
}

// RecordLatency records the latency of a gRPC request
func (m *GRPCServerMetrics) RecordLatency(ctx context.Context, method string, duration time.Duration, statusCode string) error {
	metricsLock.Lock()
	defer metricsLock.Unlock()

	attrs := []attribute.KeyValue{
		semconv.RPCMethodKey.String(method),
		semconv.RPCGRPCStatusCodeKey.String(statusCode),
	}
	m.serverLatency.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	return nil
}

// IncRequests increments the total requests counter
func (m *GRPCServerMetrics) IncRequests(ctx context.Context, method string) error {
	metricsLock.Lock()
	defer metricsLock.Unlock()

	attrs := []attribute.KeyValue{
		semconv.RPCMethodKey.String(method),
	}
	m.requestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	return nil
}

// AddInFlightRequests adds to the in-flight requests counter
func (m *GRPCServerMetrics) AddInFlightRequests(ctx context.Context, delta int64) error {
	metricsLock.Lock()
	defer metricsLock.Unlock()

	m.requestsInFlight.Add(ctx, delta)
	return nil
}

// IncErrors increments the error counter
func (m *GRPCServerMetrics) IncErrors(ctx context.Context, method string, statusCode string) error {
	metricsLock.Lock()
	defer metricsLock.Unlock()

	attrs := []attribute.KeyValue{
		semconv.RPCMethodKey.String(method),
		semconv.RPCGRPCStatusCodeKey.String(statusCode),
	}
	m.errorTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	return nil
}

// SetGoroutines sets the number of goroutines
func (m *GRPCServerMetrics) SetGoroutines(ctx context.Context, count int64) error {
	metricsLock.Lock()
	defer metricsLock.Unlock()

	m.goroutinesCount.Add(ctx, count)
	return nil
}
