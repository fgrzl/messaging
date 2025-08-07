package messaging

import (
	"context"
	"testing"

	"github.com/fgrzl/telemetry"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGetCorrelationIDString(t *testing.T) {
	t.Run("ShouldReturnStringWhenCorrelationIDExists", func(t *testing.T) {
		// Arrange
		correlationID := uuid.New()
		ctx := telemetry.WithCorrelationID(context.Background(), correlationID)

		// Act
		result, ok := GetCorrelationIDString(ctx)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, correlationID.String(), result)
	})

	t.Run("ShouldReturnFalseWhenCorrelationIDDoesNotExist", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		// Act
		result, ok := GetCorrelationIDString(ctx)

		// Assert
		assert.False(t, ok)
		assert.Empty(t, result)
	})

	t.Run("ShouldReturnFalseWhenCorrelationIDIsNil", func(t *testing.T) {
		// Arrange
		ctx := telemetry.WithCorrelationID(context.Background(), uuid.Nil)

		// Act
		result, ok := GetCorrelationIDString(ctx)

		// Assert
		assert.False(t, ok)
		assert.Empty(t, result)
	})
}

func TestGetCausationIDString(t *testing.T) {
	t.Run("ShouldReturnStringWhenCausationIDExists", func(t *testing.T) {
		// Arrange
		causationID := uuid.New()
		ctx := telemetry.WithCausationID(context.Background(), causationID)

		// Act
		result, ok := GetCausationIDString(ctx)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, causationID.String(), result)
	})

	t.Run("ShouldReturnFalseWhenCausationIDDoesNotExist", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		// Act
		result, ok := GetCausationIDString(ctx)

		// Assert
		assert.False(t, ok)
		assert.Empty(t, result)
	})

	t.Run("ShouldReturnFalseWhenCausationIDIsNil", func(t *testing.T) {
		// Arrange
		ctx := telemetry.WithCausationID(context.Background(), uuid.Nil)

		// Act
		result, ok := GetCausationIDString(ctx)

		// Assert
		assert.False(t, ok)
		assert.Empty(t, result)
	})
}

func TestMustGetCorrelationID(t *testing.T) {
	t.Run("ShouldReturnCorrelationIDWhenExists", func(t *testing.T) {
		// Arrange
		correlationID := uuid.New()
		ctx := telemetry.WithCorrelationID(context.Background(), correlationID)

		// Act
		result := MustGetCorrelationID(ctx)

		// Assert
		assert.Equal(t, correlationID, result)
	})

	t.Run("ShouldPanicWhenCorrelationIDDoesNotExist", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		// Act & Assert
		assert.Panics(t, func() {
			MustGetCorrelationID(ctx)
		})
	})

	t.Run("ShouldPanicWhenCorrelationIDIsNil", func(t *testing.T) {
		// Arrange
		ctx := telemetry.WithCorrelationID(context.Background(), uuid.Nil)

		// Act & Assert
		assert.Panics(t, func() {
			MustGetCorrelationID(ctx)
		})
	})
}

func TestMustGetCausationID(t *testing.T) {
	t.Run("ShouldReturnCausationIDWhenExists", func(t *testing.T) {
		// Arrange
		causationID := uuid.New()
		ctx := telemetry.WithCausationID(context.Background(), causationID)

		// Act
		result := MustGetCausationID(ctx)

		// Assert
		assert.Equal(t, causationID, result)
	})

	t.Run("ShouldPanicWhenCausationIDDoesNotExist", func(t *testing.T) {
		// Arrange
		ctx := context.Background()

		// Act & Assert
		assert.Panics(t, func() {
			MustGetCausationID(ctx)
		})
	})

	t.Run("ShouldPanicWhenCausationIDIsNil", func(t *testing.T) {
		// Arrange
		ctx := telemetry.WithCausationID(context.Background(), uuid.Nil)

		// Act & Assert
		assert.Panics(t, func() {
			MustGetCausationID(ctx)
		})
	})
}
