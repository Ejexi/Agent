package subagent

import (
	"sync"
	"testing"
	"time"

	sa "github.com/SecDuckOps/agent/internal/domain/subagent"
)

func TestEventLog_AppendAndReplay(t *testing.T) {
	el := NewEventLog("test-session")
	defer el.Close()

	// Append 3 events
	e1 := el.Append(sa.SubagentEvent{Type: sa.EventLog, Message: "msg1"})
	e2 := el.Append(sa.SubagentEvent{Type: sa.EventLog, Message: "msg2"})
	e3 := el.Append(sa.SubagentEvent{Type: sa.EventLog, Message: "msg3"})

	if e1.SeqID != 1 || e2.SeqID != 2 || e3.SeqID != 3 {
		t.Fatalf("expected seq IDs 1,2,3 got %d,%d,%d", e1.SeqID, e2.SeqID, e3.SeqID)
	}

	if el.Len() != 3 {
		t.Fatalf("expected len 3 got %d", el.Len())
	}

	if el.LastSeqID() != 3 {
		t.Fatalf("expected lastSeqID 3 got %d", el.LastSeqID())
	}

	// Replay all
	all := el.Replay(0)
	if len(all) != 3 {
		t.Fatalf("expected 3 events, got %d", len(all))
	}

	// Replay from seq 2
	since2 := el.Replay(2)
	if len(since2) != 1 || since2[0].SeqID != 3 {
		t.Fatalf("expected 1 event with seqID 3, got %d events", len(since2))
	}
}

func TestEventLog_RingBufferOverflow(t *testing.T) {
	el := NewEventLogWithSize("test-session", 4)
	defer el.Close()

	// Write 6 events into a ring of size 4
	for i := 0; i < 6; i++ {
		el.Append(sa.SubagentEvent{Type: sa.EventLog, Message: "msg"})
	}

	if el.Len() != 4 {
		t.Fatalf("expected len 4 (ring size), got %d", el.Len())
	}

	// Should only have events 3,4,5,6
	all := el.Replay(0)
	if len(all) != 4 {
		t.Fatalf("expected 4 events, got %d", len(all))
	}
	if all[0].SeqID != 3 || all[3].SeqID != 6 {
		t.Fatalf("expected first=3, last=6, got first=%d, last=%d", all[0].SeqID, all[3].SeqID)
	}
}

func TestEventLog_SubscribeAndReceive(t *testing.T) {
	el := NewEventLog("test-session")
	defer el.Close()

	subID, ch := el.Subscribe()
	defer el.Unsubscribe(subID)

	el.Append(sa.SubagentEvent{Type: sa.EventLog, Message: "live"})

	select {
	case evt := <-ch:
		if evt.Event.Message != "live" {
			t.Fatalf("expected 'live', got '%s'", evt.Event.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEventLog_MultipleSubscribers(t *testing.T) {
	el := NewEventLog("test-session")
	defer el.Close()

	id1, ch1 := el.Subscribe()
	id2, ch2 := el.Subscribe()
	defer el.Unsubscribe(id1)
	defer el.Unsubscribe(id2)

	el.Append(sa.SubagentEvent{Type: sa.EventLog, Message: "broadcast"})

	for i, ch := range []<-chan IndexedEvent{ch1, ch2} {
		select {
		case evt := <-ch:
			if evt.Event.Message != "broadcast" {
				t.Fatalf("subscriber %d: expected 'broadcast', got '%s'", i, evt.Event.Message)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out", i)
		}
	}
}

func TestEventLog_CloseStopsSubscribers(t *testing.T) {
	el := NewEventLog("test-session")
	_, ch := el.Subscribe()

	el.Close()

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed after Close()")
	}

	if !el.IsClosed() {
		t.Fatal("expected IsClosed to be true")
	}
}

func TestEventLog_ConcurrentAppend(t *testing.T) {
	el := NewEventLog("test-session")
	defer el.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			el.Append(sa.SubagentEvent{Type: sa.EventLog, Message: "concurrent"})
		}()
	}
	wg.Wait()

	if el.LastSeqID() != 100 {
		t.Fatalf("expected 100 events, got lastSeqID=%d", el.LastSeqID())
	}
}
