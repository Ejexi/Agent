package memory

import "sync"

// represents a single message in the conversation
type Message struct {
	Role    string // "user", "assistant", or "system"
	Content string
}

// manages conversation history
type Memory struct {
	messages    []Message
	maxMessages int
	mu          sync.RWMutex //protection from (race condition)
}

// New creates a new Memory instance and reutnr address
// maxMessages: how many messages to keep
func New(maxMessages int) *Memory {
	return &Memory{
		messages:    make([]Message, 0), //store messages slice
		maxMessages: maxMessages,
	}
}

//AddMessage adds a new message to history reciver
func (m *Memory) AddMessage(msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	//add the message
	m.messages = append(m.messages, msg)

	// Keep only last N message and remove the old
	if len(m.messages) > m.maxMessages {
		m.messages = m.messages[len(m.messages)-m.maxMessages:]
	}
}

// GetHistory returns all messages
func (m *Memory) GetHistory() []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modification
	history := make([]Message, len(m.messages))
	copy(history, m.messages)
	return history
}

func (m *Memory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = make([]Message, 0)
}

// Count returns the number of messages
func (m *Memory) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.messages)
}
