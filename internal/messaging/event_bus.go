package messaging

import "context"

// EventBus publishes and subscribes to domain events.
// MVP: in-memory; later: NATS JetStream.
type EventBus interface {
	Publish(ctx context.Context, topic string, payload any) error
	Subscribe(topic string, handler func(payload any)) (unsubscribe func(), err error)
}
