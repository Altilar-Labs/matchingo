package otel

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	ServiceOrder          = "order-service"
	ServiceMatchingEngine = "matching-engine"
)

var (
	orderServiceTracer     trace.Tracer
	matchingEngineTracer   trace.Tracer
	orderResource          *sdkresource.Resource
	matchingEngineResource *sdkresource.Resource
	initResourcesOnce      sync.Once
	orderTracerProvider    *sdktrace.TracerProvider
	matchingTracerProvider *sdktrace.TracerProvider
	meterProvider          *sdkmetric.MeterProvider
)

// Config holds the OpenTelemetry configuration
type Config struct {
	ServiceName      string
	ServiceVersion   string
	Endpoint         string
	ConnectTimeout   time.Duration
	ReconnectDelay   time.Duration
	CollectorEnabled bool
}

// Init initializes OpenTelemetry with the given configuration
func Init(cfg Config) (func(), error) {
	if cfg.ServiceVersion == "" {
		cfg.ServiceVersion = "0.1.0"
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "localhost:4317"
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 5 * time.Second
	}
	if cfg.ReconnectDelay == 0 {
		cfg.ReconnectDelay = 10 * time.Second
	}

	var cleanup []func()

	// Initialize resources for both services
	orderResource = initResource(ServiceOrder, cfg.ServiceVersion)
	matchingEngineResource = initResource(ServiceMatchingEngine, cfg.ServiceVersion)

	if cfg.CollectorEnabled {
		// Create shared gRPC connection
		ctx := context.Background()
		conn, err := grpc.DialContext(ctx, cfg.Endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithTimeout(cfg.ConnectTimeout),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
		}
		cleanup = append(cleanup, func() {
			if err := conn.Close(); err != nil {
				log.Printf("Error closing gRPC connection: %v", err)
			}
		})

		// Create trace exporter using shared connection
		traceExporter, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithGRPCConn(conn),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create trace exporter: %w", err)
		}

		// Initialize Order Service tracer provider
		orderTP := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(orderResource),
			sdktrace.WithSampler(sdktrace.ParentBased(
				sdktrace.TraceIDRatioBased(1),
			)),
		)
		orderTracerProvider = orderTP
		cleanup = append(cleanup, func() {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
			defer cancel()
			if err := orderTP.Shutdown(ctx); err != nil {
				log.Printf("Error shutting down order service tracer provider: %v", err)
			}
		})

		// Initialize Matching Engine tracer provider
		matchingTP := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(matchingEngineResource),
			sdktrace.WithSampler(sdktrace.ParentBased(
				sdktrace.TraceIDRatioBased(1),
			)),
		)
		matchingTracerProvider = matchingTP
		cleanup = append(cleanup, func() {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
			defer cancel()
			if err := matchingTP.Shutdown(ctx); err != nil {
				log.Printf("Error shutting down matching engine tracer provider: %v", err)
			}
		})

		// Create metric exporter using the same connection
		metricExporter, err := otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithGRPCConn(conn),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create metric exporter: %w", err)
		}

		// Create meter provider
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(5*time.Second))),
			sdkmetric.WithResource(orderResource),
		)
		meterProvider = mp
		cleanup = append(cleanup, func() {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
			defer cancel()
			if err := mp.Shutdown(ctx); err != nil {
				log.Printf("Error shutting down meter provider: %v", err)
			}
		})

		// Set global providers and propagator
		otel.SetTracerProvider(orderTracerProvider)
		otel.SetMeterProvider(mp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
	}

	// Create tracers for each service
	if orderTracerProvider != nil {
		orderServiceTracer = orderTracerProvider.Tracer(ServiceOrder)
	}
	if matchingTracerProvider != nil {
		matchingEngineTracer = matchingTracerProvider.Tracer(ServiceMatchingEngine)
	}

	// Return cleanup function that executes all cleanup functions in reverse order
	return func() {
		for i := len(cleanup) - 1; i >= 0; i-- {
			cleanup[i]()
		}
	}, nil
}

func initResource(serviceName, serviceVersion string) *sdkresource.Resource {
	extraResources, err := sdkresource.New(
		context.Background(),
		sdkresource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
		sdkresource.WithOS(),
		sdkresource.WithProcess(),
		sdkresource.WithContainer(),
		sdkresource.WithHost(),
	)
	if err != nil {
		log.Printf("Failed to create resource: %v", err)
		return sdkresource.Default()
	}

	resource, err := sdkresource.Merge(
		sdkresource.Default(),
		extraResources,
	)
	if err != nil {
		log.Printf("Failed to merge resources: %v", err)
		return sdkresource.Default()
	}

	return resource
}

// GetOrderServiceTracer returns the tracer for the order service
func GetOrderServiceTracer() trace.Tracer {
	return orderServiceTracer
}

// GetMatchingEngineTracer returns the tracer for the matching engine
func GetMatchingEngineTracer() trace.Tracer {
	return matchingEngineTracer
}

// GetTracerProvider returns the appropriate tracer provider based on the service name
func GetTracerProvider(serviceName string) trace.TracerProvider {
	switch serviceName {
	case ServiceOrder:
		if orderTracerProvider != nil {
			return orderTracerProvider
		}
	case ServiceMatchingEngine:
		if matchingTracerProvider != nil {
			return matchingTracerProvider
		}
	}
	return otel.GetTracerProvider()
}

// GetTextMapPropagator returns the configured propagator
func GetTextMapPropagator() propagation.TextMapPropagator {
	return otel.GetTextMapPropagator()
}

// GetMeterProvider returns the global meter provider
func GetMeterProvider() metric.MeterProvider {
	return meterProvider
}

// ResetForTesting resets the global variables for testing
func ResetForTesting() {
	orderServiceTracer = nil
	matchingEngineTracer = nil
	orderTracerProvider = nil
	matchingTracerProvider = nil
	meterProvider = nil
}

// InitForTesting initializes the tracers for testing
func InitForTesting(tracer trace.Tracer) error {
	orderServiceTracer = tracer
	matchingEngineTracer = tracer
	return nil
}
