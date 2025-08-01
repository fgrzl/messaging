# Messaging Library Documentation

## Overview

The messaging library provides a unified interface for building scalable messaging applications with support for both asynchronous (fire-and-forget) and synchronous (request-response) messaging patterns.

## Package Structure

### Core Packages

- **messaging** - Core interfaces and types for messaging
- **messaging/pkg/natsbroker** - Embedded NATS broker implementation  
- **messaging/pkg/natsbus** - NATS message bus implementation
- **messaging/pkg/natsclaims** - JWT claims utilities for NATS

### Key Interfaces

- `MessageBus` - Main interface for publishing and subscribing to messages
- `Broker` - Interface for starting and stopping a message broker
- `Processor` - Provides lifecycle management for message handlers

### Message Types

- `Message` - Represents an event-style message that does not expect a response
- `Request` - Represents a message that expects a response  
- `Response` - A polymorphic response to a request message

### Routing

Messages are routed using `Route` objects that define:
- **Scope** - Visibility level (Global, Internal, Tenant, Inbox)
- **Area** - Logical domain or subsystem (e.g., "auth", "billing")
- **Name** - Specific message or event type (e.g., "user.created")
- **ID** - Optional UUID for tenant/inbox-scoped routes

## Usage Examples

See the main README.md for comprehensive usage examples including:
- Basic message publishing and subscribing
- Request-response patterns
- Using the embedded NATS broker
- Context-based message tracing
- Multi-tenant messaging

## Testing

All packages include comprehensive unit tests with behavioral naming patterns. Run tests with:

```bash
go test ./... -v
```

## Configuration

The broker can be configured via environment variables or programmatically. See `pkg/natsbroker.BrokerOptions` for available options.