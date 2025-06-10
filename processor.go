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

type Processor interface {
	Start(context.Context) error
	Stop(context.Context) error
	GetBus() MessageBus
	Attach(Subscription)
}

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

func (p *processorBase) GetBus() MessageBus {
	return p.bus
}

// Start implements Processor.
func (p *processorBase) Start(ctx context.Context) error {
	return nil
}

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

func (p *processorBase) Attach(sub Subscription) {
	p.subscriptions[sub.GetID()] = sub
}
