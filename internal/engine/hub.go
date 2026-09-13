package engine

import (
	"encoding/json"
	"sync"
)

// Event is one server-sent event.
type Event struct {
	Name string
	Data []byte
}

// Hub fans events out to every open dashboard. A subscriber that cannot keep
// up loses events rather than stalling the collector; the next status
// snapshot carries the full picture anyway.
type Hub struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

// NewHub returns an empty hub.
func NewHub() *Hub { return &Hub{subs: map[chan Event]struct{}{}} }

// Subscribe returns a channel of events and a function to leave.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 16)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}
}

// Publish encodes v as JSON and sends it to every subscriber.
func (h *Hub) Publish(name string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	ev := Event{Name: name, Data: data}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Subscribers is how many dashboards are listening.
func (h *Hub) Subscribers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}
