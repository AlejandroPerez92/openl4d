package function

type PendingQueue struct {
	ch chan *Invocation
}

func NewPendingQueue() *PendingQueue {
	return &PendingQueue{
		ch: make(chan *Invocation),
	}
}

func (q *PendingQueue) Enqueue(invocation *Invocation) {
	//@TODO: add capacity control and sigterm control
	q.ch <- invocation
}

func (q *PendingQueue) Dequeue() *Invocation {
	//@TODO: add sigterm control
	return <-q.ch
}
