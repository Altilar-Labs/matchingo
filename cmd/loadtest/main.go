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
	"sync/atomic"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
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

	// Metrics: HDR histogram recorder and atomic counters
	recorder := hdrhistogram.NewRecorder(1, 10_000_000, 3) // value in microseconds
	var reqCount, errCount int64

	// Reporter: log interval metrics every 30s using histogram
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// swap counters
				count := atomic.SwapInt64(&reqCount, 0)
				errs := atomic.SwapInt64(&errCount, 0)

				if count == 0 {
					log.Println("Interval metrics: no requests")
					continue
				}
				// snapshot and reset histogram
				snap := recorder.Histogram()
				recorder.Reset()
				// compute percentiles from snapshot
				p50 := time.Duration(snap.ValueAtQuantile(50.0)) * time.Microsecond
				p75 := time.Duration(snap.ValueAtQuantile(75.0)) * time.Microsecond
				p90 := time.Duration(snap.ValueAtQuantile(90.0)) * time.Microsecond
				p95 := time.Duration(snap.ValueAtQuantile(95.0)) * time.Microsecond
				rps := float64(count) / 30.0
				log.Printf("Interval metrics: requests=%d, errors=%d, rps=%.2f, p50=%v, p75=%v, p90=%v, p95=%v",
					count, errs, rps, p50, p75, p90, p95)
			}
		}
	}()

	// Start workers
	start := time.Now()
	log.Printf("Starting %d workers, %d orders per worker...", numWorkers, ordersPerWorker)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < ordersPerWorker; j++ {
				if err := limiter.Wait(ctx); err != nil {
					atomic.AddInt64(&reqCount, 1)
					atomic.AddInt64(&errCount, 1)
					errChan <- fmt.Errorf("rate limiter error: %v", err)
					return
				}
				startReq := time.Now()
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
				// record metrics
				latency := time.Since(startReq)
				recorder.RecordValue(latency.Microseconds())
				atomic.AddInt64(&reqCount, 1)
				if err != nil {
					atomic.AddInt64(&errCount, 1)
				}
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
