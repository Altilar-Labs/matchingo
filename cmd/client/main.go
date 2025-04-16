package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"text/tabwriter"

	"github.com/erain9/matchingo/pkg/api/proto"
	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	serverAddr = flag.String("addr", "localhost:50051", "The server address in the format host:port")
)

func main() {
	// Configure zerolog
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to the gRPC server
	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to server")
	}
	defer conn.Close()

	// Create a client
	client := proto.NewOrderBookServiceClient(conn)

	// Check if we have enough arguments
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Get the command
	command := os.Args[1]

	// Remove the command from os.Args to make flag parsing work
	os.Args = append(os.Args[:1], os.Args[2:]...)

	// Execute the appropriate command
	switch command {
	case "create-book":
		createOrderBook(ctx, client)
	case "get-book":
		getOrderBook(ctx, client)
	case "list-books":
		listOrderBooks(ctx, client)
	case "delete-book":
		deleteOrderBook(ctx, client)
	case "create-order":
		createOrder(ctx, client, os.Args[1:]...)
	case "get-order":
		if len(os.Args) < 3 {
			fmt.Println("Usage: get-order <book> <id>")
			os.Exit(1)
		}
		bookName := os.Args[1]
		orderID := os.Args[2]
		getOrder(ctx, client, bookName, orderID)
	case "cancel-order":
		if len(os.Args) < 3 {
			fmt.Println("Usage: cancel-order <book> <id>")
			os.Exit(1)
		}
		bookName := os.Args[1]
		orderID := os.Args[2]
		cancelOrder(ctx, client, bookName, orderID)
	case "get-state":
		if len(os.Args) < 2 {
			fmt.Println("Usage: get-state <book>")
			os.Exit(1)
		}
		bookName := os.Args[1]
		getOrderBookState(ctx, client, bookName)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func createOrderBook(ctx context.Context, client proto.OrderBookServiceClient) {
	// Parse command line arguments
	bookName := flag.String("name", "default", "Order book name")
	backendType := flag.String("backend", "memory", "Backend type (memory or redis)")
	flag.Parse()

	// Convert backend type string to enum
	var backendEnum proto.BackendType
	switch *backendType {
	case "memory":
		backendEnum = proto.BackendType_MEMORY
	case "redis":
		backendEnum = proto.BackendType_REDIS
	default:
		log.Fatal().Str("backend", *backendType).Msg("Unsupported backend type")
	}

	// Create options map for Redis if needed
	options := make(map[string]string)
	if backendEnum == proto.BackendType_REDIS {
		options["addr"] = "localhost:6379"
		options["db"] = "0"
		options["prefix"] = *bookName
	}

	// Create request
	req := &proto.CreateOrderBookRequest{
		Name:        *bookName,
		BackendType: backendEnum,
		Options:     options,
	}

	// Call RPC
	resp, err := client.CreateOrderBook(ctx, req)
	if err != nil {
		log.Fatal().Err(err).Msg("CreateOrderBook failed")
	}

	// Print response
	log.Info().
		Str("name", resp.Name).
		Str("backend", resp.BackendType.String()).
		Time("created_at", resp.CreatedAt.AsTime()).
		Msg("Created order book")
}

func getOrderBook(ctx context.Context, client proto.OrderBookServiceClient) {
	// Parse command line arguments
	bookName := flag.String("name", "default", "Order book name")
	flag.Parse()

	// Create request
	req := &proto.GetOrderBookRequest{
		Name: *bookName,
	}

	// Call RPC
	resp, err := client.GetOrderBook(ctx, req)
	if err != nil {
		log.Fatal().Err(err).Msg("GetOrderBook failed")
	}

	// Print response
	log.Info().
		Str("name", resp.Name).
		Str("backend", resp.BackendType.String()).
		Time("created_at", resp.CreatedAt.AsTime()).
		Int("order_count", int(resp.OrderCount)).
		Msg("Retrieved order book")
}

func listOrderBooks(ctx context.Context, client proto.OrderBookServiceClient) {
	// Parse command line arguments
	limit := flag.Int("limit", 10, "Maximum number of order books to list")
	offset := flag.Int("offset", 0, "Offset for pagination")
	flag.Parse()

	// Create request
	req := &proto.ListOrderBooksRequest{
		Limit:  int32(*limit),
		Offset: int32(*offset),
	}

	// Call RPC
	resp, err := client.ListOrderBooks(ctx, req)
	if err != nil {
		log.Fatal().Err(err).Msg("ListOrderBooks failed")
	}

	// Print response
	log.Info().
		Int("total", int(resp.Total)).
		Int("showing", len(resp.OrderBooks)).
		Int("offset", *offset).
		Msg("Listed order books")

	for i, book := range resp.OrderBooks {
		log.Info().
			Int("index", i+1).
			Str("name", book.Name).
			Str("backend", book.BackendType.String()).
			Time("created_at", book.CreatedAt.AsTime()).
			Int("order_count", int(book.OrderCount)).
			Msg("Order book")
	}
}

func deleteOrderBook(ctx context.Context, client proto.OrderBookServiceClient) {
	// Parse command line arguments
	bookName := flag.String("name", "default", "Order book name")
	flag.Parse()

	// Create request
	req := &proto.DeleteOrderBookRequest{
		Name: *bookName,
	}

	// Call RPC
	_, err := client.DeleteOrderBook(ctx, req)
	if err != nil {
		log.Fatal().Err(err).Msg("DeleteOrderBook failed")
	}

	log.Info().Str("name", *bookName).Msg("Order book deleted")
}

func createOrder(ctx context.Context, client proto.OrderBookServiceClient, args ...string) {
	// Define flags
	bookName := flag.String("book", "", "Order book name")
	orderID := flag.String("id", "", "Order ID")
	side := flag.String("side", "", "Order side (BUY/SELL)")
	orderType := flag.String("type", "", "Order type (MARKET/LIMIT/STOP/STOP_LIMIT)")
	quantity := flag.String("qty", "", "Order quantity")
	price := flag.String("price", "", "Order price")
	userAddress := flag.String("user", "", "User's wallet address")
	flag.Parse()

	// If no flags are set, use positional arguments
	if *bookName == "" && len(args) >= 7 {
		bookName = &args[0]
		side = &args[1]
		orderType = &args[2]
		quantity = &args[3]
		price = &args[4]
		orderID = &args[5]
		userAddress = &args[6]
	}

	// Validate required fields
	if *bookName == "" || *orderID == "" || *side == "" || *orderType == "" || *quantity == "" || *userAddress == "" {
		fmt.Println("Usage: create-order <book> <side> <type> <quantity> <price> <id> <user_address>")
		fmt.Println("   or: create-order --book=<name> --id=<id> --side=<side> --type=<type> --qty=<quantity> --price=<price> --user=<user_address>")
		os.Exit(1)
	}

	// Convert side string to enum
	var sideEnum proto.OrderSide
	switch strings.ToUpper(*side) {
	case "BUY":
		sideEnum = proto.OrderSide_BUY
	case "SELL":
		sideEnum = proto.OrderSide_SELL
	default:
		log.Fatal().Str("side", *side).Msg("Unsupported side")
	}

	// Convert order type string to enum
	var typeEnum proto.OrderType
	switch strings.ToUpper(*orderType) {
	case "MARKET":
		typeEnum = proto.OrderType_MARKET
	case "LIMIT":
		typeEnum = proto.OrderType_LIMIT
	case "STOP":
		typeEnum = proto.OrderType_STOP
	case "STOP_LIMIT":
		typeEnum = proto.OrderType_STOP_LIMIT
	default:
		log.Fatal().Str("type", *orderType).Msg("Unsupported order type")
	}

	// Create request
	req := &proto.CreateOrderRequest{
		OrderBookName: *bookName,
		OrderId:       *orderID,
		Side:          sideEnum,
		OrderType:     typeEnum,
		Quantity:      *quantity,
		Price:         *price,
		TimeInForce:   proto.TimeInForce_GTC,
		UserAddress:   *userAddress,
	}

	// Call RPC
	resp, err := client.CreateOrder(ctx, req)
	if err != nil {
		log.Fatal().Err(err).Msg("CreateOrder failed")
	}

	// Print response
	log.Info().
		Str("order_id", resp.OrderId).
		Str("status", resp.Status.String()).
		Str("filled_quantity", resp.FilledQuantity).
		Str("remaining_quantity", resp.RemainingQuantity).
		Msg("Created order")

	if len(resp.Fills) > 0 {
		for i, fill := range resp.Fills {
			log.Info().
				Int("index", i+1).
				Str("quantity", fill.Quantity).
				Str("price", fill.Price).
				Time("timestamp", fill.Timestamp.AsTime()).
				Msg("Fill")
		}
	}
}

func getOrder(ctx context.Context, client proto.OrderBookServiceClient, bookName, orderID string) {
	// Create request
	req := &proto.GetOrderRequest{
		OrderBookName: bookName,
		OrderId:       orderID,
	}

	// Call RPC
	resp, err := client.GetOrder(ctx, req)
	if err != nil {
		log.Fatal().Err(err).Msg("GetOrder failed")
	}

	// Print response
	log.Info().
		Str("order_id", resp.OrderId).
		Str("book", resp.OrderBookName).
		Str("side", resp.Side.String()).
		Str("type", resp.OrderType.String()).
		Str("quantity", resp.Quantity).
		Str("price", resp.Price).
		Str("time_in_force", resp.TimeInForce.String()).
		Str("status", resp.Status.String()).
		Str("filled_quantity", resp.FilledQuantity).
		Str("remaining_quantity", resp.RemainingQuantity).
		Time("created_at", resp.CreatedAt.AsTime()).
		Time("updated_at", resp.UpdatedAt.AsTime()).
		Msg("Retrieved order")

	if resp.StopPrice != "" {
		log.Info().Str("stop_price", resp.StopPrice).Msg("Stop price")
	}
}

func cancelOrder(ctx context.Context, client proto.OrderBookServiceClient, bookName, orderID string) {
	// Create request
	req := &proto.CancelOrderRequest{
		OrderBookName: bookName,
		OrderId:       orderID,
	}

	// Call RPC
	_, err := client.CancelOrder(ctx, req)
	if err != nil {
		log.Fatal().Err(err).Msg("CancelOrder failed")
	}

	log.Info().Str("order_id", orderID).Msg("Order canceled")
}

func getOrderBookState(ctx context.Context, client proto.OrderBookServiceClient, name string) error {
	color.NoColor = false
	cyan := color.New(color.FgCyan).SprintfFunc()
	red := color.New(color.FgRed).SprintfFunc()
	green := color.New(color.FgGreen).SprintfFunc()

	req := &proto.GetOrderBookStateRequest{
		Name: name,
	}

	resp, err := client.GetOrderBookState(ctx, req)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight)

	// Print headers with consistent spacing
	fmt.Fprintf(w, "%15s|%15s|%15s|%15s|%s\n",
		cyan("Price"),
		cyan("Quantity"),
		cyan("Orders"),
		cyan("Address"),
		cyan("Side"))

	// Print separator with matching column widths
	fmt.Fprintf(w, "%15s|%15s|%15s|%15s|%s\n",
		"---------------",
		"---------------",
		"---------------",
		"---------------",
		"----")

	// Print asks (sells)
	for _, level := range resp.Asks {
		price, _ := strconv.ParseFloat(level.Price, 64)
		qty, _ := strconv.ParseFloat(level.TotalQuantity, 64)
		fmt.Fprintf(w, "%15.3f|%15.3f|%15d|%15s|%s\n",
			price,
			qty,
			level.OrderCount,
			level.UserAddress,
			red("ASK"))
	}

	// Print separator between asks and bids
	fmt.Fprintf(w, "%15s|%15s|%15s|%15s|%s\n",
		"---------------",
		"---------------",
		"---------------",
		"---------------",
		"----")

	// Print bids (buys)
	for _, level := range resp.Bids {
		price, _ := strconv.ParseFloat(level.Price, 64)
		qty, _ := strconv.ParseFloat(level.TotalQuantity, 64)
		fmt.Fprintf(w, "%15.3f|%15.3f|%15d|%15s|%s\n",
			price,
			qty,
			level.OrderCount,
			level.UserAddress,
			green("BID"))
	}

	return w.Flush()
}

// Helper function to parse float strings safely
func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  create-book <name> [--backend=memory|redis]")
	fmt.Println("  get-book <name>")
	fmt.Println("  list-books [--limit=N] [--offset=N]")
	fmt.Println("  delete-book <name>")
	fmt.Println("  create-order <book> <side> <type> <quantity> <price> <id> <user_address>")
	fmt.Println("  get-order <book> <id>")
	fmt.Println("  cancel-order <book> <id>")
	fmt.Println("  get-state <book>")
	fmt.Println("\nExamples:")
	fmt.Println("  create-book mybook --backend=memory")
	fmt.Println("  create-order default SELL LIMIT 0.5 100.0 sell1 0x1234567890123456789012345678901234567890")
	fmt.Println("  create-order default BUY MARKET 1.0 0.0 buy1 0x1234567890123456789012345678901234567890")
	fmt.Println("  get-order default sell1")
	fmt.Println("  cancel-order default sell1")
	fmt.Println("  get-state default")
}
