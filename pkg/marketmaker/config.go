package marketmaker

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the market maker service
type Config struct {
	// gRPC connection settings
	MatchingoGRPCAddr string
	RequestTimeout    time.Duration

	// Market settings
	MarketSymbol   string // e.g., "BTC-USDT"
	ExternalSymbol string // e.g., "BTCUSDT"
	PriceSourceURL string // e.g., "https://api.binance.com"

	// Market making parameters
	NumLevels         int
	BaseSpreadPercent float64
	PriceStepPercent  float64
	OrderSize         string // Decimal string for precise quantity
	UpdateInterval    time.Duration
	MarketMakerID     string

	// HTTP client settings
	HTTPTimeout time.Duration
	MaxRetries  int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	v := viper.New()

	// Set default values
	v.SetDefault("MATCHINGO_GRPC_ADDR", "localhost:50051")
	v.SetDefault("REQUEST_TIMEOUT_SECONDS", 5)
	v.SetDefault("MARKET_SYMBOL", "BTC-USDT")
	v.SetDefault("EXTERNAL_SYMBOL", "BTCUSDT")
	v.SetDefault("PRICE_SOURCE_URL", "https://api.binance.com")
	v.SetDefault("NUM_LEVELS", 3)
	v.SetDefault("BASE_SPREAD_PERCENT", 0.1)
	v.SetDefault("PRICE_STEP_PERCENT", 0.05)
	v.SetDefault("ORDER_SIZE", "0.01")
	v.SetDefault("UPDATE_INTERVAL_SECONDS", 10)
	v.SetDefault("MARKET_MAKER_ID", "mm-01")
	v.SetDefault("HTTP_TIMEOUT_SECONDS", 5)
	v.SetDefault("MAX_RETRIES", 3)

	// Allow environment variables
	v.AutomaticEnv()

	cfg := &Config{
		MatchingoGRPCAddr: v.GetString("MATCHINGO_GRPC_ADDR"),
		RequestTimeout:    time.Duration(v.GetInt("REQUEST_TIMEOUT_SECONDS")) * time.Second,
		MarketSymbol:      v.GetString("MARKET_SYMBOL"),
		ExternalSymbol:    v.GetString("EXTERNAL_SYMBOL"),
		PriceSourceURL:    v.GetString("PRICE_SOURCE_URL"),
		NumLevels:         v.GetInt("NUM_LEVELS"),
		BaseSpreadPercent: v.GetFloat64("BASE_SPREAD_PERCENT"),
		PriceStepPercent:  v.GetFloat64("PRICE_STEP_PERCENT"),
		OrderSize:         v.GetString("ORDER_SIZE"),
		UpdateInterval:    time.Duration(v.GetInt("UPDATE_INTERVAL_SECONDS")) * time.Second,
		MarketMakerID:     v.GetString("MARKET_MAKER_ID"),
		HTTPTimeout:       time.Duration(v.GetInt("HTTP_TIMEOUT_SECONDS")) * time.Second,
		MaxRetries:        v.GetInt("MAX_RETRIES"),
	}

	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func validateConfig(cfg *Config) error {
	if cfg.MatchingoGRPCAddr == "" {
		return fmt.Errorf("MATCHINGO_GRPC_ADDR must not be empty")
	}
	if cfg.MarketSymbol == "" {
		return fmt.Errorf("MARKET_SYMBOL must not be empty")
	}
	if cfg.ExternalSymbol == "" {
		return fmt.Errorf("EXTERNAL_SYMBOL must not be empty")
	}
	if cfg.PriceSourceURL == "" {
		return fmt.Errorf("PRICE_SOURCE_URL must not be empty")
	}
	if cfg.NumLevels <= 0 {
		return fmt.Errorf("NUM_LEVELS must be positive")
	}
	if cfg.BaseSpreadPercent <= 0 {
		return fmt.Errorf("BASE_SPREAD_PERCENT must be positive")
	}
	if cfg.PriceStepPercent <= 0 {
		return fmt.Errorf("PRICE_STEP_PERCENT must be positive")
	}
	if cfg.OrderSize == "" {
		return fmt.Errorf("ORDER_SIZE must not be empty")
	}
	if cfg.UpdateInterval <= 0 {
		return fmt.Errorf("UPDATE_INTERVAL_SECONDS must be positive")
	}
	if cfg.MarketMakerID == "" {
		return fmt.Errorf("MARKET_MAKER_ID must not be empty")
	}
	return nil
}
