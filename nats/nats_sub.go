package nats

import "github.com/nats-io/nats.go"

// NATSSubscription wraps a NATS subscription.
type NATSSubscription struct {
	sub *nats.Subscription
}

// Unsubscribe removes the subscription from NATS.
func (s *NATSSubscription) Unsubscribe() error {
	return s.sub.Unsubscribe()
}
