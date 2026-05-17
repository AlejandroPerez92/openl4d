package processing

import (
	"encoding/json"
	"net/http"
	"openlambda/internal/function"
)

type InvocationErrorController struct {
	processingMap *function.ProcessingMap
}

func NewInvocationErrorController(processingMap *function.ProcessingMap) *InvocationErrorController {
	return &InvocationErrorController{
		processingMap: processingMap,
	}
}

func (p *InvocationErrorController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	var resp function.InvocationErrorEvent
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&resp); err != nil {
		http.Error(w, "invalid response payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	response := function.HttpResponseEvent{
		StatusCode: http.StatusBadGateway,
		Body:       resp.ErrorMessage,
	}

	select {
	case invocation.ResponseChan <- &response:
		p.processingMap.Delete(requestID)
		w.WriteHeader(http.StatusAccepted)
	case <-r.Context().Done():
		http.Error(w, "client gone", 499)
	}

}
