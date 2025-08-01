package messaging

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSendRequest(t *testing.T) {
	t.Run("ShouldSendRequestAndReturnTypedResponse", func(t *testing.T) {
		// Arrange
		bus := &MockMessageBus{}
		request := &MockRequest{route: NewGlobalRoute("test", "request")}
		timeout := 5 * time.Second

		// Mock the response
		expectedResponse := &MockMessage{route: NewGlobalRoute("test", "response")}
		bus.requestError = nil

		// We need to mock the actual response behavior
		mockBus := &MockMessageBusWithResponse{
			response: expectedResponse,
		}

		// Act
		response, err := SendRequest[*MockRequest, *MockMessage](mockBus, request, timeout)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, expectedResponse, response)
	})

	t.Run("ShouldReturnErrorWhenRequestFails", func(t *testing.T) {
		// Arrange
		requestError := errors.New("request failed")
		bus := &MockMessageBus{requestError: requestError}
		request := &MockRequest{route: NewGlobalRoute("test", "request")}
		timeout := 5 * time.Second

		// Act
		response, err := SendRequest[*MockRequest, *MockMessage](bus, request, timeout)

		// Assert
		assert.Error(t, err)
		assert.Equal(t, requestError, err)
		assert.Nil(t, response)
	})

	t.Run("ShouldReturnErrorWhenResponseTypeIsIncorrect", func(t *testing.T) {
		// Arrange
		wrongResponse := &ErrorResponse{Error: "not the expected type"}
		mockBus := &MockMessageBusWithResponse{
			response: wrongResponse,
		}
		request := &MockRequest{route: NewGlobalRoute("test", "request")}
		timeout := 5 * time.Second

		// Act
		response, err := SendRequest[*MockRequest, *MockMessage](mockBus, request, timeout)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected response type")
		assert.Nil(t, response)
	})
}

func TestSendRequestWithContext(t *testing.T) {
	t.Run("ShouldSendRequestWithContextAndReturnTypedResponse", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		expectedResponse := &MockMessage{route: NewGlobalRoute("test", "response")}
		mockBus := &MockMessageBusWithResponse{
			response: expectedResponse,
		}
		request := &MockRequest{route: NewGlobalRoute("test", "request")}
		timeout := 5 * time.Second

		// Act
		response, err := SendRequestWithContext[*MockRequest, *MockMessage](ctx, mockBus, request, timeout)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, expectedResponse, response)
	})

	t.Run("ShouldReturnErrorWhenRequestWithContextFails", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		requestError := errors.New("request with context failed")
		bus := &MockMessageBus{requestError: requestError}
		request := &MockRequest{route: NewGlobalRoute("test", "request")}
		timeout := 5 * time.Second

		// Act
		response, err := SendRequestWithContext[*MockRequest, *MockMessage](ctx, bus, request, timeout)

		// Assert
		assert.Error(t, err)
		assert.Equal(t, requestError, err)
		assert.Nil(t, response)
	})
}

func TestSubscribe(t *testing.T) {
	t.Run("ShouldSubscribeWithTypedHandler", func(t *testing.T) {
		// Arrange
		bus := &MockMessageBus{}
		route := NewGlobalRoute("test", "message")
		handler := func(ctx context.Context, msg *MockMessage) error {
			return nil
		}

		// Act
		subscription, err := Subscribe(bus, route, handler)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, subscription)
		assert.Len(t, bus.subscriptions, 1)
	})

	t.Run("ShouldReturnErrorWhenSubscribeFails", func(t *testing.T) {
		// Arrange
		subscribeError := errors.New("subscribe failed")
		bus := &MockMessageBus{subscribeError: subscribeError}
		route := NewGlobalRoute("test", "message")
		handler := func(ctx context.Context, msg *MockMessage) error { return nil }

		// Act
		subscription, err := Subscribe(bus, route, handler)

		// Assert
		assert.Error(t, err)
		assert.Equal(t, subscribeError, err)
		assert.Nil(t, subscription)
	})
}

func TestSubscribeRequest(t *testing.T) {
	t.Run("ShouldSubscribeRequestWithTypedHandler", func(t *testing.T) {
		// Arrange
		bus := &MockMessageBus{}
		route := NewGlobalRoute("test", "request")
		handler := func(ctx context.Context, req *MockRequest) (*MockMessage, error) {
			return &MockMessage{route: req.GetRoute()}, nil
		}

		// Act
		subscription, err := SubscribeRequest(bus, route, handler)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, subscription)
		assert.Len(t, bus.subscriptions, 1)
	})

	t.Run("ShouldReturnErrorWhenSubscribeRequestFails", func(t *testing.T) {
		// Arrange
		subscribeError := errors.New("subscribe request failed")
		bus := &MockMessageBus{subscribeRequestError: subscribeError}
		route := NewGlobalRoute("test", "request")
		handler := func(ctx context.Context, req *MockRequest) (*MockMessage, error) {
			return &MockMessage{route: req.GetRoute()}, nil
		}

		// Act
		subscription, err := SubscribeRequest(bus, route, handler)

		// Assert
		assert.Error(t, err)
		assert.Equal(t, subscribeError, err)
		assert.Nil(t, subscription)
	})
}

// MockMessageBusWithResponse is a test implementation that can return a specific response
type MockMessageBusWithResponse struct {
	MockMessageBus
	response Response
}

func (m *MockMessageBusWithResponse) Request(msg Request, timeout time.Duration) (Response, error) {
	return m.response, nil
}

func (m *MockMessageBusWithResponse) RequestWithContext(ctx context.Context, msg Request, timeout time.Duration) (Response, error) {
	return m.response, nil
}
