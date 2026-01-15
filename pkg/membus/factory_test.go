package membus_test

import (
	"context"
	"testing"

	"github.com/fgrzl/messaging/pkg/membus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldReturnMessageBusFromFactory(t *testing.T) {
	// Arrange
	factory := membus.NewFactory()
	defer factory.Close()

	// Act
	bus, err := factory.Get(context.Background())

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, bus)
}

func TestShouldReturnSameBusInstanceFromFactory(t *testing.T) {
	// Arrange
	factory := membus.NewFactory()
	defer factory.Close()

	// Act
	bus1, err1 := factory.Get(context.Background())
	bus2, err2 := factory.Get(context.Background())

	// Assert
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, bus1, bus2)
}

func TestShouldCloseFactory(t *testing.T) {
	// Arrange
	factory := membus.NewFactory()

	// Act
	err := factory.Close()

	// Assert
	require.NoError(t, err)
}
