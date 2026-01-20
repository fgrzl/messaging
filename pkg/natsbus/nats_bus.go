package natsbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/fgrzl/claims"
	"github.com/fgrzl/json/polymorphic"
	"github.com/fgrzl/messaging"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

const (
	// Reconnection backoff configuration
	maxReconnectAttempts = 10
	initialBackoff       = 1 * time.Second
	maxBackoff           = 30 * time.Second
	backoffMultiplier    = 2.0
)

var (
	_ messaging.MessageBus   = &natsBus{}
	_ messaging.Subscription = &subscription{}
)

// subscriptionInfo holds the details needed to recreate a subscription after reconnect.
type subscriptionInfo struct {
	route          messaging.Route
	queueGroup     string
	messageHandler messaging.MessageHandler
	requestHandler messaging.RequestHandler
	isRequest      bool
}

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
	ctx, cancel := context.WithCancel(context.Background())
	bus := &natsBus{
		subscriptions: make(map[uuid.UUID]*subscriptionInfo),
		ctx:           ctx,
		cancel:        cancel,
	}

	opts := []nats.Option{
		auth,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.PingInterval(20 * time.Second),
		nats.ReconnectHandler(func(c *nats.Conn) {
			if c != nil {
				slog.Info("Reconnected to NATS", slog.String("server", c.ConnectedUrl()))
			} else {
				slog.Info("Reconnected to NATS")
			}
			bus.resubscribeAll()
		}),
		nats.DisconnectErrHandler(func(c *nats.Conn, err error) {
			if c != nil {
				slog.Warn("Disconnected from NATS", slog.String("server", c.ConnectedUrl()), slog.Any("error", err))
			} else {
				slog.Warn("Disconnected from NATS", slog.Any("error", err))
			}
		}),
	}

	conn, err := nats.Connect(endpoint, opts...)
	if err != nil {
		slog.Error("Failed to connect to NATS", slog.Any("error", err))
		return nil, err
	}

	bus.conn = conn
	return bus, nil
}

type natsBus struct {
	conn          *nats.Conn
	subscriptions map[uuid.UUID]*subscriptionInfo
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
}

func (b *natsBus) Notify(msg messaging.Message) error {
	return b.NotifyWithContext(context.Background(), msg)
}

func (b *natsBus) NotifyWithContext(ctx context.Context, msg messaging.Message) error {
	subj := toSubj(msg.GetRoute())
	data, err := encodeMessage(msg)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to serialize notification", "route", subj, "error", err)
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
		slog.ErrorContext(ctx, "Failed to serialize request", "route", subj, "error", err)
		return nil, err
	}

	res, err := b.conn.RequestMsg(&nats.Msg{
		Subject: subj,
		Data:    data,
		Header:  messageHeadersFromContext(ctx),
	}, timeout)
	if err != nil {
		slog.ErrorContext(ctx, "NATS request failed", "route", subj, "error", err)
		return nil, err
	}

	response, err := decodeMessage[messaging.Response](res.Data)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to decode response", "route", subj, "error", err)
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

	slog.Info("Subscribing to message",
		slog.String("scope", string(route.Scope)),
		slog.String("area", route.Area),
		slog.String("name", route.Name),
		slog.String("queueGroup", queue),
	)

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
		slog.Error("Failed to subscribe", slog.Any("route", route), slog.Any("error", err))
		return nil, err
	}

	s := &subscription{id: uuid.New(), sub: sub}

	// Track subscription for reconnection
	info := &subscriptionInfo{
		route:          route,
		queueGroup:     queue,
		messageHandler: handler,
		isRequest:      false,
	}
	b.mu.Lock()
	b.subscriptions[s.id] = info
	b.mu.Unlock()

	// Start monitoring for unexpected closure
	s.startMonitoring(b.ctx, b, info)

	return s, nil
}

func (b *natsBus) SubscribeRequest(route messaging.Route, handler messaging.RequestHandler) (messaging.Subscription, error) {
	subj := toSubj(route)
	slog.Info("Subscribing to request", "route", route)

	sub, err := b.conn.Subscribe(subj, func(msg *nats.Msg) {
		b.handleRequest(msg, handler)
	})
	if err != nil {
		slog.Error("Failed to subscribe to requests", slog.String("route", subj), slog.Any("error", err))
		return nil, err
	}

	s := &subscription{id: uuid.New(), sub: sub}

	// Track subscription for reconnection
	info := &subscriptionInfo{
		route:          route,
		requestHandler: handler,
		isRequest:      true,
	}
	b.mu.Lock()
	b.subscriptions[s.id] = info
	b.mu.Unlock()

	// Start monitoring for unexpected closure
	s.startMonitoring(b.ctx, b, info)

	return s, nil
}

func (b *natsBus) handleMessage(msg *nats.Msg, handler messaging.MessageHandler) {
	ctx := contextFromMsg(msg)
	message, err := decodeMessage[messaging.Message](msg.Data)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to deserialize message", "error", err)
		return
	}
	handler(ctx, message)
}

func (b *natsBus) handleRequest(msg *nats.Msg, handler messaging.RequestHandler) {
	ctx := contextFromMsg(msg)
	request, err := decodeMessage[messaging.Request](msg.Data)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to deserialize request", "error", err)
		b.respondWithError(msg, "Invalid request format")
		return
	}
	response, err := handler(ctx, request)
	if err != nil {
		slog.WarnContext(ctx, "Request handler error", "error", err)
		b.respondWithError(msg, err.Error())
		return
	}
	b.respondWithContext(ctx, msg, response)
}

func (b *natsBus) respondWithError(msg *nats.Msg, errMsg string) {
	ctx := contextFromMsg(msg)
	b.respondWithContext(ctx, msg, &messaging.ErrorResponse{Error: errMsg})
}

func (b *natsBus) respondWithContext(ctx context.Context, msg *nats.Msg, response messaging.Response) {
	data, err := encodeMessage(response)
	if err != nil {
		slog.WarnContext(ctx, "Failed to serialize response", "error", err)
		return
	}
	_ = msg.Respond(data)
}

func (b *natsBus) Unsubscribe(sub messaging.Subscription) error {
	err := sub.Unsubscribe()
	if err != nil {
		slog.Warn("Failed to unsubscribe", slog.Any("error", err))
	}

	// Remove from tracking
	b.mu.Lock()
	delete(b.subscriptions, sub.GetID())
	b.mu.Unlock()

	return err
}

func (b *natsBus) Close() error {
	slog.Info("Closing NATS connection")
	// Cancel all monitoring goroutines
	if b.cancel != nil {
		b.cancel()
	}
	b.conn.Close()
	return nil
}

// recoverSubscription attempts to re-establish a single subscription with exponential backoff.
func (b *natsBus) recoverSubscription(subID uuid.UUID, info *subscriptionInfo) {
	go func() {
		backoff := initialBackoff
		for attempt := 1; attempt <= maxReconnectAttempts; attempt++ {
			select {
			case <-b.ctx.Done():
				// Bus is shutting down, stop recovery attempts
				slog.Info("Stopping subscription recovery due to bus shutdown",
					slog.String("route", toSubj(info.route)),
				)
				return
			default:
			}

			slog.Info("Attempting to recover subscription",
				slog.String("route", toSubj(info.route)),
				slog.Int("attempt", attempt),
				slog.Duration("backoff", backoff),
			)

			// Wait before retry (except first attempt)
			if attempt > 1 {
				select {
				case <-time.After(backoff):
				case <-b.ctx.Done():
					return
				}
			}

			// Attempt to recreate subscription
			var (
				sub *nats.Subscription
				err error
			)
			subj := toSubj(info.route)

			if info.isRequest {
				sub, err = b.conn.Subscribe(subj, func(msg *nats.Msg) {
					b.handleRequest(msg, info.requestHandler)
				})
			} else {
				if info.queueGroup != "" {
					sub, err = b.conn.QueueSubscribe(subj, info.queueGroup, func(msg *nats.Msg) {
						b.handleMessage(msg, info.messageHandler)
					})
				} else {
					sub, err = b.conn.Subscribe(subj, func(msg *nats.Msg) {
						b.handleMessage(msg, info.messageHandler)
					})
				}
			}

			if err != nil {
				slog.Error("Failed to recover subscription",
					slog.String("route", subj),
					slog.Int("attempt", attempt),
					slog.Any("error", err),
				)

				// Calculate next backoff
				backoff = time.Duration(float64(backoff) * backoffMultiplier)
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			// Success - update the subscription reference and restart monitoring
			slog.Info("Successfully recovered subscription",
				slog.String("route", subj),
				slog.Int("attempt", attempt),
			)

			// Create new subscription wrapper with monitoring
			newSub := &subscription{id: subID, sub: sub}
			newSub.startMonitoring(b.ctx, b, info)

			return
		}

		// All retry attempts exhausted
		slog.Error("Failed to recover subscription after all attempts",
			slog.String("route", toSubj(info.route)),
			slog.Int("max_attempts", maxReconnectAttempts),
		)

		// Remove from tracking since we can't recover it
		b.mu.Lock()
		delete(b.subscriptions, subID)
		b.mu.Unlock()
	}()
}

// resubscribeAll re-establishes all tracked subscriptions after reconnection.
func (b *natsBus) resubscribeAll() {
	b.mu.RLock()
	count := len(b.subscriptions)
	infos := make([]*subscriptionInfo, 0, count)
	for _, info := range b.subscriptions {
		infos = append(infos, info)
	}
	b.mu.RUnlock()

	if count == 0 {
		return
	}

	slog.Info("Re-establishing subscriptions", slog.Int("count", count))

	for _, info := range infos {
		var err error
		subj := toSubj(info.route)

		if info.isRequest {
			_, err = b.conn.Subscribe(subj, func(msg *nats.Msg) {
				b.handleRequest(msg, info.requestHandler)
			})
			if err != nil {
				slog.Error("Failed to re-subscribe to request", slog.String("route", subj), slog.Any("error", err))
			} else {
				slog.Info("Re-subscribed to request", slog.String("route", subj))
			}
		} else {
			if info.queueGroup != "" {
				_, err = b.conn.QueueSubscribe(subj, info.queueGroup, func(msg *nats.Msg) {
					b.handleMessage(msg, info.messageHandler)
				})
			} else {
				_, err = b.conn.Subscribe(subj, func(msg *nats.Msg) {
					b.handleMessage(msg, info.messageHandler)
				})
			}
			if err != nil {
				slog.Error("Failed to re-subscribe to message", slog.String("route", subj), slog.Any("error", err))
			} else {
				slog.Info("Re-subscribed to message", slog.String("route", subj), slog.String("queueGroup", info.queueGroup))
			}
		}
	}
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
	if r.Scope == messaging.ScopeInbox {
		inboxID := "*"
		if r.ID != nil {
			inboxID = r.ID.String()
		}
		return fmt.Sprintf("%s.%s.%s.%s", r.Scope, inboxID, r.Area, r.Name)
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
		var zero T
		return zero, fmt.Errorf("unexpected discriminator %q", env.Discriminator)
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
			if user, err := claims.DeserializePrincipal(val); err == nil {
				ctx = messaging.ContextWithUserPrincipal(ctx, user)
			} else {
				// include any tracing info we managed to capture so far
				corr := messaging.GetCorrelationID(ctx)
				if corr != uuid.Nil {
					slog.WarnContext(ctx, "Failed to deserialize user principal", "error", err)
				} else {
					slog.Warn("Failed to deserialize user principal", slog.Any("error", err))
				}
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
		if serialized, err := claims.SerializePrincipal(user); err == nil {
			h.Set("X-User-Principal", serialized)
		} else {
			// prefer context-aware logging if tracing present
			corr := messaging.GetCorrelationID(ctx)
			if corr != uuid.Nil {
				slog.WarnContext(ctx, "Failed to serialize user principal", "error", err)
			} else {
				slog.Warn("Failed to serialize user principal", slog.Any("error", err))
			}
		}
	}

	return h
}
