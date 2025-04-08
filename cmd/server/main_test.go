package main

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/erain9/matchingo/pkg/api/proto"
	"github.com/erain9/matchingo/pkg/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

var (
	lis *bufconn.Listener
	s   *grpc.Server
)

func init() {
	lis = bufconn.Listen(bufSize)
	s = grpc.NewServer()
	manager := server.NewOrderBookManager()
	service := server.NewGRPCOrderBookService(manager)
	proto.RegisterOrderBookServiceServer(s, service)
	go func() {
		if err := s.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			panic(err)
		}
	}()
}

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestMain(m *testing.M) {
	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Run tests
	code := m.Run()

	// Clean up
	s.GracefulStop()
	lis.Close()

	os.Exit(code)
}

func TestServerStartup(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := proto.NewOrderBookServiceClient(conn)

	// Test creating an order book
	req := &proto.CreateOrderBookRequest{
		Name:        "test-book",
		BackendType: proto.BackendType_MEMORY,
	}

	resp, err := client.CreateOrderBook(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create order book: %v", err)
	}

	if resp.Name != "test-book" {
		t.Errorf("Expected name 'test-book', got '%s'", resp.Name)
	}

	if resp.BackendType != proto.BackendType_MEMORY {
		t.Errorf("Expected backend type MEMORY, got %s", resp.BackendType)
	}
}

func TestServerShutdown(t *testing.T) {
	// Create a test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a test server
	testLis := bufconn.Listen(bufSize)
	testServer := grpc.NewServer()
	manager := server.NewOrderBookManager()
	service := server.NewGRPCOrderBookService(manager)
	proto.RegisterOrderBookServiceServer(testServer, service)

	// Start server in a goroutine
	go func() {
		if err := testServer.Serve(testLis); err != nil && err != grpc.ErrServerStopped {
			t.Errorf("Failed to serve: %v", err)
		}
	}()

	// Create a test dialer
	testDialer := func(context.Context, string) (net.Conn, error) {
		return testLis.Dial()
	}

	// Connect to the server
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(testDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := proto.NewOrderBookServiceClient(conn)

	// Create an order book
	_, err = client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
		Name:        "test-book",
		BackendType: proto.BackendType_MEMORY,
	})
	if err != nil {
		t.Fatalf("Failed to create order book: %v", err)
	}

	// Gracefully stop the server
	testServer.GracefulStop()

	// Try to make a request after shutdown
	_, err = client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
		Name:        "test-book-2",
		BackendType: proto.BackendType_MEMORY,
	})

	if err == nil {
		t.Error("Expected error after server shutdown, got none")
	}

	// Clean up
	testLis.Close()
}
