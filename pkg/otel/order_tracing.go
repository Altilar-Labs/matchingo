package otel

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	// Span names
	SpanCreateOrder  = "create_order"
	SpanProcessOrder = "process_order"
	SpanMatchOrder   = "match_order"
	SpanSendToKafka  = "send_to_kafka"

	// Attribute keys
	AttributeOrderID           = "order.id"
	AttributeOrderSide         = "order.side"
	AttributeOrderType         = "order.type"
	AttributeOrderQuantity     = "order.quantity"
	AttributeOrderPrice        = "order.price"
	AttributeOrderStatus       = "order.status"
	AttributeExecutedQuantity  = "order.executed_quantity"
	AttributeRemainingQuantity = "order.remaining_quantity"
	AttributeTradeCount        = "trade.count"
)

// StartOrderSpan starts a new span for order processing
func StartOrderSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	var tracer trace.Tracer

	// Use appropriate tracer based on the span name
	switch name {
	case SpanCreateOrder:
		tracer = GetOrderServiceTracer()
	case SpanProcessOrder, SpanMatchOrder, SpanSendToKafka:
		tracer = GetMatchingEngineTracer()
	default:
		tracer = GetOrderServiceTracer()
	}

	if tracer == nil {
		return ctx, nil
	}
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// AddAttributes adds attributes to a span
func AddAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	if span == nil {
		return
	}
	span.SetAttributes(attrs...)
}
