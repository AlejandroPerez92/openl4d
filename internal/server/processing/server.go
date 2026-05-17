package processing

import (
	"net/http"
	"openlambda/internal/function"
)

func Init(pendingQueue *function.PendingQueue, processingMap *function.ProcessingMap) error {

	mux := http.NewServeMux()
	mux.Handle("/2018-06-01/runtime/invocation/next", NewNextController(pendingQueue, processingMap))
	mux.Handle("/runtime/invocation/{request_id}/response", NewSendResponseController(processingMap))
	mux.Handle("/runtime/invocation/{request_id}/error", NewInvocationErrorController(processingMap))
	server := http.Server{
		Addr:    ":8081",
		Handler: mux,
	}
	err := server.ListenAndServe()
	if err != nil {
		return err
	}

	return nil
}
