package natskit

import (
	"errors"
	"os"
	"strconv"
	"time"
)

const (
	envHost             = "BROKER_HOST"
	envWebSocketPort    = "BROKER_WEB_SOCKET_PORT"
	envMonitorPort      = "BROKER_MONITOR_PORT"
	envCertFile         = "BROKER_CERT_FILE"
	envKeyFile          = "BROKER_KEY_FILE"
	envOperatorJWT      = "BROKER_OPERATOR_JWT"
	envOperatorJWTURL   = "BROKER_OPERATOR_JWT_URL"
	envAccountJWT       = "BROKER_ACCOUNT_JWT"
	envAccountJWTURL    = "BROKER_ACCOUNT_JWT_URL"
	envReadinessTimeout = "BROKER_READINESS_TIMEOUT"
	envShutdownTimeout  = "BROKER_SHUTDOWN_TIMEOUT"
)

// BrokerOptions configures the embedded NATS broker.
// Values can be loaded from the following environment variables.
type BrokerOptions struct {
	// Host defines the address to bind the WebSocket listener (default: "localhost").
	// Environment: BROKER_HOST
	Host string

	// WebSocketPort defines the port for the WebSocket listener (default: 9222).
	// Environment: BROKER_WEB_SOCKET_PORT
	WebSocketPort int

	// MonitorPort defines the port for the WebSocket listener (default: 8222).
	// Environment: BROKER_HTTP_PORT
	MonitorPort int

	// CertFile specifies the path to the TLS certificate file.
	// Environment: BROKER_CERT_FILE
	CertFile string

	// KeyFile specifies the path to the TLS private key file.
	// Environment: BROKER_KEY_FILE
	KeyFile string

	// EnableTLS enables TLS if both CertFile and KeyFile are provided.
	EnableTLS bool

	// OperatorJWT provides an inline operator JWT used to establish trust.
	// Environment: BROKER_OPERATOR_JWT
	OperatorJWT string

	// OperatorJWTURL specifies a URL to fetch the operator JWT dynamically.
	// Environment: BROKER_OPERATOR_JWT_URL
	OperatorJWTURL string

	// AccountJWT provides an inline account JWT for in-memory resolution.
	// Environment: BROKER_ACCOUNT_JWT
	AccountJWT string

	// AccountJWTURL specifies a URL-based account resolver.
	// Environment: BROKER_ACCOUNT_JWT_URL
	AccountJWTURL string

	// ReadinessTimeout defines how long to wait for the server to become ready.
	// Environment: BROKER_READINESS_TIMEOUT (e.g., "5s")
	ReadinessTimeout time.Duration

	// ShutdownTimeout defines how long to wait for graceful shutdown.
	// Environment: BROKER_SHUTDOWN_TIMEOUT (e.g., "10s")
	ShutdownTimeout time.Duration
}

// GetDefaultOptions returns default configuration from environment variables.
func GetDefaultOptions() BrokerOptions {
	certFile := os.Getenv(envCertFile)
	keyFile := os.Getenv(envKeyFile)
	enableTLS := certFile != "" && keyFile != ""

	readiness := 5 * time.Second
	if s := os.Getenv(envReadinessTimeout); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			readiness = d
		}
	}

	shutdown := 10 * time.Second
	if s := os.Getenv(envShutdownTimeout); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			shutdown = d
		}
	}

	return BrokerOptions{
		Host:             getEnvOrDefault(envHost, "localhost"),
		WebSocketPort:    getEnvOrDefaultInt(envWebSocketPort, 9222),
		MonitorPort:      getEnvOrDefaultInt(envMonitorPort, 8222),
		CertFile:         certFile,
		KeyFile:          keyFile,
		EnableTLS:        enableTLS,
		OperatorJWT:      os.Getenv(envOperatorJWT),
		OperatorJWTURL:   os.Getenv(envOperatorJWTURL),
		AccountJWT:       os.Getenv(envAccountJWT),
		AccountJWTURL:    os.Getenv(envAccountJWTURL),
		ReadinessTimeout: readiness,
		ShutdownTimeout:  shutdown,
	}
}

// MustGetDefaultOptions returns the default options or panics if invalid.
func MustGetDefaultOptions() BrokerOptions {
	opts := GetDefaultOptions()
	if err := opts.Validate(); err != nil {
		panic("invalid broker options: " + err.Error())
	}
	return opts
}

// Validate checks that required configuration is present.
func (b *BrokerOptions) Validate() error {
	if b.EnableTLS && (b.CertFile == "" || b.KeyFile == "") {
		return errors.New("EnableTLS is true but cert or key file is missing")
	}
	if b.OperatorJWT == "" && b.OperatorJWTURL == "" {
		return errors.New("either OperatorJWT or OperatorJWTURL must be provided")
	}
	if b.AccountJWT == "" && b.AccountJWTURL == "" {
		return errors.New("either AccountJWT or AccountJWTURL must be provided")
	}
	return nil
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvOrDefaultInt(key string, fallback int) int {
	if p := os.Getenv(key); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			return parsed
		}
	}
	return fallback
}
