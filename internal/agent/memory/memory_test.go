package memory

import "testing"

func TestMemory(t *testing.T) {
	mem := New(3)

	// Add messages
	mem.AddMessage(Message{Role: "user", Content: "msg1"})
	mem.AddMessage(Message{Role: "assistant", Content: "msg2"})
	mem.AddMessage(Message{Role: "user", Content: "msg3"})

	// Check count
	if mem.Count() != 3 {
		t.Errorf("Expected 3 messages, got %d", mem.Count())
	}

	// Add one more (should drop first one)
	mem.AddMessage(Message{Role: "assistant", Content: "msg4"})
	if mem.Count() != 3 {
		t.Errorf("Expected 3 messages after overflow, got %d", mem.Count())
	}

	// Check first message (msg1 was dropped)
	history := mem.GetHistory()
	if history[0].Content != "msg2" {
		t.Errorf("Expected first message to be msg2, got %s", history[0].Content)
	}
}

func TestMemoryClear(t *testing.T) {
	mem := New(5)
	mem.AddMessage(Message{Role: "user", Content: "test"})

	if mem.Count() != 1 {
		t.Errorf("Should have 1 message")
	}
	mem.Clear()
	
	if mem.Count() != 0 {
		t.Error("Should have 0 messages after clear")
	}
}
