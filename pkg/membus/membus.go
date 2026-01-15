package membus

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fgrzl/messaging"
	"github.com/google/uuid"
)

var (
	_ messaging.MessageBus   = &Bus{}
	_ messaging.Subscription = &Subscription{}
)

// Bus is an in-memory implementation of messaging.MessageBus.
type Bus struct {
	mu          sync.RWMutex
	handlers    map[string][]*handlerWrapper
	reqHandlers map[string]*requestHandler
	closed      atomic.Bool
}

// New creates a new in-memory message bus.
func New() *Bus {
	return &Bus{
		handlers:    make(map[string][]*handlerWrapper, 16),
		reqHandlers: make(map[string]*requestHandler, 16),
	}
}

// handlerWrapper represents a registered message handler.
type handlerWrapper struct {
	fn messaging.MessageHandler
	id uuid.UUID
}

// requestHandler represents a registered request handler.
type requestHandler struct {
	fn messaging.RequestHandler
}

// Subscription represents a subscription in the in-memory bus.
type Subscription struct {
	id    uuid.UUID
	route messaging.Route
	bus   *Bus
	fn    func() error
}

// GetID returns the unique identifier for this subscription.
func (s *Subscription) GetID() uuid.UUID {
	return s.id
}

// Unsubscribe removes the subscription from the bus.
func (s *Subscription) Unsubscribe() error {
	if s.fn != nil {
		return s.fn()
	}
	return nil
}

// Notify sends a one-way message (fire-and-forget).
func (b *Bus) Notify(msg messaging.Message) error {
	return b.NotifyWithContext(context.Background(), msg)
}

// NotifyWithContext sends a one-way message using the provided context.
func (b *Bus) NotifyWithContext(ctx context.Context, msg messaging.Message) error {
	// Fast path: check closed without lock
	if b.closed.Load() {
		return fmt.Errorf("message bus is closed")
	}

	route := msg.GetRoute()
	routeKey := route.String()

	b.mu.RLock()
	handlers := b.handlers[routeKey]
	handlerCount := len(handlers)

	// Fast path: single handler - execute synchronously to avoid goroutine overhead
	if handlerCount == 1 {
		h := handlers[0]
		b.mu.RUnlock()
		if err := h.fn(ctx, msg); err != nil {
			slog.ErrorContext(ctx, "Message handler error", "route", routeKey, "error", err)
		}
		return nil
	}

	// No handlers - quick exit
	if handlerCount == 0 {
		b.mu.RUnlock()
		return nil
	}

	// Create a copy to avoid holding the lock during handler execution
	handlersCopy := make([]*handlerWrapper, handlerCount)
	copy(handlersCopy, handlers)
	b.mu.RUnlock()

	// Execute handlers concurrently
	var wg sync.WaitGroup
	wg.Add(handlerCount)
	for _, h := range handlersCopy {
		go func(h *handlerWrapper) {
			defer wg.Done()
			if err := h.fn(ctx, msg); err != nil {
				slog.ErrorContext(ctx, "Message handler error", "route", routeKey, "error", err)
			}
		}(h)
	}
	wg.Wait()

	return nil
}

// Request sends a message and waits for a reply within the given timeout.
func (b *Bus) Request(msg messaging.Request, timeout time.Duration) (messaging.Response, error) {
	return b.RequestWithContext(context.Background(), msg, timeout)
}

// RequestWithContext sends a message using the provided context and waits for a reply.
func (b *Bus) RequestWithContext(ctx context.Context, msg messaging.Request, timeout time.Duration) (messaging.Response, error) {
	// Fast path: check closed without lock
	if b.closed.Load() {
		return nil, fmt.Errorf("message bus is closed")
	}

	route := msg.GetRoute()
	routeKey := route.String()

	b.mu.RLock()
	reqHandler := b.reqHandlers[routeKey]
	b.mu.RUnlock()

	if reqHandler == nil {
		return nil, fmt.Errorf("no request handler registered for route: %s", routeKey)
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute handler and return response
	return reqHandler.fn(ctx, msg)
}

// Subscribe registers a handler for one-way messages on the given route.
func (b *Bus) Subscribe(route messaging.Route, handler messaging.MessageHandler) (messaging.Subscription, error) {
	// Fast path: check closed
	if b.closed.Load() {
		return nil, fmt.Errorf("message bus is closed")
	}

	routeKey := route.String()
	subID := uuid.New()
	h := &handlerWrapper{
		fn: handler,
		id: subID,
	}

	b.mu.Lock()
	// Double-check after acquiring lock
	if b.closed.Load() {
		b.mu.Unlock()
		return nil, fmt.Errorf("message bus is closed")
	}

	b.handlers[routeKey] = append(b.handlers[routeKey], h)
	b.mu.Unlock()

	subscription := &Subscription{
		id:    subID,
		route: route,
		bus:   b,
		fn: func() error {
			b.mu.Lock()
			defer b.mu.Unlock()

			// Remove handler from slice
			handlers := b.handlers[routeKey]
			for i, handler := range handlers {
				if handler.id == subID {
					b.handlers[routeKey] = append(handlers[:i], handlers[i+1:]...)
					break
				}
			}

			// Clean up empty route entry
			if len(b.handlers[routeKey]) == 0 {
				delete(b.handlers, routeKey)
			}

			return nil
		},
	}

	return subscription, nil
}

// SubscribeRequest registers a handler for request-response messages on the given route.
func (b *Bus) SubscribeRequest(route messaging.Route, handler messaging.RequestHandler) (messaging.Subscription, error) {
	// Fast path: check closed
	if b.closed.Load() {
		return nil, fmt.Errorf("message bus is closed")
	}

	routeKey := route.String()

	b.mu.Lock()
	// Double-check after acquiring lock
	if b.closed.Load() {
		b.mu.Unlock()
		return nil, fmt.Errorf("message bus is closed")
	}

	// Only one request handler per route
	if b.reqHandlers[routeKey] != nil {
		b.mu.Unlock()
		return nil, fmt.Errorf("request handler already registered for route: %s", routeKey)
	}

	b.reqHandlers[routeKey] = &requestHandler{fn: handler}
	b.mu.Unlock()

	subscription := &Subscription{
		id:    uuid.New(),
		route: route,
		bus:   b,
		fn: func() error {
			b.mu.Lock()
			defer b.mu.Unlock()
			delete(b.reqHandlers, routeKey)
			return nil
		},
	}

	return subscription, nil
}

// Close shuts down the message bus and cleans up any open subscriptions.
func (b *Bus) Close() error {
	// Use atomic to signal closure to fast paths
	b.closed.Store(true)

	b.mu.Lock()
	defer b.mu.Unlock()

	// Clear all handlers
	clear(b.handlers)
	clear(b.reqHandlers)

	return nil
}
