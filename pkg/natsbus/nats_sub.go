package natsbus

import (
	"github.com/fgrzl/messaging"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// NewSubscription returns a new messaging.Subscription wrapper around a NATS subscription.
func NewSubscription(sub *nats.Subscription) messaging.Subscription {
	return &subscription{
		id:  uuid.New(),
		sub: sub,
	}
}

// subscription wraps a NATS subscription.
type subscription struct {
	id  uuid.UUID
	sub *nats.Subscription
}

// GetID returns the unique identifier for this subscription.
func (s *subscription) GetID() uuid.UUID {
	return s.id
}

// Unsubscribe removes the subscription from NATS.
func (s *subscription) Unsubscribe() error {
	return s.sub.Unsubscribe()
}
