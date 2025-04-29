package nats

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/fgrzl/messaging/broker"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nkeys"
)

// BrokerOptions configures the NATS broker.
type BrokerOptions struct {
	WSPort           int           // WebSocket port
	CertFile         string        // TLS certificate file path
	KeyFile          string        // TLS key file path
	EnableTLS        bool          // Enable TLS for WebSocket
	JWKSURL          string        // JWKS URL for trusted keys
	ReadinessTimeout time.Duration // Timeout for server readiness
}

// GetDefaultOptions returns default configuration, pulling from environment variables where applicable.
func GetDefaultOptions() BrokerOptions {
	return BrokerOptions{
		WSPort: func() int {
			port, err := strconv.Atoi(os.Getenv("NATS_WS_PORT"))
			if err != nil {
				return 0 // Default to 0 if parsing fails
			}
			return port
		}(),
		CertFile:         os.Getenv("NATS_CERT_FILE"),
		KeyFile:          os.Getenv("NATS_KEY_FILE"),
		EnableTLS:        os.Getenv("NATS_CERT_FILE") != "" && os.Getenv("NATS_KEY_FILE") != "",
		JWKSURL:          os.Getenv("BROKER_JWKS_URL"),
		ReadinessTimeout: 5 * time.Second,
	}
}

type natsBroker struct {
	options    BrokerOptions
	natsServer *server.Server
	log        *slog.Logger
}

// NewBroker creates a new NATS broker with validated options.
func NewBroker(options BrokerOptions) broker.Broker {
	// Validate options
	if options.WSPort <= 0 || options.WSPort > 65535 {
		options.WSPort = 9222
		slog.Warn("Invalid WSPort, using default", slog.Int("port", options.WSPort))
	}
	if options.EnableTLS && (options.CertFile == "" || options.KeyFile == "") {
		options.EnableTLS = false
		slog.Warn("Disabling TLS: missing cert or key file")
	}
	if options.ReadinessTimeout < time.Second {
		options.ReadinessTimeout = 5 * time.Second
		slog.Warn("ReadinessTimeout too short, using default", slog.Duration("timeout", options.ReadinessTimeout))
	}

	return &natsBroker{
		options: options,
		log:     slog.With("component", "broker"),
	}
}

// Start initializes and starts the NATS server.
func (b *natsBroker) Start() error {
	opts := &server.Options{
		Port: -1, // Disable TCP — WS/WSS only
	}

	// Configure WebSocket with optional TLS
	if b.options.EnableTLS {
		cert, err := tls.LoadX509KeyPair(b.options.CertFile, b.options.KeyFile)
		if err != nil {
			return fmt.Errorf("load TLS cert: %w", err)
		}
		opts.Websocket = server.WebsocketOpts{
			Port: b.options.WSPort,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				},
			},
		}
	} else {
		opts.Websocket = server.WebsocketOpts{
			Port:  b.options.WSPort,
			NoTLS: true,
		}
	}

	// Load trusted keys from JWKS URL
	if b.options.JWKSURL != "" {
		trustedKeys, err := b.fetchTrustedKeys()
		if err != nil {
			return fmt.Errorf("fetch trusted keys: %w", err)
		}
		opts.TrustedKeys = trustedKeys
		b.log.Info("Fetched trusted keys", slog.Int("count", len(trustedKeys)))
	}

	natsServer, err := server.NewServer(opts)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	b.natsServer = natsServer
	natsServer.Start()

	if !natsServer.ReadyForConnections(b.options.ReadinessTimeout) {
		return errors.New("NATS server readiness timeout")
	}

	b.log.Info("NATS broker started",
		slog.Int("websocket_port", b.options.WSPort),
		slog.Bool("tls_enabled", b.options.EnableTLS),
		slog.String("jwks_url", b.options.JWKSURL),
	)

	return nil
}

// Stop initiates a graceful shutdown of the NATS server.
func (b *natsBroker) Stop() {
	if b.natsServer != nil {
		b.log.Info("Stopping NATS broker")
		// Shutdown server with timeout
		done := make(chan struct{})
		go func() {
			b.natsServer.Shutdown()
			close(done)
		}()
		select {
		case <-done:
			b.log.Info("NATS broker stopped")
		case <-time.After(10 * time.Second):
			b.log.Warn("NATS broker shutdown timed out")
		}
	}
}

// WaitForShutdown waits for the NATS server to fully shut down.
func (b *natsBroker) WaitForShutdown() {
	if b.natsServer != nil {
		b.log.Info("Waiting for NATS broker shutdown")
		b.natsServer.WaitForShutdown()
		b.log.Info("NATS broker shutdown complete")
	}
}

// fetchTrustedKeys retrieves and validates trusted keys from the JWKS URL with retries.
func (b *natsBroker) fetchTrustedKeys() ([]string, error) {
	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := http.Get(b.options.JWKSURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var keys struct {
				TrustedKeys []string `json:"trusted_keys"`
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("read response: %w", err)
			}
			if err := json.Unmarshal(body, &keys); err != nil {
				return nil, fmt.Errorf("parse JWKS: %w", err)
			}

			var valid []string
			for _, k := range keys.TrustedKeys {
				// Validate NATS public key
				if _, err := nkeys.FromPublicKey(k); err == nil {
					valid = append(valid, k)
				} else {
					b.log.Warn("Skipping invalid trusted key",
						slog.String("key", k),
						slog.String("error", err.Error()))
				}
			}
			if len(valid) == 0 {
				return nil, errors.New("no valid trusted keys found")
			}
			return valid, nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		logAttrs := []any{slog.Int("attempt", attempt)}
		if err != nil {
			logAttrs = append(logAttrs, slog.String("error", err.Error()))
		} else {
			logAttrs = append(logAttrs, slog.Int("status", resp.StatusCode))
		}
		b.log.Warn("JWKS fetch attempt failed", logAttrs...)
		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	return nil, fmt.Errorf("failed to fetch trusted keys after %d attempts", maxRetries)
}
