package messaging

import (
	"context"
	"fmt"
)

func RegisterRequestHandler[TRequest Request, TResponse Response](p Processor, handler func(context.Context, TRequest) (TResponse, error)) {
	var zero TRequest
	p.RegisterRequestHandler(zero, func(ctx context.Context, msg Request) (Response, error) {
		request, ok := msg.(TRequest)
		if !ok {
			panic(fmt.Sprintf("RegisterRequestHandler: message %T does not match expected type %T", msg, zero))
		}

		return handler(ctx, request)
	})
}

func RegisterEventHandler[TEvent Event](p Processor, handler func(context.Context, TEvent) error) {
	var zero TEvent
	p.RegisterEventHandler(zero, func(ctx context.Context, msg Event) error {
		event, ok := msg.(TEvent)
		if !ok {
			panic(fmt.Sprintf("RegisterEventHandler: message %T does not match expected type %T", msg, zero))
		}
		return handler(ctx, event)
	})
}

type Processor interface {
	RegisterEventHandler(Event, EventHandler)
	RegisterRequestHandler(Request, RequestHandler)
}

func NewProcessor(bus MessageBus) Processor {
	return &processorBase{
		bus:             bus,
		eventHandlers:   make(map[string]EventHandler),
		requestHandlers: make(map[string]RequestHandler),
	}
}

type processorBase struct {
	bus             MessageBus
	subscriptions   map[string]Subscription
	eventHandlers   map[string]EventHandler
	requestHandlers map[string]RequestHandler
	queueGroups     map[string]string
}

func (p *processorBase) RegisterEventHandler(event Event, handler EventHandler) {
	discriminator := event.GetDiscriminator()

	if _, exists := p.eventHandlers[discriminator]; exists {
		panic(fmt.Sprintf("RegisterEventHandler: handler for event %s already exists", discriminator))
	}

	p.eventHandlers[discriminator] = handler
	sub, err := p.bus.Subscribe(event.GetRoute(), handler)
	if err != nil {
		panic(fmt.Sprintf("RegisterEventHandler: failed to subscribe to event %s: %v", discriminator, err))
	}
	p.subscriptions[discriminator] = sub
}

func (p *processorBase) RegisterRequestHandler(request Request, handler RequestHandler) {
	discriminator := request.GetDiscriminator()

	if _, exists := p.eventHandlers[discriminator]; exists {
		panic(fmt.Sprintf("RegisterEventHandler: handler for event %s already exists", discriminator))
	}

	p.requestHandlers[discriminator] = handler

	sub, err := p.bus.SubscribeRequest(request.GetRoute(), handler)
	if err != nil {
		panic(fmt.Sprintf("RegisterRequestHandler: failed to subscribe to request %s: %v", discriminator, err))
	}
	p.subscriptions[discriminator] = sub
}
