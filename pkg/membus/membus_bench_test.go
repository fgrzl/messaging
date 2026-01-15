package membus_test

import (
	"context"
	"testing"
	"time"

	"github.com/fgrzl/messaging"
	"github.com/fgrzl/messaging/pkg/membus"
)

func BenchmarkNotifySingleHandler(b *testing.B) {
	bus := membus.New()
	defer bus.Close()

	route := messaging.NewGlobalRoute("bench", "notify")
	bus.Subscribe(route, func(ctx context.Context, msg messaging.Message) error {
		return nil
	})

	msg := &testMessage{Value: "test", route: route}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Notify(msg)
	}
}

func BenchmarkNotifyMultipleHandlers(b *testing.B) {
	bus := membus.New()
	defer bus.Close()

	route := messaging.NewGlobalRoute("bench", "notify")
	for i := 0; i < 10; i++ {
		bus.Subscribe(route, func(ctx context.Context, msg messaging.Message) error {
			return nil
		})
	}

	msg := &testMessage{Value: "test", route: route}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Notify(msg)
	}
}

func BenchmarkNotifyWithContext(b *testing.B) {
	bus := membus.New()
	defer bus.Close()

	route := messaging.NewGlobalRoute("bench", "notify")
	bus.Subscribe(route, func(ctx context.Context, msg messaging.Message) error {
		return nil
	})

	msg := &testMessage{Value: "test", route: route}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.NotifyWithContext(ctx, msg)
	}
}

func BenchmarkRequest(b *testing.B) {
	bus := membus.New()
	defer bus.Close()

	route := messaging.NewGlobalRoute("bench", "request")
	bus.SubscribeRequest(route, func(ctx context.Context, req messaging.Request) (messaging.Response, error) {
		return &testResponse{Result: "ok"}, nil
	})

	req := &testRequest2{Query: "test", route: route}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Request(req, 5*time.Second)
	}
}

func BenchmarkRequestWithContext(b *testing.B) {
	bus := membus.New()
	defer bus.Close()

	route := messaging.NewGlobalRoute("bench", "request")
	bus.SubscribeRequest(route, func(ctx context.Context, req messaging.Request) (messaging.Response, error) {
		return &testResponse{Result: "ok"}, nil
	})

	req := &testRequest2{Query: "test", route: route}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.RequestWithContext(ctx, req, 5*time.Second)
	}
}

func BenchmarkSubscribe(b *testing.B) {
	bus := membus.New()
	defer bus.Close()

	handler := func(ctx context.Context, msg messaging.Message) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create unique routes to avoid conflicts
		route := messaging.NewGlobalRoute("bench", "sub")
		bus.Subscribe(route, handler)
	}
}

func BenchmarkSubscribeRequest(b *testing.B) {
	handler := func(ctx context.Context, req messaging.Request) (messaging.Response, error) {
		return &testResponse{Result: "ok"}, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a new bus for each subscription to avoid route conflicts
		bus := membus.New()
		defer bus.Close()
		route := messaging.NewGlobalRoute("bench", "subreq")
		bus.SubscribeRequest(route, handler)
	}
}

func BenchmarkFactoryGet(b *testing.B) {
	factory := membus.NewFactory()
	defer factory.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		factory.Get(ctx)
	}
}

func BenchmarkNotifyParallel(b *testing.B) {
	bus := membus.New()
	defer bus.Close()

	route := messaging.NewGlobalRoute("bench", "notify-parallel")
	bus.Subscribe(route, func(ctx context.Context, msg messaging.Message) error {
		return nil
	})

	msg := &testMessage{Value: "test", route: route}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bus.Notify(msg)
		}
	})
}

func BenchmarkRequestParallel(b *testing.B) {
	bus := membus.New()
	defer bus.Close()

	route := messaging.NewGlobalRoute("bench", "request-parallel")
	bus.SubscribeRequest(route, func(ctx context.Context, req messaging.Request) (messaging.Response, error) {
		return &testResponse{Result: "ok"}, nil
	})

	req := &testRequest2{Query: "test", route: route}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bus.Request(req, 5*time.Second)
		}
	})
}
