package ws

import (
	"testing"
	"time"
)

func TestHub_PublishSubscribe(t *testing.T) {
	h := NewHub()
	s := h.Subscribe(8)
	defer s.Close()
	s.Listen("topic")

	h.Publish("topic", "msg", map[string]string{"k": "v"})

	select {
	case m := <-s.C():
		if m.Topic != "topic" || m.Type != "msg" {
			t.Fatalf("unexpected: %+v", m)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message")
	}
}

func TestHub_Unlisten(t *testing.T) {
	h := NewHub()
	s := h.Subscribe(8)
	defer s.Close()
	s.Listen("t")
	s.Unlisten("t")
	h.Publish("t", "x", "y")
	select {
	case <-s.C():
		t.Fatal("should not receive after unlisten")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHub_SlowSubscriberDropped(t *testing.T) {
	h := NewHub()
	s := h.Subscribe(1)
	defer s.Close()
	s.Listen("t")
	// flood; nothing consumes
	for i := 0; i < 100; i++ {
		h.Publish("t", "x", i)
	}
	// At least the first message should be in the channel.
	select {
	case <-s.C():
	default:
		t.Fatal("expected at least one message buffered")
	}
}
