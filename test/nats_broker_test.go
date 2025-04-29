package test

// import (
// 	"context"
// 	"testing"
// 	"time"

// 	"github.com/fgrzl/json/polymorphic"
// 	"github.com/fgrzl/messaging"
// 	server "github.com/fgrzl/messaging/broker/nats"
// 	client "github.com/fgrzl/messaging/client/nats"
// 	"github.com/google/uuid"

// 	"github.com/stretchr/testify/require"
// )

// // testMessage implements messaging.Message
// type testMessage struct {
// 	ID   uuid.UUID `json:"id"`
// 	Data string    `json:"data"`
// }

// func (m *testMessage) GetDiscriminator() string {
// 	return "testMessage"
// }

// func (m *testMessage) GetRoute() messaging.Route {
// 	return messaging.Route{
// 		Scope: messaging.ScopeGlobal,
// 		Area:  "test",
// 		Name:  "ping",
// 	}
// }

// func TestNATSBus_NotifyAndReceive(t *testing.T) {
// 	polymorphic.Register(func() *testMessage { return &testMessage{} })

// 	options := server.GetDefaultOptions()
// 	broker := server.NewBroker(options)
// 	err := broker.Start()
// 	require.NoError(t, err)

// 	bus1, err := client.NewAnonymousBus("nats://localhost:4222", "dummy-jwt")
// 	require.NoError(t, err)

// 	bus2, err := client.NewAnonymousBus("nats://localhost:4222", "dummy-jwt")
// 	require.NoError(t, err)

// 	received := make(chan *testMessage, 1)

// 	_, err = bus1.Subscribe(
// 		messaging.Route{
// 			Scope: messaging.ScopeGlobal,
// 			Area:  "test",
// 			Name:  "ping",
// 		}, func(ctx context.Context, msg messaging.Message) error {
// 			m, ok := msg.(*testMessage)
// 			require.True(t, ok)
// 			received <- m
// 			return nil
// 		})
// 	require.NoError(t, err)

// 	err = bus2.Notify(&testMessage{
// 		ID:   uuid.New(),
// 		Data: "hello world",
// 	})
// 	require.NoError(t, err)

// 	select {
// 	case msg := <-received:
// 		require.Equal(t, "hello world", msg.Data)
// 	case <-time.After(2 * time.Second):
// 		t.Fatal("did not receive message in time")
// 	}

// 	_ = bus1.Close()
// 	_ = bus2.Close()
// 	broker.Stop()
// }
