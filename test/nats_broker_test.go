package test

import (
	"context"
	"testing"
	"time"

	"github.com/fgrzl/json/polymorphic"
	"github.com/fgrzl/messaging"
	broker "github.com/fgrzl/messaging/broker/natskit"
	"github.com/fgrzl/messaging/busx"
	client "github.com/fgrzl/messaging/client/natskit"
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

func Test_NATSBroker_MessageBus_Notify(t *testing.T) {
	ctx := context.Background()

	mockCreds, err := GenerateMockTrustedOperatorSetup()
	require.NoError(t, err)

	// Start embedded broker
	opts := broker.BrokerOptions{
		AccountJWT:       mockCreds.AccountJWT,
		OperatorJWT:      mockCreds.OperatorJWT,
		ReadinessTimeout: 5 * time.Second,
		ShutdownTimeout:  10 * time.Second,
		Host:             "localhost",
		WebSocketPort:    9222,
	}
	embedded := broker.NewBroker(ctx, opts)
	err = embedded.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = embedded.Stop(ctx)
	})

	// Connect NATS client to embedded broker
	client, err := client.NewBus("ws://localhost:9222", mockCreds.GetJWT, mockCreds.SignFn)
	require.NoError(t, err)
	defer client.Close()

	route := messaging.NewGlobalRoute("integration", "test")
	received := make(chan string, 1)

	// Subscribe to a typed message
	_, err = busx.Subscribe(client, route, func(ctx context.Context, msg *TestEvent) error {
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
