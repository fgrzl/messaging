package messaging

import (
	"context"
	"fmt"
	"log/slog"
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

// Subscribe provides a typed wrapper for subscribing to one-way messages.
// It ensures type safety by casting the incoming message to the expected type T.
func Subscribe[T Message](
	bus MessageBus,
	route Route,
	handler func(ctx context.Context, msg T) error,
) (Subscription, error) {
	return bus.Subscribe(route, func(ctx context.Context, msg Message) error {
		tMsg, ok := msg.(T)
		if !ok {
			slog.WarnContext(ctx, "Subscribe: received unexpected message type", "expected", fmt.Sprintf("%T", *new(T)), "actual", fmt.Sprintf("%T", msg))
			return fmt.Errorf("unexpected message type: %T", msg)
		}
		return handler(ctx, tMsg)
	})
}

// SubscribeRequest provides a typed wrapper for subscribing to request-response messages.
// It ensures type safety by casting the incoming request to TRequest and the response to TResponse.
func SubscribeRequest[TRequest Request, TResponse Response](
	bus MessageBus,
	route Route,
	handler func(ctx context.Context, msg TRequest) (TResponse, error),
) (Subscription, error) {
	return bus.SubscribeRequest(route, func(ctx context.Context, msg Request) (Response, error) {
		tMsg, ok := msg.(TRequest)
		if !ok {
			slog.WarnContext(ctx, "SubscribeRequest: received unexpected request type", "expected", fmt.Sprintf("%T", *new(TRequest)), "actual", fmt.Sprintf("%T", msg))
			return nil, fmt.Errorf("unexpected request type: %T", msg)
		}
		return handler(ctx, tMsg)
	})
}
