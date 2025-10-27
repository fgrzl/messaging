package natsbus

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fgrzl/claims"
	"github.com/fgrzl/json/polymorphic"
	"github.com/fgrzl/messaging"
	"github.com/fgrzl/messaging/pkg/natsbroker"
	"github.com/fgrzl/messaging/pkg/natsclaims"
	"github.com/google/uuid"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testMessage struct {
	ID    string
	route messaging.Route
}

func (m *testMessage) GetRoute() messaging.Route { return m.route }
func (m *testMessage) GetDiscriminator() string  { return "test://message" }

type testRequest struct {
	ID    string
	route messaging.Route
}

func (r *testRequest) GetRoute() messaging.Route { return r.route }
func (r *testRequest) GetDiscriminator() string  { return "test://request" }

type testResponse struct {
	ID string
}

func (r *testResponse) GetRoute() messaging.Route { return messaging.Route{} }
func (r *testResponse) GetDiscriminator() string  { return "test://response" }

func init() {
	polymorphic.Register(func() *testMessage { return &testMessage{} })
	polymorphic.Register(func() *testRequest { return &testRequest{} })
	polymorphic.Register(func() *testResponse { return &testResponse{} })
}

func TestToSubj(t *testing.T) {
	t.Run("ShouldFormatGlobalRoute", func(t *testing.T) {
		route := messaging.NewGlobalRoute("users", "created")
		assert.Equal(t, "global.users.created", toSubj(route))
	})

	t.Run("ShouldFormatInternalRoute", func(t *testing.T) {
		route := messaging.NewInternalRoute("system", "health")
		assert.Equal(t, "internal.system.health", toSubj(route))
	})

	t.Run("ShouldFormatTenantRouteWithID", func(t *testing.T) {
		id := uuid.New()
		route := messaging.NewTenantRoute("orders", "placed", &id)
		assert.Equal(t, "tenant."+id.String()+".orders.placed", toSubj(route))
	})

	t.Run("ShouldFormatTenantRouteWithWildcard", func(t *testing.T) {
		route := messaging.Route{Scope: messaging.ScopeTenant, Area: "orders", Name: "placed"}
		assert.Equal(t, "tenant.*.orders.placed", toSubj(route))
	})

	t.Run("ShouldFormatInboxRouteWithID", func(t *testing.T) {
		id := uuid.New()
		route := messaging.NewInboxRoute("direct", "message", &id)
		assert.Equal(t, "inbox."+id.String()+".direct.message", toSubj(route))
	})

	t.Run("ShouldFormatInboxRouteWithWildcard", func(t *testing.T) {
		route := messaging.Route{Scope: messaging.ScopeInbox, Area: "direct", Name: "message"}
		assert.Equal(t, "inbox.*.direct.message", toSubj(route))
	})
}

func TestEncodeDecodeMessage(t *testing.T) {
	t.Run("ShouldEncodeAndDecodeMessage", func(t *testing.T) {
		original := &testMessage{ID: "123", route: messaging.NewGlobalRoute("test", "event")}
		encoded, err := encodeMessage(original)
		require.NoError(t, err)
		decoded, err := decodeMessage[messaging.Message](encoded)
		require.NoError(t, err)
		assert.Equal(t, original.ID, decoded.(*testMessage).ID)
	})

	t.Run("ShouldReturnErrorForInvalidJSON", func(t *testing.T) {
		_, err := decodeMessage[messaging.Message]([]byte("{invalid"))
		assert.Error(t, err)
	})
}

func TestContextFromMsg(t *testing.T) {
	t.Run("ShouldExtractCorrelationID", func(t *testing.T) {
		id := uuid.New()
		msg := &nats.Msg{Header: nats.Header{}}
		msg.Header.Set("X-Correlation-ID", id.String())
		ctx := contextFromMsg(msg)
		assert.Equal(t, id, messaging.GetCorrelationID(ctx))
	})

	t.Run("ShouldExtractCausationID", func(t *testing.T) {
		id := uuid.New()
		msg := &nats.Msg{Header: nats.Header{}}
		msg.Header.Set("X-Causation-ID", id.String())
		ctx := contextFromMsg(msg)
		assert.Equal(t, id, messaging.GetCausationID(ctx))
	})

	t.Run("ShouldExtractUserPrincipal", func(t *testing.T) {
		cs := claims.NewClaimsSet("user123")
		principal := claims.NewPrincipal(cs)
		serialized, _ := claims.SerializePrincipal(principal)
		msg := &nats.Msg{Header: nats.Header{}}
		msg.Header.Set("X-User-Principal", serialized)
		ctx := contextFromMsg(msg)
		user, ok := messaging.GetUserPrincipal(ctx)
		assert.True(t, ok)
		assert.Equal(t, "user123", user.Subject())
	})

	t.Run("ShouldHandleNilHeaders", func(t *testing.T) {
		ctx := contextFromMsg(&nats.Msg{})
		assert.Equal(t, uuid.Nil, messaging.GetCorrelationID(ctx))
	})
}

func TestMessageHeadersFromContext(t *testing.T) {
	t.Run("ShouldSetCorrelationID", func(t *testing.T) {
		id := uuid.New()
		ctx := messaging.ContextWithTracing(context.Background(), id, uuid.Nil)
		headers := messageHeadersFromContext(ctx)
		assert.Equal(t, id.String(), headers.Get("X-Correlation-ID"))
	})

	t.Run("ShouldSetCausationID", func(t *testing.T) {
		id := uuid.New()
		ctx := messaging.ContextWithTracing(context.Background(), uuid.Nil, id)
		headers := messageHeadersFromContext(ctx)
		assert.Equal(t, id.String(), headers.Get("X-Causation-ID"))
	})

	t.Run("ShouldSetUserPrincipal", func(t *testing.T) {
		cs := claims.NewClaimsSet("user123")
		principal := claims.NewPrincipal(cs)
		ctx := messaging.ContextWithUserPrincipal(context.Background(), principal)
		headers := messageHeadersFromContext(ctx)
		assert.NotEmpty(t, headers.Get("X-User-Principal"))
	})
}

func TestSubscription(t *testing.T) {
	t.Run("ShouldHaveID", func(t *testing.T) {
		sub := NewSubscription(&nats.Subscription{})
		assert.NotEqual(t, uuid.Nil, sub.GetID())
	})
}

// Test helper types and infrastructure
type testBrokerConfig struct {
	broker      messaging.Broker
	operatorKey nkeys.KeyPair
	accountKey  nkeys.KeyPair
	userKey     nkeys.KeyPair
	operatorJWT string
	accountJWT  string
	clientURL   string
}

func (c *testBrokerConfig) ClientURL() string {
	return c.clientURL
}

// startTestBroker starts an embedded NATS broker for integration testing
func startTestBroker(t *testing.T) *testBrokerConfig {
	t.Helper()

	// Generate operator key
	operatorKey, err := nkeys.CreateOperator()
	require.NoError(t, err)

	operatorPub, err := operatorKey.PublicKey()
	require.NoError(t, err)

	operatorClaims := jwt.NewOperatorClaims(operatorPub)
	operatorClaims.Name = "Test Operator"
	operatorJWT, err := operatorClaims.Encode(operatorKey)
	require.NoError(t, err)

	// Generate account key
	accountKey, err := nkeys.CreateAccount()
	require.NoError(t, err)

	accountPub, err := accountKey.PublicKey()
	require.NoError(t, err)

	accountClaims := jwt.NewAccountClaims(accountPub)
	accountClaims.Name = "Test Account"
	accountJWT, err := accountClaims.Encode(operatorKey)
	require.NoError(t, err)

	// Generate user key
	userKey, err := nkeys.CreateUser()
	require.NoError(t, err)

	// Create broker with JWT authentication
	opts := natsbroker.BrokerOptions{
		Host:             "127.0.0.1",
		WebSocketPort:    9230 + (int(t.Name()[0]) % 10), // Vary port to avoid conflicts
		EnableTLS:        false,
		OperatorJWT:      operatorJWT,
		AccountJWT:       accountJWT,
		ReadinessTimeout: 15 * time.Second, // Allow time for parallel tests
		ShutdownTimeout:  10 * time.Second,
	}

	broker := natsbroker.NewBroker(context.Background(), opts)
	err = broker.Start(context.Background())
	require.NoError(t, err)

	clientURL := fmt.Sprintf("ws://%s:%d", opts.Host, opts.WebSocketPort)

	return &testBrokerConfig{
		broker:      broker,
		operatorKey: operatorKey,
		accountKey:  accountKey,
		userKey:     userKey,
		operatorJWT: operatorJWT,
		accountJWT:  accountJWT,
		clientURL:   clientURL,
	}
}

// stopTestBroker stops the test broker
func stopTestBroker(t *testing.T, config *testBrokerConfig) {
	t.Helper()
	if config != nil && config.broker != nil {
		_ = config.broker.Stop(context.Background())
	}
}

// createTestJWTFunctions creates JWT generation and signing functions for testing
func createTestJWTFunctions(t *testing.T, config *testBrokerConfig) (func() (string, error), func([]byte) ([]byte, error)) {
	t.Helper()

	userPub, err := config.userKey.PublicKey()
	require.NoError(t, err)

	accountPub, err := config.accountKey.PublicKey()
	require.NoError(t, err)

	// Create user JWT with NATS claims
	userClaims := jwt.NewUserClaims(userPub)
	userClaims.Name = "Test User"
	userClaims.IssuerAccount = accountPub

	// Add custom claims for our application
	cs := claims.NewClaimsSet("testuser")
	natsclaims.SetPermissions(cs, jwt.Permissions{
		Pub:  jwt.Permission{Allow: []string{">"}},
		Sub:  jwt.Permission{Allow: []string{">"}},
		Resp: &jwt.ResponsePermission{MaxMsgs: 1000, Expires: time.Hour},
	})
	natsclaims.SetTags(cs, "test")
	natsclaims.SetUserPub(cs, userPub)

	natsUserClaims, err := natsclaims.ToUserClaims(cs, accountPub)
	require.NoError(t, err)

	userClaims.Permissions = natsUserClaims.Permissions
	userClaims.Tags = natsUserClaims.Tags

	userJWT, err := userClaims.Encode(config.accountKey)
	require.NoError(t, err)

	getJWT := func() (string, error) {
		return userJWT, nil
	}

	signFn := func(nonce []byte) ([]byte, error) {
		sig, err := config.userKey.Sign(nonce)
		if err != nil {
			return nil, err
		}
		return sig, nil
	}

	return getJWT, signFn
}

// Integration tests require a running NATS server
func TestNatsBusIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Increase timeout to handle resource contention when running with other tests
	t.Parallel() // Run subtests in parallel but allow for resource sharing

	// Start embedded NATS broker for testing
	broker := startTestBroker(t)
	defer stopTestBroker(t, broker)

	t.Run("ShouldConnectToNATSServer", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)

		// Act
		bus, err := NewBus(broker.ClientURL(), getJWT, signFn)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, bus)
		defer bus.Close()
	})

	t.Run("ShouldPublishNotification", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)
		bus, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)
		defer bus.Close()

		msg := &testMessage{
			ID:    "msg-123",
			route: messaging.NewGlobalRoute("test", "notification"),
		}

		// Act
		err = bus.Notify(msg)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("ShouldPublishNotificationWithContext", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)
		bus, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)
		defer bus.Close()

		correlationID := uuid.New()
		ctx := messaging.ContextWithTracing(context.Background(), correlationID, uuid.Nil)

		msg := &testMessage{
			ID:    "msg-456",
			route: messaging.NewGlobalRoute("test", "notification"),
		}

		// Act
		err = bus.NotifyWithContext(ctx, msg)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("ShouldSubscribeAndReceiveMessage", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)
		bus, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)
		defer bus.Close()

		nBus := bus.(*natsBus) // Cast to access Unsubscribe

		route := messaging.NewGlobalRoute("test", "subscribe")
		received := make(chan *testMessage, 1)

		handler := func(ctx context.Context, msg messaging.Message) error {
			if tm, ok := msg.(*testMessage); ok {
				received <- tm
			}
			return nil
		}

		// Act
		sub, err := nBus.Subscribe(route, handler)
		require.NoError(t, err)
		defer nBus.Unsubscribe(sub)

		// Publish message
		testMsg := &testMessage{ID: "test-789", route: route}
		err = nBus.Notify(testMsg)
		require.NoError(t, err)

		// Assert
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		select {
		case receivedMsg := <-received:
			assert.Equal(t, "test-789", receivedMsg.ID)
		case <-timeoutCtx.Done():
			t.Fatal("Timeout waiting for message")
		}
	})

	t.Run("ShouldSubscribeWithQueueGroup", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)
		bus1, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)
		defer bus1.Close()

		bus2, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)
		defer bus2.Close()

		nBus1 := bus1.(*natsBus)
		nBus2 := bus2.(*natsBus)

		route := messaging.NewGlobalRoute("test", "queue")
		received1 := make(chan bool, 10)
		received2 := make(chan bool, 10)

		handler1 := func(ctx context.Context, msg messaging.Message) error {
			received1 <- true
			return nil
		}
		handler2 := func(ctx context.Context, msg messaging.Message) error {
			received2 <- true
			return nil
		}

		opts := messaging.SubscriptionOpts{QueueGroup: "workers"}

		// Act
		sub1, err := nBus1.SubscribeWithOptions(route, handler1, opts)
		require.NoError(t, err)
		defer nBus1.Unsubscribe(sub1)

		sub2, err := nBus2.SubscribeWithOptions(route, handler2, opts)
		require.NoError(t, err)
		defer nBus2.Unsubscribe(sub2)

		// Publish multiple messages
		for i := 0; i < 10; i++ {
			testMsg := &testMessage{ID: fmt.Sprintf("queue-%d", i), route: route}
			err = nBus1.Notify(testMsg)
			require.NoError(t, err)
		}

		// Assert - both handlers should receive messages (load balanced)
		time.Sleep(500 * time.Millisecond)
		count1 := len(received1)
		count2 := len(received2)
		assert.Equal(t, 10, count1+count2, "Total messages should be 10")
		assert.True(t, count1 > 0 && count2 > 0, "Both subscribers should receive messages")
	})

	t.Run("ShouldPreserveContextInMessageHeaders", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)
		bus, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)
		defer bus.Close()

		nBus := bus.(*natsBus)

		route := messaging.NewGlobalRoute("test", "context")
		correlationID := uuid.New()
		causationID := uuid.New()
		cs := claims.NewClaimsSet("testuser")
		principal := claims.NewPrincipal(cs)

		ctx := messaging.ContextWithTracing(context.Background(), correlationID, causationID)
		ctx = messaging.ContextWithUserPrincipal(ctx, principal)

		receivedCtx := make(chan context.Context, 1)
		handler := func(ctx context.Context, msg messaging.Message) error {
			receivedCtx <- ctx
			return nil
		}

		// Act
		sub, err := nBus.Subscribe(route, handler)
		require.NoError(t, err)
		defer nBus.Unsubscribe(sub)

		testMsg := &testMessage{ID: "ctx-test", route: route}
		err = nBus.NotifyWithContext(ctx, testMsg)
		require.NoError(t, err)

		// Assert
		select {
		case recvCtx := <-receivedCtx:
			assert.Equal(t, correlationID, messaging.GetCorrelationID(recvCtx))
			assert.Equal(t, causationID, messaging.GetCausationID(recvCtx))
			user, ok := messaging.GetUserPrincipal(recvCtx)
			assert.True(t, ok)
			assert.Equal(t, "testuser", user.Subject())
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for message")
		}
	})

	t.Run("ShouldHandleRequestResponse", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)
		bus, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)
		defer bus.Close()

		nBus := bus.(*natsBus)

		route := messaging.NewGlobalRoute("test", "request")

		requestHandler := func(ctx context.Context, req messaging.Request) (messaging.Response, error) {
			testReq := req.(*testRequest)
			return &testResponse{ID: "response-" + testReq.ID}, nil
		}

		// Act
		sub, err := nBus.SubscribeRequest(route, requestHandler)
		require.NoError(t, err)
		defer nBus.Unsubscribe(sub)

		req := &testRequest{ID: "req-123", route: route}
		resp, err := nBus.Request(req, 2*time.Second)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, resp)
		testResp := resp.(*testResponse)
		assert.Equal(t, "response-req-123", testResp.ID)
	})

	t.Run("ShouldHandleRequestWithContext", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)
		bus, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)
		defer bus.Close()

		nBus := bus.(*natsBus)

		route := messaging.NewGlobalRoute("test", "request-ctx")
		correlationID := uuid.New()

		requestHandler := func(ctx context.Context, req messaging.Request) (messaging.Response, error) {
			// Verify context was passed through
			assert.Equal(t, correlationID, messaging.GetCorrelationID(ctx))
			return &testResponse{ID: "response-with-ctx"}, nil
		}

		// Act
		sub, err := nBus.SubscribeRequest(route, requestHandler)
		require.NoError(t, err)
		defer nBus.Unsubscribe(sub)

		ctx := messaging.ContextWithTracing(context.Background(), correlationID, uuid.Nil)
		req := &testRequest{ID: "req-ctx", route: route}
		resp, err := nBus.RequestWithContext(ctx, req, 2*time.Second)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "response-with-ctx", resp.(*testResponse).ID)
	})

	t.Run("ShouldReturnErrorOnRequestTimeout", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)
		bus, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)
		defer bus.Close()

		route := messaging.NewGlobalRoute("test", "timeout")
		req := &testRequest{ID: "timeout-req", route: route}

		// Act - no handler subscribed, should timeout
		_, err = bus.Request(req, 100*time.Millisecond)

		// Assert
		assert.Error(t, err)
	})

	t.Run("ShouldHandleRequestHandlerError", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)
		bus, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)
		defer bus.Close()

		nBus := bus.(*natsBus)

		route := messaging.NewGlobalRoute("test", "error")

		requestHandler := func(ctx context.Context, req messaging.Request) (messaging.Response, error) {
			return nil, errors.New("handler failed")
		}

		// Act
		sub, err := nBus.SubscribeRequest(route, requestHandler)
		require.NoError(t, err)
		defer nBus.Unsubscribe(sub)

		req := &testRequest{ID: "error-req", route: route}
		resp, err := nBus.Request(req, 2*time.Second)

		// Assert
		require.NoError(t, err) // Request itself succeeds
		errorResp, ok := resp.(*messaging.ErrorResponse)
		assert.True(t, ok)
		assert.Equal(t, "handler failed", errorResp.Error)
	})

	t.Run("ShouldUnsubscribeSuccessfully", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)
		bus, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)
		defer bus.Close()

		nBus := bus.(*natsBus)

		route := messaging.NewGlobalRoute("test", "unsub")
		handler := func(ctx context.Context, msg messaging.Message) error {
			return nil
		}

		sub, err := nBus.Subscribe(route, handler)
		require.NoError(t, err)

		// Act
		err = nBus.Unsubscribe(sub)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("ShouldCloseConnectionCleanly", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)
		bus, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)

		// Act
		err = bus.Close()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("ShouldIsolateInboxMessagesByID", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)
		bus, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)
		defer bus.Close()

		nBus := bus.(*natsBus)

		// Create two different inbox IDs
		inbox1 := uuid.New()
		inbox2 := uuid.New()

		route1 := messaging.NewInboxRoute("chat", "message", &inbox1)
		route2 := messaging.NewInboxRoute("chat", "message", &inbox2)

		received1 := make(chan *testMessage, 5)
		received2 := make(chan *testMessage, 5)

		handler1 := func(ctx context.Context, msg messaging.Message) error {
			if tm, ok := msg.(*testMessage); ok {
				received1 <- tm
			}
			return nil
		}

		handler2 := func(ctx context.Context, msg messaging.Message) error {
			if tm, ok := msg.(*testMessage); ok {
				received2 <- tm
			}
			return nil
		}

		// Act - subscribe to both inboxes
		sub1, err := nBus.Subscribe(route1, handler1)
		require.NoError(t, err)
		defer nBus.Unsubscribe(sub1)

		sub2, err := nBus.Subscribe(route2, handler2)
		require.NoError(t, err)
		defer nBus.Unsubscribe(sub2)

		// Send messages to inbox1
		for i := 0; i < 3; i++ {
			msg := &testMessage{ID: fmt.Sprintf("inbox1-%d", i), route: route1}
			err = nBus.Notify(msg)
			require.NoError(t, err)
		}

		// Send messages to inbox2
		for i := 0; i < 2; i++ {
			msg := &testMessage{ID: fmt.Sprintf("inbox2-%d", i), route: route2}
			err = nBus.Notify(msg)
			require.NoError(t, err)
		}

		// Assert - inbox1 should receive only its messages
		time.Sleep(300 * time.Millisecond)
		assert.Equal(t, 3, len(received1), "Inbox1 should receive 3 messages")
		assert.Equal(t, 2, len(received2), "Inbox2 should receive 2 messages")

		// Verify message IDs
		for i := 0; i < 3; i++ {
			msg := <-received1
			assert.Contains(t, msg.ID, "inbox1-")
		}
		for i := 0; i < 2; i++ {
			msg := <-received2
			assert.Contains(t, msg.ID, "inbox2-")
		}
	})

	t.Run("ShouldSupportInboxRequestResponse", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)
		bus, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)
		defer bus.Close()

		nBus := bus.(*natsBus)

		inboxID := uuid.New()
		route := messaging.NewInboxRoute("support", "ticket", &inboxID)

		requestHandler := func(ctx context.Context, req messaging.Request) (messaging.Response, error) {
			testReq := req.(*testRequest)
			return &testResponse{ID: "ticket-response-" + testReq.ID}, nil
		}

		// Act
		sub, err := nBus.SubscribeRequest(route, requestHandler)
		require.NoError(t, err)
		defer nBus.Unsubscribe(sub)

		req := &testRequest{ID: "inbox-req-123", route: route}
		resp, err := nBus.Request(req, 2*time.Second)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, resp)
		testResp := resp.(*testResponse)
		assert.Equal(t, "ticket-response-inbox-req-123", testResp.ID)
	})

	t.Run("ShouldSubscribeToWildcardInboxes", func(t *testing.T) {
		// Arrange
		getJWT, signFn := createTestJWTFunctions(t, broker)
		bus, err := NewBus(broker.ClientURL(), getJWT, signFn)
		require.NoError(t, err)
		defer bus.Close()

		nBus := bus.(*natsBus)

		// Wildcard inbox route (ID is nil)
		wildcardRoute := messaging.Route{Scope: messaging.ScopeInbox, Area: "notifications", Name: "alert"}

		// Specific inbox routes
		inbox1 := uuid.New()
		inbox2 := uuid.New()
		route1 := messaging.NewInboxRoute("notifications", "alert", &inbox1)
		route2 := messaging.NewInboxRoute("notifications", "alert", &inbox2)

		receivedWildcard := make(chan *testMessage, 10)
		handlerWildcard := func(ctx context.Context, msg messaging.Message) error {
			if tm, ok := msg.(*testMessage); ok {
				receivedWildcard <- tm
			}
			return nil
		}

		// Act - subscribe to wildcard
		sub, err := nBus.Subscribe(wildcardRoute, handlerWildcard)
		require.NoError(t, err)
		defer nBus.Unsubscribe(sub)

		// Send to different inboxes
		msg1 := &testMessage{ID: "wildcard-msg1", route: route1}
		msg2 := &testMessage{ID: "wildcard-msg2", route: route2}

		err = nBus.Notify(msg1)
		require.NoError(t, err)
		err = nBus.Notify(msg2)
		require.NoError(t, err)

		// Assert - wildcard subscriber should receive both
		time.Sleep(300 * time.Millisecond)
		assert.Equal(t, 2, len(receivedWildcard), "Wildcard subscriber should receive all inbox messages")
	})
}
