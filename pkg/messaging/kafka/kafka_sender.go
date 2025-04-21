package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/erain9/matchingo/pkg/messaging"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// KafkaMessageSender implements MessageSender using Kafka
type KafkaMessageSender struct {
	writer     *kafka.Writer
	topic      string
	propagator propagation.TextMapPropagator
}

// NewKafkaMessageSender creates a new Kafka message sender
func NewKafkaMessageSender(brokerAddr, topic string) (*KafkaMessageSender, error) {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokerAddr),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
	}

	return &KafkaMessageSender{
		writer:     writer,
		topic:      topic,
		propagator: otel.GetTextMapPropagator(),
	}, nil
}

// kafkaHeadersCarrier implements TextMapCarrier for Kafka message headers
type kafkaHeadersCarrier []kafka.Header

func (c *kafkaHeadersCarrier) Get(key string) string {
	for _, h := range *c {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *kafkaHeadersCarrier) Set(key string, value string) {
	*c = append(*c, kafka.Header{
		Key:   key,
		Value: []byte(value),
	})
}

func (c *kafkaHeadersCarrier) Keys() []string {
	out := make([]string, len(*c))
	for i, h := range *c {
		out[i] = h.Key
	}
	return out
}

// SendDoneMessage sends a done message to Kafka
func (k *KafkaMessageSender) SendDoneMessage(ctx context.Context, done *messaging.DoneMessage) error {
	data, err := json.Marshal(done)
	if err != nil {
		return fmt.Errorf("failed to marshal done message: %w", err)
	}

	// Create headers carrier and inject trace context
	headers := make(kafkaHeadersCarrier, 0)
	k.propagator.Inject(ctx, &headers)

	// Create a Kafka message with trace context headers
	msg := kafka.Message{
		Key:     []byte(done.OrderID),
		Value:   data,
		Time:    time.Now(),
		Headers: []kafka.Header(headers),
	}

	// Create timeout context while preserving parent context
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Send the message
	err = k.writer.WriteMessages(timeoutCtx, msg)
	if err != nil {
		return fmt.Errorf("failed to send message to Kafka: %w", err)
	}

	return nil
}

// Close closes the Kafka writer
func (k *KafkaMessageSender) Close() error {
	return k.writer.Close()
}
