package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fgrzl/json/polymorphic"
	"github.com/fgrzl/messaging"

	"github.com/fgrzl/messaging/pkg/natsbroker"
	"github.com/fgrzl/messaging/pkg/natsbus"

	"github.com/stretchr/testify/require"
)

func init() {
	polymorphic.Register(func() *TestEvent { return &TestEvent{} })
}

// TestEvent is a concrete polymorphic message
type TestEvent struct {
	Value string `json:"value"`
}

func (e *TestEvent) GetDiscriminator() string {
	return "test://event"
}

func (e *TestEvent) GetRoute() messaging.Route {
	return messaging.NewGlobalRoute("integration", "test")
}

func TestShouldNotifyMessageBusWithNATSBroker(t *testing.T) {
	ctx := context.Background()

	mockCreds, err := GenerateMockTrustedOperatorSetup()
	require.NoError(t, err)

	// Start embedded broker with high port numbers to reduce conflicts
	// Using fixed ports but in a high range to avoid common service ports
	wsPort := 19222 + (time.Now().Unix() % 1000)      // Range: 19222-20222
	monitorPort := 18222 + (time.Now().Unix() % 1000) // Range: 18222-19222

	opts := natsbroker.BrokerOptions{
		AccountJWT:       mockCreds.AccountJWT,
		OperatorJWT:      mockCreds.OperatorJWT,
		ReadinessTimeout: 30 * time.Second, // Generous timeout for Windows + race detector
		ShutdownTimeout:  10 * time.Second,
		Host:             "127.0.0.1",
		WebSocketPort:    int(wsPort),
		MonitorPort:      int(monitorPort),
		EnableTLS:        false,
	}
	embedded := natsbroker.NewBroker(ctx, opts)
	err = embedded.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = embedded.Stop(ctx)
	})

	// Get the actual port (should match what we configured)
	actualWSPort := embedded.GetWebSocketPort()
	t.Logf("NATS broker started on ws://127.0.0.1:%d", actualWSPort)

	// Connect NATS client to embedded broker
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d", actualWSPort)
	client, err := natsbus.NewBus(wsURL, mockCreds.GetJWT, mockCreds.SignFn)
	require.NoError(t, err)
	defer client.Close()

	route := messaging.NewGlobalRoute("integration", "test")
	received := make(chan string, 1)

	// Subscribe to a typed message
	_, err = messaging.Subscribe(client, route, func(ctx context.Context, msg *TestEvent) error {
		received <- msg.Value
		return nil
	})
	require.NoError(t, err)

	// Publish the message
	err = client.Notify(&TestEvent{Value: "hello world"})
	require.NoError(t, err)

	// Assert receipt
	select {
	case v := <-received:
		require.Equal(t, "hello world", v)
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive message")
	}
}
