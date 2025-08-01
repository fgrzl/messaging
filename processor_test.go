package messaging

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// MockMessageBus is a test implementation of MessageBus
type MockMessageBus struct {
	notifyError           error
	requestError          error
	subscribeError        error
	subscribeRequestError error
	subscriptions         []Subscription
}

func (m *MockMessageBus) Notify(msg Message) error { return m.notifyError }
func (m *MockMessageBus) NotifyWithContext(ctx context.Context, msg Message) error {
	return m.notifyError
}
func (m *MockMessageBus) Request(msg Request, timeout time.Duration) (Response, error) {
	return nil, m.requestError
}
func (m *MockMessageBus) RequestWithContext(ctx context.Context, msg Request, timeout time.Duration) (Response, error) {
	return nil, m.requestError
}
func (m *MockMessageBus) Subscribe(route Route, handler MessageHandler) (Subscription, error) {
	if m.subscribeError != nil {
		return nil, m.subscribeError
	}
	sub := &MockSubscription{id: uuid.New()}
	m.subscriptions = append(m.subscriptions, sub)
	return sub, nil
}
func (m *MockMessageBus) SubscribeRequest(route Route, handler RequestHandler) (Subscription, error) {
	if m.subscribeRequestError != nil {
		return nil, m.subscribeRequestError
	}
	sub := &MockSubscription{id: uuid.New()}
	m.subscriptions = append(m.subscriptions, sub)
	return sub, nil
}
func (m *MockMessageBus) Close() error { return nil }

// MockSubscription is a test implementation of Subscription
type MockSubscription struct {
	id               uuid.UUID
	unsubscribeError error
}

func (m *MockSubscription) GetID() uuid.UUID   { return m.id }
func (m *MockSubscription) Unsubscribe() error { return m.unsubscribeError }

// MockMessage is a test implementation of Message
type MockMessage struct {
	route Route
}

func (m *MockMessage) GetDiscriminator() string { return "test://message" }
func (m *MockMessage) GetRoute() Route          { return m.route }

// MockRequest is a test implementation of Request
type MockRequest struct {
	route Route
}

func (m *MockRequest) GetDiscriminator() string { return "test://request" }
func (m *MockRequest) GetRoute() Route          { return m.route }

func TestNewProcessor(t *testing.T) {
	t.Run("ShouldCreateProcessorWithGivenBus", func(t *testing.T) {
		// Arrange
		bus := &MockMessageBus{}

		// Act
		processor := NewProcessor(bus)

		// Assert
		assert.NotNil(t, processor)
		assert.Equal(t, bus, processor.GetBus())
	})
}

func TestProcessor_GetBus(t *testing.T) {
	t.Run("ShouldReturnUnderlyingMessageBus", func(t *testing.T) {
		// Arrange
		bus := &MockMessageBus{}
		processor := NewProcessor(bus)

		// Act
		result := processor.GetBus()

		// Assert
		assert.Equal(t, bus, result)
	})
}

func TestProcessor_Start(t *testing.T) {
	t.Run("ShouldStartSuccessfully", func(t *testing.T) {
		// Arrange
		bus := &MockMessageBus{}
		processor := NewProcessor(bus)
		ctx := context.Background()

		// Act
		err := processor.Start(ctx)

		// Assert
		assert.NoError(t, err)
	})
}

func TestProcessor_Attach(t *testing.T) {
	t.Run("ShouldAttachSubscriptionToProcessor", func(t *testing.T) {
		// Arrange
		bus := &MockMessageBus{}
		processor := NewProcessor(bus)
		subscription := &MockSubscription{id: uuid.New()}

		// Act
		processor.Attach(subscription)

		// Assert - verify subscription was attached by checking it's managed during stop
		err := processor.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestProcessor_Stop(t *testing.T) {
	t.Run("ShouldUnsubscribeAllAttachedSubscriptions", func(t *testing.T) {
		// Arrange
		bus := &MockMessageBus{}
		processor := NewProcessor(bus)
		subscription1 := &MockSubscription{id: uuid.New()}
		subscription2 := &MockSubscription{id: uuid.New()}

		processor.Attach(subscription1)
		processor.Attach(subscription2)

		// Act
		err := processor.Stop(context.Background())

		// Assert
		assert.NoError(t, err)
	})

	t.Run("ShouldReturnErrorWhenUnsubscribeFails", func(t *testing.T) {
		// Arrange
		bus := &MockMessageBus{}
		processor := NewProcessor(bus)
		unsubscribeError := errors.New("unsubscribe failed")
		subscription := &MockSubscription{
			id:               uuid.New(),
			unsubscribeError: unsubscribeError,
		}

		processor.Attach(subscription)

		// Act
		err := processor.Stop(context.Background())

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "processor stop encountered errors")
		assert.Contains(t, err.Error(), "unsubscribe failed")
	})

	t.Run("ShouldClearSubscriptionsAfterStop", func(t *testing.T) {
		// Arrange
		bus := &MockMessageBus{}
		processor := NewProcessor(bus)
		subscription := &MockSubscription{id: uuid.New()}

		processor.Attach(subscription)

		// Act
		err := processor.Stop(context.Background())

		// Assert
		assert.NoError(t, err)

		// Stopping again should not have subscriptions to process
		err = processor.Stop(context.Background())
		assert.NoError(t, err)
	})
}

func TestRegisterMessageHandler(t *testing.T) {
	t.Run("ShouldRegisterHandlerAndAttachSubscription", func(t *testing.T) {
		// Arrange
		bus := &MockMessageBus{}
		processor := NewProcessor(bus)
		route := NewGlobalRoute("test", "message")
		handler := func(ctx context.Context, msg *MockMessage) error { return nil }

		// Act
		err := RegisterMessageHandler(processor, route, handler)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, bus.subscriptions, 1)
	})

	t.Run("ShouldReturnErrorWhenSubscribeFails", func(t *testing.T) {
		// Arrange
		subscribeError := errors.New("subscribe failed")
		bus := &MockMessageBus{subscribeError: subscribeError}
		processor := NewProcessor(bus)
		route := NewGlobalRoute("test", "message")
		handler := func(ctx context.Context, msg *MockMessage) error { return nil }

		// Act
		err := RegisterMessageHandler(processor, route, handler)

		// Assert
		assert.Error(t, err)
		assert.Equal(t, subscribeError, err)
	})
}

func TestRegisterRequestHandler(t *testing.T) {
	t.Run("ShouldRegisterRequestHandlerAndAttachSubscription", func(t *testing.T) {
		// Arrange
		bus := &MockMessageBus{}
		processor := NewProcessor(bus)
		route := NewGlobalRoute("test", "request")
		handler := func(ctx context.Context, req *MockRequest) (*MockMessage, error) {
			return &MockMessage{route: req.GetRoute()}, nil
		}

		// Act
		err := RegisterRequestHandler(processor, route, handler)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, bus.subscriptions, 1)
	})

	t.Run("ShouldReturnErrorWhenSubscribeRequestFails", func(t *testing.T) {
		// Arrange
		subscribeError := errors.New("subscribe request failed")
		bus := &MockMessageBus{subscribeRequestError: subscribeError}
		processor := NewProcessor(bus)
		route := NewGlobalRoute("test", "request")
		handler := func(ctx context.Context, req *MockRequest) (*MockMessage, error) {
			return &MockMessage{route: req.GetRoute()}, nil
		}

		// Act
		err := RegisterRequestHandler(processor, route, handler)

		// Assert
		assert.Error(t, err)
		assert.Equal(t, subscribeError, err)
	})
}
