package function

import (
	"context"
	"time"
)

type Invocation struct {
	Event        HttpRequestEvent
	ResponseChan chan *HttpResponseEvent
	Ctx          context.Context
	Deadline     time.Time
}
