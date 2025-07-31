package messaging

import (
	"context"

	"github.com/google/uuid"
)

// ContextKey is a type for context keys to avoid collisions.
type ContextKey string

const (
	// correlationKey is the context key for correlation ID.
	correlationKey ContextKey = "correlation_id"
	// causationKey is the context key for causation ID.
	causationKey   ContextKey = "causation_id"
	// tenantKey is the context key for tenant ID.
	tenantKey      ContextKey = "tenant_id"
)

// ContextWithTracing adds correlation and causation IDs to the context for message tracing.
func ContextWithTracing(ctx context.Context, correlationID, causationID uuid.UUID) context.Context {
	ctx = context.WithValue(ctx, correlationKey, correlationID)
	ctx = context.WithValue(ctx, causationKey, causationID)
	return ctx
}

// GetTracing retrieves both correlation and causation IDs from the context.
func GetTracing(ctx context.Context) (correlationID, causationID uuid.UUID) {
	correlationID = GetCorrelationID(ctx)
	causationID = GetCausationID(ctx)
	return
}

// GetCorrelationID retrieves the correlation ID from the context.
func GetCorrelationID(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(correlationKey).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

// GetCausationID retrieves the causation ID from the context.
func GetCausationID(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(causationKey).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

// GetTenantID retrieves the tenant ID from the context.
func GetTenantID(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(tenantKey).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

// ContextWithTenant adds a tenant ID to the context for multi-tenant messaging.
func ContextWithTenant(ctx context.Context, tenantID uuid.UUID) context.Context {
	ctx = context.WithValue(ctx, tenantKey, tenantID)
	return ctx
}
