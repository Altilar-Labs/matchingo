package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/erain9/matchingo/pkg/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestMain(m *testing.M) {
	// Save original args and flags
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}()

	// Run tests
	os.Exit(m.Run())
}

func generateTestName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func TestCreateOrderBook(t *testing.T) {
	bookName := generateTestName("test-book")
	// Set up test args
	os.Args = []string{"orderbook-client", "create-book", "-name=" + bookName, "-backend=memory"}

	// Create a test context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to the gRPC server
	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Create a client
	client := proto.NewOrderBookServiceClient(conn)

	// Run the test
	createOrderBook(ctx, client)
}

func TestGetOrderBook(t *testing.T) {
	bookName := generateTestName("test-book")
	// Reset flags before test
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Set up test args
	os.Args = []string{"orderbook-client", "get-book", "-name=" + bookName}

	// Create a test context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to the gRPC server
	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Create a client
	client := proto.NewOrderBookServiceClient(conn)

	// First create the order book
	_, err = client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
		Name:        bookName,
		BackendType: proto.BackendType_MEMORY,
	})
	if err != nil {
		t.Fatalf("Failed to create order book: %v", err)
	}

	// Run the test
	getOrderBook(ctx, client)
}

func TestListOrderBooks(t *testing.T) {
	// Set up test args
	os.Args = []string{"orderbook-client", "list-books", "-limit=10", "-offset=0"}

	// Create a test context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to the gRPC server
	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Create a client
	client := proto.NewOrderBookServiceClient(conn)

	// Run the test
	listOrderBooks(ctx, client)
}

func TestCreateOrder(t *testing.T) {
	bookName := generateTestName("test-book")
	orderID := generateTestName("test-order")
	// Set up test args
	os.Args = []string{"orderbook-client", "create-order", bookName, "BUY", "LIMIT", "1.0", "100.0", orderID}

	// Create a test context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to the gRPC server
	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Create a client
	client := proto.NewOrderBookServiceClient(conn)

	// First create the order book
	_, err = client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
		Name:        bookName,
		BackendType: proto.BackendType_MEMORY,
	})
	if err != nil {
		t.Fatalf("Failed to create order book: %v", err)
	}

	// Run the test
	createOrder(ctx, client, bookName, "BUY", "LIMIT", "1.0", "100.0", orderID)
}

func TestGetOrder(t *testing.T) {
	bookName := generateTestName("test-book")
	orderID := generateTestName("test-order")
	// Set up test args
	os.Args = []string{"orderbook-client", "get-order", bookName, orderID}

	// Create a test context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to the gRPC server
	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Create a client
	client := proto.NewOrderBookServiceClient(conn)

	// First create the order book
	_, err = client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
		Name:        bookName,
		BackendType: proto.BackendType_MEMORY,
	})
	if err != nil {
		t.Fatalf("Failed to create order book: %v", err)
	}

	// Create an order
	_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
		OrderBookName: bookName,
		OrderId:       orderID,
		Side:          proto.OrderSide_BUY,
		OrderType:     proto.OrderType_LIMIT,
		Quantity:      "1.0",
		Price:         "100.0",
		TimeInForce:   proto.TimeInForce_GTC,
	})
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	// Run the test
	getOrder(ctx, client, bookName, orderID)
}

func TestCancelOrder(t *testing.T) {
	bookName := generateTestName("test-book")
	orderID := generateTestName("test-order")
	// Set up test args
	os.Args = []string{"orderbook-client", "cancel-order", bookName, orderID}

	// Create a test context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to the gRPC server
	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Create a client
	client := proto.NewOrderBookServiceClient(conn)

	// First create the order book
	_, err = client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
		Name:        bookName,
		BackendType: proto.BackendType_MEMORY,
	})
	if err != nil {
		t.Fatalf("Failed to create order book: %v", err)
	}

	// Create an order
	_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
		OrderBookName: bookName,
		OrderId:       orderID,
		Side:          proto.OrderSide_BUY,
		OrderType:     proto.OrderType_LIMIT,
		Quantity:      "1.0",
		Price:         "100.0",
		TimeInForce:   proto.TimeInForce_GTC,
	})
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	// Run the test
	cancelOrder(ctx, client, bookName, orderID)
}

func TestGetOrderBookState(t *testing.T) {
	bookName := generateTestName("test-book")
	// Set up test args
	os.Args = []string{"orderbook-client", "get-state", bookName}

	// Create a test context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to the gRPC server
	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Create a client
	client := proto.NewOrderBookServiceClient(conn)

	// First create the order book
	_, err = client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
		Name:        bookName,
		BackendType: proto.BackendType_MEMORY,
	})
	if err != nil {
		t.Fatalf("Failed to create order book: %v", err)
	}

	// Run the test
	getOrderBookState(ctx, client, bookName)
}
