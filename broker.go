package messaging

import "context"

type Broker interface {
	Start(context.Context) error
	Stop(context.Context) error
}
