package queue_test

import (
	"context"
	"sync"

	"github.com/ivy/kb/internal/queue"
)

// FakeQueue implements queue.Queue, capturing enqueued jobs.
type FakeQueue struct {
	mu      sync.Mutex
	jobs    []queue.Job
	started bool
	err     error
}

func NewFakeQueue() *FakeQueue {
	return &FakeQueue{}
}

func (f *FakeQueue) Enqueue(_ context.Context, job queue.Job) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobs = append(f.jobs, job)
	return nil
}

func (f *FakeQueue) Stats(_ context.Context) (queue.Stats, error) {
	if f.err != nil {
		return queue.Stats{}, f.err
	}
	return queue.Stats{Pending: len(f.jobs)}, nil
}

func (f *FakeQueue) Start(_ context.Context) error {
	if f.err != nil {
		return f.err
	}
	f.started = true
	return nil
}

func (f *FakeQueue) Stop(_ context.Context) error {
	f.started = false
	return nil
}

var _ queue.Queue = (*FakeQueue)(nil)
