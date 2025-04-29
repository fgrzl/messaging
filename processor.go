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

func RegisterMessageHandler[TMessage Message](p Processor, handler func(context.Context, TMessage) error) {
	var zero TMessage
	p.RegisterMessageHandler(zero, func(ctx context.Context, msg Message) error {
		message, ok := msg.(TMessage)
		if !ok {
			panic(fmt.Sprintf("RegisterMessageHandler: message %T does not match expected type %T", msg, zero))
		}
		return handler(ctx, message)
	})
}

type Processor interface {
	RegisterMessageHandler(Message, MessageHandler)
	RegisterRequestHandler(Request, RequestHandler)
}

func NewProcessor(bus MessageBus) Processor {
	return &processorBase{
		bus:             bus,
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

func (p *processorBase) RegisterMessageHandler(message Message, handler MessageHandler) {
	discriminator := message.GetDiscriminator()

	if _, exists := p.messageHandlers[discriminator]; exists {
		panic(fmt.Sprintf("RegisterMessageHandler: handler for message %s already exists", discriminator))
	}

	p.messageHandlers[discriminator] = handler
	sub, err := p.bus.Subscribe(message.GetRoute(), handler)
	if err != nil {
		panic(fmt.Sprintf("RegisterMessageHandler: failed to subscribe to message %s: %v", discriminator, err))
	}
	p.subscriptions[discriminator] = sub
}

func (p *processorBase) RegisterRequestHandler(request Request, handler RequestHandler) {
	discriminator := request.GetDiscriminator()

	if _, exists := p.messageHandlers[discriminator]; exists {
		panic(fmt.Sprintf("RegisterMessageHandler: handler for message %s already exists", discriminator))
	}

	p.requestHandlers[discriminator] = handler

	sub, err := p.bus.SubscribeRequest(request.GetRoute(), handler)
	if err != nil {
		panic(fmt.Sprintf("RegisterRequestHandler: failed to subscribe to request %s: %v", discriminator, err))
	}
	p.subscriptions[discriminator] = sub
}
