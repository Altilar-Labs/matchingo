package marketmaker

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestBinancePriceFetcher_FetchPrice(t *testing.T) {
	// Create a test server that simulates Binance API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path and query parameters
		if r.URL.Path != "/api/v3/ticker/price" {
			t.Errorf("Expected path /api/v3/ticker/price, got %s", r.URL.Path)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		symbol := r.URL.Query().Get("symbol")
		if symbol != "BTCUSDT" {
			t.Errorf("Expected symbol BTCUSDT, got %s", symbol)
			http.Error(w, "Invalid symbol", http.StatusBadRequest)
			return
		}

		// Return a valid response
		resp := binanceTickerResponse{
			Symbol: "BTCUSDT",
			Price:  "50000.00",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create a test configuration
	cfg := &Config{
		ExternalSymbol: "BTCUSDT",
		PriceSourceURL: server.URL,
		HTTPTimeout:    5 * time.Second,
		MaxRetries:     3,
	}

	// Create a test logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create the price fetcher
	fetcher, err := NewPriceFetcher(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create price fetcher: %v", err)
	}
	defer fetcher.Close()

	// Test successful price fetch
	ctx := context.Background()
	price, err := fetcher.FetchPrice(ctx)
	if err != nil {
		t.Errorf("FetchPrice failed: %v", err)
	}
	if price != 50000.00 {
		t.Errorf("Expected price 50000.00, got %f", price)
	}
}

func TestBinancePriceFetcher_FetchPrice_InvalidResponse(t *testing.T) {
	// Create a test server that returns invalid responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return invalid JSON
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	cfg := &Config{
		ExternalSymbol: "BTCUSDT",
		PriceSourceURL: server.URL,
		HTTPTimeout:    1 * time.Second,
		MaxRetries:     1,
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	fetcher, err := NewPriceFetcher(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create price fetcher: %v", err)
	}
	defer fetcher.Close()

	ctx := context.Background()
	_, err = fetcher.FetchPrice(ctx)
	if err == nil {
		t.Error("Expected error for invalid response, got nil")
	}
}

func TestBinancePriceFetcher_FetchPrice_ServerError(t *testing.T) {
	// Create a test server that returns server errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &Config{
		ExternalSymbol: "BTCUSDT",
		PriceSourceURL: server.URL,
		HTTPTimeout:    1 * time.Second,
		MaxRetries:     2,
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	fetcher, err := NewPriceFetcher(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create price fetcher: %v", err)
	}
	defer fetcher.Close()

	ctx := context.Background()
	_, err = fetcher.FetchPrice(ctx)
	if err == nil {
		t.Error("Expected error for server error response, got nil")
	}
}

func TestBinancePriceFetcher_FetchPrice_Timeout(t *testing.T) {
	// Create a test server that simulates timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Delay longer than timeout
		json.NewEncoder(w).Encode(binanceTickerResponse{Symbol: "BTCUSDT", Price: "50000.00"})
	}))
	defer server.Close()

	cfg := &Config{
		ExternalSymbol: "BTCUSDT",
		PriceSourceURL: server.URL,
		HTTPTimeout:    100 * time.Millisecond, // Short timeout
		MaxRetries:     1,
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	fetcher, err := NewPriceFetcher(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create price fetcher: %v", err)
	}
	defer fetcher.Close()

	ctx := context.Background()
	_, err = fetcher.FetchPrice(ctx)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}
