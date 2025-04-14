package marketmaker

import (
	"context"
	"fmt"
	"log/slog"

	pb "github.com/erain9/matchingo/pkg/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Ensure grpcOrderPlacer implements OrderPlacer interface
var _ OrderPlacer = (*grpcOrderPlacer)(nil)

// grpcOrderPlacer implements the OrderPlacer interface using a gRPC client.
type grpcOrderPlacer struct {
	client pb.OrderBookServiceClient
	conn   *grpc.ClientConn
	cfg    *Config
	logger *slog.Logger
}

// NewGRPCOrderPlacer creates a new gRPC client connection and returns an OrderPlacer.
func NewGRPCOrderPlacer(cfg *Config, logger *slog.Logger) (OrderPlacer, error) {
	logger.Info("Connecting to Matchingo gRPC server", "address", cfg.MatchingoGRPCAddr)

	// Set up a connection to the server.
	// Using insecure credentials for now. Add proper TLS in production.
	connCtx, cancel := context.WithTimeout(context.Background(), cfg.RequestTimeout)
	defer cancel()

	conn, err := grpc.DialContext(connCtx, cfg.MatchingoGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithUserAgent("MatchingoMarketMaker/0.1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server at %s: %w", cfg.MatchingoGRPCAddr, err)
	}

	client := pb.NewOrderBookServiceClient(conn)
	logger.Info("Successfully connected to Matchingo gRPC server")

	return &grpcOrderPlacer{
		client: client,
		conn:   conn,
		cfg:    cfg,
		logger: logger.With("component", "grpcOrderPlacer"),
	}, nil
}

// CreateOrder sends a CreateOrder request to the Matchingo service.
func (p *grpcOrderPlacer) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.OrderResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, p.cfg.RequestTimeout)
	defer cancel()

	p.logger.Debug("Sending CreateOrder request",
		"order_book", req.OrderBookName,
		"order_id", req.OrderId,
		"side", req.Side,
		"type", req.OrderType,
		"qty", req.Quantity,
		"price", req.Price)

	resp, err := p.client.CreateOrder(callCtx, req)
	if err != nil {
		p.logger.Error("CreateOrder RPC failed",
			"order_book", req.OrderBookName,
			"order_id", req.OrderId,
			"error", err)
		return nil, fmt.Errorf("CreateOrder failed: %w", err)
	}

	p.logger.Info("Successfully created order",
		"order_book", resp.OrderBookName,
		"order_id", resp.OrderId,
		"status", resp.Status)
	return resp, nil
}

// CancelOrder sends a CancelOrder request to the Matchingo service.
func (p *grpcOrderPlacer) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*emptypb.Empty, error) {
	callCtx, cancel := context.WithTimeout(ctx, p.cfg.RequestTimeout)
	defer cancel()

	p.logger.Debug("Sending CancelOrder request",
		"order_book", req.OrderBookName,
		"order_id", req.OrderId)

	resp, err := p.client.CancelOrder(callCtx, req)
	if err != nil {
		// Check if the error is 'NotFound'
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			// Log as info, not error, as it's expected if order was already filled/cancelled
			p.logger.Info("CancelOrder skipped as order was not found (likely filled/cancelled)",
				"order_book", req.OrderBookName,
				"order_id", req.OrderId)
			// Return nil error here because the goal (order not being active) is achieved.
			return &emptypb.Empty{}, nil
		}

		// Log other errors as actual errors
		p.logger.Error("CancelOrder RPC failed",
			"order_book", req.OrderBookName,
			"order_id", req.OrderId,
			"error", err)
		// Return the original error for other failure cases
		return nil, fmt.Errorf("CancelOrder failed: %w", err)
	}

	p.logger.Info("Successfully cancelled order",
		"order_book", req.OrderBookName,
		"order_id", req.OrderId)
	return resp, nil
}

// Close closes the underlying gRPC connection.
func (p *grpcOrderPlacer) Close() error {
	if p.conn != nil {
		p.logger.Info("Closing gRPC connection")
		return p.conn.Close()
	}
	return nil
}
