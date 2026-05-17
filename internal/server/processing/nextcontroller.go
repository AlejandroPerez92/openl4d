package processing

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"openlambda/internal/function"
)

type NextController struct {
	pendingQueue  *function.PendingQueue
	processingMap *function.ProcessingMap
}

func NewNextController(pendingQueue *function.PendingQueue, processingMap *function.ProcessingMap) *NextController {
	return &NextController{
		pendingQueue:  pendingQueue,
		processingMap: processingMap,
	}
}

func (p *NextController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Iterates until it finds a pending invocation that is not timed out.
	for {
		nextInvocation, err := p.pendingQueue.Dequeue(r.Context())
		if err != nil {
			if errors.Is(err, function.ErrQueueDrained) {
				http.Error(w, "no pending invocations", http.StatusNoContent)
				return
			}
			if errors.Is(err, context.Canceled) {
				http.Error(w, "client gone", 499)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if nextInvocation == nil {
			http.Error(w, "no pending invocations", http.StatusNoContent)
			return
		}

		if nextInvocation.IsCancelled() {
			log.Printf("invocation cancelled: %v", nextInvocation.Event.RequestContext.RequestID)
			continue
		}

		p.processingMap.Put(nextInvocation)

		jsonEvent, err := json.Marshal(nextInvocation.Event)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jsonEvent)
		return
	}
}
