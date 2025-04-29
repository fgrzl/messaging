package messaging

import (
	"context"

	"github.com/google/uuid"
)

type ContextKey string

const (
	correlationKey ContextKey = "correlation_id"
	causationKey   ContextKey = "causation_id"
	tenantKey      ContextKey = "tenant_id"
)

func ContextWithTracing(ctx context.Context, correlationID, causationID uuid.UUID) context.Context {
	ctx = context.WithValue(ctx, correlationKey, correlationID)
	ctx = context.WithValue(ctx, causationKey, causationID)
	return ctx
}

func GetTracing(ctx context.Context) (correlationID, causationID uuid.UUID) {
	correlationID = GetCorrelationID(ctx)
	causationID = GetCausationID(ctx)
	return
}

func GetCorrelationID(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(correlationKey).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

func GetCausationID(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(causationKey).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

func GetTenantID(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(tenantKey).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

func ContextWithTenant(ctx context.Context, tenantID uuid.UUID) context.Context {
	ctx = context.WithValue(ctx, tenantKey, tenantID)
	return ctx
}
