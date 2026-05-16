package ingest

import (
	"net/http"
	"openlambda/internal/function"
)

func Init(pendingQueue *function.PendingQueue) error {

	mux := http.NewServeMux()
	mux.Handle("/", NewIngestController(pendingQueue))
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
