package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fgrzl/claims"
	"github.com/fgrzl/json/polymorphic"
	"github.com/google/uuid"
)

type MessageContext struct {
	context.Context
	User claims.Principal
}

type CorrelationID string
type CausationID string

// RequestHandler processes a request message and returns a response.
type RequestHandler func(context.Context, Request) (Response, error)

// MessageHandler processes an event message.
type MessageHandler func(context.Context, Message) error

// Scope defines message visibility and access control.
type Scope string

const (
	ScopeGlobal   Scope = "global"
	ScopeInternal Scope = "internal"
	ScopeTenant   Scope = "tenant"
)

func NewGlobalRoute(area, name string) Route {
	return Route{
		Scope: ScopeGlobal,
		Area:  area,
		Name:  name,
	}
}

func NewInternalRoute(area, name string) Route {
	return Route{
		Scope: ScopeInternal,
		Area:  area,
		Name:  name,
	}
}

func NewTenantRoute(area, name string, tenantID *uuid.UUID) Route {
	return Route{
		Scope:    ScopeTenant,
		Area:     area,
		Name:     name,
		TenantID: tenantID,
	}
}

// Route defines how messages are routed.
type Route struct {
	Scope    Scope
	Area     string
	Name     string
	TenantID *uuid.UUID
}

// Message represents an asynchronous message.
type Message interface {
	polymorphic.Polymorphic
	GetRoute() Route
}

// Request represents a synchronous message expecting a response.
type Request interface {
	polymorphic.Polymorphic
	GetRoute() Route
}

// Response represents a response to a request.
type Response = polymorphic.Polymorphic

type ErrorResponse struct {
	Error string `json:"error"`
}

func (e *ErrorResponse) GetDiscriminator() string {
	return "model://error"
}

type Accepted struct {
}

func (e *Accepted) GetDiscriminator() string {
	return "model://accepted"
}

// Subscription handles event unsubscription.
type Subscription interface {
	Unsubscribe() error
}

// DurableQueueMessage is an event that must be persisted in a queue system.
type DurableQueueMessage interface {
	Message
	GetPersistentQueue() string
}

// SubscriptionOpts defines options for event subscriptions.
type SubscriptionOpts struct {
	QueueGroup string
}

// MessageBus interface for sending and receiving messages.
type MessageBus interface {
	Notify(msg Message) error
	NotifyWithContext(ctx context.Context, msg Message) error
	Request(msg Request, timeout time.Duration) (Response, error)
	RequestWithContext(ctx context.Context, msg Request, timeout time.Duration) (Response, error)
	Subscribe(route Route, handler MessageHandler) (Subscription, error)
	SubscribeRequest(route Route, handler RequestHandler) (Subscription, error)
	Close() error
}

func Subscribe[T Message](bus MessageBus, route Route, handler func(ctx context.Context, msg T) error) (Subscription, error) {
	return bus.Subscribe(route, func(ctx context.Context, msg Message) error {
		// Ensure msg is of type T before type assertion
		tMsg, ok := msg.(T)
		if !ok {
			slog.Warn("Received message of unexpected type")
			return fmt.Errorf("unexpected message type: %T", msg)
		}
		return handler(ctx, tMsg)
	})
}

func SubscribeRequest[TRequest Request, TResponse Response](bus MessageBus, route Route, handler func(ctx context.Context, msg TRequest) (TResponse, error)) (Subscription, error) {
	return bus.SubscribeRequest(route, func(ctx context.Context, msg Request) (Response, error) {
		// Ensure msg is of type T before type assertion
		tMsg, ok := msg.(TRequest)
		if !ok {
			slog.Warn("Received message of unexpected type")
			return nil, fmt.Errorf("unexpected message type: %T", msg)
		}
		return handler(ctx, tMsg)
	})
}
