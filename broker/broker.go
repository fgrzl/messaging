package broker

type Broker interface {
	Start()
	Stop()
	Wait()
}

type BrokerAuth interface {
	Fetch(clientID string) (string, error)
}
