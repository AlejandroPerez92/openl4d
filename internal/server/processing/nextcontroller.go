package processing

import (
	"encoding/json"
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
		nextInvocation := p.pendingQueue.Dequeue()
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
