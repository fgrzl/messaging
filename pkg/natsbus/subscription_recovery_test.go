package natsbus

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fgrzl/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShouldRecoverSubscriptionWhenClosedUnexpectedly tests that subscriptions
// automatically recover when they close unexpectedly.
func TestShouldRecoverSubscriptionWhenClosedUnexpectedly(t *testing.T) {
	// This test requires a running NATS server
	t.Skip("Integration test - requires NATS server")

	// Arrange
	bus, err := connectWithOptions("nats://localhost:4222", nil)
	require.NoError(t, err)
	defer bus.Close()

	route := messaging.Route{
		Scope: messaging.ScopeGlobal,
		Area:  "test",
		Name:  "recovery",
	}

	var messageCount atomic.Int32
	handler := func(ctx context.Context, msg messaging.Message) error {
		messageCount.Add(1)
		return nil
	}

	sub, err := bus.Subscribe(route, handler)
	require.NoError(t, err)

	// Send initial message to verify subscription works
	testMsg := &testMessage{ID: "1", route: route}
	err = bus.Notify(testMsg)
	require.NoError(t, err)

	// Wait for message
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), messageCount.Load())

	// Act - Force subscription to close
	natsSub := sub.(*subscription).sub
	err = natsSub.Unsubscribe()
	require.NoError(t, err)

	// Wait for recovery (with exponential backoff, should recover quickly)
	time.Sleep(2 * time.Second)

	// Send another message after recovery
	err = bus.Notify(testMsg)
	require.NoError(t, err)

	// Wait for message
	time.Sleep(100 * time.Millisecond)

	// Assert - Should have received the second message after recovery
	assert.Equal(t, int32(2), messageCount.Load())
}

// TestShouldRecoverRequestHandlerWhenClosedUnexpectedly tests that request
// handlers automatically recover when they close unexpectedly.
func TestShouldRecoverRequestHandlerWhenClosedUnexpectedly(t *testing.T) {
	// This test requires a running NATS server
	t.Skip("Integration test - requires NATS server")

	// Arrange
	bus, err := connectWithOptions("nats://localhost:4222", nil)
	require.NoError(t, err)
	defer bus.Close()

	route := messaging.Route{
		Scope: messaging.ScopeInternal,
		Area:  "users",
		Name:  "login_sso",
	}

	var requestCount atomic.Int32
	handler := func(ctx context.Context, req messaging.Request) (messaging.Response, error) {
		requestCount.Add(1)
		return &testResponse{ID: "response"}, nil
	}

	sub, err := bus.SubscribeRequest(route, handler)
	require.NoError(t, err)

	// Send initial request to verify handler works
	testReq := &testRequest{ID: "1", route: route}
	resp, err := bus.Request(testReq, 5*time.Second)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(1), requestCount.Load())

	// Act - Force subscription to close
	natsSub := sub.(*subscription).sub
	err = natsSub.Unsubscribe()
	require.NoError(t, err)

	// Wait for recovery
	time.Sleep(2 * time.Second)

	// Send another request after recovery
	resp, err = bus.Request(testReq, 5*time.Second)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Assert - Should have processed the second request after recovery
	assert.Equal(t, int32(2), requestCount.Load())
}

// TestShouldStopRecoveryAttemptsWhenBusIsClosed verifies that recovery
// attempts are cancelled when the bus is closed.
func TestShouldStopRecoveryAttemptsWhenBusIsClosed(t *testing.T) {
	// This test requires a running NATS server
	t.Skip("Integration test - requires NATS server")

	// Arrange
	bus, err := connectWithOptions("nats://localhost:4222", nil)
	require.NoError(t, err)

	route := messaging.Route{
		Scope: messaging.ScopeGlobal,
		Area:  "test",
		Name:  "stop_recovery",
	}

	handler := func(ctx context.Context, msg messaging.Message) error {
		return nil
	}

	sub, err := bus.Subscribe(route, handler)
	require.NoError(t, err)

	// Force subscription to close
	natsSub := sub.(*subscription).sub
	err = natsSub.Unsubscribe()
	require.NoError(t, err)

	// Act - Close bus before recovery completes
	err = bus.Close()
	require.NoError(t, err)

	// Assert - Recovery goroutine should exit cleanly
	// If this test hangs, it means the recovery goroutine didn't respect the context cancellation
	time.Sleep(100 * time.Millisecond)
}

// TestShouldUseExponentialBackoffForRecovery verifies that the recovery
// mechanism uses exponential backoff.
func TestShouldUseExponentialBackoffForRecovery(t *testing.T) {
	// Arrange
	backoffs := []time.Duration{}
	expectedBackoffs := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second, // capped at maxBackoff
		30 * time.Second,
	}

	currentBackoff := initialBackoff
	for i := 0; i < 7; i++ {
		if i > 0 {
			currentBackoff = time.Duration(float64(currentBackoff) * backoffMultiplier)
			if currentBackoff > maxBackoff {
				currentBackoff = maxBackoff
			}
		}
		backoffs = append(backoffs, currentBackoff)
	}

	// Assert
	assert.Equal(t, expectedBackoffs, backoffs)
}

// TestShouldHandleMultipleSubscriptionsRecoveryIndependently verifies that
// multiple subscriptions can recover independently without affecting each other.
func TestShouldHandleMultipleSubscriptionsRecoveryIndependently(t *testing.T) {
	// This test requires a running NATS server
	t.Skip("Integration test - requires NATS server")

	// Arrange
	bus, err := connectWithOptions("nats://localhost:4222", nil)
	require.NoError(t, err)
	defer bus.Close()

	route1 := messaging.Route{Scope: messaging.ScopeGlobal, Area: "test", Name: "multi1"}
	route2 := messaging.Route{Scope: messaging.ScopeGlobal, Area: "test", Name: "multi2"}

	var count1, count2 atomic.Int32
	handler1 := func(ctx context.Context, msg messaging.Message) error {
		count1.Add(1)
		return nil
	}
	handler2 := func(ctx context.Context, msg messaging.Message) error {
		count2.Add(1)
		return nil
	}

	sub1, err := bus.Subscribe(route1, handler1)
	require.NoError(t, err)

	_, err = bus.Subscribe(route2, handler2)
	require.NoError(t, err)

	// Verify both subscriptions work
	err = bus.Notify(&testMessage{ID: "1", route: route1})
	require.NoError(t, err)
	err = bus.Notify(&testMessage{ID: "2", route: route2})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), count1.Load())
	assert.Equal(t, int32(1), count2.Load())

	// Act - Close only the first subscription
	natsSub1 := sub1.(*subscription).sub
	err = natsSub1.Unsubscribe()
	require.NoError(t, err)

	// Wait for recovery of first subscription
	time.Sleep(2 * time.Second)

	// Send messages to both routes
	err = bus.Notify(&testMessage{ID: "3", route: route1})
	require.NoError(t, err)
	err = bus.Notify(&testMessage{ID: "4", route: route2})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Assert - Both should have received messages
	assert.Equal(t, int32(2), count1.Load(), "First subscription should have recovered")
	assert.Equal(t, int32(2), count2.Load(), "Second subscription should still be working")
}

// TestShouldNotMonitorAfterExplicitUnsubscribe verifies that monitoring
// stops when a subscription is explicitly unsubscribed.
func TestShouldNotMonitorAfterExplicitUnsubscribe(t *testing.T) {
	// This test requires a running NATS server
	t.Skip("Integration test - requires NATS server")

	// Arrange
	bus, err := connectWithOptions("nats://localhost:4222", nil)
	require.NoError(t, err)
	defer bus.Close()

	route := messaging.Route{
		Scope: messaging.ScopeGlobal,
		Area:  "test",
		Name:  "explicit_unsub",
	}

	handler := func(ctx context.Context, msg messaging.Message) error {
		return nil
	}

	sub, err := bus.Subscribe(route, handler)
	require.NoError(t, err)

	natsBus := bus.(*natsBus)
	initialSubCount := len(natsBus.subscriptions)

	// Act - Explicitly unsubscribe through the bus
	err = natsBus.Unsubscribe(sub)
	require.NoError(t, err)

	// Wait to ensure no recovery happens
	time.Sleep(3 * time.Second)

	// Assert - Subscription should be removed and not recovered
	natsBus.mu.RLock()
	finalSubCount := len(natsBus.subscriptions)
	natsBus.mu.RUnlock()

	assert.Less(t, finalSubCount, initialSubCount, "Subscription should be removed")
}

// TestShouldPreserveQueueGroupOnRecovery verifies that queue group subscriptions
// maintain their queue group membership after recovery.
func TestShouldPreserveQueueGroupOnRecovery(t *testing.T) {
	// This test requires a running NATS server
	t.Skip("Integration test - requires NATS server")

	// Arrange
	bus, err := connectWithOptions("nats://localhost:4222", nil)
	require.NoError(t, err)
	defer bus.Close()

	route := messaging.Route{
		Scope: messaging.ScopeGlobal,
		Area:  "test",
		Name:  "queue_recovery",
	}

	var mu sync.Mutex
	receivedBy := make(map[string]int)

	handler1 := func(ctx context.Context, msg messaging.Message) error {
		mu.Lock()
		receivedBy["worker1"]++
		mu.Unlock()
		return nil
	}

	handler2 := func(ctx context.Context, msg messaging.Message) error {
		mu.Lock()
		receivedBy["worker2"]++
		mu.Unlock()
		return nil
	}

	opts := messaging.SubscriptionOpts{QueueGroup: "workers"}
	sub1, err := bus.(*natsBus).SubscribeWithOptions(route, handler1, opts)
	require.NoError(t, err)

	_, err = bus.(*natsBus).SubscribeWithOptions(route, handler2, opts)
	require.NoError(t, err)

	// Send messages and verify load balancing
	for i := 0; i < 10; i++ {
		err = bus.Notify(&testMessage{ID: fmt.Sprintf("%d", i), route: route})
		require.NoError(t, err)
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	totalBefore := receivedBy["worker1"] + receivedBy["worker2"]
	mu.Unlock()
	assert.Equal(t, 10, totalBefore, "All messages should be received by the queue group")

	// Act - Force first subscription to close
	natsSub1 := sub1.(*subscription).sub
	err = natsSub1.Unsubscribe()
	require.NoError(t, err)

	// Wait for recovery
	time.Sleep(2 * time.Second)

	// Send more messages
	for i := 0; i < 10; i++ {
		err = bus.Notify(&testMessage{ID: fmt.Sprintf("%d", i+10), route: route})
		require.NoError(t, err)
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	totalAfter := receivedBy["worker1"] + receivedBy["worker2"]
	mu.Unlock()

	// Assert - Should receive all 20 messages total
	assert.Equal(t, 20, totalAfter, "Queue group should still work after recovery")
	assert.Greater(t, receivedBy["worker1"], 0, "Worker 1 should have received messages after recovery")
	assert.Greater(t, receivedBy["worker2"], 0, "Worker 2 should continue receiving messages")
}
