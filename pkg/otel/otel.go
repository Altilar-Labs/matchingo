package otel

import (
	"context"
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

	// Initialize tracer providers for both services
	if cfg.CollectorEnabled {
		// Initialize Order Service tracer provider
		orderTP, err := initTracerProvider(cfg, orderResource)
		if err != nil {
			log.Printf("Warning: Failed to initialize order service tracer provider: %v", err)
		} else {
			orderTracerProvider = orderTP
			cleanup = append(cleanup, func() {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
				defer cancel()
				if err := orderTP.Shutdown(ctx); err != nil {
					log.Printf("Error shutting down order service tracer provider: %v", err)
				}
			})
		}

		// Initialize Matching Engine tracer provider
		matchingTP, err := initTracerProvider(cfg, matchingEngineResource)
		if err != nil {
			log.Printf("Warning: Failed to initialize matching engine tracer provider: %v", err)
		} else {
			matchingTracerProvider = matchingTP
			cleanup = append(cleanup, func() {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
				defer cancel()
				if err := matchingTP.Shutdown(ctx); err != nil {
					log.Printf("Error shutting down matching engine tracer provider: %v", err)
				}
			})
		}
	}

	// Initialize meter provider (can be shared between services)
	if cfg.CollectorEnabled {
		mp, err := initMeterProvider(cfg, orderResource) // Using order resource as default for metrics
		if err != nil {
			log.Printf("Warning: Failed to initialize meter provider: %v. Continuing without metrics.", err)
		} else {
			meterProvider = mp
			cleanup = append(cleanup, func() {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
				defer cancel()
				if err := mp.Shutdown(ctx); err != nil {
					log.Printf("Error shutting down meter provider: %v", err)
				}
			})
		}
	}

	// Create tracers for each service
	if orderTracerProvider != nil {
		orderServiceTracer = orderTracerProvider.Tracer(ServiceOrder)
	}
	if matchingTracerProvider != nil {
		matchingEngineTracer = matchingTracerProvider.Tracer(ServiceMatchingEngine)
	}

	// Return cleanup function that executes all cleanup functions
	return func() {
		for _, fn := range cleanup {
			fn()
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

func initTracerProvider(cfg Config, resource *sdkresource.Resource) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	// Create gRPC connection to collector
	conn, err := grpc.DialContext(ctx, cfg.Endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTimeout(cfg.ConnectTimeout),
	)
	if err != nil {
		return nil, err
	}

	// Create exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return nil, err
	}

	// Create tracer provider with the specific resource
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource),
		sdktrace.WithSampler(sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(1),
		)),
	)

	// Set the text map propagator (this is shared between services)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// Set the tracer provider
	otel.SetTracerProvider(tp)

	return tp, nil
}

func initMeterProvider(cfg Config, resource *sdkresource.Resource) (*sdkmetric.MeterProvider, error) {
	ctx := context.Background()

	// Create gRPC connection to collector
	conn, err := grpc.DialContext(ctx, cfg.Endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTimeout(cfg.ConnectTimeout),
	)
	if err != nil {
		return nil, err
	}

	// Create exporter
	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return nil, err
	}

	// Create meter provider
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(5*time.Second))),
		sdkmetric.WithResource(resource),
	)

	// Set global meter provider
	otel.SetMeterProvider(mp)

	return mp, nil
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
}

// InitForTesting initializes the tracers for testing
func InitForTesting(tracer trace.Tracer) error {
	orderServiceTracer = tracer
	matchingEngineTracer = tracer
	return nil
}
