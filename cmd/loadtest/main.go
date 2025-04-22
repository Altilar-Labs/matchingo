package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "github.com/erain9/matchingo/pkg/api/proto"
)

const (
	numWorkers        = 10000
	ordersPerWorker   = 100
	maxConcurrentReqs = 100
)

func main() {
	grpcAddr := flag.String("grpc-addr", "localhost:50051", "gRPC server address")
	flag.Parse()

	// Set up gRPC connection
	conn, err := grpc.Dial(*grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewOrderBookServiceClient(conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		log.Println("Received interrupt signal, cleaning up...")
		cancel()
	}()

	// Ensure clean state for test order book
	bookName := "load-test-order-book"
	log.Printf("Checking for existing order book: %s", bookName)
	_, err = client.GetOrderBook(ctx, &pb.GetOrderBookRequest{Name: bookName})
	if err == nil {
		// Order book exists, delete it first
		log.Printf("Order book '%s' found, deleting it...", bookName)
		_, err = client.DeleteOrderBook(ctx, &pb.DeleteOrderBookRequest{Name: bookName})
		if err != nil {
			log.Fatalf("Failed to delete existing order book '%s': %v", bookName, err)
		}
		log.Printf("Successfully deleted existing order book: %s", bookName)
	} else {
		// Check if the error is 'NotFound'
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			// Unexpected error during GetOrderBook
			log.Fatalf("Failed to check for order book '%s': %v", bookName, err)
		}
		// Order book does not exist, which is fine
		log.Printf("Order book '%s' not found, proceeding to create.", bookName)
	}

	// Create test order book
	log.Printf("Creating order book: %s", bookName)
	_, err = client.CreateOrderBook(ctx, &pb.CreateOrderBookRequest{
		Name:        bookName,
		BackendType: pb.BackendType_MEMORY,
	})
	if err != nil {
		log.Fatalf("Failed to create order book '%s': %v", bookName, err)
	}
	log.Printf("Successfully created order book: %s", bookName)

	// Set up rate limiter and wait group
	limiter := rate.NewLimiter(rate.Limit(maxConcurrentReqs), maxConcurrentReqs)
	var wg sync.WaitGroup
	errChan := make(chan error, numWorkers*ordersPerWorker)

	// Start workers
	start := time.Now()
	log.Printf("Starting %d workers, %d orders per worker...", numWorkers, ordersPerWorker)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < ordersPerWorker; j++ {
				if err := limiter.Wait(ctx); err != nil {
					errChan <- fmt.Errorf("rate limiter error: %v", err)
					return
				}

				order := generateRandomOrder(bookName, workerID*ordersPerWorker+j)
				_, err := client.CreateOrder(ctx, &pb.CreateOrderRequest{
					OrderBookName: order.OrderBookName,
					OrderId:       order.OrderId,
					Side:          order.Side,
					Quantity:      order.Quantity,
					Price:         order.Price,
					OrderType:     order.OrderType,
					TimeInForce:   pb.TimeInForce_GTC,
				})
				if err != nil {
					errChan <- fmt.Errorf("failed to create order: %v", err)
					continue
				}
			}
		}(i)
	}

	// Wait for all workers to finish
	wg.Wait()
	duration := time.Since(start)
	close(errChan)

	// Process errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	// Print results
	log.Printf("Load test completed in %v", duration)
	log.Printf("Total orders attempted: %d", numWorkers*ordersPerWorker)
	log.Printf("Errors encountered: %d", len(errors))

	// Clean up order book
	_, err = client.DeleteOrderBook(ctx, &pb.DeleteOrderBookRequest{
		Name: bookName,
	})
	if err != nil {
		log.Printf("Failed to delete order book: %v", err)
	} else {
		log.Printf("Successfully deleted order book: %s", bookName)
	}

	if len(errors) > 0 {
		log.Printf("First error: %v", errors[0])
		os.Exit(1)
	}
}

func generateRandomOrder(bookName string, orderNum int) *pb.OrderResponse {
	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(orderNum)))
	side := pb.OrderSide_BUY
	if r.Float64() < 0.5 {
		side = pb.OrderSide_SELL
	}

	// Use fixed price and quantity for higher matching probability
	const (
		fixedPrice    = "100.00"
		fixedQuantity = "10.00"
	)

	return &pb.OrderResponse{
		OrderId:       fmt.Sprintf("order-%d", orderNum),
		OrderBookName: bookName,
		Side:          side,
		Price:         fixedPrice,
		Quantity:      fixedQuantity,
		OrderType:     pb.OrderType_LIMIT,
	}
}
