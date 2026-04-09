package bus

import (
	"context"
	"sync"
)

// Handler receives published payloads for a topic.
type Handler func(ctx context.Context, payload any)

// BeforeBlockSaveHook runs before a block's markdown lines are written to disk.
// Handlers run in registration order; each receives the latest content string and returns the value to pass to the next hook.
type BeforeBlockSaveHook func(ctx context.Context, data BeforeBlockSaveData) (content string, err error)

// BeforeBlockSaveData is input for before:block:save interceptors.
type BeforeBlockSaveData struct {
	BlockID    string
	SourcePath string
	Content    string
}

// Bus is a tiny in-process pub/sub for decoupled features (future plugins, sync, etc.).
type Bus struct {
	mu              sync.RWMutex
	subs            map[string][]Handler
	beforeBlockSave []BeforeBlockSaveHook
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

// RegisterBeforeBlockSave adds an interceptor for TopicBeforeBlockSave (runs before disk write).
func (b *Bus) RegisterBeforeBlockSave(h BeforeBlockSaveHook) {
	if b == nil || h == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.beforeBlockSave = append(b.beforeBlockSave, h)
}

// BeforeBlockSave runs registered hooks in order. Returns transformed content or an error from a hook.
func (b *Bus) BeforeBlockSave(ctx context.Context, data BeforeBlockSaveData) (string, error) {
	if b == nil {
		return data.Content, nil
	}
	b.mu.RLock()
	hs := append([]BeforeBlockSaveHook(nil), b.beforeBlockSave...)
	b.mu.RUnlock()
	cur := data.Content
	for _, h := range hs {
		data.Content = cur
		var err error
		cur, err = h(ctx, data)
		if err != nil {
			return "", err
		}
	}
	return cur, nil
}
