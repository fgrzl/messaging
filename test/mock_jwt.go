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
	GetJWT      func() (string, error)
	SignFn      func(nonce []byte) ([]byte, error)
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

	return &MockJWTSetup{
		OperatorJWT: operatorJWT,
		AccountJWT:  accountJWT,
		AccountPub:  accountPub,
		GetJWT: func() (string, error) {
			userClaims := jwt.NewUserClaims(userPub)
			userClaims.Issuer = accountPub
			userClaims.Name = "test-user"
			userClaims.IssuedAt = time.Now().Unix()
			userClaims.Expires = time.Now().Add(time.Hour).Unix()
			return userClaims.Encode(accountKP)
		},
		SignFn: func(nonce []byte) ([]byte, error) {
			return userKP.Sign(nonce)
		},
	}, nil
}
