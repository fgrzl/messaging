package natsbus

import (
	"github.com/fgrzl/messaging"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

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

func (s *subscription) GetID() uuid.UUID {
	return s.id
}

// Unsubscribe removes the subscription from NATS.
func (s *subscription) Unsubscribe() error {
	return s.sub.Unsubscribe()
}
