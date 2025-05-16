package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fgrzl/json/polymorphic"
	"github.com/fgrzl/messaging"
	"github.com/nats-io/nats.go"
)

var (
	_ messaging.MessageBus   = &natsBus{}
	_ messaging.Subscription = &NATSSubscription{}
)

func NewBus(endpoint string, jwt string, signFn func([]byte) ([]byte, error)) (messaging.MessageBus, error) {
	return connectWithOptions(
		endpoint,
		nats.UserJWT(
			func() (string, error) { return jwt, nil },
			signFn,
		),
	)
}

func connectWithOptions(endpoint string, auth nats.Option) (messaging.MessageBus, error) {
	opts := []nats.Option{
		auth,
		nats.ReconnectWait(5 * time.Second),
		nats.MaxReconnects(-1),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			slog.Info("Reconnected to NATS")
		}),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.Warn("Disconnected from NATS", slog.Any("error", err))
		}),
	}

	conn, err := nats.Connect(endpoint, opts...)
	if err != nil {
		slog.Error("Failed to connect to NATS", slog.Any("error", err))
		return nil, err
	}

	return &natsBus{conn: conn}, nil
}

type natsBus struct {
	conn *nats.Conn
}

func (b *natsBus) Notify(msg messaging.Message) error {
	return b.NotifyWithContext(context.Background(), msg)
}

func (b *natsBus) NotifyWithContext(ctx context.Context, msg messaging.Message) error {
	subj := toSubj(msg.GetRoute())
	data, err := encodeMessage(msg)
	if err != nil {
		slog.Error("Failed to serialize notification", "route", subj, "error", err)
		return err
	}

	return b.conn.PublishMsg(&nats.Msg{
		Subject: subj,
		Data:    data,
		Header:  messageHeadersFromContext(ctx),
	})
}

func (b *natsBus) Request(msg messaging.Request, timeout time.Duration) (messaging.Response, error) {
	return b.RequestWithContext(context.Background(), msg, timeout)
}

func (b *natsBus) RequestWithContext(ctx context.Context, msg messaging.Request, timeout time.Duration) (messaging.Response, error) {
	subj := toSubj(msg.GetRoute())
	data, err := encodeMessage(msg)
	if err != nil {
		slog.Error("Failed to serialize request", "route", subj, "error", err)
		return nil, err
	}

	res, err := b.conn.RequestMsg(&nats.Msg{
		Subject: subj,
		Data:    data,
		Header:  messageHeadersFromContext(ctx),
	}, timeout)
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

func (b *natsBus) Subscribe(route messaging.Route, handler messaging.MessageHandler) (messaging.Subscription, error) {
	return b.SubscribeWithOptions(route, handler, messaging.SubscriptionOpts{})
}

func (b *natsBus) SubscribeWithOptions(route messaging.Route, handler messaging.MessageHandler, opts messaging.SubscriptionOpts) (messaging.Subscription, error) {
	subj := toSubj(route)
	queue := opts.QueueGroup

	slog.Info("Subscribing to message", "route", route, "queueGroup", queue)

	var (
		sub *nats.Subscription
		err error
		cb  = func(msg *nats.Msg) { b.handleMessage(msg, handler) }
	)

	if queue != "" {
		sub, err = b.conn.QueueSubscribe(subj, queue, cb)
	} else {
		sub, err = b.conn.Subscribe(subj, cb)
	}

	if err != nil {
		slog.Error("Failed to subscribe", "route", route, "error", err)
		return nil, err
	}

	return &NATSSubscription{sub: sub}, nil
}

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

func (b *natsBus) handleMessage(msg *nats.Msg, handler messaging.MessageHandler) {
	message, err := decodeMessage[messaging.Message](msg.Data)
	if err != nil {
		slog.Error("Failed to deserialize message", "error", err)
		return
	}
	handler(contextFromMsg(msg), message)
}

func (b *natsBus) handleRequest(msg *nats.Msg, handler messaging.RequestHandler) {
	request, err := decodeMessage[messaging.Request](msg.Data)
	if err != nil {
		slog.Error("Failed to deserialize request", "error", err)
		b.respondWithError(msg, "Invalid request format")
		return
	}
	response, err := handler(contextFromMsg(msg), request)
	if err != nil {
		slog.Warn("Request handler error", "error", err)
		b.respondWithError(msg, err.Error())
		return
	}
	b.respond(msg, response)
}

func (b *natsBus) respondWithError(msg *nats.Msg, errMsg string) {
	b.respond(msg, &messaging.ErrorResponse{Error: errMsg})
}

func (b *natsBus) respond(msg *nats.Msg, response messaging.Response) {
	data, err := encodeMessage(response)
	if err != nil {
		slog.Warn("Failed to serialize response", "error", err)
		return
	}
	_ = msg.Respond(data)
}

func (b *natsBus) Unsubscribe(sub messaging.Subscription) error {
	err := sub.Unsubscribe()
	if err != nil {
		slog.Warn("Failed to unsubscribe", "error", err)
	}
	return err
}

func (b *natsBus) Close() error {
	slog.Info("Closing NATS connection")
	b.conn.Close()
	return nil
}

// --- Helpers ---

func toSubj(r messaging.Route) string {
	if r.Scope == messaging.ScopeTenant {
		tenantID := "*"
		if r.TenantID != nil {
			tenantID = r.TenantID.String()
		}
		return fmt.Sprintf("%s.%s.%s.%s", r.Scope, tenantID, r.Area, r.Name)
	}
	return fmt.Sprintf("%s.%s.%s", r.Scope, r.Area, r.Name)
}

func encodeMessage(msg polymorphic.Polymorphic) ([]byte, error) {
	return json.Marshal(polymorphic.NewEnvelope(msg))
}

func decodeMessage[T polymorphic.Polymorphic](data []byte) (T, error) {
	var env polymorphic.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return *new(T), err
	}
	content, ok := env.Content.(T)
	if !ok {
		return *new(T), fmt.Errorf("unexpected type: %T", env.Discriminator)
	}
	return content, nil
}

func contextFromMsg(msg *nats.Msg) context.Context {
	ctx := context.Background()
	if msg.Header != nil {
		if val := msg.Header.Get("X-Correlation-ID"); val != "" {
			ctx = context.WithValue(ctx, messaging.CorrelationID("CorrelationID"), val)
		}
		if val := msg.Header.Get("X-Causation-ID"); val != "" {
			ctx = context.WithValue(ctx, messaging.CausationID("CausationID"), val)
		}
	}
	return ctx
}

func messageHeadersFromContext(ctx context.Context) nats.Header {
	h := nats.Header{}
	if cid, ok := ctx.Value(messaging.CorrelationID("CorrelationID")).(string); ok && strings.TrimSpace(cid) != "" {
		h.Set("X-Correlation-ID", cid)
	}
	if caus, ok := ctx.Value(messaging.CausationID("CausationID")).(string); ok && strings.TrimSpace(caus) != "" {
		h.Set("X-Causation-ID", caus)
	}
	return h
}
