package messaging

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegrationUsagePatterns demonstrates the expected usage patterns from the issue
func TestIntegrationUsagePatterns(t *testing.T) {
	t.Run("ShouldDemonstrateExpectedUsageInHandler", func(t *testing.T) {
		// Arrange - simulate a request handler scenario
		user := &MockPrincipal{SubjectValue: "user123"}
		correlationID := uuid.New()
		causationID := uuid.New()

		// Create a context with tracing and user
		ctx := ContextWithTracing(context.Background(), correlationID, causationID)
		ctx = ContextWithUserPrincipal(ctx, user)

		// Act & Assert - demonstrate safe access patterns

		// Get user principal safely
		if retrievedUser, ok := GetUserPrincipal(ctx); ok {
			assert.Equal(t, "user123", retrievedUser.Subject())
		} else {
			t.Fatal("Expected user principal to be found")
		}

		// Get correlation ID for logging
		if corrID, ok := GetCorrelationIDString(ctx); ok {
			assert.Equal(t, correlationID.String(), corrID)
		} else {
			t.Fatal("Expected correlation ID to be found")
		}

		// Get causation ID for logging
		if causID, ok := GetCausationIDString(ctx); ok {
			assert.Equal(t, causationID.String(), causID)
		} else {
			t.Fatal("Expected causation ID to be found")
		}
	})

	t.Run("ShouldDemonstrateMustVariantsWhenValuesExpected", func(t *testing.T) {
		// Arrange
		user := &MockPrincipal{SubjectValue: "user456"}
		correlationID := uuid.New()
		causationID := uuid.New()

		ctx := ContextWithTracing(context.Background(), correlationID, causationID)
		ctx = ContextWithUserPrincipal(ctx, user)

		// Act & Assert - use Must variants when you expect values to be present
		retrievedUser := MustGetUserPrincipal(ctx)
		assert.Equal(t, "user456", retrievedUser.Subject())

		retrievedCorrID := MustGetCorrelationID(ctx)
		assert.Equal(t, correlationID, retrievedCorrID)

		retrievedCausID := MustGetCausationID(ctx)
		assert.Equal(t, causationID, retrievedCausID)
	})

	t.Run("ShouldWorkWithBothContextTypesSeamlessly", func(t *testing.T) {
		// Arrange
		user := &MockPrincipal{SubjectValue: "user789"}
		correlationID := uuid.New()
		causationID := uuid.New()

		// Start with regular context
		regularCtx := ContextWithTracing(context.Background(), correlationID, causationID)
		regularCtx = ContextWithUserPrincipal(regularCtx, user)

		// Create another context with the same values
		sameCtx := ContextWithTracing(context.Background(), correlationID, causationID)
		sameCtx = ContextWithUserPrincipal(sameCtx, user)

		// Act & Assert - both should work the same way

		// Test with regular context
		regularUser, ok := GetUserPrincipal(regularCtx)
		require.True(t, ok)
		assert.Equal(t, "user789", regularUser.Subject())

		regularCorrStr, ok := GetCorrelationIDString(regularCtx)
		require.True(t, ok)
		assert.Equal(t, correlationID.String(), regularCorrStr)

		// Test with same context
		sameUser, ok := GetUserPrincipal(sameCtx)
		require.True(t, ok)
		assert.Equal(t, "user789", sameUser.Subject())

		sameCorrStr, ok := GetCorrelationIDString(sameCtx)
		require.True(t, ok)
		assert.Equal(t, correlationID.String(), sameCorrStr)
	})

	t.Run("ShouldHandleGracefullyWhenValuesAreMissing", func(t *testing.T) {
		// Arrange
		ctx := context.Background() // Empty context

		// Act & Assert - safe access should return false
		_, userFound := GetUserPrincipal(ctx)
		assert.False(t, userFound)

		_, corrFound := GetCorrelationIDString(ctx)
		assert.False(t, corrFound)

		_, causFound := GetCausationIDString(ctx)
		assert.False(t, causFound)

		// Must variants should panic
		assert.Panics(t, func() { MustGetUserPrincipal(ctx) })
		assert.Panics(t, func() { MustGetCorrelationID(ctx) })
		assert.Panics(t, func() { MustGetCausationID(ctx) })
	})
}

// TestBackwardCompatibility ensures existing functionality is preserved
func TestBackwardCompatibility(t *testing.T) {
	t.Run("ShouldPreserveExistingTracingFunctionality", func(t *testing.T) {
		// Arrange
		correlationID := uuid.New()
		causationID := uuid.New()
		ctx := ContextWithTracing(context.Background(), correlationID, causationID)

		// Act & Assert - existing functions should still work
		retrievedCorr, retrievedCaus := GetTracing(ctx)
		assert.Equal(t, correlationID, retrievedCorr)
		assert.Equal(t, causationID, retrievedCaus)

		assert.Equal(t, correlationID, GetCorrelationID(ctx))
		assert.Equal(t, causationID, GetCausationID(ctx))
	})

	t.Run("ShouldPreserveExistingMessageContextStructure", func(t *testing.T) {
		// Arrange
		user := &MockPrincipal{SubjectValue: "test"}
		ctx := context.Background()

		// Act - use standard context with user principal
		ctxWithUser := ContextWithUserPrincipal(ctx, user)

		// Assert - helpers should work with standard context
		retrievedUser, ok := GetUserPrincipal(ctxWithUser)
		require.True(t, ok)
		assert.Equal(t, user, retrievedUser)
	})
}
