package membus_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fgrzl/messaging"
	"github.com/fgrzl/messaging/pkg/membus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testMessage is a simple test message.
type testMessage struct {
	Value string
	route messaging.Route
}

func (m *testMessage) GetDiscriminator() string {
	return "test://message"
}

func (m *testMessage) GetRoute() messaging.Route {
	if m.route.Area == "" {
		return messaging.NewGlobalRoute("test", "message")
	}
	return m.route
}

// testRequest is a simple test request.
type testRequest struct {
	Query string
}

func (r *testRequest) GetDiscriminator() string {
	return "test://request"
}

func (r *testRequest) GetRoute() messaging.Route {
	return messaging.NewGlobalRoute("test", "request")
}

// testResponse is a simple test response.
type testResponse struct {
	Result string
}

func (r *testResponse) GetDiscriminator() string {
	return "test://response"
}

func TestShouldNotifyMultipleHandlers(t *testing.T) {
	// Arrange
	bus := membus.New()
	defer bus.Close()

	route := messaging.NewGlobalRoute("test", "notify")
	var calls []string
	var mu sync.Mutex

	handler1 := func(ctx context.Context, msg messaging.Message) error {
		mu.Lock()
		calls = append(calls, "handler1")
		mu.Unlock()
		return nil
	}

	handler2 := func(ctx context.Context, msg messaging.Message) error {
		mu.Lock()
		calls = append(calls, "handler2")
		mu.Unlock()
		return nil
	}

	sub1, err := bus.Subscribe(route, handler1)
	require.NoError(t, err)
	require.NotNil(t, sub1)

	sub2, err := bus.Subscribe(route, handler2)
	require.NoError(t, err)
	require.NotNil(t, sub2)

	// Act
	msg := &testMessage{Value: "test", route: route}
	err = bus.Notify(msg)

	// Assert
	require.NoError(t, err)
	mu.Lock()
	assert.Equal(t, 2, len(calls))
	assert.Contains(t, calls, "handler1")
	assert.Contains(t, calls, "handler2")
	mu.Unlock()
}

func TestShouldUnsubscribeHandler(t *testing.T) {
	// Arrange
	bus := membus.New()
	defer bus.Close()

	route := messaging.NewGlobalRoute("test", "unsubscribe")
	var calls []string
	var mu sync.Mutex

	handler := func(ctx context.Context, msg messaging.Message) error {
		mu.Lock()
		calls = append(calls, "called")
		mu.Unlock()
		return nil
	}

	sub, err := bus.Subscribe(route, handler)
	require.NoError(t, err)

	// Act - first notify should call handler
	msg := &testMessage{Value: "test", route: route}
	err = bus.Notify(msg)
	require.NoError(t, err)

	mu.Lock()
	assert.Equal(t, 1, len(calls))
	mu.Unlock()

	// Unsubscribe
	err = sub.Unsubscribe()
	require.NoError(t, err)

	// Second notify should not call handler
	err = bus.Notify(msg)
	require.NoError(t, err)

	// Assert
	mu.Lock()
	assert.Equal(t, 1, len(calls))
	mu.Unlock()
}

func TestShouldReturnErrorWhenSubscribingToClosedBus(t *testing.T) {
	// Arrange
	bus := membus.New()
	err := bus.Close()
	require.NoError(t, err)

	// Act
	route := messaging.NewGlobalRoute("test", "closed")
	handler := func(ctx context.Context, msg messaging.Message) error {
		return nil
	}

	_, err = bus.Subscribe(route, handler)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestShouldReturnErrorWhenNotifyingToClosedBus(t *testing.T) {
	// Arrange
	bus := membus.New()
	err := bus.Close()
	require.NoError(t, err)

	// Act
	msg := &testMessage{Value: "test", route: messaging.NewGlobalRoute("test", "closed")}
	err = bus.Notify(msg)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestShouldHandleRequestResponse(t *testing.T) {
	// Arrange
	bus := membus.New()
	defer bus.Close()

	route := messaging.NewGlobalRoute("test", "request")

	handler := func(ctx context.Context, req messaging.Request) (messaging.Response, error) {
		testReq, ok := req.(*testRequest)
		if !ok {
			return nil, fmt.Errorf("invalid request type")
		}
		return &testResponse{Result: "Response to: " + testReq.Query}, nil
	}

	sub, err := bus.SubscribeRequest(route, handler)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Act
	req := &testRequest{Query: "Hello"}
	resp, err := bus.Request(req, 5*time.Second)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)

	testResp, ok := resp.(*testResponse)
	require.True(t, ok)
	assert.Equal(t, "Response to: Hello", testResp.Result)
}

func TestShouldReturnErrorWhenNoRequestHandlerRegistered(t *testing.T) {
	// Arrange
	bus := membus.New()
	defer bus.Close()

	// Act
	req := &testRequest{Query: "Hello"}
	resp, err := bus.Request(req, 5*time.Second)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "no request handler")
}

func TestShouldReturnErrorWhenRegisteringMultipleRequestHandlers(t *testing.T) {
	// Arrange
	bus := membus.New()
	defer bus.Close()

	route := messaging.NewGlobalRoute("test", "request")

	handler := func(ctx context.Context, req messaging.Request) (messaging.Response, error) {
		return &testResponse{Result: "ok"}, nil
	}

	sub1, err := bus.SubscribeRequest(route, handler)
	require.NoError(t, err)
	defer sub1.Unsubscribe()

	// Act
	_, err = bus.SubscribeRequest(route, handler)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestShouldRespectRequestTimeout(t *testing.T) {
	// Arrange
	bus := membus.New()
	defer bus.Close()

	route := messaging.NewGlobalRoute("test", "timeout")

	handler := func(ctx context.Context, req messaging.Request) (messaging.Response, error) {
		select {
		case <-time.After(500 * time.Millisecond):
			return &testResponse{Result: "ok"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	sub, err := bus.SubscribeRequest(route, handler)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Act
	req := &testRequest3{Query: "test", route: route}
	start := time.Now()
	_, err = bus.Request(req, 200*time.Millisecond)
	elapsed := time.Since(start)

	// Assert
	assert.Error(t, err)
	// The request should timeout, so elapsed time should be at least close to timeout
	assert.Greater(t, elapsed, 50*time.Millisecond)
}

func TestShouldNotifyWithContext(t *testing.T) {
	// Arrange
	bus := membus.New()
	defer bus.Close()

	route := messaging.NewGlobalRoute("test", "context")
	var receivedContext context.Context
	var once sync.Once

	handler := func(ctx context.Context, msg messaging.Message) error {
		once.Do(func() {
			receivedContext = ctx
		})
		return nil
	}

	sub, err := bus.Subscribe(route, handler)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Act
	ctx := context.WithValue(context.Background(), "key", "value")
	msg := &testMessage{Value: "test", route: route}
	err = bus.NotifyWithContext(ctx, msg)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, receivedContext)
	assert.Equal(t, "value", receivedContext.Value("key"))
}

// testRequest2 is a test request with a custom route.
type testRequest2 struct {
	Query string
	route messaging.Route
}

func (r *testRequest2) GetDiscriminator() string {
	return "test://request2"
}

func (r *testRequest2) GetRoute() messaging.Route {
	return r.route
}

// testRequest3 is another test request with a custom route.
type testRequest3 struct {
	Query string
	route messaging.Route
}

func (r *testRequest3) GetDiscriminator() string {
	return "test://request3"
}

func (r *testRequest3) GetRoute() messaging.Route {
	return r.route
}

func TestShouldRequestWithContext(t *testing.T) {
	// Arrange
	bus := membus.New()
	defer bus.Close()

	route := messaging.NewGlobalRoute("test", "request-context")
	var receivedContext context.Context

	handler := func(ctx context.Context, req messaging.Request) (messaging.Response, error) {
		receivedContext = ctx
		return &testResponse{Result: "ok"}, nil
	}

	sub, err := bus.SubscribeRequest(route, handler)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Act
	ctx := context.WithValue(context.Background(), "key", "value")
	req := &testRequest2{Query: "test", route: route}
	resp, err := bus.RequestWithContext(ctx, req, 5*time.Second)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, receivedContext)
	assert.Equal(t, "value", receivedContext.Value("key"))
}
