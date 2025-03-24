package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/fgrzl/json/polymorphic"
	"github.com/fgrzl/messaging"
	"github.com/nats-io/nats.go"
)

var (
	_ messaging.MessageBus   = &natsBus{}
	_ messaging.Subscription = &NATSSubscription{}
)

// NewMessageBus initializes a NATS-backed MessageBus.
func NewMessageBus(endpoint string) (*natsBus, error) {

	nc, err := nats.Connect(endpoint,
		nats.UserJWT(func() (string, error) { return "jwt-token", nil }, nil),
		nats.ReconnectWait(5*time.Second),
		nats.MaxReconnects(-1),
	)

	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err)
		return nil, err
	}
	return &natsBus{conn: nc}, nil
}

// natsBus implements the MessageBus interface.
type natsBus struct {
	conn *nats.Conn
}

// Notify sends a fire-and-forget event.
func (b *natsBus) Notify(msg messaging.Event) error {
	subj := toSubj(msg.GetRoute())

	data, err := encodeMessage(msg)
	if err != nil {
		slog.Error("Failed to serialize notification", "route", subj, "error", err)
		return err
	}

	natsMsg := &nats.Msg{
		Subject: subj,
		Data:    data,
	}

	if err = b.conn.PublishMsg(natsMsg); err != nil {
		slog.Error("Failed to publish notification", "route", subj, "error", err)
	}
	return err
}

// Request sends a synchronous request and waits for a response.
func (b *natsBus) Request(msg messaging.Request, timeout time.Duration) (messaging.Response, error) {
	subj := toSubj(msg.GetRoute())

	data, err := encodeMessage(msg)
	if err != nil {
		slog.Error("Failed to serialize request", "route", subj, "error", err)
		return nil, err
	}

	res, err := b.conn.Request(subj, data, timeout)
	if err != nil {
		slog.Error("NATS request failed", "route", subj, "error", err)
		return nil, err
	}

	response, err := decodeMessage[messaging.Response](res.Data)
	if err != nil {
		slog.Error("Failed to decode response", "route", subj, "error", err)
		return nil, err
	}

	return response, nil
}

// Subscribe implements MessageBus.
func (b *natsBus) Subscribe(route messaging.Route, handler messaging.EventHandler) (messaging.Subscription, error) {
	return b.SubscribeWithOptions(route, handler, messaging.SubscriptionOpts{})
}

// SubscribeWithOptions registers an event handler with queue group support.
func (b *natsBus) SubscribeWithOptions(route messaging.Route, handler messaging.EventHandler, opts messaging.SubscriptionOpts) (messaging.Subscription, error) {
	subj := toSubj(route)
	queueGroup := opts.QueueGroup

	slog.Info("Subscribing to event", "route", route, "queueGroup", queueGroup)

	var sub *nats.Subscription
	var err error

	if queueGroup != "" {
		sub, err = b.conn.QueueSubscribe(subj, queueGroup, func(msg *nats.Msg) {
			b.handleEvent(msg, handler)
		})
	} else {
		sub, err = b.conn.Subscribe(subj, func(msg *nats.Msg) {
			b.handleEvent(msg, handler)
		})
	}

	if err != nil {
		slog.Error("Failed to subscribe", "route", route, "error", err)
		return nil, err
	}

	return &NATSSubscription{sub: sub}, nil
}

// SubscribeRequest registers a handler for request-response.
func (b *natsBus) SubscribeRequest(route messaging.Route, handler messaging.RequestHandler) (messaging.Subscription, error) {
	subj := toSubj(route)

	slog.Info("Subscribing to request", "route", route)

	sub, err := b.conn.Subscribe(subj, func(msg *nats.Msg) {
		b.handleRequest(msg, handler)
	})

	if err != nil {
		slog.Error("Failed to subscribe to requests", "route", subj, "error", err)
		return nil, err
	}

	return &NATSSubscription{sub: sub}, nil
}

// handleEvent processes received events.
func (b *natsBus) handleEvent(msg *nats.Msg, handler messaging.EventHandler) {
	event, err := decodeMessage[messaging.Event](msg.Data)
	if err != nil {
		slog.Error("Failed to deserialize event", "error", err)
		return
	}
	ctx := contextFromMsg(msg)
	handler(ctx, event)
}

// handleRequest processes received requests and sends a response.
func (b *natsBus) handleRequest(msg *nats.Msg, handler messaging.RequestHandler) {
	request, err := decodeMessage[messaging.Request](msg.Data)
	if err != nil {
		slog.Error("Failed to deserialize request", "error", err)
		b.respondWithError(msg, "Invalid request format")
		return
	}
	ctx := contextFromMsg(msg)
	response, err := handler(ctx, request)
	if err != nil {
		slog.Warn("Request handler error", "error", err)
		b.respondWithError(msg, err.Error())
		return
	}

	b.respond(msg, response)
}

// respondWithError sends an error response.
func (b *natsBus) respondWithError(msg *nats.Msg, errorMessage string) {
	response := &messaging.ErrorResponse{Error: errorMessage}
	b.respond(msg, response)
}

// respond sends a response back to the requester.
func (b *natsBus) respond(msg *nats.Msg, response messaging.Response) {
	data, err := encodeMessage(response)
	if err != nil {
		slog.Warn("Failed to serialize response", "error", err)
		return
	}
	_ = msg.Respond(data)
}

// Unsubscribe removes a subscription.
func (b *natsBus) Unsubscribe(subscription messaging.Subscription) error {
	err := subscription.Unsubscribe()
	if err != nil {
		slog.Warn("Failed to unsubscribe", "error", err)
	}
	return err
}

// Close shuts down the NATS connection.
func (b *natsBus) Close() error {
	slog.Info("Closing NATS connection")
	b.conn.Close()
	return nil
}

// toSubj converts Route to a NATS subject string.
func toSubj(r messaging.Route) string {
	if r.Scope == messaging.ScopeOrg {
		organizationID := "*"
		if r.OrganizationID != nil {
			organizationID = r.OrganizationID.String()
		}
		return fmt.Sprintf("%s.%s.%s.%s", r.Scope, organizationID, r.Area, r.Name)
	}
	return fmt.Sprintf("%s.%s.%s", r.Scope, r.Area, r.Name)
}

// encodeMessage serializes an event or request into JSON.
func encodeMessage(msg polymorphic.Polymorphic) ([]byte, error) {
	envelope := polymorphic.NewEnvelope(msg)
	return json.Marshal(envelope)
}

// decodeMessage deserializes JSON into a polymorphic message.
func decodeMessage[T polymorphic.Polymorphic](data []byte) (T, error) {
	var envelope polymorphic.Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return *new(T), err
	}

	content, ok := envelope.Content.(T)
	if !ok {
		return *new(T), fmt.Errorf("invalid message type: %T", envelope.Discriminator)
	}

	return content, nil
}

func contextFromMsg(msg *nats.Msg) context.Context {
	ctx := context.Background()

	if msg.Header != nil {
		if correlationID := msg.Header.Get("X-Correlation-ID"); correlationID != "" {
			ctx = context.WithValue(ctx, messaging.CorrelationID("CorrelationID"), correlationID)
		}
	}

	return ctx
}
