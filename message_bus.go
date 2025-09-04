package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

//
// Message Bus Interfaces
//

// MessageBusFactory provides a way to retrieve a scoped MessageBus instance.
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

//
// Typed Helpers
//

// SendRequest sends a typed request over the MessageBus and returns a strongly-typed response.
// It wraps the context-free case by delegating to SendRequestWithContext.
func SendRequest[TRequest Request, TResponse Response](
	bus MessageBus,
	msg TRequest,
	timeout time.Duration,
) (TResponse, error) {
	return SendRequestWithContext[TRequest, TResponse](context.Background(), bus, msg, timeout)
}

// SendRequestWithContext sends a typed request over the MessageBus using the provided context.
// It casts the raw Response to TResponse and returns an error if the types do not match.
func SendRequestWithContext[TRequest Request, TResponse Response](
	ctx context.Context,
	bus MessageBus,
	msg TRequest,
	timeout time.Duration,
) (TResponse, error) {
	resp, err := bus.RequestWithContext(ctx, msg, timeout)
	if err != nil {
		var zero TResponse
		return zero, err
	}

	tResp, ok := resp.(TResponse)
	if !ok {
		var zero TResponse
		return zero, fmt.Errorf("unexpected response type: %T", resp)
	}

	return tResp, nil
}

// Subscribe registers a strongly-typed one-way message handler on the given route.
// The internal handler performs a type assertion to ensure the message matches the expected type T.
func Subscribe[T Message](
	bus MessageBus,
	route Route,
	handler func(ctx context.Context, msg T) error,
) (Subscription, error) {
	return bus.Subscribe(route, func(ctx context.Context, msg Message) error {
		tMsg, ok := msg.(T)
		if !ok {
			slog.WarnContext(ctx,
				"Subscribe: unexpected message type",
				"expected", fmt.Sprintf("%T", *new(T)),
				"actual", fmt.Sprintf("%T", msg),
			)
			return fmt.Errorf("unexpected message type: %T", msg)
		}
		return handler(ctx, tMsg)
	})
}

// SubscribeRequest registers a strongly-typed request-response handler on the given route.
// It casts the incoming request to TRequest and ensures the returned value satisfies Response.
func SubscribeRequest[TRequest Request, TResponse Response](
	bus MessageBus,
	route Route,
	handler func(ctx context.Context, msg TRequest) (TResponse, error),
) (Subscription, error) {
	return bus.SubscribeRequest(route, func(ctx context.Context, msg Request) (Response, error) {
		tMsg, ok := msg.(TRequest)
		if !ok {
			slog.WarnContext(ctx,
				"SubscribeRequest: unexpected request type",
				"expected", fmt.Sprintf("%T", *new(TRequest)),
				"actual", fmt.Sprintf("%T", msg),
			)
			return nil, fmt.Errorf("unexpected request type: %T", msg)
		}
		return handler(ctx, tMsg)
	})
}

// Notify sends a strongly-typed one-way message (fire-and-forget).
// It wraps the context-free case by delegating to NotifyWithContext.
func Notify[T Message](bus MessageBus, msg T) error {
	return NotifyWithContext(context.Background(), bus, msg)
}

// NotifyWithContext sends a strongly-typed one-way message using the provided context.
func NotifyWithContext[T Message](ctx context.Context, bus MessageBus, msg T) error {
	return bus.NotifyWithContext(ctx, msg)
}
