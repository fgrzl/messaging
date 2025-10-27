[![ci](https://github.com/fgrzl/messaging/actions/workflows/ci.yml/badge.svg)](https://github.com/fgrzl/messaging/actions/workflows/ci.yml)
[![Dependabot Updates](https://github.com/fgrzl/messaging/actions/workflows/dependabot/dependabot-updates/badge.svg)](https://github.com/fgrzl/messaging/actions/workflows/dependabot/dependabot-updates)

# Messaging

A Go library for building scalable messaging applications with support for both asynchronous (fire-and-forget) and synchronous (request-response) messaging patterns. Built on top of NATS with JWT-based authentication and multi-tenant support.

## Features

- **Unified Messaging Interface**: Single API for pub-sub notifications and request-response patterns
- **Type-Safe Messaging**: Generic helpers for strongly-typed message handling
- **NATS Integration**: Built-in NATS broker and message bus implementations
- **JWT Authentication**: Secure messaging with JWT-based user and account authentication
- **Multi-Tenant Support**: Scope-based routing for tenant isolation
- **Message Tracing**: Built-in correlation and causation ID support
- **Processor Framework**: Lifecycle management for message processors

## Installation

```bash
go get github.com/fgrzl/messaging
```

## Quick Start

### Basic Message Publishing and Subscribing

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/fgrzl/messaging"
    "github.com/fgrzl/messaging/pkg/natsbus"
)

// Define your message type
type UserCreated struct {
    UserID string `json:"user_id"`
    Email  string `json:"email"`
}

func (e *UserCreated) GetDiscriminator() string {
    return "events://user/created"
}

func (e *UserCreated) GetRoute() messaging.Route {
    return messaging.NewGlobalRoute("users", "created")
}

func main() {
    // Connect to NATS
    bus, err := natsbus.NewBus("ws://localhost:9222", getJWT, signFn)
    if err != nil {
        log.Fatal(err)
    }
    defer bus.Close()

    // Subscribe to messages
    sub, err := messaging.Subscribe(bus, 
        messaging.NewGlobalRoute("users", "created"),
        func(ctx context.Context, msg *UserCreated) error {
            log.Printf("User created: %s (%s)", msg.UserID, msg.Email)
            return nil
        })
    if err != nil {
        log.Fatal(err)
    }
    defer sub.Unsubscribe()

    // Publish a message
    err = bus.Notify(&UserCreated{
        UserID: "123",
        Email:  "user@example.com",
    })
    if err != nil {
        log.Fatal(err)
    }

    time.Sleep(time.Second) // Wait for message delivery
}
```

### Request-Response Pattern

```go
// Define request and response types
type GetUserRequest struct {
    UserID string `json:"user_id"`
}

func (r *GetUserRequest) GetDiscriminator() string {
    return "requests://user/get"
}

func (r *GetUserRequest) GetRoute() messaging.Route {
    return messaging.NewGlobalRoute("users", "get")
}

type GetUserResponse struct {
    UserID string `json:"user_id"`
    Email  string `json:"email"`
}

func (r *GetUserResponse) GetDiscriminator() string {
    return "responses://user/get"
}

// Subscribe to handle requests
sub, err := messaging.SubscribeRequest(bus,
    messaging.NewGlobalRoute("users", "get"),
    func(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
        // Handle the request
        return &GetUserResponse{
            UserID: req.UserID,
            Email:  "user@example.com",
        }, nil
    })

// Send a request
response, err := messaging.SendRequest[*GetUserRequest, *GetUserResponse](
    bus,
    &GetUserRequest{UserID: "123"},
    5*time.Second,
)
```

### Using the Embedded NATS Broker

```go
import (
    "github.com/fgrzl/messaging/pkg/natsbroker"
)

func main() {
    // Configure and start embedded broker
    opts := natsbroker.BrokerOptions{
        Host:             "localhost",
        WebSocketPort:    9222,
        OperatorJWT:      operatorJWT,
        AccountJWT:       accountJWT,
        ReadinessTimeout: 5 * time.Second,
    }

    broker := natsbroker.NewBroker(ctx, opts)
    err := broker.Start(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer broker.Stop(ctx)

    // Now connect clients to ws://localhost:9222
}
```

## Architecture

### Core Components

- **MessageBus**: Main interface for publishing and subscribing to messages
- **Broker**: Manages the underlying message transport (NATS server)
- **Processor**: Provides lifecycle management for message handlers
- **Route**: Defines message routing with scope-based isolation

### Message Scopes

Messages are routed using scopes that provide different levels of isolation:

- **Global**: System-wide messages visible to all services
- **Internal**: Private service-to-service messages
- **Tenant**: Messages scoped to a specific tenant
- **Inbox**: Direct messages to a specific recipient

### Package Structure

```
github.com/fgrzl/messaging/
├── messaging.go          # Core interfaces and types
├── broker.go            # Broker interface
├── message_bus.go       # MessageBus interface and helpers
├── messaging_route.go   # Route definitions
├── context.go           # Context utilities
├── processor.go         # Processor framework
└── pkg/
    ├── natsbroker/      # Embedded NATS broker
    ├── natsbus/         # NATS message bus implementation
    └── natsclaims/      # JWT claims utilities
```

## Testing

This library includes comprehensive unit and integration tests with behavioral naming patterns.

### Running Tests

**Fast (unit tests only)**:
```bash
go test -short ./... -cover
```

**Full (including integration tests)**:
```bash
# Integration tests use embedded NATS brokers and work best when run per-package
go test ./pkg/natsbus -cover
go test ./pkg/natsbroker -cover
go test ./... -cover
```

**Coverage**: All packages maintain >75% test coverage:
- `messaging`: 80.0%
- `pkg/natsbroker`: 91.8%
- `pkg/natsbus`: 75.9%
- `pkg/natsclaims`: 100%
- `test`: 91.7%

See [docs/SPEC.md](docs/SPEC.md) for the complete behavioral specification.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality (follow behavioral naming: `TestShouldDoSomethingWhenCondition`)
5. Run tests: `go test -short ./...` (unit) or `go test ./...` (full)
6. Submit a pull request

## License

This project is licensed under the terms specified in the LICENSE file.
