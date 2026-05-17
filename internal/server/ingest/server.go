package ingest

import (
	"net/http"
	"openlambda/internal/function"
)

func Init(pendingQueue *function.PendingQueue, processingMap *function.ProcessingMap) error {

	mux := http.NewServeMux()
	mux.Handle("/", NewIngestController(pendingQueue, processingMap))
	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	err := server.ListenAndServe()

	if err != nil {
		return err
	}

	return nil
}
