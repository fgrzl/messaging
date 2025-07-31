package messaging

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// RegisterMessageHandler is a generic helper to safely cast and register message handlers.
func RegisterMessageHandler[TMessage Message](
	p Processor,
	route Route,
	handler func(context.Context, TMessage) error,
) error {
	sub, err := Subscribe(p.GetBus(), route, handler)
	if err != nil {
		return err
	}
	p.Attach(sub)
	return nil
}

// RegisterRequestHandler is a generic helper to safely cast and register request handlers.
func RegisterRequestHandler[TRequest Request, TResponse Response](
	p Processor,
	route Route,
	handler func(context.Context, TRequest) (TResponse, error),
) error {
	sub, err := SubscribeRequest(p.GetBus(), route, handler)
	if err != nil {
		return err
	}
	p.Attach(sub)
	return nil
}

// Processor provides lifecycle management and subscription handling for message processors.
type Processor interface {
	// Start initializes the processor with the given context.
	Start(context.Context) error
	// Stop gracefully shuts down the processor and unsubscribes all handlers.
	Stop(context.Context) error
	// GetBus returns the underlying message bus for direct access.
	GetBus() MessageBus
	// Attach adds a subscription to be managed by this processor.
	Attach(Subscription)
}

// NewProcessor creates a new processor instance with the given message bus.
func NewProcessor(bus MessageBus) Processor {
	return &processorBase{
		bus:           bus,
		subscriptions: make(map[uuid.UUID]Subscription),
	}
}

type processorBase struct {
	bus           MessageBus
	subscriptions map[uuid.UUID]Subscription
}

// GetBus returns the underlying message bus for direct access.
func (p *processorBase) GetBus() MessageBus {
	return p.bus
}

// Start implements Processor.
func (p *processorBase) Start(ctx context.Context) error {
	return nil
}

// Stop implements Processor by unsubscribing all active subscriptions.
func (p *processorBase) Stop(ctx context.Context) error {
	var errs []error

	// Unsubscribe all active subscriptions
	for key, sub := range p.subscriptions {
		if err := sub.Unsubscribe(); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe failed for %s: %w", key, err))
		}
	}
	// Clear handler maps
	p.subscriptions = nil

	if len(errs) > 0 {
		return fmt.Errorf("processor stop encountered errors: %v", errs)
	}
	return nil
}

// Attach adds a subscription to be managed by this processor.
func (p *processorBase) Attach(sub Subscription) {
	p.subscriptions[sub.GetID()] = sub
}
