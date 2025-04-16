package marketmaker

import (
	"context"

	pb "github.com/erain9/matchingo/pkg/api/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// PriceFetcher defines the interface for fetching current market prices
type PriceFetcher interface {
	// FetchPrice returns the current market price for the configured symbol
	FetchPrice(ctx context.Context) (float64, error)
	// Close releases any resources held by the price fetcher
	Close() error
}

// OrderPlacer defines the interface for placing and canceling orders
type OrderPlacer interface {
	CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.OrderResponse, error)
	CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*emptypb.Empty, error)
	Close() error
}

// MarketMakerStrategy defines the interface for market making strategies
type MarketMakerStrategy interface {
	// CalculateOrders calculates the orders to be placed based on the current price
	CalculateOrders(ctx context.Context, currentPrice float64, userAddress string) ([]*pb.CreateOrderRequest, error)
}
