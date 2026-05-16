package ingest

import (
	"log"
	"net/http"
)

func Init() {
	pendingQueue := NewPendingQueue()

	mux := http.NewServeMux()
	mux.Handle("/", NewProxyController(pendingQueue))
	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go ProcessMockInvocation(pendingQueue)
	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
