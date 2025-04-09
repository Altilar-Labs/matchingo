package queue

import (
	"fmt"

	"github.com/IBM/sarama"
	orderbookpb "github.com/erain9/matchingo/pkg/api/proto"
	"github.com/erain9/matchingo/pkg/messaging"
	"google.golang.org/protobuf/proto"
)

const (
	brokerList = "localhost:9092"
	topic      = "test-msg-queue"
	maxRetry   = 5
)

// QueueMessageSender implements the MessageSender interface
// for sending messages to Kafka
type QueueMessageSender struct{}

// SendDoneMessage sends the DoneMessage to the Kafka queue
func (q *QueueMessageSender) SendDoneMessage(done *messaging.DoneMessage) error {
	// Convert to proto message
	protoMsg := &orderbookpb.DoneMessage{
		OrderId:           done.OrderID,
		ExecutedQuantity:  done.ExecutedQty,
		RemainingQuantity: done.RemainingQty,
	}

	// Convert trades to proto format
	if len(done.Trades) > 0 {
		protoMsg.Trades = make([]*orderbookpb.Trade, 0, len(done.Trades))
		for _, trade := range done.Trades {
			protoMsg.Trades = append(protoMsg.Trades, &orderbookpb.Trade{
				Price:    trade.Price,
				Quantity: trade.Quantity,
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
		Topic: topic,
		Value: sarama.ByteEncoder(messageBytes),
	}

	// Send the message to Kafka
	producer, err := sarama.NewSyncProducer([]string{brokerList}, nil)
	if err != nil {
		return fmt.Errorf("failed to create Kafka producer: %v", err)
	}
	defer producer.Close()

	_, _, err = producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to send message to Kafka: %v", err)
	}

	return nil
}
