package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/erain9/matchingo/pkg/marketmaker"
)

func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Load configuration
	cfg, err := marketmaker.LoadConfig()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the order placer (gRPC client)
	orderPlacer, err := marketmaker.NewGRPCOrderPlacer(cfg, logger)
	if err != nil {
		logger.Error("Failed to create order placer", "error", err)
		os.Exit(1)
	}
	defer orderPlacer.Close()

	// Initialize the price fetcher
	priceFetcher, err := marketmaker.NewPriceFetcher(cfg, logger)
	if err != nil {
		logger.Error("Failed to create price fetcher", "error", err)
		os.Exit(1)
	}

	// Initialize the market maker strategy
	strategy := marketmaker.NewLayeredSymmetricQuoting(cfg, logger)

	// Create and start the market maker service
	mm, err := marketmaker.NewMarketMaker(cfg, logger, orderPlacer, priceFetcher, strategy)
	if err != nil {
		logger.Error("Failed to create market maker", "error", err)
		os.Exit(1)
	}

	if err := mm.Start(ctx); err != nil {
		logger.Error("Failed to start market maker", "error", err)
		os.Exit(1)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	sig := <-sigChan
	logger.Info("Received shutdown signal", "signal", sig)

	// Create a context with timeout for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Stop the market maker service
	if err := mm.Stop(shutdownCtx); err != nil {
		logger.Error("Error during shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Market maker service stopped successfully")
}
