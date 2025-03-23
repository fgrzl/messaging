package broker

type Broker interface {
	Start()
	Stop()
	Wait()
}
