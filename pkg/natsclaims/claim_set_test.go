package natsclaims

import (
	"encoding/json"
	"testing"

	"github.com/fgrzl/claims"
	"github.com/nats-io/jwt/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetPermissions(t *testing.T) {
	t.Run("ShouldStorePermissionsInClaimSet", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")
		permissions := jwt.Permissions{
			Pub: jwt.Permission{Allow: []string{"users.>"}},
			Sub: jwt.Permission{Allow: []string{"events.>"}},
		}

		// Act
		SetPermissions(cs, permissions)

		// Assert
		claim, exists := cs.Get("nats.permissions")
		require.True(t, exists)

		var storedPermissions jwt.Permissions
		err := json.Unmarshal([]byte(claim.Value()), &storedPermissions)
		require.NoError(t, err)
		assert.Equal(t, permissions, storedPermissions)
	})
}

func TestGetPermissions(t *testing.T) {
	t.Run("ShouldRetrievePermissionsFromClaimSet", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")
		expectedPermissions := jwt.Permissions{
			Pub: jwt.Permission{Allow: []string{"users.>"}},
			Sub: jwt.Permission{Allow: []string{"events.>"}},
		}

		// Store permissions first
		SetPermissions(cs, expectedPermissions)

		// Act
		permissions, err := GetPermissions(cs)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedPermissions, permissions)
	})

	t.Run("ShouldReturnErrorWhenPermissionsNotFound", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")

		// Act
		permissions, err := GetPermissions(cs)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nats.permissions not found")
		assert.Equal(t, jwt.Permissions{}, permissions)
	})

	t.Run("ShouldReturnErrorWhenPermissionsAreInvalid", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")
		cs.Set("nats.permissions", "invalid-json")

		// Act
		permissions, err := GetPermissions(cs)

		// Assert
		assert.Error(t, err)
		assert.Equal(t, jwt.Permissions{}, permissions)
	})
}

func TestSetUserPub(t *testing.T) {
	t.Run("ShouldStoreUserPublicKeyInClaimSet", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")
		userPub := "UABC123XYZ"

		// Act
		SetUserPub(cs, userPub)

		// Assert
		value := cs.Value("nats.user_pub")
		assert.Equal(t, userPub, value)
	})
}

func TestGetUserPub(t *testing.T) {
	t.Run("ShouldRetrieveUserPublicKeyFromClaimSet", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")
		expectedUserPub := "UABC123XYZ"
		SetUserPub(cs, expectedUserPub)

		// Act
		userPub, err := GetUserPub(cs)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedUserPub, userPub)
	})

	t.Run("ShouldReturnErrorWhenUserPubNotFound", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")

		// Act
		userPub, err := GetUserPub(cs)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nats.user_pub not found")
		assert.Empty(t, userPub)
	})
}

func TestSetTags(t *testing.T) {
	t.Run("ShouldStoreTagsAsCommaSeparatedString", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")
		tags := []string{"admin", "user", "billing"}

		// Act
		SetTags(cs, tags...)

		// Assert
		value := cs.Value("nats.tags")
		assert.Equal(t, "admin,user,billing", value)
	})

	t.Run("ShouldHandleEmptyTags", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")

		// Act
		SetTags(cs)

		// Assert
		value := cs.Value("nats.tags")
		assert.Equal(t, "", value)
	})
}

func TestGetTags(t *testing.T) {
	t.Run("ShouldRetrieveTagsAsSlice", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")
		expectedTags := []string{"admin", "user", "billing"}
		SetTags(cs, expectedTags...)

		// Act
		tags := GetTags(cs)

		// Assert
		assert.Equal(t, expectedTags, tags)
	})

	t.Run("ShouldReturnEmptySliceWhenTagsNotFound", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")

		// Act
		tags := GetTags(cs)

		// Assert
		assert.Empty(t, tags)
		assert.NotNil(t, tags) // Should return empty slice, not nil
	})
}

func TestToUserClaims(t *testing.T) {
	t.Run("ShouldConvertClaimSetToUserClaims", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")
		cs.SetEmail("test@example.com")
		accountPub := "AABC123XYZ"
		userPub := "UABC123XYZ"

		SetUserPub(cs, userPub)
		SetTags(cs, "admin", "user")
		permissions := jwt.Permissions{
			Pub: jwt.Permission{Allow: []string{"users.>"}},
			Sub: jwt.Permission{Allow: []string{"events.>"}},
		}
		SetPermissions(cs, permissions)

		// Act
		userClaims, err := ToUserClaims(cs, accountPub)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, cs.Subject(), userClaims.Subject) // Subject comes from claimSet, not userPub
		assert.Equal(t, cs.Username(), userClaims.Name)
		assert.Equal(t, accountPub, userClaims.Issuer)
		assert.Equal(t, permissions, userClaims.Permissions)
		assert.Contains(t, userClaims.Tags, "admin")
		assert.Contains(t, userClaims.Tags, "user")

		// Check time fields are set reasonably
		assert.True(t, userClaims.IssuedAt > 0)
		assert.True(t, userClaims.Expires > userClaims.IssuedAt)
	})

	t.Run("ShouldReturnErrorWhenUserPubMissing", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")
		accountPub := "AABC123XYZ"

		// Act
		userClaims, err := ToUserClaims(cs, accountPub)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing nats.user_pub")
		assert.Nil(t, userClaims)
	})

	t.Run("ShouldHandleMissingPermissionsGracefully", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")
		accountPub := "AABC123XYZ"
		userPub := "UABC123XYZ"

		SetUserPub(cs, userPub)

		// Act
		userClaims, err := ToUserClaims(cs, accountPub)

		// Assert
		require.Error(t, err) // GetPermissions will return error but ToUserClaims still returns the claims
		require.NotNil(t, userClaims)
		assert.Equal(t, cs.Subject(), userClaims.Subject) // Subject comes from claimSet
	})

	t.Run("ShouldHandleEmptyTagsGracefully", func(t *testing.T) {
		// Arrange
		cs := claims.NewClaimsSet("test-subject")
		accountPub := "AABC123XYZ"
		userPub := "UABC123XYZ"

		SetUserPub(cs, userPub)
		permissions := jwt.Permissions{
			Pub: jwt.Permission{Allow: []string{"users.>"}},
		}
		SetPermissions(cs, permissions)

		// Act
		userClaims, err := ToUserClaims(cs, accountPub)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, cs.Subject(), userClaims.Subject) // Subject comes from claimSet
		assert.Empty(t, userClaims.Tags)                  // No tags should be set
	})
}
