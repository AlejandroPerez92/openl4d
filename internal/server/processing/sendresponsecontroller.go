package processing

import (
	"encoding/json"
	"net/http"
	"openlambda/internal/function"
)

type SendResponseController struct {
	processingMap *function.ProcessingMap
}

func NewSendResponseController(processingMap *function.ProcessingMap) *SendResponseController {
	return &SendResponseController{
		processingMap: processingMap,
	}
}

func (p *SendResponseController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requestID := r.PathValue("request_id")
	if requestID == "" {
		http.Error(w, "missing RequestId", http.StatusBadRequest)
		return
	}

	invocation, ok := p.processingMap.Get(requestID)
	if !ok {
		http.Error(w, "no such invocation", http.StatusNotFound)
		return
	}
	if invocation.IsCancelled() || invocation.Ctx.Err() != nil {
		p.processingMap.Delete(requestID)
		http.Error(w, "invocation no longer active", http.StatusGone)
		return
	}

	var resp function.HttpResponseEvent
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&resp); err != nil {
		http.Error(w, "invalid response payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	select {
	case invocation.ResponseChan <- &resp:
		p.processingMap.Delete(requestID)
		w.WriteHeader(http.StatusAccepted)
	case <-invocation.Ctx.Done():
		p.processingMap.Delete(requestID)
		http.Error(w, "invocation no longer active", http.StatusGone)
	case <-r.Context().Done():
		http.Error(w, "client gone", 499)
	}

}
