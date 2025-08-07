package messaging

import (
	"context"

	"github.com/fgrzl/claims"
	"github.com/fgrzl/telemetry"
	"github.com/google/uuid"
)

// ContextKey is a type for context keys to avoid collisions.
// tenantKeyType is an unexported struct type for context keys to avoid collisions.
type tenantKeyType struct{}

// tenantKey is the context key for tenant ID.
var tenantKey = tenantKeyType{}

// ContextWithTracing adds correlation and causation IDs to the context for message tracing.
func ContextWithTracing(ctx context.Context, correlationID, causationID uuid.UUID) context.Context {
	ctx = telemetry.WithCorrelationID(ctx, correlationID)
	ctx = telemetry.WithCausationID(ctx, causationID)
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
	if c, ok := telemetry.CorrelationIDFromContext(ctx); ok {
		return c
	}
	return uuid.Nil
}

// GetCausationID retrieves the causation ID from the context.
func GetCausationID(ctx context.Context) uuid.UUID {
	if c, ok := telemetry.CausationIDFromContext(ctx); ok {
		return c
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

// GetCorrelationIDString retrieves the correlation ID from context as a string.
// Returns the string representation of the UUID and a boolean indicating if it was found.
// GetCorrelationIDString retrieves the correlation ID from context as a string using telemetry.
// Returns the string representation of the UUID and a boolean indicating if it was found.
func GetCorrelationIDString(ctx context.Context) (string, bool) {
	if v, ok := telemetry.CorrelationIDFromContext(ctx); ok && v != uuid.Nil {
		return v.String(), true
	}
	return "", false
}

// GetCausationIDString retrieves the causation ID from context as a string.
// Returns the string representation of the UUID and a boolean indicating if it was found.
// GetCausationIDString retrieves the causation ID from context as a string using telemetry.
// Returns the string representation of the UUID and a boolean indicating if it was found.
func GetCausationIDString(ctx context.Context) (string, bool) {
	if v, ok := telemetry.CausationIDFromContext(ctx); ok && v != uuid.Nil {
		return v.String(), true
	}
	return "", false
}

// MustGetCorrelationID retrieves the correlation ID from context.
// Panics if the correlation ID is not found or is nil.
// MustGetCorrelationID retrieves the correlation ID from context using telemetry.
// Panics if the correlation ID is not found or is nil.
func MustGetCorrelationID(ctx context.Context) uuid.UUID {
	if v, ok := telemetry.CorrelationIDFromContext(ctx); ok && v != uuid.Nil {
		return v
	}
	panic("correlation ID not found in context")
}

// MustGetCausationID retrieves the causation ID from context.
// Panics if the causation ID is not found or is nil.
// MustGetCausationID retrieves the causation ID from context using telemetry.
// Panics if the causation ID is not found or is nil.
func MustGetCausationID(ctx context.Context) uuid.UUID {
	if v, ok := telemetry.CausationIDFromContext(ctx); ok && v != uuid.Nil {
		return v
	}
	panic("causation ID not found in context")
}

// GetUserPrincipal retrieves the user principal from context.
// Returns the principal and a boolean indicating if it was found.
func GetUserPrincipal(ctx context.Context) (claims.Principal, bool) {
	return claims.UserFromContext(ctx)
}

// MustGetUserPrincipal retrieves the user principal from context.
// Panics if the user principal is not found.
func MustGetUserPrincipal(ctx context.Context) claims.Principal {
	if user, ok := GetUserPrincipal(ctx); ok {
		return user
	}
	panic("user principal not found in context")
}

// ContextWithUserPrincipal adds a user principal to the context.
func ContextWithUserPrincipal(ctx context.Context, user claims.Principal) context.Context {
	return claims.WithUser(ctx, user)
}
