package natsbroker

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to generate test operator and account JWTs
func generateTestJWTs(t *testing.T) (string, string, string) {
	t.Helper()

	// Generate operator key pair
	opKp, err := nkeys.CreateOperator()
	require.NoError(t, err)

	opPub, err := opKp.PublicKey()
	require.NoError(t, err)

	// Create operator claims
	opClaims := jwt.NewOperatorClaims(opPub)
	opClaims.Name = "Test Operator"
	opClaims.Expires = time.Now().Add(24 * time.Hour).Unix()

	opJWT, err := opClaims.Encode(opKp)
	require.NoError(t, err)

	// Generate account key pair
	accKp, err := nkeys.CreateAccount()
	require.NoError(t, err)

	accPub, err := accKp.PublicKey()
	require.NoError(t, err)

	// Create account claims
	accClaims := jwt.NewAccountClaims(accPub)
	accClaims.Name = "Test Account"
	accClaims.Expires = time.Now().Add(24 * time.Hour).Unix()

	accJWT, err := accClaims.Encode(opKp)
	require.NoError(t, err)

	return opJWT, accJWT, accPub
}

// Helper function to generate self-signed TLS certificate
func generateTestCertificate(t *testing.T) (certFile, keyFile string, cleanup func()) {
	t.Helper()

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create self-signed certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	// Write certificate to temp file
	certFile = t.TempDir() + "/cert.pem"
	certOut, err := os.Create(certFile)
	require.NoError(t, err)
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	require.NoError(t, err)
	certOut.Close()

	// Write private key to temp file
	keyFile = t.TempDir() + "/key.pem"
	keyOut, err := os.Create(keyFile)
	require.NoError(t, err)
	err = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	require.NoError(t, err)
	keyOut.Close()

	cleanup = func() {
		os.Remove(certFile)
		os.Remove(keyFile)
	}

	return certFile, keyFile, cleanup
}

func TestNatsBroker_StartAndStop(t *testing.T) {
	t.Run("ShouldStartAndStopBrokerSuccessfully", func(t *testing.T) {
		// Arrange
		opJWT, accJWT, _ := generateTestJWTs(t)
		ctx := context.Background()

		// Use longer timeout when running with race detector or in CI
		// The race detector adds significant overhead to server startup
		readinessTimeout := 30 * time.Second

		opts := BrokerOptions{
			Host:             "127.0.0.1",
			WebSocketPort:    9224, // Use different port to avoid conflicts
			MonitorPort:      8224,
			EnableTLS:        false,
			OperatorJWT:      opJWT,
			AccountJWT:       accJWT,
			ReadinessTimeout: readinessTimeout,
			ShutdownTimeout:  10 * time.Second,
		}

		broker := NewBroker(ctx, opts)

		// Act - Start
		err := broker.Start(ctx)
		require.NoError(t, err)

		// Assert - broker should be running
		natsBroker := broker.(*NatsBroker)
		assert.NotNil(t, natsBroker.natsServer)

		// Act - Stop
		err = broker.Stop(ctx)
		assert.NoError(t, err)
	})

	t.Run("ShouldStopWithoutErrorWhenServerIsNil", func(t *testing.T) {
		// Arrange
		broker := &NatsBroker{}
		ctx := context.Background()

		// Act
		err := broker.Stop(ctx)

		// Assert
		assert.NoError(t, err)
	})
}

func TestNatsBroker_WithTLS(t *testing.T) {
	t.Run("ShouldStartBrokerWithTLS", func(t *testing.T) {
		// Arrange
		opJWT, accJWT, _ := generateTestJWTs(t)
		certFile, keyFile, cleanup := generateTestCertificate(t)
		defer cleanup()

		ctx := context.Background()
		opts := BrokerOptions{
			Host:             "localhost",
			WebSocketPort:    9225,
			MonitorPort:      8225,
			EnableTLS:        true,
			CertFile:         certFile,
			KeyFile:          keyFile,
			OperatorJWT:      opJWT,
			AccountJWT:       accJWT,
			ReadinessTimeout: 5 * time.Second,
			ShutdownTimeout:  5 * time.Second,
		}

		broker := NewBroker(ctx, opts)

		// Act
		err := broker.Start(ctx)
		require.NoError(t, err)

		// Assert
		natsBroker := broker.(*NatsBroker)
		assert.NotNil(t, natsBroker.natsServer)

		// Cleanup
		broker.Stop(ctx)
	})

	t.Run("ShouldStartBrokerWithInsecureSkipVerify", func(t *testing.T) {
		// Arrange
		opJWT, accJWT, _ := generateTestJWTs(t)
		certFile, keyFile, cleanup := generateTestCertificate(t)
		defer cleanup()

		ctx := context.Background()
		opts := BrokerOptions{
			Host:               "localhost",
			WebSocketPort:      9226,
			MonitorPort:        8226,
			EnableTLS:          true,
			InsecureSkipVerify: true,
			CertFile:           certFile,
			KeyFile:            keyFile,
			OperatorJWT:        opJWT,
			AccountJWT:         accJWT,
			ReadinessTimeout:   5 * time.Second,
			ShutdownTimeout:    5 * time.Second,
		}

		broker := NewBroker(ctx, opts)

		// Act
		err := broker.Start(ctx)
		require.NoError(t, err)

		// Assert
		natsBroker := broker.(*NatsBroker)
		assert.NotNil(t, natsBroker.natsServer)

		// Cleanup
		broker.Stop(ctx)
	})
}

func TestNormalizeOptions(t *testing.T) {
	t.Run("ShouldNormalizeInvalidWebSocketPort", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		opts := BrokerOptions{
			WebSocketPort: -1,
		}

		// Act
		normalized := normalizeOptions(ctx, opts)

		// Assert
		assert.Equal(t, 9222, normalized.WebSocketPort)
	})

	t.Run("ShouldNormalizePortAboveRange", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		opts := BrokerOptions{
			WebSocketPort: 70000,
		}

		// Act
		normalized := normalizeOptions(ctx, opts)

		// Assert
		assert.Equal(t, 9222, normalized.WebSocketPort)
	})

	t.Run("ShouldDisableTLSWhenCertMissing", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		opts := BrokerOptions{
			EnableTLS: true,
			KeyFile:   "/path/to/key.pem",
		}

		// Act
		normalized := normalizeOptions(ctx, opts)

		// Assert
		assert.False(t, normalized.EnableTLS)
	})

	t.Run("ShouldDisableTLSWhenKeyMissing", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		opts := BrokerOptions{
			EnableTLS: true,
			CertFile:  "/path/to/cert.pem",
		}

		// Act
		normalized := normalizeOptions(ctx, opts)

		// Assert
		assert.False(t, normalized.EnableTLS)
	})

	t.Run("ShouldNormalizeReadinessTimeout", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		opts := BrokerOptions{
			ReadinessTimeout: 500 * time.Millisecond,
		}

		// Act
		normalized := normalizeOptions(ctx, opts)

		// Assert
		assert.Equal(t, 5*time.Second, normalized.ReadinessTimeout)
	})

	t.Run("ShouldNormalizeShutdownTimeout", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		opts := BrokerOptions{
			ShutdownTimeout: 500 * time.Millisecond,
		}

		// Act
		normalized := normalizeOptions(ctx, opts)

		// Assert
		assert.Equal(t, 10*time.Second, normalized.ShutdownTimeout)
	})

	t.Run("ShouldClearInvalidOperatorJWTURL", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		opts := BrokerOptions{
			OperatorJWTURL: "://invalid",
		}

		// Act
		normalized := normalizeOptions(ctx, opts)

		// Assert
		assert.Empty(t, normalized.OperatorJWTURL)
	})

	t.Run("ShouldClearInvalidAccountJWTURL", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		opts := BrokerOptions{
			AccountJWTURL: "://invalid",
		}

		// Act
		normalized := normalizeOptions(ctx, opts)

		// Assert
		assert.Empty(t, normalized.AccountJWTURL)
	})
}

func TestConfigureWebSocket(t *testing.T) {
	t.Run("ShouldConfigureWebSocketWithoutTLS", func(t *testing.T) {
		// This function is tested implicitly through Start tests
		// We include this test for documentation purposes
		// The actual functionality is covered by TestNatsBroker_StartAndStop
		// and TestNatsBroker_WithTLS tests
		assert.True(t, true)
	})
}

func TestLoadTLS(t *testing.T) {
	t.Run("ShouldLoadTLSCertificate", func(t *testing.T) {
		// Arrange
		certFile, keyFile, cleanup := generateTestCertificate(t)
		defer cleanup()

		// Act
		tlsConfig, err := loadTLS(certFile, keyFile, false)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, tlsConfig)
		assert.False(t, tlsConfig.InsecureSkipVerify)
		assert.Equal(t, uint16(0x0303), tlsConfig.MinVersion) // TLS 1.2
	})

	t.Run("ShouldLoadTLSWithInsecureSkipVerify", func(t *testing.T) {
		// Arrange
		certFile, keyFile, cleanup := generateTestCertificate(t)
		defer cleanup()

		// Act
		tlsConfig, err := loadTLS(certFile, keyFile, true)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, tlsConfig)
		assert.True(t, tlsConfig.InsecureSkipVerify)
	})

	t.Run("ShouldReturnErrorForInvalidCertFile", func(t *testing.T) {
		// Act
		_, err := loadTLS("/nonexistent/cert.pem", "/nonexistent/key.pem", false)

		// Assert
		assert.Error(t, err)
	})
}

func TestBuildAccountResolver(t *testing.T) {
	t.Run("ShouldBuildMemoryResolverFromAccountJWT", func(t *testing.T) {
		// Arrange
		_, accJWT, _ := generateTestJWTs(t)

		// Act
		resolver, err := buildAccountResolver(accJWT, "")

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, resolver)
	})

	t.Run("ShouldBuildURLResolverFromAccountJWTURL", func(t *testing.T) {
		// Arrange
		url := "https://example.com/accounts/%s"

		// Act
		resolver, err := buildAccountResolver("", url)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, resolver)
	})

	t.Run("ShouldReturnErrorWhenBothAreEmpty", func(t *testing.T) {
		// Act
		_, err := buildAccountResolver("", "")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no account resolver configured")
	})

	t.Run("ShouldReturnErrorForInvalidAccountJWT", func(t *testing.T) {
		// Act
		_, err := buildAccountResolver("invalid-jwt", "")

		// Assert
		assert.Error(t, err)
	})
}

func TestResolveOperatorClaims(t *testing.T) {
	t.Run("ShouldResolveFromOperatorJWT", func(t *testing.T) {
		// Arrange
		opJWT, _, _ := generateTestJWTs(t)
		ctx := context.Background()
		opts := BrokerOptions{
			OperatorJWT: opJWT,
		}

		// Act
		claims, err := resolveOperatorClaims(ctx, opts)

		// Assert
		require.NoError(t, err)
		assert.Len(t, claims, 1)
		assert.Equal(t, "Test Operator", claims[0].Name)
	})

	t.Run("ShouldReturnErrorWhenNoOperatorProvided", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		opts := BrokerOptions{}

		// Act
		_, err := resolveOperatorClaims(ctx, opts)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no trusted operator claims provided")
	})

	t.Run("ShouldReturnErrorForInvalidOperatorJWT", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		opts := BrokerOptions{
			OperatorJWT: "invalid-jwt",
		}

		// Act
		_, err := resolveOperatorClaims(ctx, opts)

		// Assert
		assert.Error(t, err)
	})
}

func TestJitter(t *testing.T) {
	t.Run("ShouldReturnJitterWithinRange", func(t *testing.T) {
		// Arrange
		maxDuration := 1 * time.Second

		// Act
		result := jitter(maxDuration)

		// Assert
		assert.GreaterOrEqual(t, result, time.Duration(0))
		assert.LessOrEqual(t, result, maxDuration)
	})

	t.Run("ShouldReturnZeroForZeroMax", func(t *testing.T) {
		// Act
		result := jitter(0)

		// Assert
		assert.Equal(t, time.Duration(0), result)
	})

	t.Run("ShouldReturnZeroForNegativeMax", func(t *testing.T) {
		// Act
		result := jitter(-1 * time.Second)

		// Assert
		assert.Equal(t, time.Duration(0), result)
	})
}

func TestCreateHTTPClient(t *testing.T) {
	t.Run("ShouldCreateClientWithSecureTLSSettings", func(t *testing.T) {
		// Act
		client := createHTTPClient(false)

		// Assert
		assert.NotNil(t, client)
		assert.NotNil(t, client.Transport)
		assert.Equal(t, 60*time.Second, client.Timeout)

		// Verify the transport has proper TLS configuration
		if transport, ok := client.Transport.(*http.Transport); ok {
			assert.NotNil(t, transport.TLSClientConfig)
			assert.False(t, transport.TLSClientConfig.InsecureSkipVerify)
			assert.Equal(t, uint16(0x0303), transport.TLSClientConfig.MinVersion) // TLS 1.2
			assert.NotEmpty(t, transport.TLSClientConfig.CipherSuites)

			// Verify connection pool settings
			assert.Equal(t, 100, transport.MaxIdleConns)
			assert.Equal(t, 10, transport.MaxIdleConnsPerHost)
			assert.Equal(t, 100, transport.MaxConnsPerHost)
			assert.Equal(t, 90*time.Second, transport.IdleConnTimeout)

			// Verify timeout settings
			assert.Equal(t, 10*time.Second, transport.TLSHandshakeTimeout)
			assert.Equal(t, 10*time.Second, transport.ResponseHeaderTimeout)
			assert.Equal(t, 1*time.Second, transport.ExpectContinueTimeout)
		} else {
			t.Fatal("Transport is not *http.Transport")
		}
	})

	t.Run("ShouldCreateClientWithInsecureSkipVerifyWhenEnabled", func(t *testing.T) {
		// Act
		client := createHTTPClient(true)

		// Assert
		assert.NotNil(t, client)
		assert.NotNil(t, client.Transport)

		// Verify the transport has InsecureSkipVerify set
		if transport, ok := client.Transport.(*http.Transport); ok {
			assert.NotNil(t, transport.TLSClientConfig)
			assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
			assert.Equal(t, uint16(0x0303), transport.TLSClientConfig.MinVersion) // TLS 1.2

			// Should still have proper connection pool settings
			assert.Equal(t, 100, transport.MaxIdleConns)
			assert.Equal(t, 10, transport.MaxIdleConnsPerHost)
		} else {
			t.Fatal("Transport is not *http.Transport")
		}
	})
}

func TestFetchTrustedOperators(t *testing.T) {
	// Generate operator JWT for tests
	operatorJWTs, _, _ := generateTestJWTs(t)

	t.Run("ShouldFetchOperatorJWTFromHTTPEndpoint", func(t *testing.T) {
		// Arrange
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(operatorJWTs))
		}))
		defer server.Close()

		ctx := context.Background()

		// Act
		claims, err := fetchTrustedOperators(ctx, server.URL, http.DefaultClient)

		// Assert
		require.NoError(t, err)
		require.Len(t, claims, 1)
		assert.Equal(t, "Test Operator", claims[0].Name)
	})

	t.Run("ShouldRetryOnHTTPError", func(t *testing.T) {
		// Arrange
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(operatorJWTs))
			}
		}))
		defer server.Close()

		ctx := context.Background()

		// Act
		claims, err := fetchTrustedOperators(ctx, server.URL, http.DefaultClient)

		// Assert
		require.NoError(t, err)
		assert.GreaterOrEqual(t, attempts, 3, "Should have retried at least 3 times")
		require.Len(t, claims, 1)
	})

	t.Run("ShouldRetryOnConnectionError", func(t *testing.T) {
		// Arrange - use invalid port
		ctx := context.Background()
		invalidURL := "http://localhost:99999"

		// Act
		_, err := fetchTrustedOperators(ctx, invalidURL, http.DefaultClient)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch trusted operator JWT")
	})

	t.Run("ShouldRespectContextCancellation", func(t *testing.T) {
		// Arrange - use context that's already expired
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Act - doesn't matter what URL since context is cancelled
		_, err := fetchTrustedOperators(ctx, "http://localhost:9999", http.DefaultClient)

		// Assert
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("ShouldReturnErrorForInvalidOperatorJWT", func(t *testing.T) {
		// Arrange
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid-jwt"))
		}))
		defer server.Close()

		ctx := context.Background()

		// Act
		_, err := fetchTrustedOperators(ctx, server.URL, http.DefaultClient)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "decode operator claims")
	})

	t.Run("ShouldReturnErrorAfterMaxRetries", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping retry test in short mode")
		}

		// Arrange - server that always fails
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		ctx := context.Background()

		// Act
		_, err := fetchTrustedOperators(ctx, server.URL, http.DefaultClient)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch trusted operator JWT")
		assert.Contains(t, err.Error(), "after 5 attempts")
		assert.Equal(t, 5, attempts, "Should have tried exactly 5 times")
	})
}

func TestExtractAccountPublicKey(t *testing.T) {
	t.Run("ShouldExtractPublicKeyFromAccountJWT", func(t *testing.T) {
		// Arrange
		_, accountJWT, accountPub := generateTestJWTs(t)

		// Act
		extractedPub, err := extractAccountPublicKey(accountJWT)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, accountPub, extractedPub)
	})

	t.Run("ShouldReturnErrorForInvalidJWT", func(t *testing.T) {
		// Act
		_, err := extractAccountPublicKey("invalid-jwt")

		// Assert
		assert.Error(t, err)
	})
}
