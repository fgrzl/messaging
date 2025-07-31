package messaging

import (
	"context"
	"testing"

	"github.com/fgrzl/claims"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockPrincipal is a test implementation of claims.Principal
type MockPrincipal struct {
	SubjectValue string
}

func (m *MockPrincipal) Subject() string                      { return m.SubjectValue }
func (m *MockPrincipal) Issuer() string                       { return "test-issuer" }
func (m *MockPrincipal) Audience() []string                   { return []string{"test-audience"} }
func (m *MockPrincipal) ExpirationTime() int64                { return 0 }
func (m *MockPrincipal) NotBefore() int64                     { return 0 }
func (m *MockPrincipal) IssuedAt() int64                      { return 0 }
func (m *MockPrincipal) JWTI() string                         { return "test-jwt-id" }
func (m *MockPrincipal) Scopes() []string                     { return []string{"read", "write"} }
func (m *MockPrincipal) Roles() []string                      { return []string{"user"} }
func (m *MockPrincipal) Email() string                        { return "test@example.com" }
func (m *MockPrincipal) Username() string                     { return "testuser" }
func (m *MockPrincipal) CustomClaim(name string) claims.Claim { return nil }
func (m *MockPrincipal) CustomClaimValue(name string) string  { return "" }
func (m *MockPrincipal) Claims() *claims.ClaimSet             { return nil }

func TestGetUserPrincipal(t *testing.T) {
	t.Run("ShouldReturnUserWhenFoundInRegularContext", func(t *testing.T) {
		// Arrange
		user := &MockPrincipal{SubjectValue: "test-user"}
		ctx := ContextWithUserPrincipal(context.Background(), user)

		// Act
		result, ok := GetUserPrincipal(ctx)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, user, result)
		assert.Equal(t, "test-user", result.Subject())
	})

	t.Run("ShouldReturnFalseWhenUserNotFound", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		// Act
		result, ok := GetUserPrincipal(ctx)

		// Assert
		assert.False(t, ok)
		assert.Nil(t, result)
	})
}

func TestMustGetUserPrincipal(t *testing.T) {
	t.Run("ShouldReturnUserWhenFound", func(t *testing.T) {
		// Arrange
		user := &MockPrincipal{SubjectValue: "test-user"}
		ctx := ContextWithUserPrincipal(context.Background(), user)

		// Act
		result := MustGetUserPrincipal(ctx)

		// Assert
		assert.Equal(t, user, result)
		assert.Equal(t, "test-user", result.Subject())
	})

	t.Run("ShouldPanicWhenUserNotFound", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		// Act & Assert
		assert.Panics(t, func() {
			MustGetUserPrincipal(ctx)
		})
	})
}

func TestContextWithUserPrincipal(t *testing.T) {
	t.Run("ShouldAddUserPrincipalToContext", func(t *testing.T) {
		// Arrange
		user := &MockPrincipal{SubjectValue: "test-user"}
		ctx := context.Background()

		// Act
		result := ContextWithUserPrincipal(ctx, user)

		// Assert
		retrievedUser, ok := GetUserPrincipal(result)
		require.True(t, ok)
		assert.Equal(t, user, retrievedUser)
		assert.Equal(t, "test-user", retrievedUser.Subject())
	})
}
