package ingest

import (
	"net/http"
	"openlambda/internal/function"
)

func ProcessMockInvocation(pendingQueue *PendingQueue) {
	for {
		inv := pendingQueue.Dequeue()
		resp := &function.HttpResponseEvent{
			StatusCode: http.StatusOK,
			Body:       "Mock response",
		}
		inv.ResponseChan <- resp
	}
}
