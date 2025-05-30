package messaging

import (
	"context"
	"time"
)

type MessageBusFactory interface {
	Get(ctx context.Context) (MessageBus, error)
}

// MessageBus defines a unified interface for pub-sub and request-response messaging.
type MessageBus interface {
	// Notify sends a one-way message (fire-and-forget).
	Notify(msg Message) error

	// NotifyWithContext sends a one-way message using the provided context.
	NotifyWithContext(ctx context.Context, msg Message) error

	// Request sends a message and waits for a reply within the given timeout.
	Request(msg Request, timeout time.Duration) (Response, error)

	// RequestWithContext sends a message using the provided context and waits for a reply.
	RequestWithContext(ctx context.Context, msg Request, timeout time.Duration) (Response, error)

	// Subscribe registers a handler for one-way messages on the given route.
	Subscribe(route Route, handler MessageHandler) (Subscription, error)

	// SubscribeRequest registers a handler for request-response messages on the given route.
	SubscribeRequest(route Route, handler RequestHandler) (Subscription, error)

	// Close shuts down the message bus and cleans up any open subscriptions.
	Close() error
}
