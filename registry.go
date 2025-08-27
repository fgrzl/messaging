package messaging

import (
	"sync"

	"github.com/fgrzl/json/polymorphic"
)

var once sync.Once

func init() { EnsureRegistered() }

func EnsureRegistered() {
	once.Do(registerAll)
}

func registerAll() {
	polymorphic.RegisterType[BoolResult]()
	polymorphic.RegisterType[ErrorResponse]()
	polymorphic.RegisterType[Accepted]()
}
