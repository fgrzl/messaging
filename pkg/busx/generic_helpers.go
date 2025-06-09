package busx

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fgrzl/messaging"
)

func Request[TRequest messaging.Request, TResponse messaging.Response](
	bus messaging.MessageBus,
	msg TRequest,
	timeout time.Duration,
) (TResponse, error) {
	return RequestWithContext[TRequest, TResponse](context.Background(), bus, msg, timeout)
}

func RequestWithContext[TRequest messaging.Request, TResponse messaging.Response](
	ctx context.Context,
	bus messaging.MessageBus,
	msg TRequest,
	timeout time.Duration,
) (TResponse, error) {
	resp, err := bus.RequestWithContext(ctx, msg, timeout)
	if err != nil {
		var zero TResponse
		return zero, err
	}

	casted, ok := resp.(TResponse)
	if !ok {
		var zero TResponse
		return zero, fmt.Errorf("unexpected response type: %T", resp)
	}

	return casted, nil
}

// Subscribe provides a typed wrapper for subscribing to one-way messages.
// It ensures type safety by casting the incoming message to the expected type T.
func Subscribe[T messaging.Message](
	bus messaging.MessageBus,
	route messaging.Route,
	handler func(ctx context.Context, msg T) error,
) (messaging.Subscription, error) {
	return bus.Subscribe(route, func(ctx context.Context, msg messaging.Message) error {
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
func SubscribeRequest[TRequest messaging.Request, TResponse messaging.Response](
	bus messaging.MessageBus,
	route messaging.Route,
	handler func(ctx context.Context, msg TRequest) (TResponse, error),
) (messaging.Subscription, error) {
	return bus.SubscribeRequest(route, func(ctx context.Context, msg messaging.Request) (messaging.Response, error) {
		tMsg, ok := msg.(TRequest)
		if !ok {
			slog.WarnContext(ctx, "SubscribeRequest: received unexpected request type", "expected", fmt.Sprintf("%T", *new(TRequest)), "actual", fmt.Sprintf("%T", msg))
			return nil, fmt.Errorf("unexpected request type: %T", msg)
		}
		return handler(ctx, tMsg)
	})
}
