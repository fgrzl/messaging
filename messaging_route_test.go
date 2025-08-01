package messaging

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewGlobalRoute(t *testing.T) {
	t.Run("ShouldCreateGlobalRouteWithAreaAndName", func(t *testing.T) {
		// Arrange
		area := "users"
		name := "created"

		// Act
		route := NewGlobalRoute(area, name)

		// Assert
		assert.Equal(t, ScopeGlobal, route.Scope)
		assert.Equal(t, area, route.Area)
		assert.Equal(t, name, route.Name)
		assert.Nil(t, route.ID)
	})
}

func TestNewInternalRoute(t *testing.T) {
	t.Run("ShouldCreateInternalRouteWithAreaAndName", func(t *testing.T) {
		// Arrange
		area := "auth"
		name := "validated"

		// Act
		route := NewInternalRoute(area, name)

		// Assert
		assert.Equal(t, ScopeInternal, route.Scope)
		assert.Equal(t, area, route.Area)
		assert.Equal(t, name, route.Name)
		assert.Nil(t, route.ID)
	})
}

func TestNewTenantRoute(t *testing.T) {
	t.Run("ShouldCreateTenantRouteWithAreaNameAndTenantID", func(t *testing.T) {
		// Arrange
		area := "billing"
		name := "payment.processed"
		tenantID := uuid.New()

		// Act
		route := NewTenantRoute(area, name, &tenantID)

		// Assert
		assert.Equal(t, ScopeTenant, route.Scope)
		assert.Equal(t, area, route.Area)
		assert.Equal(t, name, route.Name)
		assert.Equal(t, &tenantID, route.ID)
	})
}

func TestNewInboxRoute(t *testing.T) {
	t.Run("ShouldCreateInboxRouteWithAreaNameAndInboxID", func(t *testing.T) {
		// Arrange
		area := "direct"
		name := "message"
		inboxID := uuid.New()

		// Act
		route := NewInboxRoute(area, name, &inboxID)

		// Assert
		assert.Equal(t, ScopeInbox, route.Scope)
		assert.Equal(t, area, route.Area)
		assert.Equal(t, name, route.Name)
		assert.Equal(t, &inboxID, route.ID)
	})
}

func TestRoute_String(t *testing.T) {
	tests := []struct {
		name     string
		route    Route
		expected string
	}{
		{
			name:     "ShouldFormatGlobalRouteWithoutID",
			route:    NewGlobalRoute("users", "created"),
			expected: "global.users.created",
		},
		{
			name:     "ShouldFormatInternalRouteWithoutID",
			route:    NewInternalRoute("auth", "validated"),
			expected: "internal.auth.validated",
		},
		{
			name: "ShouldFormatTenantRouteWithID",
			route: func() Route {
				id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
				return NewTenantRoute("billing", "payment.processed", &id)
			}(),
			expected: "tenant.billing.payment.processed[123e4567-e89b-12d3-a456-426614174000]",
		},
		{
			name: "ShouldFormatInboxRouteWithID",
			route: func() Route {
				id := uuid.MustParse("987fcdeb-51a2-43e1-b456-426614174321")
				return NewInboxRoute("direct", "message", &id)
			}(),
			expected: "inbox.direct.message[987fcdeb-51a2-43e1-b456-426614174321]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := tt.route.String()

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScopeConstants(t *testing.T) {
	t.Run("ShouldHaveCorrectScopeValues", func(t *testing.T) {
		// Assert
		assert.Equal(t, Scope("global"), ScopeGlobal)
		assert.Equal(t, Scope("internal"), ScopeInternal)
		assert.Equal(t, Scope("tenant"), ScopeTenant)
		assert.Equal(t, Scope("inbox"), ScopeInbox)
	})
}
