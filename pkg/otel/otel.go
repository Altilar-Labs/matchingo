package otel

import (
	"context"
	"log"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	tracer            trace.Tracer
	resource          *sdkresource.Resource
	initResourcesOnce sync.Once
	tracerProvider    *sdktrace.TracerProvider
	meterProvider     *sdkmetric.MeterProvider
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
	if cfg.ServiceName == "" {
		cfg.ServiceName = "matchingo"
	}
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

	// Initialize resource
	resource = initResource(cfg.ServiceName, cfg.ServiceVersion)

	var cleanup []func()

	// Initialize tracer provider
	if cfg.CollectorEnabled {
		tp, err := initTracerProvider(cfg)
		if err != nil {
			log.Printf("Warning: Failed to initialize tracer provider: %v. Continuing without tracing.", err)
		} else {
			tracerProvider = tp
			cleanup = append(cleanup, func() {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
				defer cancel()
				if err := tp.Shutdown(ctx); err != nil {
					log.Printf("Error shutting down tracer provider: %v", err)
				}
			})
		}
	}

	// Initialize meter provider
	if cfg.CollectorEnabled {
		mp, err := initMeterProvider(cfg)
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

	// Create a tracer for this service
	if tracerProvider != nil {
		tracer = tracerProvider.Tracer(cfg.ServiceName)
	}

	// Return cleanup function that executes all cleanup functions
	return func() {
		for _, fn := range cleanup {
			fn()
		}
	}, nil
}

func initResource(serviceName, serviceVersion string) *sdkresource.Resource {
	initResourcesOnce.Do(func() {
		extraResources, _ := sdkresource.New(
			context.Background(),
			sdkresource.WithOS(),
			sdkresource.WithProcess(),
			sdkresource.WithContainer(),
			sdkresource.WithHost(),
			sdkresource.WithAttributes(
				semconv.ServiceName(serviceName),
				semconv.ServiceVersion(serviceVersion),
			),
		)
		resource, _ = sdkresource.Merge(
			sdkresource.Default(),
			extraResources,
		)
	})
	return resource
}

func initTracerProvider(cfg Config) (*sdktrace.TracerProvider, error) {
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

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource),
	)

	// Set global tracer provider and propagator
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tp, nil
}

func initMeterProvider(cfg Config) (*sdkmetric.MeterProvider, error) {
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
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(resource),
	)

	// Set global meter provider
	otel.SetMeterProvider(mp)

	return mp, nil
}

// GetTracer returns the tracer for this service
func GetTracer() trace.Tracer {
	return tracer
}
