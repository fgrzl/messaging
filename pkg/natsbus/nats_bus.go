package natsbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/fgrzl/json/polymorphic"
	"github.com/fgrzl/messaging"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

var (
	_ messaging.MessageBus   = &natsBus{}
	_ messaging.Subscription = &subscription{}
)

// NewBus returns a new NATS message bus connection with JWT-based authentication.
func NewBus(endpoint string, getJWT func() (string, error), signFn func([]byte) ([]byte, error)) (messaging.MessageBus, error) {
	return connectWithOptions(
		endpoint,
		nats.UserJWT(
			getJWT,
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

	return &subscription{sub: sub}, nil
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

	return &subscription{sub: sub}, nil
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
		if r.ID != nil {
			tenantID = r.ID.String()
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
		// Handle correlation ID
		if val := msg.Header.Get("X-Correlation-ID"); val != "" {
			if correlationID, err := uuid.Parse(val); err == nil {
				ctx = messaging.ContextWithTracing(ctx, correlationID, uuid.Nil)
			}
		}

		// Handle causation ID
		if val := msg.Header.Get("X-Causation-ID"); val != "" {
			if causationID, err := uuid.Parse(val); err == nil {
				// If we already have correlation ID, preserve it
				correlationID := messaging.GetCorrelationID(ctx)
				ctx = messaging.ContextWithTracing(ctx, correlationID, causationID)
			}
		}

		// Handle user principal
		if val := msg.Header.Get("X-User-Principal"); val != "" {
			if user, err := messaging.DeserializePrincipal(val); err == nil {
				ctx = messaging.ContextWithUserPrincipal(ctx, user)
			} else {
				slog.Warn("Failed to deserialize user principal", "error", err)
			}
		}
	}
	return ctx
}

func messageHeadersFromContext(ctx context.Context) nats.Header {
	h := nats.Header{}

	// Handle correlation ID
	correlationID := messaging.GetCorrelationID(ctx)
	if correlationID != uuid.Nil {
		h.Set("X-Correlation-ID", correlationID.String())
	}

	// Handle causation ID
	causationID := messaging.GetCausationID(ctx)
	if causationID != uuid.Nil {
		h.Set("X-Causation-ID", causationID.String())
	}

	// Handle user principal
	if user, ok := messaging.GetUserPrincipal(ctx); ok {
		if serialized, err := messaging.SerializePrincipal(user); err == nil {
			h.Set("X-User-Principal", serialized)
		} else {
			slog.Warn("Failed to serialize user principal", "error", err)
		}
	}

	return h
}
