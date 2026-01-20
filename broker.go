package messaging

import "context"

// Broker defines the interface for starting and stopping a message broker.
type Broker interface {
	// Start initializes and starts the broker with the given context.
	Start(context.Context) error
	// Stop gracefully shuts down the broker with the given context.
	Stop(context.Context) error
	// GetWebSocketPort returns the actual WebSocket port the broker is listening on.
	// This is useful when using port 0 for dynamic port assignment.
	GetWebSocketPort() int
	// GetMonitorPort returns the HTTP monitoring port the broker is listening on.
	GetMonitorPort() int
}
