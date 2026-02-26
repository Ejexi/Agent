package subagent

import (
	"sync"
	"sync/atomic"
	"time"

	sa "github.com/SecDuckOps/agent/internal/domain/subagent"
)

const defaultRingSize = 1024

// IndexedEvent wraps a SubagentEvent with a monotonic sequence ID.
type IndexedEvent struct {
	SeqID uint64           `json:"seq_id"`
	Event sa.SubagentEvent `json:"event"`
}

// EventLog provides durable, ordered event storage for a single session.
//   - Monotonic event IDs per session
//   - Ring buffer for efficient replay
//   - Fan-out to multiple SSE subscribers
//   - Thread-safe
type EventLog struct {
	sessionID string

	// Ring buffer
	ring     []IndexedEvent
	ringSize int
	head     int // next write position
	count    int // total items currently in buffer (≤ ringSize)
	nextSeq  atomic.Uint64

	// Fan-out subscribers
	subs       map[uint64]chan IndexedEvent
	subCounter atomic.Uint64

	mu     sync.RWMutex
	closed bool
}

// NewEventLog creates a new EventLog for the given session.
func NewEventLog(sessionID string) *EventLog {
	return NewEventLogWithSize(sessionID, defaultRingSize)
}

// NewEventLogWithSize creates an EventLog with a custom ring buffer size.
func NewEventLogWithSize(sessionID string, ringSize int) *EventLog {
	if ringSize <= 0 {
		ringSize = defaultRingSize
	}
	el := &EventLog{
		sessionID: sessionID,
		ring:      make([]IndexedEvent, ringSize),
		ringSize:  ringSize,
		subs:      make(map[uint64]chan IndexedEvent),
	}
	// Start sequence at 1 so 0 means "replay everything"
	el.nextSeq.Store(1)
	return el
}

// Append adds an event to the log, stamps it, and broadcasts to all subscribers.
func (el *EventLog) Append(evt sa.SubagentEvent) IndexedEvent {
	// Stamp metadata
	evt.SessionID = el.sessionID
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}

	seqID := el.nextSeq.Add(1) - 1
	indexed := IndexedEvent{SeqID: seqID, Event: evt}

	el.mu.Lock()
	if el.closed {
		el.mu.Unlock()
		return indexed
	}

	// Write to ring buffer
	el.ring[el.head] = indexed
	el.head = (el.head + 1) % el.ringSize
	if el.count < el.ringSize {
		el.count++
	}

	// Fan-out to all subscribers (non-blocking)
	for _, ch := range el.subs {
		select {
		case ch <- indexed:
		default:
			// Subscriber is slow — drop this event for them
		}
	}
	el.mu.Unlock()

	return indexed
}

// Replay returns all events with SeqID > sinceSeqID.
// Pass sinceSeqID=0 to get all buffered events.
func (el *EventLog) Replay(sinceSeqID uint64) []IndexedEvent {
	el.mu.RLock()
	defer el.mu.RUnlock()

	if el.count == 0 {
		return nil
	}

	result := make([]IndexedEvent, 0, el.count)

	// Walk the ring buffer in order
	start := el.head - el.count
	if start < 0 {
		start += el.ringSize
	}

	for i := 0; i < el.count; i++ {
		idx := (start + i) % el.ringSize
		evt := el.ring[idx]
		if evt.SeqID > sinceSeqID {
			result = append(result, evt)
		}
	}

	return result
}

// Subscribe creates a new subscription channel that receives live events.
// The returned channel is buffered (256). Call Unsubscribe with the returned ID
// when done (e.g., when the SSE client disconnects).
// If the log is already closed, returns a closed channel.
func (el *EventLog) Subscribe() (uint64, <-chan IndexedEvent) {
	ch := make(chan IndexedEvent, 256)

	el.mu.Lock()
	if el.closed {
		el.mu.Unlock()
		close(ch)
		return 0, ch
	}
	id := el.subCounter.Add(1)
	el.subs[id] = ch
	el.mu.Unlock()

	return id, ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (el *EventLog) Unsubscribe(id uint64) {
	el.mu.Lock()
	if ch, ok := el.subs[id]; ok {
		delete(el.subs, id)
		close(ch)
	}
	el.mu.Unlock()
}

// Close marks the log as closed and closes all subscriber channels.
// After Close, Append is a no-op and Subscribe returns a closed channel.
func (el *EventLog) Close() {
	el.mu.Lock()
	if el.closed {
		el.mu.Unlock()
		return
	}
	el.closed = true
	for id, ch := range el.subs {
		delete(el.subs, id)
		close(ch)
	}
	el.mu.Unlock()
}

// Len returns the number of events currently in the buffer.
func (el *EventLog) Len() int {
	el.mu.RLock()
	defer el.mu.RUnlock()
	return el.count
}

// LastSeqID returns the sequence ID of the most recent event (0 if empty).
func (el *EventLog) LastSeqID() uint64 {
	return el.nextSeq.Load() - 1
}

// IsClosed returns whether the log has been closed.
func (el *EventLog) IsClosed() bool {
	el.mu.RLock()
	defer el.mu.RUnlock()
	return el.closed
}
