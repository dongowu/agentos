package nats

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/dongowu/agentos/internal/adapter"
	"github.com/dongowu/agentos/internal/messaging"
	"github.com/dongowu/agentos/pkg/config"
	"github.com/nats-io/nats.go"
)

func init() {
	adapter.RegisterEventBus("nats", func(cfg config.MessagingConfig) (messaging.EventBus, error) {
		return NewEventBus(cfg.NATS.URL, cfg.NATS.Stream)
	})
}

func normalize(url, stream string) (string, string) {
	if url == "" {
		url = "nats://localhost:4222"
	}
	if stream == "" {
		stream = "AGENTOS"
	}
	return url, stream
}

// OpenJetStream connects to NATS, creates the configured stream if needed, and returns both connection and JetStream context.
func OpenJetStream(url, stream string) (*nats.Conn, nats.JetStreamContext, string, error) {
	url, stream = normalize(url, stream)
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, nil, "", err
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, nil, "", err
	}
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     stream,
		Subjects: []string{stream + ".>"},
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		nc.Close()
		return nil, nil, "", err
	}
	return nc, js, stream, nil
}

// EventBus is a NATS JetStream implementation of messaging.EventBus.
type EventBus struct {
	nc     *nats.Conn
	js     nats.JetStreamContext
	stream string
	mu     sync.Mutex
	subs   map[string]*nats.Subscription
}

// NewEventBus connects to NATS and returns an EventBus.
func NewEventBus(url, stream string) (*EventBus, error) {
	nc, js, stream, err := OpenJetStream(url, stream)
	if err != nil {
		return nil, err
	}
	return &EventBus{nc: nc, js: js, stream: stream, subs: make(map[string]*nats.Subscription)}, nil
}

// Publish implements messaging.EventBus.
func (b *EventBus) Publish(ctx context.Context, topic string, payload any) error {
	subject := b.stream + "." + topic
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = b.js.Publish(subject, data)
	return err
}

// Subscribe implements messaging.EventBus.
func (b *EventBus) Subscribe(topic string, handler func(any)) (func(), error) {
	subject := b.stream + "." + topic
	sub, err := b.js.Subscribe(subject, func(msg *nats.Msg) {
		var v any
		if err := json.Unmarshal(msg.Data, &v); err != nil {
			return
		}
		handler(v)
	})
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	key := topic
	b.subs[key] = sub
	b.mu.Unlock()
	return func() {
		_ = sub.Unsubscribe()
		b.mu.Lock()
		delete(b.subs, key)
		b.mu.Unlock()
	}, nil
}

// Close closes the NATS connection.
func (b *EventBus) Close() error {
	b.nc.Close()
	return nil
}

var _ messaging.EventBus = (*EventBus)(nil)
