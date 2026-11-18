package events

import "sync"

// Event is a message delivered to stream subscribers of a topic.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

const subBufferSize = 8

// Hub fans out events to subscribers grouped by topic.
type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[chan Event]struct{}
}

// DefaultHub is the process-wide hub used by the HTTP routes.
var DefaultHub = NewHub()

func NewHub() *Hub {
	return &Hub{
		subs: make(map[string]map[chan Event]struct{}),
	}
}

// Subscribe registers a buffered channel for the topic. The returned
// unsubscribe function closes the channel and must be called exactly once.
func (h *Hub) Subscribe(topic string) (<-chan Event, func()) {
	ch := make(chan Event, subBufferSize)
	h.mu.Lock()
	if h.subs[topic] == nil {
		h.subs[topic] = make(map[chan Event]struct{})
	}
	h.subs[topic][ch] = struct{}{}
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		conns := h.subs[topic]
		if conns == nil {
			return
		}
		delete(conns, ch)
		close(ch)
		if len(conns) == 0 {
			delete(h.subs, topic)
		}
	}
	return ch, unsubscribe
}

// Publish delivers the event to every subscriber of the topic. Subscribers
// whose buffer is full miss the event instead of blocking the publisher.
func (h *Hub) Publish(topic string, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.subs[topic] {
		select {
		case ch <- event:
		default:
		}
	}
}

// SubscriberCount returns the number of active subscribers for the topic.
func (h *Hub) SubscriberCount(topic string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs[topic])
}
