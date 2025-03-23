package broker

// import (
// 	"fmt"
// 	"log"
// 	"sync"

// 	"github.com/golang-jwt/jwt/v5"
// )

// // JWT secret key (replace with your key or use a public key for verification)
// var jwtSecret = []byte("your-secret-key")

// // JWTAuth is a custom authentication handler that validates JWTs.
// type JWTAuth struct {
// 	mu sync.Mutex
// }

// // Authenticate implements the server.ClientAuthentication interface.
// func (a *JWTAuth) Authenticate(c *server.) error {
// 	a.mu.Lock()
// 	defer a.mu.Unlock()

// 	// Extract the JWT from the connection token
// 	token := c.GetOpts().Token
// 	if token == "" {
// 		return fmt.Errorf("auth failed: no token provided")
// 	}

// 	// Validate the JWT
// 	claims, err := validateJWT(token)
// 	if err != nil {
// 		return fmt.Errorf("auth failed: invalid token: %v", err)
// 	}

// 	// Extract the tenant ID from JWT claims
// 	tenantID, ok := claims["tenant_id"].(string)
// 	if !ok {
// 		return fmt.Errorf("auth failed: missing tenant_id claim")
// 	}

// 	// Assign the client to a tenant-specific NATS account
// 	accountName := "tenant-" + tenantID
// 	acc, err := c.Server().LookupOrRegisterAccount(accountName)
// 	if err != nil {
// 		return fmt.Errorf("failed to register tenant account: %v", err)
// 	}

// 	c.RegisterWithAccount(acc)
// 	log.Printf("Client authenticated: Tenant %s", tenantID)
// 	return nil
// }

// // validateJWT parses and validates the JWT token
// func validateJWT(tokenString string) (jwt.MapClaims, error) {
// 	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
// 		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
// 			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
// 		}
// 		return jwtSecret, nil // Use public key if using asymmetric signing
// 	})

// 	if err != nil {
// 		return nil, err
// 	}

// 	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
// 		return claims, nil
// 	}

// 	return nil, fmt.Errorf("invalid token")
// }
