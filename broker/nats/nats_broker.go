package broker

import (
	"crypto/tls"
	"embed"
	"log/slog"
	"time"

	"github.com/fgrzl/messaging"
	"github.com/fgrzl/messaging/broker"
	"github.com/nats-io/nats-server/v2/server"
)

//go:embed nats.conf
var natsConfig embed.FS

type natsBroker struct {
	server *server.Server
}

func NewBroker() broker.Broker {
	return &natsBroker{}
}

func (b *natsBroker) Start() {
	log := slog.With("component", "broker")

	// Read the embedded configuration file
	configData, err := natsConfig.ReadFile("nats.conf")
	if err != nil {
		log.Error("Error reading embedded config file", "error", err)
		return
	}

	// Parse the configuration options
	opts, err := server.ProcessConfigFile(string(configData))
	if err != nil {
		log.Error("Error processing config file", "error", err)
		return
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

		opts = &server.Options{
			Websocket: server.WebsocketOpts{
				Port: port,
				TLSConfig: &tls.Config{
					Certificates: []tls.Certificate{cert},
				},
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
