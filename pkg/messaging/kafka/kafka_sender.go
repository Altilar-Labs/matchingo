package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/erain9/matchingo/pkg/messaging"
	"github.com/segmentio/kafka-go"
)

// KafkaMessageSender implements MessageSender using Kafka
type KafkaMessageSender struct {
	writer *kafka.Writer
	topic  string
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
		writer: writer,
		topic:  topic,
	}, nil
}

// SendDoneMessage sends a done message to Kafka
func (k *KafkaMessageSender) SendDoneMessage(done *messaging.DoneMessage) error {
	data, err := json.Marshal(done)
	if err != nil {
		return fmt.Errorf("failed to marshal done message: %w", err)
	}

	// Create a Kafka message
	msg := kafka.Message{
		Key:   []byte(done.OrderID),
		Value: data,
		Time:  time.Now(),
	}

	// Send the message
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = k.writer.WriteMessages(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to send message to Kafka: %w", err)
	}

	return nil
}

// Close closes the Kafka writer
func (k *KafkaMessageSender) Close() error {
	return k.writer.Close()
}
