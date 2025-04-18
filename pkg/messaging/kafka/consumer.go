package kafka

import (
	"context"

	"github.com/erain9/matchingo/pkg/db/queue"
	"github.com/erain9/matchingo/pkg/messaging"
	"github.com/rs/zerolog"
)

// SetupConsumer initializes and starts the Kafka consumer for processing done messages
func SetupConsumer(ctx context.Context, logger zerolog.Logger) (*queue.QueueMessageConsumer, error) {
	kafkaConsumer, err := queue.NewQueueMessageConsumer()
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to create Kafka consumer - continuing without Kafka support")
		return nil, err
	}

	// Start Kafka consumer in a goroutine
	go func() {
		logger.Info().Msg("Starting Kafka consumer")
		err := kafkaConsumer.ConsumeDoneMessages(func(msg *messaging.DoneMessage) error {
			logger.Info().
				Str("order_id", msg.OrderID).
				Str("executed_qty", msg.ExecutedQty).
				Str("remaining_qty", msg.RemainingQty).
				Strs("canceled", msg.Canceled).
				Strs("activated", msg.Activated).
				Bool("stored", msg.Stored).
				Str("quantity", msg.Quantity).
				Str("processed", msg.Processed).
				Str("left", msg.Left).
				Interface("trades", msg.Trades).
				Msg("Received done message")
			return nil
		})
		if err != nil {
			logger.Error().Err(err).Msg("Kafka consumer error")
		}
	}()

	return kafkaConsumer, nil
}
