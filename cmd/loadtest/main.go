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
	numWorkers      = 200
	ordersPerWorker = 10000
	numConnections  = 20
	reportInterval  = time.Second
)

func main() {
	grpcAddr := flag.String("grpc-addr", "localhost:50051", "gRPC server address")
	flag.Parse()

	// Create gRPC connections
	var clients []pb.OrderBookServiceClient
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		// Add performance optimizations
		grpc.WithInitialWindowSize(1 << 24),     // 16MB
		grpc.WithInitialConnWindowSize(1 << 24), // 16MB
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(1024*1024), // 1MB
			grpc.MaxCallRecvMsgSize(1024*1024), // 1MB
		),
		grpc.WithWriteBufferSize(64 * 1024), // 64KB
		grpc.WithReadBufferSize(64 * 1024),  // 64KB
	}

	log.Printf("Creating %d gRPC connections...", numConnections)
	for i := 0; i < numConnections; i++ {
		conn, err := grpc.Dial(*grpcAddr, opts...)
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

				log.Printf("Throughput: %s ops/sec | Total: %s | Errors: %s",
					humanize.Comma(int64(throughput)),
					humanize.Comma(int64(current)),
					humanize.Comma(int64(errors)))

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

			// Each worker uses a dedicated connection in round-robin
			client := clients[workerID%len(clients)]

			// Pre-generate orders for this worker
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

			// Send orders with minimal overhead
			for j := 0; j < ordersPerWorker; j++ {
				if ctx.Err() != nil {
					return
				}

				_, err := client.CreateOrder(ctx, orders[j])
				if err != nil {
					atomic.AddUint64(&errorCount, 1)
					continue
				}
				atomic.AddUint64(&completedOrders, 1)
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
