package ingest

import (
	"net/http"
	"openlambda/internal/function"
)

func NewServer(pendingQueue *function.PendingQueue, processingMap *function.ProcessingMap) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/", NewIngestController(pendingQueue, processingMap))
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	return server
}

func Init(pendingQueue *function.PendingQueue, processingMap *function.ProcessingMap) error {
	return NewServer(pendingQueue, processingMap).ListenAndServe()
}
