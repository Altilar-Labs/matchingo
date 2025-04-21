package queue

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	orderbookpb "github.com/erain9/matchingo/pkg/api/proto"
	"github.com/erain9/matchingo/pkg/messaging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/protobuf/proto"
)

var (
	brokerList      = "localhost:9092"
	topic           = "test-msg-queue"
	newSyncProducer = sarama.NewSyncProducer
)

// SetBrokerList sets the Kafka broker address
func SetBrokerList(broker string) {
	brokerList = broker
}

// SetTopic sets the Kafka topic name
func SetTopic(topicName string) {
	topic = topicName
}

// QueueMessageSender implements the MessageSender interface
// for sending messages to Kafka
type QueueMessageSender struct {
	producer   sarama.SyncProducer
	propagator propagation.TextMapPropagator
}

// NewQueueMessageSender creates a new QueueMessageSender with an initialized Kafka producer
func NewQueueMessageSender() (*QueueMessageSender, error) {
	producer, err := newSyncProducer([]string{brokerList}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %v", err)
	}

	return &QueueMessageSender{
		producer:   producer,
		propagator: otel.GetTextMapPropagator(),
	}, nil
}

// Close closes the Kafka producer
func (q *QueueMessageSender) Close() error {
	if q.producer != nil {
		return q.producer.Close()
	}
	return nil
}

// SendDoneMessage sends the DoneMessage to the Kafka queue
func (q *QueueMessageSender) SendDoneMessage(ctx context.Context, done *messaging.DoneMessage) error {
	// Convert to proto message
	protoMsg := &orderbookpb.DoneMessage{
		OrderId:           done.OrderID,
		ExecutedQuantity:  done.ExecutedQty,
		RemainingQuantity: done.RemainingQty,
		Canceled:          done.Canceled,
		Activated:         done.Activated,
		Stored:            done.Stored,
		Quantity:          done.Quantity,
		Processed:         done.Processed,
		Left:              done.Left,
		UserAddress:       done.UserAddress,
	}

	// Convert trades to proto format
	if len(done.Trades) > 0 {
		protoMsg.Trades = make([]*orderbookpb.Trade, 0, len(done.Trades))
		for _, trade := range done.Trades {
			protoMsg.Trades = append(protoMsg.Trades, &orderbookpb.Trade{
				OrderId:     trade.OrderID,
				Role:        trade.Role,
				Price:       trade.Price,
				Quantity:    trade.Quantity,
				IsQuote:     trade.IsQuote,
				UserAddress: trade.UserAddress,
			})
		}
	}

	// Serialize to protobuf
	messageBytes, err := proto.Marshal(protoMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal done message: %v", err)
	}

	// Create a Kafka producer message
	msg := &sarama.ProducerMessage{
		Topic:   topic,
		Value:   sarama.ByteEncoder(messageBytes),
		Headers: []sarama.RecordHeader{},
	}

	// Inject OpenTelemetry context into headers
	carrier := propagation.MapCarrier{}
	q.propagator.Inject(ctx, carrier)
	for k, v := range carrier {
		msg.Headers = append(msg.Headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}

	// Send the message to Kafka using the existing producer
	_, _, err = q.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to send message to Kafka: %v", err)
	}

	return nil
}

// QueueMessageConsumer implements the MessageConsumer interface
// for consuming messages from Kafka
type QueueMessageConsumer struct {
	consumer sarama.Consumer
	done     chan struct{}
}

// NewQueueMessageConsumer creates a new Kafka consumer
func NewQueueMessageConsumer() (*QueueMessageConsumer, error) {
	consumer, err := sarama.NewConsumer([]string{brokerList}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka consumer: %v", err)
	}

	return &QueueMessageConsumer{
		consumer: consumer,
		done:     make(chan struct{}),
	}, nil
}

// Close closes the Kafka consumer
func (q *QueueMessageConsumer) Close() error {
	close(q.done)
	return q.consumer.Close()
}

// ConsumeDoneMessages starts consuming DoneMessages from Kafka
func (q *QueueMessageConsumer) ConsumeDoneMessages(handler func(*messaging.DoneMessage) error) error {
	partitionConsumer, err := q.consumer.ConsumePartition(topic, 0, sarama.OffsetNewest)
	if err != nil {
		return fmt.Errorf("failed to start consumer for partition: %v", err)
	}
	defer partitionConsumer.Close()

	for {
		select {
		case msg := <-partitionConsumer.Messages():
			// Deserialize the protobuf message
			protoMsg := &orderbookpb.DoneMessage{}
			if err := proto.Unmarshal(msg.Value, protoMsg); err != nil {
				fmt.Printf("Failed to unmarshal message: %v\n", err)
				continue
			}

			// Convert to DoneMessage
			doneMsg := &messaging.DoneMessage{
				OrderID:      protoMsg.OrderId,
				ExecutedQty:  protoMsg.ExecutedQuantity,
				RemainingQty: protoMsg.RemainingQuantity,
				Canceled:     protoMsg.Canceled,
				Activated:    protoMsg.Activated,
				Stored:       protoMsg.Stored,
				Quantity:     protoMsg.Quantity,
				Processed:    protoMsg.Processed,
				Left:         protoMsg.Left,
				UserAddress:  protoMsg.UserAddress,
			}

			// Convert trades
			if len(protoMsg.Trades) > 0 {
				doneMsg.Trades = make([]messaging.Trade, 0, len(protoMsg.Trades))
				for _, trade := range protoMsg.Trades {
					doneMsg.Trades = append(doneMsg.Trades, messaging.Trade{
						OrderID:     trade.OrderId,
						Role:        trade.Role,
						Price:       trade.Price,
						Quantity:    trade.Quantity,
						IsQuote:     trade.IsQuote,
						UserAddress: trade.UserAddress,
					})
				}
			}

			// Process the message
			if err := handler(doneMsg); err != nil {
				fmt.Printf("Failed to process message: %v\n", err)
			}

		case <-q.done:
			return nil
		}
	}
}
