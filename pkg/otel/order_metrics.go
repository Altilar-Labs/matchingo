package otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	// orderBookMetrics holds the singleton instance
	orderBookMetrics *OrderBookMetrics
	// meter is the global meter for order book metrics
	meter = otel.GetMeterProvider().Meter(instrumentationName)
)

// OrderBookMetrics holds metrics for order book operations
type OrderBookMetrics struct {
	// Tracks the total number of matched orders by type (market, limit)
	matchedOrdersTotal metric.Int64Counter
}

// GetOrderBookMetrics returns the OrderBookMetrics singleton
func GetOrderBookMetrics() *OrderBookMetrics {
	if orderBookMetrics == nil {
		// Initialize metrics
		matchedOrdersTotal, err := meter.Int64Counter(
			"orderbook.matched_orders.total",
			metric.WithDescription("Total number of orders matched"),
			metric.WithUnit("{order}"),
		)
		if err != nil {
			return &OrderBookMetrics{}
		}

		orderBookMetrics = &OrderBookMetrics{
			matchedOrdersTotal: matchedOrdersTotal,
		}
	}

	return orderBookMetrics
}

// RecordMatchedOrders increments the matched orders counter
func (m *OrderBookMetrics) RecordMatchedOrders(ctx context.Context, orderType string, count int64) {
	if m.matchedOrdersTotal == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("order.type", orderType),
	}
	m.matchedOrdersTotal.Add(ctx, count, metric.WithAttributes(attrs...))
}
