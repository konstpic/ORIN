// Package ws is a tiny in-process pub/sub hub for the WebSocket gateway.
// Topics are simple strings (e.g. "app:<name>:status"). Subscribers receive
// fan-out via buffered channels; slow consumers are dropped to keep
// publishers non-blocking.
package ws

import (
	"encoding/json"
	"log/slog"
	"sync"

	apiv1 "github.com/orin/orin/pkg/api/v1"
)

// Hub broadcasts messages to per-topic subscribers.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[*Subscriber]struct{}
}

// NewHub constructs an empty hub.
func NewHub() *Hub {
	return &Hub{subscribers: make(map[string]map[*Subscriber]struct{})}
}

// Subscriber receives messages on a per-connection channel.
type Subscriber struct {
	ch     chan apiv1.WSMessage
	topics map[string]struct{}
	hub    *Hub
}

// Subscribe registers a new subscriber.
func (h *Hub) Subscribe(bufSize int) *Subscriber {
	if bufSize <= 0 {
		bufSize = 64
	}
	return &Subscriber{
		ch:     make(chan apiv1.WSMessage, bufSize),
		topics: make(map[string]struct{}),
		hub:    h,
	}
}

// Listen adds a topic to a subscriber.
func (s *Subscriber) Listen(topic string) {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	s.topics[topic] = struct{}{}
	if _, ok := s.hub.subscribers[topic]; !ok {
		s.hub.subscribers[topic] = make(map[*Subscriber]struct{})
	}
	s.hub.subscribers[topic][s] = struct{}{}
}

// Unlisten removes a topic from a subscriber.
func (s *Subscriber) Unlisten(topic string) {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	delete(s.topics, topic)
	if subs, ok := s.hub.subscribers[topic]; ok {
		delete(subs, s)
		if len(subs) == 0 {
			delete(s.hub.subscribers, topic)
		}
	}
}

// Close removes the subscriber from every topic and closes its channel.
func (s *Subscriber) Close() {
	s.hub.mu.Lock()
	for topic := range s.topics {
		if subs, ok := s.hub.subscribers[topic]; ok {
			delete(subs, s)
			if len(subs) == 0 {
				delete(s.hub.subscribers, topic)
			}
		}
	}
	s.topics = nil
	s.hub.mu.Unlock()
	close(s.ch)
}

// C returns the receive channel.
func (s *Subscriber) C() <-chan apiv1.WSMessage { return s.ch }

// Publish fans out to subscribers of topic. Encoding the payload is the
// caller's responsibility; we just forward.
func (h *Hub) Publish(topic, msgType string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("ws publish: marshal payload", "err", err, "topic", topic)
		return
	}
	msg := apiv1.WSMessage{Topic: topic, Type: msgType, Payload: raw}

	h.mu.RLock()
	subs := h.subscribers[topic]
	dropped := 0
	for s := range subs {
		select {
		case s.ch <- msg:
		default:
			dropped++
		}
	}
	h.mu.RUnlock()
	if dropped > 0 {
		slog.Debug("ws publish dropped slow subscribers", "topic", topic, "dropped", dropped)
	}
}
