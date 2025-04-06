package main

import (
	"context"
	"fmt"
	"time"

	"github.com/nikolaydubina/fpdecimal"
	"github.com/redis/go-redis/v9"
	redisbackend "github.com/erain9/matchingo/pkg/backend/redis"
	"github.com/erain9/matchingo/pkg/core"
)

const (
	redisAddr = "localhost:6379"
	redisDB   = 0
	prefix    = "matchingo"
)

func main() {
	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // no password set
		DB:       redisDB,
	})

	// Check Redis connection
	pong, err := client.Ping(context.Background()).Result()
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to Redis: %v", err))
	}
	fmt.Printf("Redis connection established: %s\n", pong)

	// Flush the database to start fresh
	client.FlushDB(context.Background())

	// Initialize order book with Redis backend
	backend := redisbackend.NewRedisBackend(client, prefix)
	book := core.NewOrderBook(backend)

	// Create order IDs with timestamp to ensure uniqueness
	buyOrderID := fmt.Sprintf("buy_%d", time.Now().UnixMilli())
	sellOrderID := fmt.Sprintf("sell_%d", time.Now().UnixMilli())

	// Create a sell limit order
	sellPrice := fpdecimal.FromFloat(10.0)
	sellQuantity := fpdecimal.FromFloat(10.0)
	sellOrder := core.NewLimitOrder(sellOrderID, core.Sell, sellQuantity, sellPrice, core.GTC, "")

	// Process the sell order
	_, err = book.Process(sellOrder)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Created sell order: %s\n", sellOrder.ID())

	// Create a buy limit order with partial fill
	buyPrice := fpdecimal.FromFloat(10.0)
	buyQuantity := fpdecimal.FromFloat(5.0)
	buyOrder := core.NewLimitOrder(buyOrderID, core.Buy, buyQuantity, buyPrice, core.GTC, "")

	// Process the buy order
	buyDone, err := book.Process(buyOrder)
	if err != nil {
		panic(err)
	}

	// Retrieve the updated sell order from Redis
	updatedSellOrder := book.GetOrder(sellOrderID)

	// Print the results
	fmt.Printf("Processing buy order: %s\n", buyOrder.ID())
	fmt.Printf("Trade executed: sell=%s (matched), buy=%s\n", updatedSellOrder.ID(), buyOrder.ID())
	fmt.Printf("Sell order remaining quantity: %s\n", updatedSellOrder.Quantity().String())
	fmt.Printf("Buy order processed quantity: %s\n", buyDone.Processed.String())

	// Print Redis storage details
	fmt.Println("\nOrders stored in Redis:")
	jsonData, _ := client.Get(context.Background(), fmt.Sprintf("%s:order:%s", prefix, sellOrderID)).Result()
	fmt.Printf("- Sell Order Redis data: %s\n", jsonData)

	// Summary
	fmt.Println("\nSummary of orders:")
	fmt.Printf("- Sell Order: ID=%s, Price=%s, Quantity=%s/%s\n",
		updatedSellOrder.ID(), updatedSellOrder.Price(), updatedSellOrder.Quantity(), updatedSellOrder.OriginalQty())
	fmt.Printf("- Buy Order: ID=%s, Price=%s, Quantity=%s/%s\n",
		buyOrder.ID(), buyOrder.Price(), buyOrder.Quantity(), buyOrder.OriginalQty())
}
