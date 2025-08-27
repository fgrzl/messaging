package messaging

import (
	"context"

	"github.com/fgrzl/json/polymorphic"
	"github.com/google/uuid"
)

// CorrelationID is used to associate related messages across systems.
type CorrelationID string

// CausationID is used to identify the originating message that triggered the current one.
type CausationID string

// MessageHandler processes an asynchronous (fire-and-forget) message.
type MessageHandler func(ctx context.Context, msg Message) error

// RequestHandler processes a synchronous message and returns a response.
type RequestHandler func(ctx context.Context, req Request) (Response, error)

// Message represents an event-style message that does not expect a response.
type Message interface {
	polymorphic.Polymorphic

	// GetRoute returns the routing metadata for this message.
	GetRoute() Route
}

// Request represents a message that expects a response.
type Request interface {
	polymorphic.Polymorphic

	// GetRoute returns the routing metadata for this request.
	GetRoute() Route
}

// Response is a polymorphic response to a request message.
type Response = polymorphic.Polymorphic

// ErrorResponse represents an error returned from a request handler.
type ErrorResponse struct {
	// Error is a human-readable error message.
	Error string `json:"error"`
}

// GetDiscriminator returns the type identifier for ErrorResponse.
func (e *ErrorResponse) GetDiscriminator() string {
	return "messaging://err_response"
}

// BoolResult represents a simple boolean response value.
type BoolResult struct {
	Value bool `json:"value"`
}

// GetDiscriminator returns the type identifier for BoolResult.
func (e *BoolResult) GetDiscriminator() string {
	return "messaging://bool_result"
}

// Accepted represents a successful but content-less acknowledgment.
type Accepted struct {
	// Reason is an optional explanation for the acceptance (e.g., "queued", "acknowledged").
	Reason string `json:"reason,omitempty"`
}

// GetDiscriminator returns the type identifier for Accepted.
func (e *Accepted) GetDiscriminator() string {
	return "messaging://accepted"
}

// Subscription represents an active subscription that can be unsubscribed.
type Subscription interface {
	// GetID returns the subscription ID.
	GetID() uuid.UUID
	// Unsubscribe cancels the subscription and releases any related resources.
	Unsubscribe() error
}

// DurableQueueMessage is a Message that must be persisted by the transport.
type DurableQueueMessage interface {
	Message

	// GetPersistentQueue returns the queue name where this message should be persisted.
	GetPersistentQueue() string
}

// SubscriptionOpts defines options for subscribing to a message route.
type SubscriptionOpts struct {
	// QueueGroup is the queue group name for load-balanced delivery.
	QueueGroup string
}
