package natsbroker

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"github.com/fgrzl/messaging"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats-server/v2/server"
)

// NewBroker returns a new NATS broker with the specified options.
func NewBroker(ctx context.Context, options BrokerOptions) messaging.Broker {
	options = normalizeOptions(ctx, options)
	return &NatsBroker{
		options: options,
	}
}

// NatsBroker implements the Broker interface using an embedded NATS server.
type NatsBroker struct {
	options    BrokerOptions
	natsServer *server.Server
}

// Start initializes and starts the embedded NATS server.
func (b *NatsBroker) Start(ctx context.Context) error {
	opts := &server.Options{
		Host:     b.options.Host,
		HTTPPort: b.options.MonitorPort, // Enable http monitoring (e.g. /healthz)
		HTTPHost: b.options.Host,
	}

	if err := configureWebSocket(opts, b.options); err != nil {
		return err
	}

	opClaims, err := resolveOperatorClaims(ctx, b.options)
	if err != nil {
		return err
	}
	opts.TrustedOperators = opClaims

	accResolver, err := buildAccountResolver(b.options.AccountJWT, b.options.AccountJWTURL)
	if err != nil {
		return err
	}
	opts.AccountResolver = accResolver

	ns, err := server.NewServer(opts)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	b.natsServer = ns
	ns.Start()

	if !ns.ReadyForConnections(b.options.ReadinessTimeout) {
		return fmt.Errorf("NATS server readiness timeout")
	}

	slog.InfoContext(ctx, "NATS broker started",
		slog.Int("websocket_port", b.options.WebSocketPort),
		slog.Bool("tls_enabled", b.options.EnableTLS),
	)
	return nil
}

// Stop gracefully shuts down the embedded NATS server.
func (b *NatsBroker) Stop(ctx context.Context) error {
	if b.natsServer == nil {
		return nil
	}
	b.natsServer.Shutdown()
	b.natsServer.WaitForShutdown()
	return nil
}

func normalizeOptions(ctx context.Context, opt BrokerOptions) BrokerOptions {
	if opt.WebSocketPort <= 0 || opt.WebSocketPort > 65535 {
		opt.WebSocketPort = 9222
		slog.WarnContext(ctx, "Invalid WSPort, using default", slog.Int("port", opt.WebSocketPort))
	}
	if opt.EnableTLS && (opt.CertFile == "" || opt.KeyFile == "") {
		opt.EnableTLS = false
		slog.WarnContext(ctx, "Disabling TLS: missing cert or key file")
	}
	if opt.EnableTLS && opt.InsecureSkipVerify {
		slog.WarnContext(ctx, "TLS certificate validation is disabled (InsecureSkipVerify=true) - NOT for production use")
	}
	if opt.ReadinessTimeout < time.Second {
		opt.ReadinessTimeout = 5 * time.Second
		slog.WarnContext(ctx, "ReadinessTimeout too short, using default", slog.Duration("timeout", opt.ReadinessTimeout))
	}
	if opt.ShutdownTimeout < time.Second {
		opt.ShutdownTimeout = 10 * time.Second
		slog.WarnContext(ctx, "ShutdownTimeout too short, using default", slog.Duration("timeout", opt.ShutdownTimeout))
	}
	if opt.OperatorJWTURL != "" {
		if _, err := url.Parse(opt.OperatorJWTURL); err != nil {
			slog.WarnContext(ctx, "Invalid OperatorJWTURL, ignoring", slog.String("url", opt.OperatorJWTURL))
			opt.OperatorJWTURL = ""
		}
	}
	if opt.AccountJWTURL != "" {
		if _, err := url.Parse(opt.AccountJWTURL); err != nil {
			slog.WarnContext(ctx, "Invalid AccountJWTURL, ignoring", slog.String("url", opt.AccountJWTURL))
			opt.AccountJWTURL = ""
		}
	}
	return opt
}

func configureWebSocket(opts *server.Options, options BrokerOptions) error {
	if options.EnableTLS {
		tlsConfig, err := loadTLS(options.CertFile, options.KeyFile, options.InsecureSkipVerify)
		if err != nil {
			return fmt.Errorf("load TLS config: %w", err)
		}
		opts.Websocket = server.WebsocketOpts{
			Port:      options.WebSocketPort,
			Host:      options.Host,
			TLSConfig: tlsConfig,
		}
	} else {
		opts.Websocket = server.WebsocketOpts{
			Port:  options.WebSocketPort,
			Host:  options.Host,
			NoTLS: true,
		}
	}
	return nil
}

func extractAccountPublicKey(jwtStr string) (string, error) {
	claims, err := jwt.DecodeAccountClaims(jwtStr)
	if err != nil {
		return "", err
	}
	return claims.Subject, nil
}

func resolveOperatorClaims(ctx context.Context, options BrokerOptions) ([]*jwt.OperatorClaims, error) {
	if options.OperatorJWTURL != "" {
		return fetchTrustedOperators(ctx, options.OperatorJWTURL)
	} else if options.OperatorJWT != "" {
		claim, err := jwt.DecodeOperatorClaims(options.OperatorJWT)
		if err != nil {
			return nil, fmt.Errorf("decode operator JWT: %w", err)
		}
		return []*jwt.OperatorClaims{claim}, nil
	}
	return nil, fmt.Errorf("no trusted operator claims provided")
}

func buildAccountResolver(accountJWT, accountJWTURL string) (server.AccountResolver, error) {
	if accountJWT != "" {
		accPub, err := extractAccountPublicKey(accountJWT)
		if err != nil {
			return nil, fmt.Errorf("invalid account JWT: %w", err)
		}

		resolver := &server.MemAccResolver{}
		if err := resolver.Store(accPub, accountJWT); err != nil {
			return nil, fmt.Errorf("store account JWT in memory resolver: %w", err)
		}
		return resolver, nil

	} else if accountJWTURL != "" {
		return server.NewURLAccResolver(accountJWTURL)

	}
	return nil, errors.New("no account resolver configured: either AccountJWT or AccountJWTURL must be provided")
}

func loadTLS(certFile, keyFile string, insecureSkipVerify bool) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates:             []tls.Certificate{cert},
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		InsecureSkipVerify:       insecureSkipVerify,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}, nil
}

func fetchTrustedOperators(ctx context.Context, url string) ([]*jwt.OperatorClaims, error) {
	const (
		maxRetries    = 5
		initialDelay  = 500 * time.Millisecond
		backoffFactor = 2.0
		jitterRange   = 250 * time.Millisecond
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	delay := initialDelay

	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.WarnContext(ctx, "Failed to fetch operator JWT",
				slog.Int("attempt", attempt),
				slog.String("error", err.Error()))
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("read operator JWT: %w", err)
				}
				opClaims, err := jwt.DecodeOperatorClaims(string(body))
				if err != nil {
					return nil, fmt.Errorf("decode operator claims: %w", err)
				}
				slog.InfoContext(ctx, "Fetched trusted operator",
					slog.String("issuer", opClaims.Issuer),
					slog.String("name", opClaims.Name))
				return []*jwt.OperatorClaims{opClaims}, nil
			}
			slog.WarnContext(ctx, "Unexpected status from operator JWT endpoint",
				slog.Int("attempt", attempt),
				slog.Int("status", resp.StatusCode))
		}

		// Check context before sleeping
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay + time.Duration(jitter(jitterRange))):
			delay = time.Duration(float64(delay) * backoffFactor)
		}
	}

	return nil, fmt.Errorf("failed to fetch trusted operator JWT from %s after %d attempts", url, maxRetries)
}

func jitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		// Fallback to no jitter on failure — shouldn't happen
		return 0
	}
	return time.Duration(n.Int64())
}
