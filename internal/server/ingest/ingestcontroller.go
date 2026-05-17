package ingest

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/AlejandroPerez92/openl4d/internal/function"
)

type IngestController struct {
	pendingQueue  *function.PendingQueue
	processingMap *function.ProcessingMap
}

func NewIngestController(pendingQueue *function.PendingQueue, processingMap *function.ProcessingMap) *IngestController {
	return &IngestController{
		pendingQueue:  pendingQueue,
		processingMap: processingMap,
	}
}

func (p *IngestController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	maxLifetime := time.Now().Add(30 * time.Second)

	ctx, cancel := context.WithDeadline(r.Context(), maxLifetime)
	defer cancel()

	invocation := &function.Invocation{
		Event:        function.FromRequest(r),
		ResponseChan: make(chan *function.HttpResponseEvent, 1),
		Ctx:          ctx,
		Deadline:     maxLifetime,
	}

	if err := p.pendingQueue.Enqueue(invocation); err != nil {
		log.Printf("invocation finished: %v", invocation.Event.RequestContext.RequestID)
		log.Printf("invocation finished err: %v", err)
		p.cleanup(invocation)
		if errors.Is(err, function.ErrQueueClosed) {
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
		} else if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "function timeout", http.StatusGatewayTimeout)
		} else {
			http.Error(w, "client gone", 499)
		}
		return
	}

	select {
	case resp := <-invocation.ResponseChan:
		err := writeResponse(w, *resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case <-ctx.Done():
		log.Printf("context done: %v", invocation.Event.RequestContext.RequestID)
		p.cleanup(invocation)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			http.Error(w, "function timeout", http.StatusGatewayTimeout)
			return
		}

		http.Error(w, "client gone", 499)

	}
}

func (p *IngestController) cleanup(invocation *function.Invocation) {
	log.Printf("cleaning up invocation: %v", invocation.Event.RequestContext.RequestID)
	reqId := invocation.Event.RequestContext.RequestID
	p.processingMap.Delete(reqId)
	invocation.Cancel()
}

func writeResponse(w http.ResponseWriter, resp function.HttpResponseEvent) error {
	// 1. Headers. Skip Set-Cookie here; we render cookies separately.
	for k, v := range resp.Headers {
		if strings.EqualFold(k, "Set-Cookie") {
			continue
		}
		w.Header().Set(k, v)
	}

	// 2. Cookies become individual Set-Cookie headers. Each entry is a full
	// "name=value; Attr=...; Attr=..." string already formatted by the handler.
	for _, c := range resp.Cookies {
		w.Header().Add("Set-Cookie", c)
	}

	// Sensible default content-type if the handler didn't set one.
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}

	// 3. Status. Default to 200 if unset.
	status := resp.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)

	// 4. Body. Decode if the handler base64-encoded it (binary payload).
	if resp.Body == "" {
		return nil
	}
	if resp.IsBase64Encoded {
		bytes, err := base64.StdEncoding.DecodeString(resp.Body)
		if err != nil {
			return fmt.Errorf("decode base64 body: %w", err)
		}
		_, err = w.Write(bytes)
		return err
	}
	_, err := w.Write([]byte(resp.Body))
	return err
}
