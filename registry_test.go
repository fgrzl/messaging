package messaging

import (
	"testing"

	"github.com/fgrzl/json/polymorphic/testkit"
)

func TestShouldRegisterPolymorphicDomainRequests(t *testing.T) {
	testkit.TestPolymorphicRegistrations(t, map[string]any{
		"messaging://bool_result":  &BoolResult{},
		"messaging://err_response": &ErrorResponse{},
		"messaging://accepted":     &Accepted{},
	})
}
