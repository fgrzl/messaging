package nats

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

func GetAccountResolver() *server.AccountResolver {

	resolver := &server.URLAccResolver{
		URL: "https://auth.yourcompany.com/accounts/jwt/",
		HTTPClient: &http.Client{
			Timeout:   2 * time.Second,
			Transport: &http.Transport{
				// Optional: TLS config or mTLS
			},
		},
		// Optionally customize HTTP requests (e.g., add auth header)
		Fetch: func(url string) (string, error) {
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("Authorization", "Bearer YOUR_NATS_SECRET")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return "", fmt.Errorf("JWT fetch failed: %s", resp.Status)
			}

			jwtBytes, _ := io.ReadAll(resp.Body)
			return string(jwtBytes), nil
		},
	}
	return resolver
}
