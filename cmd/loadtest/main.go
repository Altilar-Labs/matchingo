package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"github.com/dustin/go-humanize"
	pb "github.com/erain9/matchingo/pkg/api/proto"
)

const (
	numWorkers      = 100 // Reduced from 200 to avoid overwhelming
	ordersPerWorker = 10000
	numConnections  = 10 // Reduced from 20 for better connection management
	reportInterval  = time.Second
	batchSize       = 50 // Reduced batch size for faster processing
)

func main() {
	grpcAddr := flag.String("grpc-addr", "localhost:50051", "gRPC server address")
	flag.Parse()

	// Create gRPC connections with optimized settings
	var clients []pb.OrderBookServiceClient
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		// Add performance optimizations
		grpc.WithInitialWindowSize(64 * 1024),     // 64KB
		grpc.WithInitialConnWindowSize(64 * 1024), // 64KB
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(100*1024*1024), // 100MB
			grpc.MaxCallRecvMsgSize(100*1024*1024), // 100MB
		),
		grpc.WithWriteBufferSize(64 * 1024), // 64KB
		grpc.WithReadBufferSize(64 * 1024),  // 64KB
		grpc.WithBlock(),                    // Wait for connections to establish
	}

	log.Printf("Creating %d gRPC connections...", numConnections)
	for i := 0; i < numConnections; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		conn, err := grpc.DialContext(ctx, *grpcAddr, opts...)
		cancel()
		if err != nil {
			log.Fatalf("Failed to create connection %d: %v", i, err)
		}
		defer conn.Close()
		clients = append(clients, pb.NewOrderBookServiceClient(conn))
	}

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

	// Create test order book
	bookName := "load-test-order-book"
	_, err := clients[0].CreateOrderBook(ctx, &pb.CreateOrderBookRequest{
		Name:        bookName,
		BackendType: pb.BackendType_MEMORY,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			log.Printf("Order book already exists, continuing...")
		} else {
			log.Fatalf("Failed to create order book: %v", err)
		}
	}

	var (
		completedOrders uint64
		errorCount      uint64
		wg              sync.WaitGroup
	)

	// Start metrics reporting
	go func() {
		ticker := time.NewTicker(reportInterval)
		defer ticker.Stop()
		lastCount := uint64(0)
		lastTime := time.Now()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				current := atomic.LoadUint64(&completedOrders)
				errors := atomic.LoadUint64(&errorCount)
				throughput := float64(current-lastCount) / now.Sub(lastTime).Seconds()

				// Get server-side order book status
				if current > lastCount {
					status, err := clients[0].GetOrderBook(ctx, &pb.GetOrderBookRequest{Name: bookName})
					if err == nil {
						log.Printf("Throughput: %s ops/sec | Client Total: %s | Server Total: %s | Errors: %s",
							humanize.Comma(int64(throughput)),
							humanize.Comma(int64(current)),
							humanize.Comma(int64(status.OrderCount)),
							humanize.Comma(int64(errors)))
					} else {
						log.Printf("Throughput: %s ops/sec | Client Total: %s | Server Status Error: %v | Errors: %s",
							humanize.Comma(int64(throughput)),
							humanize.Comma(int64(current)),
							err,
							humanize.Comma(int64(errors)))
					}
				}

				lastCount = current
				lastTime = now
			}
		}
	}()

	// Start workers
	start := time.Now()
	log.Printf("Starting %d workers...", numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			client := clients[workerID%len(clients)]

			// Pre-generate all orders for this worker
			orders := make([]*pb.CreateOrderRequest, ordersPerWorker)
			for j := 0; j < ordersPerWorker; j++ {
				order := generateRandomOrder(bookName, workerID*ordersPerWorker+j)
				orders[j] = &pb.CreateOrderRequest{
					OrderBookName: order.OrderBookName,
					OrderId:       order.OrderId,
					Side:          order.Side,
					Quantity:      order.Quantity,
					Price:         order.Price,
					OrderType:     order.OrderType,
					TimeInForce:   pb.TimeInForce_GTC,
				}
			}

			// Process orders in batches with adaptive pacing
			for j := 0; j < ordersPerWorker; j += batchSize {
				if ctx.Err() != nil {
					return
				}

				end := j + batchSize
				if end > ordersPerWorker {
					end = ordersPerWorker
				}

				// Send batch of orders with minimal delay
				for k := j; k < end; k++ {
					_, err := client.CreateOrder(ctx, orders[k])
					if err != nil {
						atomic.AddUint64(&errorCount, 1)
						// Add small backoff on error
						if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
							time.Sleep(time.Millisecond * 100)
						}
						continue
					}
					atomic.AddUint64(&completedOrders, 1)
				}

				// Dynamic backoff based on error rate
				errorRate := float64(atomic.LoadUint64(&errorCount)) / float64(atomic.LoadUint64(&completedOrders)+1)
				if errorRate > 0.1 {
					time.Sleep(time.Millisecond * 50)
				} else if errorRate > 0.05 {
					time.Sleep(time.Millisecond * 20)
				} else {
					time.Sleep(time.Millisecond * 5)
				}
			}
		}(i)
	}

	// Wait for all workers to finish
	wg.Wait()
	duration := time.Since(start)

	// Print final results
	totalOrders := atomic.LoadUint64(&completedOrders)
	totalErrors := atomic.LoadUint64(&errorCount)
	averageOPS := float64(totalOrders) / duration.Seconds()

	log.Printf("\nLoad test completed in %v", duration)
	log.Printf("Total orders completed: %s", humanize.Comma(int64(totalOrders)))
	log.Printf("Average throughput: %s orders/sec", humanize.Comma(int64(averageOPS)))
	log.Printf("Total errors: %s", humanize.Comma(int64(totalErrors)))

	// Clean up
	_, err = clients[0].DeleteOrderBook(ctx, &pb.DeleteOrderBookRequest{Name: bookName})
	if err != nil {
		log.Printf("Failed to delete order book: %v", err)
	}
}

func generateRandomOrder(bookName string, orderNum int) *pb.OrderResponse {
	// Ensure exact 50-50 distribution of buy/sell orders for maximum matching
	side := pb.OrderSide_BUY
	if orderNum%2 == 0 {
		side = pb.OrderSide_SELL
	}

	return &pb.OrderResponse{
		OrderId:       fmt.Sprintf("order-%d", orderNum),
		OrderBookName: bookName,
		Side:          side,
		Price:         "100.00",
		Quantity:      "10.00",
		OrderType:     pb.OrderType_LIMIT,
	}
}
