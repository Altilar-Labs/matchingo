// Package server contains the order book service implementation.
package server

import (
	"context"
	"errors"
	"time"

	"github.com/erain9/matchingo/pkg/api/proto"
	"github.com/erain9/matchingo/pkg/backend/memory"
	"github.com/erain9/matchingo/pkg/core"
	"github.com/erain9/matchingo/pkg/logging"
	"github.com/nikolaydubina/fpdecimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCOrderBookService implements the OrderBookService gRPC interface
type GRPCOrderBookService struct {
	proto.UnimplementedOrderBookServiceServer
	manager *OrderBookManager
}

// NewGRPCOrderBookService creates a new GRPCOrderBookService
func NewGRPCOrderBookService(manager *OrderBookManager) *GRPCOrderBookService {
	return &GRPCOrderBookService{
		manager: manager,
	}
}

// Helper function to convert proto TIF enum to core TIF string
func convertProtoTIFToCore(tif proto.TimeInForce) core.TIF {
	switch tif {
	case proto.TimeInForce_IOC:
		return core.IOC
	case proto.TimeInForce_FOK:
		return core.FOK
	case proto.TimeInForce_GTC:
		fallthrough // Default to GTC
	default:
		return core.GTC
	}
}

// CreateOrderBook implements the CreateOrderBook RPC method
func (s *GRPCOrderBookService) CreateOrderBook(ctx context.Context, req *proto.CreateOrderBookRequest) (*proto.OrderBookResponse, error) {
	logger := logging.FromContext(ctx).With().Str("method", "CreateOrderBook").Logger()
	logger.Debug().Str("name", req.Name).Str("backend", req.BackendType.String()).Msg("Request received")

	var info *OrderBookInfo
	var err error

	switch req.BackendType {
	case proto.BackendType_MEMORY:
		info, err = s.manager.CreateMemoryOrderBook(ctx, req.Name)
	case proto.BackendType_REDIS:
		info, err = s.manager.CreateRedisOrderBook(ctx, req.Name, req.Options)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unsupported backend type: %v", req.BackendType)
	}

	if err != nil {
		if err == ErrOrderBookExists {
			return nil, status.Errorf(codes.AlreadyExists, "order book %s already exists", req.Name)
		}
		logger.Error().Err(err).Msg("Failed to create order book")
		return nil, status.Errorf(codes.Internal, "failed to create order book: %v", err)
	}

	return &proto.OrderBookResponse{
		Name:        info.Name,
		BackendType: req.BackendType,
		CreatedAt:   timestamppb.New(info.CreatedAt),
		OrderCount:  uint64(info.OrderCount),
	}, nil
}

// GetOrderBook retrieves information about an order book
func (s *GRPCOrderBookService) GetOrderBook(ctx context.Context, req *proto.GetOrderBookRequest) (*proto.OrderBookResponse, error) {
	logger := logging.FromContext(ctx).With().Str("method", "GetOrderBook").Logger()
	logger.Debug().Str("name", req.Name).Msg("Request received")

	_, info, err := s.manager.GetOrderBook(ctx, req.Name)
	if err != nil {
		if err == ErrOrderBookNotFound {
			return nil, status.Errorf(codes.NotFound, "order book %s not found", req.Name)
		}
		logger.Error().Err(err).Msg("Failed to get order book")
		return nil, status.Errorf(codes.Internal, "failed to get order book: %v", err)
	}

	// Determine backend type
	backendType := proto.BackendType_MEMORY
	if info.Backend == "redis" {
		backendType = proto.BackendType_REDIS
	}

	return &proto.OrderBookResponse{
		Name:        info.Name,
		BackendType: backendType,
		CreatedAt:   timestamppb.New(info.CreatedAt),
		OrderCount:  uint64(info.OrderCount),
	}, nil
}

// ListOrderBooks lists all available order books
func (s *GRPCOrderBookService) ListOrderBooks(ctx context.Context, req *proto.ListOrderBooksRequest) (*proto.ListOrderBooksResponse, error) {
	logger := logging.FromContext(ctx).With().Str("method", "ListOrderBooks").Logger()
	logger.Debug().Int32("limit", req.Limit).Int32("offset", req.Offset).Msg("Request received")

	// Get all order books
	infoList := s.manager.ListOrderBooks(ctx)

	// Apply pagination
	offset := int(req.Offset)
	if offset < 0 {
		offset = 0
	}

	limit := int(req.Limit)
	if limit <= 0 || limit > 100 {
		limit = 100 // Default limit
	}

	// Calculate bounds
	total := len(infoList)
	end := offset + limit
	if end > total {
		end = total
	}

	// Handle out-of-bounds offset
	if offset >= total {
		return &proto.ListOrderBooksResponse{
			OrderBooks: []*proto.OrderBookResponse{},
			Total:      int32(total),
		}, nil
	}

	// Create response slice with capacity for all items
	responseItems := make([]*proto.OrderBookResponse, 0, end-offset)

	// Add paginated items
	for i := offset; i < end; i++ {
		info := infoList[i]

		// Determine backend type
		backendType := proto.BackendType_MEMORY
		if info.Backend == "redis" {
			backendType = proto.BackendType_REDIS
		}

		responseItems = append(responseItems, &proto.OrderBookResponse{
			Name:        info.Name,
			BackendType: backendType,
			CreatedAt:   timestamppb.New(info.CreatedAt),
			OrderCount:  uint64(info.OrderCount),
		})
	}

	return &proto.ListOrderBooksResponse{
		OrderBooks: responseItems,
		Total:      int32(total),
	}, nil
}

// DeleteOrderBook deletes an order book
func (s *GRPCOrderBookService) DeleteOrderBook(ctx context.Context, req *proto.DeleteOrderBookRequest) (*emptypb.Empty, error) {
	logger := logging.FromContext(ctx).With().Str("method", "DeleteOrderBook").Logger()
	logger.Debug().Str("name", req.Name).Msg("Request received")

	err := s.manager.DeleteOrderBook(ctx, req.Name)
	if err != nil {
		if err == ErrOrderBookNotFound {
			return nil, status.Errorf(codes.NotFound, "order book %s not found", req.Name)
		}
		logger.Error().Err(err).Msg("Failed to delete order book")
		return nil, status.Errorf(codes.Internal, "failed to delete order book: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// CreateOrder submits a new order to the specified order book
func (s *GRPCOrderBookService) CreateOrder(ctx context.Context, req *proto.CreateOrderRequest) (*proto.OrderResponse, error) {
	logger := logging.FromContext(ctx).With().
		Str("method", "CreateOrder").
		Str("order_book", req.OrderBookName).
		Str("order_id", req.OrderId).
		Logger()

	logger.Debug().
		Str("side", req.Side.String()).
		Str("type", req.OrderType.String()).
		Str("quantity", req.Quantity).
		Str("price", req.Price).
		Msg("Request received")

	// Get the order book
	orderBook, info, err := s.manager.GetOrderBook(ctx, req.OrderBookName)
	if err != nil {
		if err == ErrOrderBookNotFound {
			return nil, status.Errorf(codes.NotFound, "order book %s not found", req.OrderBookName)
		}
		logger.Error().Err(err).Msg("Failed to get order book")
		return nil, status.Errorf(codes.Internal, "failed to get order book: %v", err)
	}

	// Parse decimal values
	quantity, err := fpdecimal.FromString(req.Quantity)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid quantity: %v", err)
	}

	// Convert side to core.Side
	side := core.Buy
	if req.Side == proto.OrderSide_SELL {
		side = core.Sell
	}

	// Create core order
	var order *core.Order
	var done *core.Done
	now := time.Now()

	switch req.OrderType {
	case proto.OrderType_MARKET:
		// Market orders don't use IsQuote based on proto definition
		order, err = core.NewMarketOrder(req.OrderId, side, quantity)
	case proto.OrderType_LIMIT:
		price, err := fpdecimal.FromString(req.Price)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid price format: %v", err)
		}
		tif := convertProtoTIFToCore(req.TimeInForce)
		order, err = core.NewLimitOrder(req.OrderId, side, quantity, price, tif, req.OcoId)
	case proto.OrderType_STOP:
		// Parse stop price
		stopPrice, err := fpdecimal.FromString(req.StopPrice)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid stop price: %v", err)
		}

		// Create a limit order with the stop price
		order, err = core.NewLimitOrder(req.OrderId, side, quantity, stopPrice, core.GTC, req.OcoId)
	case proto.OrderType_STOP_LIMIT:
		price, err := fpdecimal.FromString(req.Price)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid price: %v", err)
		}
		stopPrice, err := fpdecimal.FromString(req.StopPrice)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid stop price: %v", err)
		}

		// Create a stop limit order
		order, err = core.NewStopLimitOrder(req.OrderId, side, quantity, price, stopPrice, req.OcoId)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unsupported order type: %v", req.OrderType)
	}

	// Check for order creation errors (e.g., invalid quantity/price from core)
	if err != nil {
		if errors.Is(err, core.ErrInvalidQuantity) || errors.Is(err, core.ErrInvalidPrice) || errors.Is(err, core.ErrInvalidTif) {
			return nil, status.Errorf(codes.InvalidArgument, "order creation failed: %v", err)
		}
		// Handle other potential core errors as Internal
		logger.Error().Err(err).Msg("Internal error creating core order")
		return nil, status.Errorf(codes.Internal, "failed to create order: %v", err)
	}

	// Add nil check to prevent panic
	if order == nil {
		logger.Error().Msg("Order creation returned nil without error")
		return nil, status.Error(codes.Internal, "order creation failed: nil order")
	}

	// Process the order
	done, err = orderBook.Process(order)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to process order")
		if errors.Is(err, core.ErrOrderExists) {
			return nil, status.Errorf(codes.AlreadyExists, "order with ID %s already exists", req.OrderId)
		}
		return nil, status.Errorf(codes.Internal, "failed to process order: %v", err)
	}

	// Create order response
	resp := &proto.OrderResponse{
		OrderId:       req.OrderId,
		OrderBookName: req.OrderBookName,
		Side:          req.Side,
		Quantity:      req.Quantity,
		Price:         req.Price,
		OrderType:     req.OrderType,
		TimeInForce:   req.TimeInForce,
		StopPrice:     req.StopPrice,
		CreatedAt:     timestamppb.New(now),
		UpdatedAt:     timestamppb.New(now),
		OcoId:         req.OcoId,
	}

	// Get remaining quantity
	remainingQty := quantity
	filledQty := fpdecimal.Zero

	// Set filled quantity and status from Done object
	if done != nil {
		filledQty = done.Processed
		remainingQty = done.Left

		resp.FilledQuantity = filledQty.String()
		resp.RemainingQuantity = remainingQty.String()

		// Create fill records
		if len(done.Trades) > 0 {
			fills := make([]*proto.Fill, 0, len(done.Trades))
			for _, trade := range done.Trades {
				fills = append(fills, &proto.Fill{
					Price:     trade.Price.String(),
					Quantity:  trade.Quantity.String(),
					Timestamp: timestamppb.New(now),
				})
			}
			resp.Fills = fills
		}

		// Determine order status
		if filledQty.Equal(quantity) {
			resp.Status = proto.OrderStatus_FILLED
		} else if filledQty.Equal(fpdecimal.Zero) {
			resp.Status = proto.OrderStatus_OPEN
		} else {
			resp.Status = proto.OrderStatus_PARTIALLY_FILLED
		}
	} else {
		resp.Status = proto.OrderStatus_OPEN
		resp.RemainingQuantity = quantity.String()
		resp.FilledQuantity = "0"
	}

	// Increment order count
	s.manager.UpdateOrderBookInfo(ctx, req.OrderBookName, info.OrderCount+1)

	return resp, nil
}

// GetOrder retrieves information about a specific order
func (s *GRPCOrderBookService) GetOrder(ctx context.Context, req *proto.GetOrderRequest) (*proto.OrderResponse, error) {
	logger := logging.FromContext(ctx).With().
		Str("method", "GetOrder").
		Str("order_book", req.OrderBookName).
		Str("order_id", req.OrderId).
		Logger()

	logger.Debug().Msg("Request received")

	// Get the order book
	orderBook, _, err := s.manager.GetOrderBook(ctx, req.OrderBookName)
	if err != nil {
		if err == ErrOrderBookNotFound {
			return nil, status.Errorf(codes.NotFound, "order book %s not found", req.OrderBookName)
		}
		logger.Error().Err(err).Msg("Failed to get order book")
		return nil, status.Errorf(codes.Internal, "failed to get order book: %v", err)
	}

	// Get the order
	order := orderBook.GetOrder(req.OrderId)
	if order == nil {
		return nil, status.Errorf(codes.NotFound, "order %s not found", req.OrderId)
	}

	// Convert order side
	side := proto.OrderSide_BUY
	if order.Side() == core.Sell {
		side = proto.OrderSide_SELL
	}

	// Convert order type
	orderType := proto.OrderType_LIMIT
	if order.IsMarketOrder() {
		orderType = proto.OrderType_MARKET
	} else if order.IsStopOrder() {
		if order.IsLimitOrder() {
			orderType = proto.OrderType_STOP_LIMIT
		} else {
			orderType = proto.OrderType_STOP
		}
	}

	// Convert time in force
	timeInForce := proto.TimeInForce_GTC
	if !order.IsMarketOrder() && !order.IsStopOrder() {
		switch order.TIF() {
		case core.GTC:
			timeInForce = proto.TimeInForce_GTC
		case core.IOC:
			timeInForce = proto.TimeInForce_IOC
		case core.FOK:
			timeInForce = proto.TimeInForce_FOK
		}
	}

	// Create response
	resp := &proto.OrderResponse{
		OrderId:           req.OrderId,
		OrderBookName:     req.OrderBookName,
		Side:              side,
		Quantity:          order.OriginalQty().String(),
		RemainingQuantity: order.Quantity().String(),
		OrderType:         orderType,
		TimeInForce:       timeInForce,
		CreatedAt:         timestamppb.New(time.Now()), // We don't track creation time in the core lib
		UpdatedAt:         timestamppb.New(time.Now()),
		OcoId:             order.OCO(),
	}

	// Add price if it's a limit order
	if order.IsLimitOrder() {
		resp.Price = order.Price().String()
	}

	// Add stop price if it's a stop order
	if order.IsStopOrder() {
		resp.StopPrice = order.StopPrice().String()
	}

	// Calculate filled quantity and status
	filledQty := order.OriginalQty().Sub(order.Quantity())
	resp.FilledQuantity = filledQty.String()

	// Determine order status
	if filledQty.Equal(order.OriginalQty()) {
		resp.Status = proto.OrderStatus_FILLED
	} else if filledQty.Equal(fpdecimal.Zero) {
		resp.Status = proto.OrderStatus_OPEN
	} else {
		resp.Status = proto.OrderStatus_PARTIALLY_FILLED
	}

	return resp, nil
}

// CancelOrder cancels an order in the specified order book
func (s *GRPCOrderBookService) CancelOrder(ctx context.Context, req *proto.CancelOrderRequest) (*emptypb.Empty, error) {
	logger := logging.FromContext(ctx).With().
		Str("method", "CancelOrder").
		Str("order_book", req.OrderBookName).
		Str("order_id", req.OrderId).
		Logger()

	logger.Debug().Msg("Request received")

	// Get the order book
	orderBook, _, err := s.manager.GetOrderBook(ctx, req.OrderBookName)
	if err != nil {
		if err == ErrOrderBookNotFound {
			return nil, status.Errorf(codes.NotFound, "order book %s not found", req.OrderBookName)
		}
		logger.Error().Err(err).Msg("Failed to get order book")
		return nil, status.Errorf(codes.Internal, "failed to get order book: %v", err)
	}

	// Cancel the order
	canceledOrder := orderBook.CancelOrder(req.OrderId)
	if canceledOrder == nil {
		return nil, status.Errorf(codes.NotFound, "order %s not found", req.OrderId)
	}

	logger.Info().Str("order_id", req.OrderId).Msg("Order canceled")
	return &emptypb.Empty{}, nil
}

// GetOrderBookState retrieves the current state of an order book
func (s *GRPCOrderBookService) GetOrderBookState(ctx context.Context, req *proto.GetOrderBookStateRequest) (*proto.OrderBookStateResponse, error) {
	logger := logging.FromContext(ctx).With().
		Str("method", "GetOrderBookState").
		Str("order_book", req.Name).
		Int32("depth", req.Depth).
		Logger()

	logger.Debug().Msg("Request received")

	// Get the order book
	orderBook, _, err := s.manager.GetOrderBook(ctx, req.Name)
	if err != nil {
		if err == ErrOrderBookNotFound {
			return nil, status.Errorf(codes.NotFound, "order book %s not found", req.Name)
		}
		logger.Error().Err(err).Msg("Failed to get order book")
		return nil, status.Errorf(codes.Internal, "failed to get order book: %v", err)
	}

	// Set default depth if not specified
	depth := int(req.Depth)
	if depth <= 0 {
		depth = 10 // Default depth
	}

	// Create response
	response := &proto.OrderBookStateResponse{
		Name:      req.Name,
		Timestamp: timestamppb.New(time.Now()),
		Bids:      []*proto.PriceLevel{},
		Asks:      []*proto.PriceLevel{},
	}

	// Get bids
	if bids := orderBook.GetBids(); bids != nil {
		if bidSide, ok := bids.(*memory.OrderSide); ok {
			prices := bidSide.Prices()
			for i := 0; i < len(prices) && i < depth; i++ {
				price := prices[i]
				orders := bidSide.Orders(price)
				totalQuantity := fpdecimal.Zero
				for _, order := range orders {
					totalQuantity = totalQuantity.Add(order.Quantity())
				}
				response.Bids = append(response.Bids, &proto.PriceLevel{
					Price:         price.String(),
					TotalQuantity: totalQuantity.String(),
					OrderCount:    int32(len(orders)),
				})
			}
		}
	}

	// Get asks
	if asks := orderBook.GetAsks(); asks != nil {
		if askSide, ok := asks.(*memory.OrderSide); ok {
			prices := askSide.Prices()
			for i := 0; i < len(prices) && i < depth; i++ {
				price := prices[i]
				orders := askSide.Orders(price)
				totalQuantity := fpdecimal.Zero
				for _, order := range orders {
					totalQuantity = totalQuantity.Add(order.Quantity())
				}
				response.Asks = append(response.Asks, &proto.PriceLevel{
					Price:         price.String(),
					TotalQuantity: totalQuantity.String(),
					OrderCount:    int32(len(orders)),
				})
			}
		}
	}

	logger.Info().Msg("Returning order book state")
	return response, nil
}
