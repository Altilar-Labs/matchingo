package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
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
		ServiceName:    "matchingo",
		ServiceVersion: "1.0.0",
		Endpoint:       "localhost:4317", // Change this to your collector endpoint
	})
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	defer cleanup()
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

	// Start gRPC server
	grpcAddr := cfg.Server.GRPCAddr
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	// Create gRPC server with the order book service
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	orderBookService := server.NewGRPCOrderBookService(manager)
	proto.RegisterOrderBookServiceServer(grpcServer, orderBookService)

	// Enable reflection for tools like grpcurl
	reflection.Register(grpcServer)

	// Start gRPC server in a goroutine
	go func() {
		logger.Info().Str("addr", grpcAddr).Msg("Starting gRPC server")
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal().Err(err).Msg("Failed to serve gRPC")
		}
	}()
	return grpcServer, nil
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
