package integration

import (
	"context"
	"testing"

	"github.com/erain9/matchingo/pkg/api/proto"
	testutil "github.com/erain9/matchingo/test/utils"
	"github.com/stretchr/testify/require"
)

// TestServer_CreateOrderBook verifies order book creation
func TestServer_CreateOrderBook(t *testing.T) {
	testutil.RunIntegrationTest(t, func(redisAddr, kafkaAddr string) {
		// Setup client
		client, teardown := setupRealIntegrationTest(t, redisAddr, kafkaAddr)
		defer teardown()

		ctx := context.Background()
		bookName := "market-order-test-book"

		// 1. Create Order Book using Redis backend
		_, err := client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
			Name:        bookName,
			BackendType: proto.BackendType_REDIS,
		})
		require.NoError(t, err, "Failed to create order book")
	})
}

// TestServer_DeleteOrderBook verifies order book deletion
func TestServer_DeleteOrderBook(t *testing.T) {
	testutil.RunIntegrationTest(t, func(redisAddr, kafkaAddr string) {
		// Setup client
		client, teardown := setupRealIntegrationTest(t, redisAddr, kafkaAddr)
		defer teardown()

		ctx := context.Background()
		bookName := "stop-limit-test-book"

		// 1. Create Order Book using Redis backend
		_, err := client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
			Name:        bookName,
			BackendType: proto.BackendType_REDIS,
		})
		require.NoError(t, err, "Failed to create order book")
	})
}
