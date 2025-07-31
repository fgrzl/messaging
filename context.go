package messaging

import (
	"context"
	"encoding/json"

	"github.com/fgrzl/claims"
	"github.com/google/uuid"
)

// MessageContext wraps a context and includes user claims for authorization-aware handlers.
type MessageContext struct {
	context.Context

	// User represents the authenticated principal associated with this message.
	User claims.Principal
}

// SerializablePrincipal represents the serializable fields of a Principal
// This is used for serializing user principal data in message headers
type SerializablePrincipal struct {
	Subject        string   `json:"subject"`
	Issuer         string   `json:"issuer"`
	Audience       []string `json:"audience"`
	Scopes         []string `json:"scopes"`
	Roles          []string `json:"roles"`
	Email          string   `json:"email"`
	Username       string   `json:"username"`
	ExpirationTime int64    `json:"exp"`
	NotBefore      int64    `json:"nbf"`
	IssuedAt       int64    `json:"iat"`
	JWTI           string   `json:"jti"`
}

// ToSerializablePrincipal converts a claims.Principal to a SerializablePrincipal
func ToSerializablePrincipal(p claims.Principal) SerializablePrincipal {
	return SerializablePrincipal{
		Subject:        p.Subject(),
		Issuer:         p.Issuer(),
		Audience:       p.Audience(),
		Scopes:         p.Scopes(),
		Roles:          p.Roles(),
		Email:          p.Email(),
		Username:       p.Username(),
		ExpirationTime: p.ExpirationTime(),
		NotBefore:      p.NotBefore(),
		IssuedAt:       p.IssuedAt(),
		JWTI:           p.JWTI(),
	}
}

// SerializePrincipal serializes a claims.Principal to JSON string
func SerializePrincipal(p claims.Principal) (string, error) {
	serializable := ToSerializablePrincipal(p)
	data, err := json.Marshal(serializable)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DeserializePrincipalFields deserializes a JSON string to SerializablePrincipal
func DeserializePrincipalFields(jsonStr string) (*SerializablePrincipal, error) {
	var sp SerializablePrincipal
	if err := json.Unmarshal([]byte(jsonStr), &sp); err != nil {
		return nil, err
	}
	return &sp, nil
}

// ReconstructedPrincipal implements claims.Principal from deserialized data
type ReconstructedPrincipal struct {
	fields SerializablePrincipal
}

// NewReconstructedPrincipal creates a new ReconstructedPrincipal from SerializablePrincipal
func NewReconstructedPrincipal(fields SerializablePrincipal) *ReconstructedPrincipal {
	return &ReconstructedPrincipal{fields: fields}
}

// Subject implements claims.Principal
func (r *ReconstructedPrincipal) Subject() string { return r.fields.Subject }

// Issuer implements claims.Principal
func (r *ReconstructedPrincipal) Issuer() string { return r.fields.Issuer }

// Audience implements claims.Principal
func (r *ReconstructedPrincipal) Audience() []string { return r.fields.Audience }

// ExpirationTime implements claims.Principal
func (r *ReconstructedPrincipal) ExpirationTime() int64 { return r.fields.ExpirationTime }

// NotBefore implements claims.Principal
func (r *ReconstructedPrincipal) NotBefore() int64 { return r.fields.NotBefore }

// IssuedAt implements claims.Principal
func (r *ReconstructedPrincipal) IssuedAt() int64 { return r.fields.IssuedAt }

// JWTI implements claims.Principal
func (r *ReconstructedPrincipal) JWTI() string { return r.fields.JWTI }

// Scopes implements claims.Principal
func (r *ReconstructedPrincipal) Scopes() []string { return r.fields.Scopes }

// Roles implements claims.Principal
func (r *ReconstructedPrincipal) Roles() []string { return r.fields.Roles }

// Email implements claims.Principal
func (r *ReconstructedPrincipal) Email() string { return r.fields.Email }

// Username implements claims.Principal
func (r *ReconstructedPrincipal) Username() string { return r.fields.Username }

// CustomClaim implements claims.Principal (returns nil for reconstructed principals)
func (r *ReconstructedPrincipal) CustomClaim(name string) claims.Claim { return nil }

// CustomClaimValue implements claims.Principal (returns empty string for reconstructed principals)
func (r *ReconstructedPrincipal) CustomClaimValue(name string) string { return "" }

// Claims implements claims.Principal (returns nil for reconstructed principals)
func (r *ReconstructedPrincipal) Claims() *claims.ClaimSet { return nil }

// DeserializePrincipal deserializes a JSON string to a claims.Principal
func DeserializePrincipal(jsonStr string) (claims.Principal, error) {
	fields, err := DeserializePrincipalFields(jsonStr)
	if err != nil {
		return nil, err
	}
	return NewReconstructedPrincipal(*fields), nil
}

// ContextKey is a type for context keys to avoid collisions.
type ContextKey string

const (
	// correlationKey is the context key for correlation ID.
	correlationKey ContextKey = "correlation_id"
	// causationKey is the context key for causation ID.
	causationKey ContextKey = "causation_id"
	// tenantKey is the context key for tenant ID.
	tenantKey ContextKey = "tenant_id"
	// userKey is the context key for user principal.
	userKey ContextKey = "user_principal"
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

// GetCorrelationIDString retrieves the correlation ID from context as a string.
// Returns the string representation of the UUID and a boolean indicating if it was found.
func GetCorrelationIDString(ctx context.Context) (string, bool) {
	if v, ok := ctx.Value(correlationKey).(uuid.UUID); ok && v != uuid.Nil {
		return v.String(), true
	}
	return "", false
}

// GetCausationIDString retrieves the causation ID from context as a string.
// Returns the string representation of the UUID and a boolean indicating if it was found.
func GetCausationIDString(ctx context.Context) (string, bool) {
	if v, ok := ctx.Value(causationKey).(uuid.UUID); ok && v != uuid.Nil {
		return v.String(), true
	}
	return "", false
}

// MustGetCorrelationID retrieves the correlation ID from context.
// Panics if the correlation ID is not found or is nil.
func MustGetCorrelationID(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(correlationKey).(uuid.UUID); ok && v != uuid.Nil {
		return v
	}
	panic("correlation ID not found in context")
}

// MustGetCausationID retrieves the causation ID from context.
// Panics if the causation ID is not found or is nil.
func MustGetCausationID(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(causationKey).(uuid.UUID); ok && v != uuid.Nil {
		return v
	}
	panic("causation ID not found in context")
}

// GetUserPrincipal retrieves the user principal from context.
// Works with both context.Context and *MessageContext types.
// Returns the principal and a boolean indicating if it was found.
func GetUserPrincipal(ctx context.Context) (claims.Principal, bool) {
	// Check if it's a MessageContext first
	if msgCtx, ok := ctx.(*MessageContext); ok {
		if msgCtx.User != nil {
			return msgCtx.User, true
		}
		// Fall back to checking the wrapped context
		ctx = msgCtx.Context
	}

	// Check if it's a regular context
	if c, ok := ctx.(context.Context); ok {
		if v, exists := c.Value(userKey).(claims.Principal); exists {
			return v, true
		}
	}

	return nil, false
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
	return context.WithValue(ctx, userKey, user)
}

// NewMessageContext creates a new MessageContext with the given context and user principal.
func NewMessageContext(ctx context.Context, user claims.Principal) *MessageContext {
	return &MessageContext{
		Context: ctx,
		User:    user,
	}
}
