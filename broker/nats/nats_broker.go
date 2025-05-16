package nats

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/fgrzl/messaging/broker"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats-server/v2/server"
)

func NewBroker(ctx context.Context, options BrokerOptions) broker.Broker {
	options = normalizeOptions(ctx, options)
	return &NatsBroker{
		options: options,
	}
}

type NatsBroker struct {
	options    BrokerOptions
	natsServer *server.Server
}

func (b *NatsBroker) Start(ctx context.Context) error {
	opts := &server.Options{Port: -1} // Disable TCP
	if err := configureWebSocket(opts, b.options); err != nil {
		return err
	}

	opClaims, err := resolveOperatorClaims(ctx, b.options)
	if err != nil {
		return err
	}
	opts.TrustedOperators = opClaims

	accResolver, err := buildAccountResolver(b.options.AccountJWT, b.options.OperatorJWTURL)
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

	slog.Info("NATS broker started",
		slog.Int("websocket_port", b.options.WSPort),
		slog.Bool("tls_enabled", b.options.EnableTLS),
	)
	return nil
}

func (b *NatsBroker) Stop(ctx context.Context) error {
	if b.natsServer == nil {
		return nil
	}
	slog.Info("Stopping NATS broker")
	ctx, cancel := context.WithTimeout(ctx, b.options.ShutdownTimeout)
	defer cancel()
	done := make(chan struct{})
	go func() {
		b.natsServer.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		slog.InfoContext(ctx, "NATS broker stopped")
	case <-ctx.Done():
		slog.WarnContext(ctx, "NATS broker shutdown timed out")
	}
	return nil
}

func (b *NatsBroker) WaitForShutdown() {
	if b.natsServer != nil {
		slog.Info("Waiting for NATS broker shutdown")
		b.natsServer.WaitForShutdown()
		slog.Info("NATS broker shutdown complete")
	}
}

func normalizeOptions(ctx context.Context, opt BrokerOptions) BrokerOptions {
	if opt.WSPort <= 0 || opt.WSPort > 65535 {
		opt.WSPort = 9222
		slog.WarnContext(ctx, "Invalid WSPort, using default", slog.Int("port", opt.WSPort))
	}
	if opt.EnableTLS && (opt.CertFile == "" || opt.KeyFile == "") {
		opt.EnableTLS = false
		slog.WarnContext(ctx, "Disabling TLS: missing cert or key file")
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
	return opt
}

func configureWebSocket(opts *server.Options, options BrokerOptions) error {
	if options.EnableTLS {
		tlsConfig, err := loadTLS(options.CertFile, options.KeyFile)
		if err != nil {
			return fmt.Errorf("load TLS config: %w", err)
		}
		opts.Websocket = server.WebsocketOpts{
			Port:      options.WSPort,
			Host:      options.WSHost,
			TLSConfig: tlsConfig,
		}
	} else {
		opts.Websocket = server.WebsocketOpts{
			Port:  options.WSPort,
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

func loadTLS(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates:             []tls.Certificate{cert},
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}, nil
}

func fetchTrustedOperators(ctx context.Context, url string) ([]*jwt.OperatorClaims, error) {
	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.WarnContext(ctx, "Trusted operator fetch failed", slog.Int("attempt", attempt), slog.String("error", err.Error()))
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			slog.WarnContext(ctx, "Trusted operator fetch failed", slog.Int("attempt", attempt), slog.String("status", resp.Status))
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read operator JWT: %w", err)
		}
		opClaims, err := jwt.DecodeOperatorClaims(string(body))
		if err != nil {
			return nil, fmt.Errorf("decode operator claims: %w", err)
		}
		slog.Info("Fetched trusted operator", slog.String("issuer", opClaims.Issuer), slog.String("name", opClaims.Name))
		return []*jwt.OperatorClaims{opClaims}, nil
	}
	return nil, fmt.Errorf("failed to fetch trusted operator JWT from %s after %d attempts", url, maxRetries)
}
