package ingest

import "openlambda/internal/function"

type PendingQueue struct {
	ch chan *function.Invocation
}

func NewPendingQueue() *PendingQueue {
	return &PendingQueue{
		ch: make(chan *function.Invocation),
	}
}

func (q *PendingQueue) Enqueue(invocation *function.Invocation) {
	//@TODO: add capacity control and sigterm control
	q.ch <- invocation
}

func (q *PendingQueue) Dequeue() *function.Invocation {
	//@TODO: add sigterm control
	return <-q.ch
}
