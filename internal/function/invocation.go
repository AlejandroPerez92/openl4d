package function

import (
	"context"
	"sync/atomic"
	"time"
)

type Invocation struct {
	Event        HttpRequestEvent
	ResponseChan chan *HttpResponseEvent
	Ctx          context.Context
	Deadline     time.Time
	cancelled    atomic.Bool
}

func (i *Invocation) Cancel() {
	i.cancelled.Store(true)
}

func (i *Invocation) IsCancelled() bool {
	return i.cancelled.Load()
}
