package membus

import (
	"context"

	"github.com/fgrzl/messaging"
)

var _ messaging.MessageBusFactory = &Factory{}

// Factory is a factory for creating in-memory message bus instances.
type Factory struct {
	bus *Bus
}

// NewFactory creates a new in-memory message bus factory.
// It uses a singleton bus instance shared across all Get() calls.
func NewFactory() *Factory {
	return &Factory{
		bus: New(),
	}
}

// Get returns the singleton message bus instance.
func (f *Factory) Get(ctx context.Context) (messaging.MessageBus, error) {
	return f.bus, nil
}

// Close shuts down the factory's message bus.
func (f *Factory) Close() error {
	return f.bus.Close()
}
