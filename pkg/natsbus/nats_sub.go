package natsbus

import (
	"context"
	"log/slog"
	"time"

	"github.com/fgrzl/messaging"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

const (
	// subscriptionCheckInterval is how often we check if a subscription is still valid
	subscriptionCheckInterval = 1 * time.Second
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
	id     uuid.UUID
	sub    *nats.Subscription
	cancel context.CancelFunc
}

// GetID returns the unique identifier for this subscription.
func (s *subscription) GetID() uuid.UUID {
	return s.id
}

// Unsubscribe removes the subscription from NATS.
func (s *subscription) Unsubscribe() error {
	// Stop monitoring if active
	if s.cancel != nil {
		s.cancel()
	}
	return s.sub.Unsubscribe()
}

// startMonitoring monitors the subscription for closure and triggers recovery.
func (s *subscription) startMonitoring(ctx context.Context, bus *natsBus, info *subscriptionInfo) {
	monitorCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	go func() {
		ticker := time.NewTicker(subscriptionCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-monitorCtx.Done():
				// Monitoring cancelled (likely due to explicit unsubscribe)
				return
			case <-ticker.C:
				if !s.sub.IsValid() {
					slog.Warn("Subscription closed unexpectedly",
						slog.String("id", s.id.String()),
						slog.String("route", toSubj(info.route)),
					)
					bus.recoverSubscription(s.id, info)
					return
				}
			}
		}
	}()
}
