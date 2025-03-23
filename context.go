package messaging

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const (
	correlationKey contextKey = "correlation_id"
	causationKey   contextKey = "causation_id"
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
