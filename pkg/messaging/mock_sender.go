package messaging

import (
	"sync"
)

// MockMessageSender implements the MessageSender interface for testing purposes.
// It captures sent messages instead of sending them to a real queue.
type MockMessageSender struct {
	mu           sync.Mutex
	SentMessages []*DoneMessage
	SendError    error // Optional error to return on SendDoneMessage
}

// NewMockMessageSender creates a new mock sender.
func NewMockMessageSender() *MockMessageSender {
	return &MockMessageSender{
		SentMessages: make([]*DoneMessage, 0),
	}
}

// SendDoneMessage captures the message and returns an optional pre-configured error.
func (m *MockMessageSender) SendDoneMessage(done *DoneMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.SendError != nil {
		return m.SendError
	}

	m.SentMessages = append(m.SentMessages, done)
	return nil
}

// GetSentMessages returns a copy of the captured messages.
func (m *MockMessageSender) GetSentMessages() []*DoneMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return a copy to prevent race conditions if the caller modifies the slice
	msgsCopy := make([]*DoneMessage, len(m.SentMessages))
	copy(msgsCopy, m.SentMessages)
	return msgsCopy
}

// ClearSentMessages removes all captured messages.
func (m *MockMessageSender) ClearSentMessages() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SentMessages = make([]*DoneMessage, 0)
}

// SetSendError allows configuring an error to be returned by SendDoneMessage.
func (m *MockMessageSender) SetSendError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SendError = err
}
