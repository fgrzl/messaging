package messaging

import (
	"context"
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

// EventHandler processes an event message.
type EventHandler func(context.Context, Event) error

// Scope defines message visibility and access control.
type Scope string

const (
	ScopeGlobal   Scope = "global"
	ScopeInternal Scope = "internal"
	ScopeOrg      Scope = "org"
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

func NewOrgRoute(area, name string, orgID *uuid.UUID) Route {
	return Route{
		Scope:          ScopeOrg,
		Area:           area,
		Name:           name,
		OrganizationID: orgID,
	}
}

// Route defines how messages are routed.
type Route struct {
	Scope          Scope
	Area           string
	Name           string
	OrganizationID *uuid.UUID
}

// Event represents an asynchronous message.
type Event interface {
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
	Event
	GetPersistentQueue() string
}

// SubscriptionOpts defines options for event subscriptions.
type SubscriptionOpts struct {
	QueueGroup string
}

// MessageBus interface for sending and receiving messages.
type MessageBus interface {
	Notify(msg Event) error
	Request(msg Request, timeout time.Duration) (Response, error)
	Subscribe(route Route, handler EventHandler) (Subscription, error)
	SubscribeRequest(route Route, handler RequestHandler) (Subscription, error)
	Close() error
}
