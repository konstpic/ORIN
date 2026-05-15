package controller

import (
	"context"
	"sync"
	"time"
)

// workqueue is a tiny FIFO-with-deduplication queue inspired by
// client-go's workqueue but without rate limiting (which the controller
// itself handles via AddAfter on error).
type workqueue struct {
	mu      sync.Mutex
	dirty   map[string]struct{}
	in      map[string]struct{}
	queue   []string
	cond    *sync.Cond
	closed  bool
}

func newWorkqueue() *workqueue {
	q := &workqueue{
		dirty: make(map[string]struct{}),
		in:    make(map[string]struct{}),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Add enqueues a key if it isn't already queued or being processed.
func (q *workqueue) Add(key string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	if _, ok := q.dirty[key]; ok {
		return
	}
	q.dirty[key] = struct{}{}
	if _, processing := q.in[key]; processing {
		return
	}
	q.queue = append(q.queue, key)
	q.cond.Signal()
}

// AddAfter schedules an enqueue d in the future.
func (q *workqueue) AddAfter(key string, d time.Duration) {
	go func() {
		time.Sleep(d)
		q.Add(key)
	}()
}

// Get blocks until a key is available or ctx is done.
func (q *workqueue) Get(ctx context.Context) (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.queue) == 0 && !q.closed {
		// We need to release the lock during ctx.Done; use a goroutine to
		// signal the cond.
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				q.cond.Broadcast()
			case <-done:
			}
		}()
		q.cond.Wait()
		close(done)
		if ctx.Err() != nil {
			return "", false
		}
	}
	if q.closed && len(q.queue) == 0 {
		return "", false
	}
	key := q.queue[0]
	q.queue = q.queue[1:]
	q.in[key] = struct{}{}
	delete(q.dirty, key)
	return key, true
}

// Done marks a key processed; if it was re-Added while processing, it
// will be re-enqueued now.
func (q *workqueue) Done(key string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.in, key)
	if _, dirty := q.dirty[key]; dirty {
		q.queue = append(q.queue, key)
		q.cond.Signal()
	}
}

// Close shuts the queue down; pending Get calls return ok=false.
func (q *workqueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

// Len returns the current queue length (for metrics).
func (q *workqueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queue)
}
