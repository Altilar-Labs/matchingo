package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/erain9/matchingo/config"
	"github.com/erain9/matchingo/pkg/api/proto"
	"github.com/erain9/matchingo/pkg/db/queue"
	"github.com/erain9/matchingo/pkg/messaging/kafka"
	"github.com/erain9/matchingo/pkg/otel"
	"github.com/erain9/matchingo/pkg/server"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logging
	level, err := zerolog.ParseLevel(cfg.Server.LogLevel)
	if err != nil {
		log.Fatalf("Invalid log level: %v", err)
	}

	// Configure global logger
	logger := zerolog.New(os.Stdout).Level(level).With().Timestamp().Logger()
	if cfg.Server.LogFormat == "pretty" {
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	}

	// Create default context with logger
	ctx := logger.WithContext(context.Background())

	// Create a new order book manager
	manager := server.NewOrderBookManager()
	defer manager.Close()

	// Create a test order book
	_, err = manager.CreateMemoryOrderBook(ctx, "test")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create test order book")
	}

	logger.Info().Str("name", "test").Msg("Created test order book")

	// Initialize Kafka consumer (optional)
	// The consumer is for developer purpose which helps pretty print the message
	// in the queue.
	var kafkaConsumer *queue.QueueMessageConsumer
	kafkaConsumer, err = kafka.SetupConsumer(ctx, logger)
	if err == nil && kafkaConsumer != nil {
		defer kafkaConsumer.Close()
	}

	// Initialize OpenTelemetry
	cleanup, err := otel.Init(otel.Config{
		ServiceVersion:   "1.0.0",
		Endpoint:         "localhost:4317", // Change this to your collector endpoint
		ConnectTimeout:   5 * time.Second,
		ReconnectDelay:   10 * time.Second,
		CollectorEnabled: true, // Can be disabled via configuration
	})
	if err != nil {
		log.Printf("Warning: OpenTelemetry initialization failed: %v. Continuing without telemetry.", err)
	} else {
		defer cleanup()
		logger.Info().
			Str("order_service", otel.ServiceOrder).
			Str("matching_engine", otel.ServiceMatchingEngine).
			Msg("OpenTelemetry initialized with multiple services")
	}

	// Setup gRPC server
	grpcServer, err := setupGRPCServer(ctx, cfg, manager)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to setup gRPC server")
	}

	// Setup HTTP server
	httpServer, err := setupHTTPServer(ctx, cfg, cfg.Server.GRPCAddr)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to setup HTTP server")
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	sig := <-sigCh

	logger.Info().Str("signal", sig.String()).Msg("Received signal, shutting down")

	// Graceful shutdown
	grpcServer.GracefulStop()

	// Create a context with timeout for HTTP server shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("HTTP server shutdown error")
	}

	logger.Info().Msg("Servers shutdown complete")
}

// setupGRPCServer initializes and starts a gRPC server
func setupGRPCServer(ctx context.Context, cfg *config.Config, manager *server.OrderBookManager) (*grpc.Server, error) {
	logger := zerolog.Ctx(ctx)

	// Optimize system limits
	if err := setSystemLimits(); err != nil {
		logger.Warn().Err(err).Msg("Failed to set system limits")
	}

	// Start gRPC server with TCP optimizations
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				// Increase socket buffer sizes
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, 1024*1024)
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 1024*1024)
				// Enable TCP keepalive
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_KEEPALIVE, 1)
				// Enable TCP_NODELAY
				syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
			})
		},
	}

	grpcAddr := cfg.Server.GRPCAddr
	lis, err := lc.Listen(ctx, "tcp", grpcAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	// Create metrics interceptors
	metricsUnaryInterceptor, err := otel.MetricsServerInterceptor()
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics interceptor: %w", err)
	}

	metricsStreamInterceptor, err := otel.MetricsStreamServerInterceptor()
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics stream interceptor: %w", err)
	}

	// Configure OpenTelemetry interceptor options for gRPC spans
	otelOpts := []otelgrpc.Option{
		otelgrpc.WithTracerProvider(otel.GetTracerProvider(otel.ServiceOrder)),
		otelgrpc.WithPropagators(otel.GetTextMapPropagator()),
	}

	// Create gRPC server with optimized settings
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			otelgrpc.UnaryServerInterceptor(otelOpts...),
			metricsUnaryInterceptor,
		),
		grpc.ChainStreamInterceptor(
			otelgrpc.StreamServerInterceptor(otelOpts...),
			metricsStreamInterceptor,
		),
		// Optimize for high throughput
		grpc.MaxConcurrentStreams(200000),
		grpc.MaxRecvMsgSize(100*1024*1024),
		grpc.MaxSendMsgSize(100*1024*1024),
		grpc.WriteBufferSize(256*1024),
		grpc.ReadBufferSize(256*1024),
		grpc.InitialConnWindowSize(1024*1024),
		grpc.InitialWindowSize(1024*1024),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     time.Minute * 5,
			MaxConnectionAge:      time.Hour * 2,
			MaxConnectionAgeGrace: time.Minute * 5,
			Time:                  time.Second * 20,
			Timeout:               time.Second * 10,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             time.Second * 10,
			PermitWithoutStream: true,
		}),
	)

	// Create order book service with optimized settings
	orderBookService := server.NewGRPCOrderBookService(manager)
	proto.RegisterOrderBookServiceServer(grpcServer, orderBookService)
	reflection.Register(grpcServer)

	// Start server with multiple worker goroutines
	numCPU := runtime.NumCPU()
	for i := 0; i < numCPU; i++ {
		go func() {
			if err := grpcServer.Serve(lis); err != nil {
				logger.Error().Err(err).Msg("Failed to serve gRPC")
			}
		}()
	}

	logger.Info().
		Int("num_cpus", numCPU).
		Str("addr", grpcAddr).
		Msg("Starting gRPC server with multiple workers")

	return grpcServer, nil
}

// setSystemLimits attempts to increase system resource limits
func setSystemLimits() error {
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		return err
	}
	rLimit.Cur = rLimit.Max
	return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
}

// setupHTTPServer initializes and starts an HTTP server
func setupHTTPServer(ctx context.Context, cfg *config.Config, grpcAddr string) (*http.Server, error) {
	logger := zerolog.Ctx(ctx)

	// Start HTTP server for REST API (optional)
	httpAddr := cfg.Server.HTTPAddr
	httpServer := &http.Server{
		Addr: httpAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add request context with logger
			reqLogger := logger.With().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Logger()
			ctx := reqLogger.WithContext(r.Context())

			// Simple welcome page
			if r.URL.Path == "/" {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, "<html><body>")
				fmt.Fprintf(w, "<h1>Matchingo Order Book Server</h1>")
				fmt.Fprintf(w, "<p>The gRPC server is running on %s</p>", grpcAddr)
				fmt.Fprintf(w, "<p>This is a simple HTTP interface. Use the gRPC client for full functionality.</p>")
				fmt.Fprintf(w, "</body></html>")
				return
			}

			http.NotFound(w, r.WithContext(ctx))
		}),
	}

	// Start HTTP server in a goroutine
	go func() {
		logger.Info().Str("addr", httpAddr).Msg("Starting HTTP server")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Failed to serve HTTP")
		}
	}()

	return httpServer, nil
}
