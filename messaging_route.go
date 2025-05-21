package messaging

import (
	"fmt"

	"github.com/google/uuid"
)

// Scope defines the visibility and access control level of a message route.
type Scope string

const (
	// ScopeGlobal indicates a message is globally visible to all services.
	ScopeGlobal Scope = "global"

	// ScopeInternal is reserved for intra-service or system-level messages.
	ScopeInternal Scope = "internal"

	// ScopeTenant limits the message to a specific tenant.
	ScopeTenant Scope = "tenant"

	// ScopeInbox targets a specific peer or session (e.g., direct or ephemeral messaging).
	ScopeInbox Scope = "inbox"
)

// Route defines how a message is routed within the system.
type Route struct {
	// Scope determines the visibility and routing behavior of the message.
	Scope Scope

	// Area represents the logical domain or subsystem (e.g., "auth", "billing").
	Area string

	// Name defines the specific message or event type (e.g., "user.created").
	Name string

	// ID is used for tenant- or inbox-scoped routes to identify the target entity.
	ID *uuid.UUID
}

// NewGlobalRoute creates a global-scoped route for system-wide messages.
func NewGlobalRoute(area, name string) Route {
	return Route{
		Scope: ScopeGlobal,
		Area:  area,
		Name:  name,
	}
}

// NewInternalRoute creates an internal-scoped route for private service messages.
func NewInternalRoute(area, name string) Route {
	return Route{
		Scope: ScopeInternal,
		Area:  area,
		Name:  name,
	}
}

// NewTenantRoute creates a tenant-scoped route restricted to a specific tenant ID.
func NewTenantRoute(area, name string, tenantID *uuid.UUID) Route {
	return Route{
		Scope: ScopeTenant,
		Area:  area,
		Name:  name,
		ID:    tenantID,
	}
}

// NewInboxRoute creates an inbox-scoped route for direct messaging to a specific recipient.
func NewInboxRoute(area, name string, inboxID *uuid.UUID) Route {
	return Route{
		Scope: ScopeInbox,
		Area:  area,
		Name:  name,
		ID:    inboxID,
	}
}

// String returns a formatted string representation of the route for debugging or logging.
func (r Route) String() string {
	if r.ID != nil {
		return fmt.Sprintf("%s.%s.%s[%s]", r.Scope, r.Area, r.Name, r.ID)
	}
	return fmt.Sprintf("%s.%s.%s", r.Scope, r.Area, r.Name)
}
