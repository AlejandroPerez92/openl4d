package function

type PendingQueue struct {
	ch chan *Invocation
}

func NewPendingQueue() *PendingQueue {
	return &PendingQueue{
		ch: make(chan *Invocation),
	}
}

func (q *PendingQueue) Enqueue(invocation *Invocation) error {
	//@TODO: add capacity control and sigterm control
	select {
	case q.ch <- invocation:
		return nil
	case <-invocation.Ctx.Done():
		return invocation.Ctx.Err()
	}
}

func (q *PendingQueue) Dequeue() *Invocation {
	//@TODO: add sigterm control
	return <-q.ch
}
