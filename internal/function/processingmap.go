package function

import "sync"

type ProcessingMap struct {
	mu          sync.RWMutex
	invocations map[string]*Invocation
}

func NewProcessingMap() *ProcessingMap {
	return &ProcessingMap{
		invocations: make(map[string]*Invocation),
	}
}

func (m *ProcessingMap) Put(invocation *Invocation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invocations[invocation.Event.RequestContext.RequestID] = invocation
}

func (m *ProcessingMap) Get(eventID string) (*Invocation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	invocation, ok := m.invocations[eventID]
	return invocation, ok
}

func (m *ProcessingMap) Delete(eventID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.invocations, eventID)
}

func (m *ProcessingMap) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.invocations)
}
