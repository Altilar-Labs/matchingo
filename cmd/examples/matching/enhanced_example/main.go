package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/erain9/matchingo/pkg/backend/memory"
	"github.com/erain9/matchingo/pkg/core"
	"github.com/nikolaydubina/fpdecimal"
)

// A demonstration of the enhanced order matching engine
func main() {
	// Create an order book with memory backend
	backend := memory.NewMemoryBackend()
	book := core.NewOrderBook(backend)

	fmt.Println("===== MATCHINGO ENHANCED ORDER MATCHING DEMONSTRATION =====")
	fmt.Println("This example shows how orders match and trades execute with the enhanced matching engine")
	fmt.Println()

	// Step 1: Create several sell (ask) orders
	fmt.Println("STEP 1: Adding sell orders to the order book")
	fmt.Println("------------------------------------------")

	// Add sell orders at different price levels (from lowest to highest)
	sellOrder1 := createLimitOrder("sell-1", core.Sell, 10.0, 5.0) // 5 units at $10.00
	sellOrder2 := createLimitOrder("sell-2", core.Sell, 10.5, 3.0) // 3 units at $10.50
	sellOrder3 := createLimitOrder("sell-3", core.Sell, 11.0, 7.0) // 7 units at $11.00

	// Process each sell order (they'll be added to the book)
	for _, order := range []*core.Order{sellOrder1, sellOrder2, sellOrder3} {
		done, err := book.Process(order)
		if err != nil {
			fmt.Printf("Error processing %s: %v\n", order.ID(), err)
			continue
		}
		fmt.Printf("Added sell order: ID=%s, Price=$%.2f, Quantity=%.2f\n",
			order.ID(),
			toFloat64(order.Price()),
			toFloat64(order.Quantity()))

		printDoneObject(done)
	}

	// Print the current state of the order book
	fmt.Println("\nCurrent order book status:")
	fmt.Println(book.String())
	fmt.Println()

	// Step 2: Create buy (bid) orders that will be matched with the sell orders
	fmt.Println("STEP 2: Adding a buy order that matches the lowest sell price")
	fmt.Println("----------------------------------------------------------")

	// Add a buy order at $10.00 (matching the lowest sell price)
	buyOrder1 := createLimitOrder("buy-1", core.Buy, 10.0, 3.0) // 3 units at $10.00
	fmt.Printf("Processing buy order: ID=%s, Price=$%.2f, Quantity=%.2f\n",
		buyOrder1.ID(), toFloat64(buyOrder1.Price()), toFloat64(buyOrder1.Quantity()))

	done1, err := book.Process(buyOrder1)
	if err != nil {
		fmt.Printf("Error processing %s: %v\n", buyOrder1.ID(), err)
	} else {
		// This order should match with sell-1
		fmt.Println("Buy order processed:")
		printDoneObject(done1)
	}

	// Print updated order book
	fmt.Println("\nUpdated order book status:")
	fmt.Println(book.String())
	fmt.Println()

	// Step 3: Add a buy order that matches with multiple sell orders
	fmt.Println("STEP 3: Adding a buy order that crosses multiple sell orders")
	fmt.Println("---------------------------------------------------------")

	buyOrder2 := createLimitOrder("buy-2", core.Buy, 11.0, 8.0) // 8 units at $11.00
	fmt.Printf("Processing buy order: ID=%s, Price=$%.2f, Quantity=%.2f\n",
		buyOrder2.ID(), toFloat64(buyOrder2.Price()), toFloat64(buyOrder2.Quantity()))

	done2, err := book.Process(buyOrder2)
	if err != nil {
		fmt.Printf("Error processing %s: %v\n", buyOrder2.ID(), err)
	} else {
		// This order should match with remaining sell-1, all of sell-2, and part of sell-3
		fmt.Println("Buy order processed:")
		printDoneObject(done2)
	}

	// Print updated order book
	fmt.Println("\nUpdated order book status:")
	fmt.Println(book.String())
	fmt.Println()

	// Step 4: Add a market buy order
	fmt.Println("STEP 4: Adding a market buy order")
	fmt.Println("-------------------------------")

	marketBuyID := fmt.Sprintf("market-buy-%d", time.Now().UnixMilli())
	marketBuyOrder := core.NewMarketOrder(marketBuyID, core.Buy, fpdecimal.FromFloat(4.0))
	fmt.Printf("Processing market buy order: ID=%s, Quantity=%.2f\n",
		marketBuyOrder.ID(), toFloat64(marketBuyOrder.Quantity()))

	doneMarket, err := book.Process(marketBuyOrder)
	if err != nil {
		fmt.Printf("Error processing %s: %v\n", marketBuyOrder.ID(), err)
	} else {
		// This should match with the remaining sell-3 order
		fmt.Println("Market buy order processed:")
		printDoneObject(doneMarket)
	}

	// Print final order book status
	fmt.Println("\nFinal order book status:")
	fmt.Println(book.String())
	fmt.Println()

	// Explanation section
	fmt.Println("===== MATCHING ENGINE EXPLANATION =====")
	fmt.Println("The enhanced matching engine implements these key principles:")
	fmt.Println("1. Price-Time Priority: Orders are filled from best to worst price")
	fmt.Println("2. For buy orders: matches with the lowest-priced sell orders first")
	fmt.Println("3. For sell orders: matches with the highest-priced buy orders first")
	fmt.Println("4. When a match occurs (buy price >= sell price), a trade is executed")
	fmt.Println("5. Trades are recorded in the Done object with quantities and prices")
	fmt.Println("6. Partial fills occur when order quantities don't match exactly")
	fmt.Println("7. Any remaining quantity is added to the book (for limit orders)")
	fmt.Println()
	fmt.Println("In this example, we saw:")
	fmt.Println("- A buy order matching with a sell order at the same price")
	fmt.Println("- A buy order matching with multiple sell orders across price levels")
	fmt.Println("- A market order executing immediately at the best available price")
	fmt.Println("- Partial fills where remaining quantities were either removed or added to the book")
}

// Helper function to create a limit order
func createLimitOrder(id string, side core.Side, price, quantity float64) *core.Order {
	return core.NewLimitOrder(
		id,
		side,
		fpdecimal.FromFloat(quantity),
		fpdecimal.FromFloat(price),
		core.GTC,
		"",
	)
}

// Helper function to print the Done object
func printDoneObject(done *core.Done) {
	if done == nil {
		fmt.Println("No processing data available")
		return
	}

	// Print original and processed quantities
	fmt.Printf("  -> Original quantity: %.2f\n", toFloat64(done.Quantity))
	fmt.Printf("  -> Processed quantity: %.2f\n", toFloat64(done.Processed))
	fmt.Printf("  -> Remaining quantity: %.2f\n", toFloat64(done.Left))
	fmt.Printf("  -> Order stored in book: %v\n", done.Stored)

	if len(done.Trades) > 0 {
		fmt.Println("  -> Trades executed:")
		for i, trade := range done.Trades {
			// Skip the first entry as it's the original order
			if i == 0 {
				continue
			}
			fmt.Printf("     #%d: OrderID: %s, Quantity: %.2f, Price: $%.2f\n",
				i, trade.OrderID, toFloat64(trade.Quantity), toFloat64(trade.Price))
		}
	} else {
		fmt.Println("  -> No trades executed")
	}
}

// Helper function to convert fpdecimal.Decimal to float64 for easier display
func toFloat64(dec fpdecimal.Decimal) float64 {
	s := dec.String()
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
