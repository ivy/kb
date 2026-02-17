package queue

import "context"

// Queue abstracts the job queue backend.
type Queue interface {
	Enqueue(ctx context.Context, job Job) error
	Stats(ctx context.Context) (Stats, error)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Job represents a unit of work.
type Job struct {
	Kind    string
	Payload []byte
}

// Stats contains queue statistics.
type Stats struct {
	Pending int
	Running int
	Failed  int
}

// HandlerFunc processes a job payload.
type HandlerFunc func(ctx context.Context, payload []byte) error
