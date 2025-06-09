package natsclaims

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fgrzl/claims"
	"github.com/nats-io/jwt/v2"
)

func SetPermissions(cs *claims.ClaimSet, p jwt.Permissions) {
	b, _ := json.Marshal(p) // intentionally ignoring error
	cs.Set("nats.permissions", string(b))
}

func GetPermissions(cs *claims.ClaimSet) (jwt.Permissions, error) {
	rawClaim, ok := cs.Get("nats.permissions")
	if !ok {
		return jwt.Permissions{}, errors.New("nats.permissions not found")
	}
	var p jwt.Permissions
	err := json.Unmarshal([]byte(rawClaim.Value()), &p)
	return p, err
}

func SetUserPub(cs *claims.ClaimSet, userPub string) {
	cs.Set("nats.user_pub", userPub)
}

func GetUserPub(cs *claims.ClaimSet) (string, error) {
	claim, ok := cs.Get("nats.user_pub")
	if !ok {
		return "", errors.New("nats.user_pub not found")
	}
	return claim.Value(), nil
}

func SetTags(cs *claims.ClaimSet, tags ...string) {
	cs.Set("nats.tags", strings.Join(tags, ","))
}

func GetTags(cs *claims.ClaimSet) []string {
	claim, ok := cs.Get("nats.tags")
	if !ok {
		return make([]string, 0)
	}
	return claim.Values(",")
}

func ToUserClaims(claimSet *claims.ClaimSet, accountPub string) (*jwt.UserClaims, error) {

	userPub, err := GetUserPub(claimSet)
	if err != nil {
		return nil, fmt.Errorf("missing nats.user_pub %w", err)
	}
	userClaims := jwt.NewUserClaims(userPub)
	userClaims.Subject = claimSet.Subject()
	userClaims.Name = claimSet.Username()
	userClaims.Issuer = accountPub
	userClaims.IssuedAt = time.Now().Unix()
	userClaims.Expires = time.Now().Add(1 * time.Hour).Unix()

	tags := GetTags(claimSet)

	if len(tags) > 0 {
		userClaims.Tags.Add(tags...)
	}

	permissions, err := GetPermissions(claimSet)
	if err != nil {
		return userClaims, err
	}

	userClaims.Permissions = permissions

	return userClaims, nil
}
