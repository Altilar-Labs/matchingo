package main

import (
	"fmt"
	"time"

	"github.com/nikolaydubina/fpdecimal"
	"github.com/erain9/matchingo/pkg/backend/memory"
	"github.com/erain9/matchingo/pkg/core"
)

func main() {
	// Initialize order book with in-memory backend
	backend := memory.NewMemoryBackend()
	book := core.NewOrderBook(backend)

	// Create order IDs
	buyOrderID := fmt.Sprintf("buy_%d", time.Now().UnixMilli())
	sellOrderID := fmt.Sprintf("sell_%d", time.Now().UnixMilli())

	// Create a sell limit order
	sellPrice := fpdecimal.FromFloat(10.0)
	sellQuantity := fpdecimal.FromFloat(10.0)
	sellOrder := core.NewLimitOrder(sellOrderID, core.Sell, sellQuantity, sellPrice, core.GTC, "")

	// Process the sell order
	_, err := book.Process(sellOrder)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Created sell order: %s\n", sellOrder.ID())

	// Create a buy limit order
	buyPrice := fpdecimal.FromFloat(10.0)
	buyQuantity := fpdecimal.FromFloat(5.0)
	buyOrder := core.NewLimitOrder(buyOrderID, core.Buy, buyQuantity, buyPrice, core.GTC, "")

	// Process the buy order
	buyDone, err := book.Process(buyOrder)
	if err != nil {
		panic(err)
	}

	// Print the results
	fmt.Printf("Processing buy order: %s\n", buyOrder.ID())
	fmt.Printf("Trade executed: sell=%s (matched), buy=%s\n", sellOrder.ID(), buyOrder.ID())
	fmt.Printf("Sell order remaining quantity: %s\n", sellOrder.Quantity().String())
	fmt.Printf("Buy order processed quantity: %s\n", buyDone.Processed.String())

	// Summary
	fmt.Println("\nSummary of orders:")
	fmt.Printf("- Sell Order: ID=%s, Price=%s, Quantity=%s/%s\n",
		sellOrder.ID(), sellOrder.Price(), sellOrder.Quantity(), sellOrder.OriginalQty())
	fmt.Printf("- Buy Order: ID=%s, Price=%s, Quantity=%s/%s\n",
		buyOrder.ID(), buyOrder.Price(), buyOrder.Quantity(), buyOrder.OriginalQty())
}
