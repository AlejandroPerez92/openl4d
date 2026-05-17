package function

import (
	"context"
	"errors"
	"sync"
)

var ErrQueueClosed = errors.New("queue closed")
var ErrQueueDrained = errors.New("queue drained")

type PendingQueue struct {
	ch        chan *Invocation
	closed    chan struct{}
	closeOnce sync.Once
}

func NewPendingQueue(capacity int) *PendingQueue {
	return &PendingQueue{
		ch:     make(chan *Invocation, capacity),
		closed: make(chan struct{}),
	}
}

func (q *PendingQueue) Enqueue(invocation *Invocation) error {
	select {
	case <-q.closed:
		return ErrQueueClosed
	case <-invocation.Ctx.Done():
		return invocation.Ctx.Err()
	case q.ch <- invocation:
		return nil
	}
}

func (q *PendingQueue) Dequeue(ctx context.Context) (*Invocation, error) {
	select {
	case inv := <-q.ch:
		return inv, nil
	default:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case inv := <-q.ch:
		return inv, nil
	case <-q.closed:
		select {
		case inv := <-q.ch:
			return inv, nil
		default:
			return nil, ErrQueueDrained
		}
	}
}

func (q *PendingQueue) Close() {
	q.closeOnce.Do(func() {
		close(q.closed)
	})
}

func (q *PendingQueue) Len() int {
	return len(q.ch)
}
