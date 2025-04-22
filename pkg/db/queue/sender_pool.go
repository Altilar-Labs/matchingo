package queue

import (
	"context"
	"fmt"
	"sync"

	"github.com/erain9/matchingo/pkg/messaging"
)

var (
	senderPool   chan messaging.MessageSender
	poolInitOnce sync.Once
	maxPoolSize  = 32 // Pool size optimized for 100k msgs/sec
)

// initSenderPool initializes the sender pool
func initSenderPool() {
	poolInitOnce.Do(func() {
		senderPool = make(chan messaging.MessageSender, maxPoolSize)
		// Pre-populate the entire pool
		for i := 0; i < maxPoolSize; i++ {
			sender, err := NewQueueMessageSender()
			if err != nil {
				fmt.Printf("Error creating sender: %v\n", err)
				continue
			}
			if sender != nil {
				senderPool <- sender
			}
		}
	})
}

// GetSender gets a sender from the pool
func GetSender() messaging.MessageSender {
	initSenderPool()

	// Simple non-blocking get from pool
	select {
	case sender := <-senderPool:
		return sender
	default:
		// If pool is empty, something is wrong - log and return nil
		fmt.Printf("Warning: sender pool is empty\n")
		return nil
	}
}

// ReturnSender returns a sender to the pool
func ReturnSender(sender messaging.MessageSender) {
	if sender == nil {
		return
	}

	// Simple non-blocking return to pool
	select {
	case senderPool <- sender:
		// Successfully returned to pool
	default:
		// If pool is full, something is wrong - log and close
		fmt.Printf("Warning: sender pool is full\n")
		_ = sender.Close()
	}
}

// SendMessage sends a message using a pooled sender
func SendMessage(ctx context.Context, msg *messaging.DoneMessage) error {
	// Get a sender from the pool
	sender := GetSender()
	if sender == nil {
		return fmt.Errorf("failed to get message sender from pool")
	}
	defer ReturnSender(sender)

	// Send the message
	err := sender.SendDoneMessage(ctx, msg)
	if err != nil {
		fmt.Printf("Error sending message: %v\n", err)
		// If we get a connection error, don't return this sender to the pool
		_ = sender.Close()
		return err
	}

	return nil
}
