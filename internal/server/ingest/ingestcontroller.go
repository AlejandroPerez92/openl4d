package ingest

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"openlambda/internal/function"
	"strings"
	"time"
)

type ProxyController struct {
	pendingQueue *PendingQueue
}

func NewProxyController(pendingQueue *PendingQueue) *ProxyController {
	return &ProxyController{
		pendingQueue: pendingQueue,
	}
}

func (p *ProxyController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	invocation := function.Invocation{
		Event:        function.FromRequest(r),
		ResponseChan: make(chan *function.HttpResponseEvent),
		Ctx:          context.Background(),
		Deadline:     time.Now().Add(30 * time.Second),
	}

	p.pendingQueue.Enqueue(&invocation)

	select {
	case resp := <-invocation.ResponseChan:
		err := writeResponse(w, *resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case <-r.Context().Done():
		http.Error(w, "client gone", 499)
	case <-time.After(invocation.Deadline.Sub(time.Now())):
		http.Error(w, "function timeout", http.StatusGatewayTimeout)
	}
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
