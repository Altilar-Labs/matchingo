package server

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/erain9/matchingo/pkg/backend/memory"
	"github.com/erain9/matchingo/pkg/backend/redis"
	"github.com/erain9/matchingo/pkg/core"
	"github.com/erain9/matchingo/pkg/logging"
	redisClient "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

var (
	// ErrOrderBookExists is returned when trying to create an order book that already exists
	ErrOrderBookExists = errors.New("order book with this name already exists")

	// ErrOrderBookNotFound is returned when trying to access a non-existent order book
	ErrOrderBookNotFound = errors.New("order book not found")
)

// OrderBookInfo contains metadata about an order book
type OrderBookInfo struct {
	Name       string
	Backend    string
	CreatedAt  time.Time
	OrderCount int
}

// OrderBookManager manages multiple order books
type OrderBookManager struct {
	mu         sync.RWMutex
	orderBooks map[string]*core.OrderBook
	info       map[string]*OrderBookInfo
	redisPool  map[string]*redisClient.Client
}

// NewOrderBookManager creates a new OrderBookManager
func NewOrderBookManager() *OrderBookManager {
	return &OrderBookManager{
		orderBooks: make(map[string]*core.OrderBook),
		info:       make(map[string]*OrderBookInfo),
		redisPool:  make(map[string]*redisClient.Client),
	}
}

// CreateMemoryOrderBook creates a new order book with in-memory backend
func (m *OrderBookManager) CreateMemoryOrderBook(ctx context.Context, name string) (*OrderBookInfo, error) {
	logger := logging.FromContext(ctx).With().Str("order_book", name).Logger()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if order book already exists
	if _, exists := m.orderBooks[name]; exists {
		logger.Error().Msg("Order book already exists")
		return nil, ErrOrderBookExists
	}

	// Create in-memory backend
	backend := memory.NewMemoryBackend()

	// Create order book
	orderBook := core.NewOrderBook(backend)

	// Store order book
	m.orderBooks[name] = orderBook

	// Store metadata
	info := &OrderBookInfo{
		Name:      name,
		Backend:   "memory",
		CreatedAt: time.Now(),
	}
	m.info[name] = info

	logger.Info().Str("backend", "memory").Msg("Created new memory order book")
	return info, nil
}

// CreateRedisOrderBook creates a new order book with Redis backend
func (m *OrderBookManager) CreateRedisOrderBook(ctx context.Context, name string, options map[string]string) (*OrderBookInfo, error) {
	logger := logging.FromContext(ctx).With().Str("order_book", name).Logger()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if order book already exists
	if _, exists := m.orderBooks[name]; exists {
		logger.Error().Msg("Order book already exists")
		return nil, ErrOrderBookExists
	}

	// Extract Redis options
	addr := "localhost:6379"
	password := ""
	dbStr := "0"
	prefix := name

	if val, ok := options["addr"]; ok && val != "" {
		addr = val
	}
	if val, ok := options["password"]; ok {
		password = val
	}
	if val, ok := options["db"]; ok && val != "" {
		dbStr = val
	}
	if val, ok := options["prefix"]; ok && val != "" {
		prefix = val
	}

	// Create a key for the Redis client pool
	redisKey := addr + ":" + dbStr

	// Get or create Redis client
	var client *redisClient.Client
	var exists bool

	if client, exists = m.redisPool[redisKey]; !exists {
		// Parse Redis options
		db := 0
		// Ignore error, default is 0
		// db, _ = strconv.Atoi(dbStr)

		// Create new Redis client
		client = redisClient.NewClient(&redisClient.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		})

		// Test connection
		if _, err := client.Ping(ctx).Result(); err != nil {
			logger.Error().Err(err).Msg("Failed to connect to Redis")
			return nil, err
		}

		// Store in pool
		m.redisPool[redisKey] = client
	}

	// Create Redis backend
	backend := redis.NewRedisBackend(client, prefix)

	// Create order book
	orderBook := core.NewOrderBook(backend)

	// Store order book
	m.orderBooks[name] = orderBook

	// Store metadata
	info := &OrderBookInfo{
		Name:      name,
		Backend:   "redis",
		CreatedAt: time.Now(),
	}
	m.info[name] = info

	logger.Info().
		Str("backend", "redis").
		Str("addr", addr).
		Str("db", dbStr).
		Str("prefix", prefix).
		Msg("Created new Redis order book")
	return info, nil
}

// GetOrderBook retrieves an order book by name
func (m *OrderBookManager) GetOrderBook(ctx context.Context, name string) (*core.OrderBook, *OrderBookInfo, error) {
	logger := logging.FromContext(ctx).With().Str("order_book", name).Logger()

	m.mu.RLock()
	defer m.mu.RUnlock()

	orderBook, exists := m.orderBooks[name]
	if !exists {
		logger.Debug().Msg("Order book not found")
		return nil, nil, ErrOrderBookNotFound
	}

	info := m.info[name]
	logger.Debug().Msg("Retrieved order book")
	return orderBook, info, nil
}

// DeleteOrderBook removes an order book
func (m *OrderBookManager) DeleteOrderBook(ctx context.Context, name string) error {
	logger := logging.FromContext(ctx).With().Str("order_book", name).Logger()

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.orderBooks[name]; !exists {
		logger.Debug().Msg("Order book not found")
		return ErrOrderBookNotFound
	}

	// Remove from maps
	delete(m.orderBooks, name)
	delete(m.info, name)

	logger.Info().Msg("Deleted order book")
	return nil
}

// ListOrderBooks returns information about all order books
func (m *OrderBookManager) ListOrderBooks(ctx context.Context) []*OrderBookInfo {
	logger := logging.FromContext(ctx)

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create slice with capacity for all order books
	result := make([]*OrderBookInfo, 0, len(m.info))

	// Add each order book info to the result
	for _, info := range m.info {
		result = append(result, info)
	}

	logger.Debug().Int("count", len(result)).Msg("Listed order books")
	return result
}

// UpdateOrderBookInfo updates the order count for an order book
func (m *OrderBookManager) UpdateOrderBookInfo(ctx context.Context, name string, orderCount int) error {
	logger := logging.FromContext(ctx).With().Str("order_book", name).Logger()

	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.info[name]
	if !exists {
		logger.Debug().Msg("Order book not found")
		return ErrOrderBookNotFound
	}

	info.OrderCount = orderCount
	logger.Debug().Int("order_count", orderCount).Msg("Updated order book info")
	return nil
}

// Close closes all resources used by the manager
func (m *OrderBookManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close all Redis clients
	for _, client := range m.redisPool {
		client.Close()
	}

	// Clear maps
	m.orderBooks = make(map[string]*core.OrderBook)
	m.info = make(map[string]*OrderBookInfo)
	m.redisPool = make(map[string]*redisClient.Client)
}

// LogOrderBookSummary logs summary information about an order book
func LogOrderBookSummary(ctx context.Context, logger zerolog.Logger, book *core.OrderBook, info *OrderBookInfo) {
	// Just log the basic information about the order book
	logger.Info().
		Str("name", info.Name).
		Str("backend", info.Backend).
		Time("created_at", info.CreatedAt).
		Int("order_count", info.OrderCount).
		Msg("Order book summary")
}
