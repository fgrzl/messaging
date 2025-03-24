package nats

import (
	"crypto/tls"
	"log/slog"
	"time"

	"github.com/fgrzl/messaging"
	"github.com/fgrzl/messaging/broker"
	"github.com/nats-io/nats-server/v2/server"
)

type natsBroker struct {
	server          *server.Server
	accountResolver server.AccountResolver
}

func NewBroker() broker.Broker {
	return &natsBroker{
		accountResolver: &server.MemAccResolver{},
	}
}

func (b *natsBroker) Start() {
	log := slog.With("component", "broker")

	opts := &server.Options{
		AccountResolver: b.accountResolver,
		SystemAccount:   messaging.GetBrokerUser(),
	}

	useTLS := messaging.GetBrokerUseTLS()
	port := messaging.GetBrokerPort()

	log.Info("Starting NATS broker", slog.Int("port", port), slog.Bool("tls", useTLS))

	if useTLS {
		cert, err := tls.LoadX509KeyPair(messaging.GetCertFilePath(), messaging.GetKeyFilePath())
		if err != nil {
			slog.Error("Error loading TLS certificate", slog.Any("error", err))
			return
		}

		opts.Websocket = server.WebsocketOpts{
			Port: port,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
			},
		}

	} else {
		opts = &server.Options{
			Websocket: server.WebsocketOpts{
				Port:  port,
				NoTLS: true,
			},
		}
	}

	// Create a new NATS server instance
	natsServer, err := server.NewServer(opts)
	if err != nil {
		log.Error("Error creating NATS server", slog.Any("error", err))
		return
	}

	natsServer.Start()
	log.Info("NATS broker started", slog.Int("port", port))

	b.server = natsServer
	if !natsServer.ReadyForConnections(10 * time.Second) {
		log.Error("NATS server did not start in time")
		return
	}
}

func (b *natsBroker) Stop() {
	if b.server != nil {
		slog.Info("Stopping NATS broker")
		b.server.Shutdown()
	}
}

func (b *natsBroker) Wait() {
	if b.server != nil {
		slog.Info("Waiting for NATS broker shutdown")
		b.server.WaitForShutdown()
	}
}
