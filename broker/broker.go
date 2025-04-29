package broker

type Broker interface {
	Start() error
	Stop()
	WaitForShutdown()
}

type BrokerAuth interface {
	Fetch(clientID string) (string, error)
}
