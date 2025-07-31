package messaging

import "context"

// Broker defines the interface for starting and stopping a message broker.
type Broker interface {
	// Start initializes and starts the broker with the given context.
	Start(context.Context) error
	// Stop gracefully shuts down the broker with the given context.
	Stop(context.Context) error
}
