package messaging

// MockMessageSender is a no-op implementation of MessageSender for testing.
type MockMessageSender struct{}

// NewMockMessageSender creates a new MockMessageSender.
func NewMockMessageSender() *MockMessageSender {
	return &MockMessageSender{}
}

// SendDoneMessage does nothing.
func (m *MockMessageSender) SendDoneMessage(done *DoneMessage) error {
	// No-op
	return nil
}

// Close does nothing.
func (m *MockMessageSender) Close() error {
	// No-op
	return nil
}

// Ensure MockMessageSender implements MessageSender
var _ MessageSender = (*MockMessageSender)(nil)
