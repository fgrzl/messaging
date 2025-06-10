package messaging

import (
	"context"
	"fmt"
)

// RegisterRequestHandler is a generic helper to safely cast and register request handlers.
func RegisterRequestHandler[TRequest Request, TResponse Response](
	p Processor,
	handler func(context.Context, TRequest) (TResponse, error),
) {
	var zero TRequest
	p.RegisterRequestHandler(zero, func(ctx context.Context, msg Request) (Response, error) {
		req, ok := msg.(TRequest)
		if !ok {
			panic(fmt.Sprintf("RegisterRequestHandler: expected %T, got %T", zero, msg))
		}
		return handler(ctx, req)
	})
}

// RegisterMessageHandler is a generic helper to safely cast and register message handlers.
func RegisterMessageHandler[TMessage Message](
	p Processor,
	handler func(context.Context, TMessage) error,
) {
	var zero TMessage
	p.RegisterMessageHandler(zero, func(ctx context.Context, msg Message) error {
		tMsg, ok := msg.(TMessage)
		if !ok {
			panic(fmt.Sprintf("RegisterMessageHandler: expected %T, got %T", zero, msg))
		}
		return handler(ctx, tMsg)
	})
}

type Processor interface {
	RegisterMessageHandler(Message, MessageHandler)
	RegisterRequestHandler(Request, RequestHandler)
}

func NewProcessor(bus MessageBus) Processor {
	return &processorBase{
		bus:             bus,
		subscriptions:   make(map[string]Subscription),
		messageHandlers: make(map[string]MessageHandler),
		requestHandlers: make(map[string]RequestHandler),
	}
}

type processorBase struct {
	bus             MessageBus
	subscriptions   map[string]Subscription
	messageHandlers map[string]MessageHandler
	requestHandlers map[string]RequestHandler
}

func (p *processorBase) RegisterMessageHandler(msg Message, handler MessageHandler) {
	discriminator := msg.GetDiscriminator()
	if _, exists := p.messageHandlers[discriminator]; exists {
		panic(fmt.Sprintf("RegisterMessageHandler: handler for %s already registered", discriminator))
	}

	sub, err := Subscribe(p.bus, msg.GetRoute(), handler)
	if err != nil {
		panic(fmt.Sprintf("failed to subscribe to message %s: %v", discriminator, err))
	}

	p.messageHandlers[discriminator] = handler
	p.subscriptions[discriminator] = sub
}

func (p *processorBase) RegisterRequestHandler(req Request, handler RequestHandler) {
	discriminator := req.GetDiscriminator()
	if _, exists := p.requestHandlers[discriminator]; exists {
		panic(fmt.Sprintf("RegisterRequestHandler: handler for %s already registered", discriminator))
	}

	sub, err := SubscribeRequest(p.bus, req.GetRoute(), handler)
	if err != nil {
		panic(fmt.Sprintf("failed to subscribe to request %s: %v", discriminator, err))
	}

	p.requestHandlers[discriminator] = handler
	p.subscriptions[discriminator] = sub
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
	p.messageHandlers = nil
	p.requestHandlers = nil

	if len(errs) > 0 {
		return fmt.Errorf("processor stop encountered errors: %v", errs)
	}
	return nil
}
