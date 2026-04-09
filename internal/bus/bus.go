package bus

import (
	"context"
	"sync"
)

// Handler receives published payloads for a topic.
type Handler func(ctx context.Context, payload any)

// Bus is a tiny in-process pub/sub for decoupled features (future plugins, sync, etc.).
type Bus struct {
	mu   sync.RWMutex
	subs map[string][]Handler
}

// New returns an empty bus.
func New() *Bus {
	return &Bus{subs: make(map[string][]Handler)}
}

// Subscribe registers a handler for topic. Multiple handlers run in registration order.
func (b *Bus) Subscribe(topic string, h Handler) {
	if b == nil || h == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[topic] = append(b.subs[topic], h)
}

// Publish invokes all subscribers for topic with a shallow copy of the handler slice.
func (b *Bus) Publish(ctx context.Context, topic string, payload any) {
	if b == nil {
		return
	}
	b.mu.RLock()
	hs := append([]Handler(nil), b.subs[topic]...)
	b.mu.RUnlock()
	for _, h := range hs {
		h(ctx, payload)
	}
}
