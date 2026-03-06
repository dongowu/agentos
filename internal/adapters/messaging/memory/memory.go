package memory

import (
	"context"
	"sync"

	"github.com/dongowu/ai-orchestrator/internal/adapter"
	"github.com/dongowu/ai-orchestrator/internal/messaging"
	"github.com/dongowu/ai-orchestrator/pkg/config"
)

func init() {
	adapter.RegisterEventBus("memory", func(cfg config.MessagingConfig) (messaging.EventBus, error) {
		return NewEventBus(), nil
	})
}

type sub struct {
	id int
	fn func(any)
}

// EventBus is an in-memory implementation of messaging.EventBus.
type EventBus struct {
	mu     sync.RWMutex
	subs   map[string][]sub
	nextID int
}

// NewEventBus returns a new in-memory event bus.
func NewEventBus() *EventBus {
	return &EventBus{subs: make(map[string][]sub)}
}

// Publish implements messaging.EventBus.
func (b *EventBus) Publish(ctx context.Context, topic string, payload any) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, s := range b.subs[topic] {
		s.fn(payload)
	}
	return nil
}

// Subscribe implements messaging.EventBus.
func (b *EventBus) Subscribe(topic string, handler func(any)) (func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	id := b.nextID
	b.subs[topic] = append(b.subs[topic], sub{id: id, fn: handler})
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		list := b.subs[topic]
		for i, s := range list {
			if s.id == id {
				b.subs[topic] = append(list[:i], list[i+1:]...)
				break
			}
		}
	}, nil
}

var _ messaging.EventBus = (*EventBus)(nil)
