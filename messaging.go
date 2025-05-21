package messaging

import (
	"context"

	"github.com/fgrzl/claims"
	"github.com/fgrzl/json/polymorphic"
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
	return "messaging://api/v1/err_response"
}

type Accepted struct {
}

func (e *Accepted) GetDiscriminator() string {
	return "messaging://api/v1/accepted"
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
