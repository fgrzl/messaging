# In-Memory Message Bus (`membus`)

The `membus` package provides a lightweight, in-memory implementation of the `messaging.MessageBus` and `messaging.MessageBusFactory` interfaces. It's designed for local development, testing, and scenarios where a full NATS broker is not needed.

## Features

- **Fire-and-Forget Messaging**: Send one-way messages to multiple handlers
- **Request-Response Messaging**: Send requests and wait for responses with timeout support
- **Context Support**: Full context propagation for cancellation and timeouts
- **Thread-Safe**: Concurrent handler registration and message processing
- **Lightweight**: No external dependencies beyond the messaging interfaces

## Usage

### Creating a Message Bus

```go
import "github.com/fgrzl/messaging/pkg/membus"

// Create a new bus instance
bus := membus.New()
defer bus.Close()

// Or use the factory
factory := membus.NewFactory()
defer factory.Close()
bus, _ := factory.Get(ctx)
```

### Sending One-Way Messages

```go
// Define a message type
type UserCreated struct {
	UserID string
	Name   string
}

func (m *UserCreated) GetDiscriminator() string {
	return "app://user.created"
}

func (m *UserCreated) GetRoute() messaging.Route {
	return messaging.NewGlobalRoute("user", "created")
}

// Subscribe to messages
route := messaging.NewGlobalRoute("user", "created")
sub, _ := bus.Subscribe(route, func(ctx context.Context, msg messaging.Message) error {
	userCreated := msg.(*UserCreated)
	// Handle the message
	return nil
})
defer sub.Unsubscribe()

// Send a message
msg := &UserCreated{UserID: "123", Name: "Alice"}
bus.Notify(msg)
```

### Request-Response Messaging

```go
// Define request and response types
type GetUser struct {
	UserID string
}

func (r *GetUser) GetDiscriminator() string {
	return "app://get.user"
}

func (r *GetUser) GetRoute() messaging.Route {
	return messaging.NewGlobalRoute("user", "get")
}

// Register a request handler
route := messaging.NewGlobalRoute("user", "get")
sub, _ := bus.SubscribeRequest(route, func(ctx context.Context, req messaging.Request) (messaging.Response, error) {
	getUserReq := req.(*GetUser)
	// Handle the request and return a response
	return &UserResponse{UserID: getUserReq.UserID, Name: "Alice"}, nil
})
defer sub.Unsubscribe()

// Send a request and wait for response
req := &GetUser{UserID: "123"}
resp, _ := bus.Request(req, 5*time.Second)
userResp := resp.(*UserResponse)
```

## Implementation Details

- **Concurrent Handler Execution**: One-way message handlers are executed concurrently to all subscribed handlers
- **Single Request Handler**: Only one request handler can be registered per route
- **Context Propagation**: Both notify and request methods support context for cancellation and timeouts
- **Cleanup**: Calling `Close()` cleans up all handlers and marks the bus as closed
