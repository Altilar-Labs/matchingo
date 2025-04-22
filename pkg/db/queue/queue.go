package queue

import (
	"context"
	"fmt"
	"time"

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
	producer   sarama.AsyncProducer
	propagator propagation.TextMapPropagator
}

// NewQueueMessageSender creates a new QueueMessageSender with an initialized Kafka producer
func NewQueueMessageSender() (*QueueMessageSender, error) {
	config := sarama.NewConfig()

	// Maximum Performance Settings
	config.Producer.RequiredAcks = sarama.NoResponse        // Don't wait for any acks - fastest
	config.Producer.Compression = sarama.CompressionLZ4     // LZ4 compression - best balance of CPU/compression
	config.Producer.Partitioner = sarama.NewHashPartitioner // Hash partitioning for better distribution

	// Aggressive Buffer Settings for 100k msgs/sec
	config.Producer.Flush.Bytes = 8 * 1024 * 1024      // 8MB buffer
	config.Producer.Flush.Messages = 10000             // Flush every 10k messages
	config.Producer.Flush.Frequency = time.Millisecond // Or every 1ms
	config.Producer.Flush.MaxMessages = 10000          // Max messages per batch

	// Large Channel Buffers
	config.ChannelBufferSize = 256 * 1024 // 256K channel buffer size

	// Memory Management
	config.Producer.MaxMessageBytes = 1024 * 1024 // 1MB max message size
	config.Producer.Return.Successes = false      // Don't send success responses
	config.Producer.Return.Errors = true          // But do send errors

	// Network Optimization
	config.Net.MaxOpenRequests = 16 // More concurrent requests
	config.Net.DialTimeout = time.Second * 10
	config.Net.ReadTimeout = time.Second * 30
	config.Net.WriteTimeout = time.Second * 30
	config.Net.KeepAlive = time.Second * 60

	// Retry Settings
	config.Producer.Retry.Max = 0 // No retries - fail fast
	config.Producer.Retry.Backoff = time.Millisecond

	producer, err := sarama.NewAsyncProducer([]string{brokerList}, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %v", err)
	}

	// Start error handling goroutine
	go func() {
		for err := range producer.Errors() {
			fmt.Printf("Failed to send message: %v\n", err)
		}
	}()

	return &QueueMessageSender{
		producer:   producer,
		propagator: otel.GetTextMapPropagator(),
	}, nil
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

	// Send the message asynchronously
	select {
	case q.producer.Input() <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close closes the Kafka producer
func (q *QueueMessageSender) Close() error {
	if q.producer != nil {
		return q.producer.Close()
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
