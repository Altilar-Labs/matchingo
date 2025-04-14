package marketmaker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// binancePriceFetcher implements PriceFetcher using the Binance public API
type binancePriceFetcher struct {
	client  *http.Client
	cfg     *Config
	logger  *slog.Logger
	baseURL string
}

// binanceTickerResponse represents the response from Binance's ticker price endpoint
type binanceTickerResponse struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

// NewPriceFetcher creates a new PriceFetcher that uses the Binance API
func NewPriceFetcher(cfg *Config, logger *slog.Logger) (PriceFetcher, error) {
	client := &http.Client{
		Timeout: cfg.HTTPTimeout,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: true,
		},
	}

	return &binancePriceFetcher{
		client:  client,
		cfg:     cfg,
		logger:  logger.With("component", "binancePriceFetcher"),
		baseURL: cfg.PriceSourceURL,
	}, nil
}

// FetchPrice fetches the current price from Binance's API
func (f *binancePriceFetcher) FetchPrice(ctx context.Context) (float64, error) {
	url := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s", f.baseURL, f.cfg.ExternalSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	var attempts int
	var lastErr error
	for attempts = 1; attempts <= f.cfg.MaxRetries; attempts++ {
		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed (attempt %d/%d): %w",
				attempts, f.cfg.MaxRetries, err)
			f.logger.Warn("Price fetch request failed",
				"attempt", attempts,
				"max_retries", f.cfg.MaxRetries,
				"error", err)
			time.Sleep(time.Duration(attempts) * 100 * time.Millisecond) // Exponential backoff
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP request returned non-200 status (attempt %d/%d): %d",
				attempts, f.cfg.MaxRetries, resp.StatusCode)
			f.logger.Warn("Price fetch returned non-200 status",
				"attempt", attempts,
				"max_retries", f.cfg.MaxRetries,
				"status", resp.StatusCode)
			time.Sleep(time.Duration(attempts) * 100 * time.Millisecond)
			continue
		}

		var tickerResp binanceTickerResponse
		if err := json.NewDecoder(resp.Body).Decode(&tickerResp); err != nil {
			lastErr = fmt.Errorf("failed to decode response (attempt %d/%d): %w",
				attempts, f.cfg.MaxRetries, err)
			f.logger.Warn("Failed to decode price response",
				"attempt", attempts,
				"max_retries", f.cfg.MaxRetries,
				"error", err)
			time.Sleep(time.Duration(attempts) * 100 * time.Millisecond)
			continue
		}

		price, err := strconv.ParseFloat(tickerResp.Price, 64)
		if err != nil {
			lastErr = fmt.Errorf("failed to parse price '%s' (attempt %d/%d): %w",
				tickerResp.Price, attempts, f.cfg.MaxRetries, err)
			f.logger.Warn("Failed to parse price value",
				"attempt", attempts,
				"max_retries", f.cfg.MaxRetries,
				"price_str", tickerResp.Price,
				"error", err)
			time.Sleep(time.Duration(attempts) * 100 * time.Millisecond)
			continue
		}

		f.logger.Debug("Successfully fetched price",
			"symbol", f.cfg.ExternalSymbol,
			"price", price,
			"attempt", attempts)
		return price, nil
	}

	return 0, fmt.Errorf("failed to fetch price after %d attempts: %w", attempts-1, lastErr)
}

// Close implements PriceFetcher
func (f *binancePriceFetcher) Close() error {
	f.client.CloseIdleConnections()
	return nil
}
