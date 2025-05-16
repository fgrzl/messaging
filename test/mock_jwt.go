package test

import (
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

type MockJWTSetup struct {
	OperatorJWT string
	AccountJWT  string
	AccountPub  string
	UserJWT     string
	UserPubKey  string
	SignNonce   func(nonce []byte) ([]byte, error)
}

func GenerateMockTrustedOperatorSetup() (*MockJWTSetup, error) {
	// Operator
	operatorKP, _ := nkeys.CreateOperator()
	operatorPub, _ := operatorKP.PublicKey()
	operatorClaims := jwt.NewOperatorClaims(operatorPub)
	operatorClaims.Name = "test-operator"
	operatorJWT, err := operatorClaims.Encode(operatorKP)
	if err != nil {
		return nil, err
	}

	// Account
	accountKP, _ := nkeys.CreateAccount()
	accountPub, _ := accountKP.PublicKey()
	accountClaims := jwt.NewAccountClaims(accountPub)
	accountClaims.Issuer = operatorPub
	accountJWT, err := accountClaims.Encode(operatorKP)
	if err != nil {
		return nil, err
	}

	// User
	userKP, _ := nkeys.CreateUser()
	userPub, _ := userKP.PublicKey()
	userClaims := jwt.NewUserClaims(userPub)
	userClaims.Issuer = accountPub
	userClaims.Name = "test-user"
	userClaims.IssuedAt = time.Now().Unix()
	userClaims.Expires = time.Now().Add(time.Hour).Unix()
	userJWT, err := userClaims.Encode(accountKP)
	if err != nil {
		return nil, err
	}

	return &MockJWTSetup{
		OperatorJWT: operatorJWT,
		AccountJWT:  accountJWT,
		AccountPub:  accountPub,
		UserJWT:     userJWT,
		UserPubKey:  userPub,
		SignNonce: func(nonce []byte) ([]byte, error) {
			return userKP.Sign(nonce)
		},
	}, nil
}
