package controller

import (
	"context"
	"testing"
	"time"
)

func TestWorkqueue_Dedup(t *testing.T) {
	q := newWorkqueue()
	defer q.Close()
	q.Add("a")
	q.Add("a")
	q.Add("a")
	if q.Len() != 1 {
		t.Fatalf("expected len 1 (dedup), got %d", q.Len())
	}
}

func TestWorkqueue_DoneReAdded(t *testing.T) {
	q := newWorkqueue()
	defer q.Close()
	q.Add("a")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, ok := q.Get(ctx)
	if !ok || got != "a" {
		t.Fatalf("expected a, got %q ok=%v", got, ok)
	}
	q.Add("a") // re-added while processing
	q.Done("a")
	got2, ok := q.Get(ctx)
	if !ok || got2 != "a" {
		t.Fatalf("expected a redelivered, got %q ok=%v", got2, ok)
	}
}

func TestWorkqueue_CloseUnblocksGet(t *testing.T) {
	q := newWorkqueue()
	done := make(chan struct{})
	go func() {
		_, _ = q.Get(context.Background())
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	q.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close did not unblock Get")
	}
}
