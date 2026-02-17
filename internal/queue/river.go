package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"github.com/ivy/kb/internal/config"
)

// GenericJobArgs is a single River job type that wraps all KB job kinds.
// River requires typed args, but our Queue interface is kind+payload.
type GenericJobArgs struct {
	JobKind string `json:"job_kind"`
	Payload []byte `json:"payload"`
}

func (GenericJobArgs) Kind() string { return "kb_job" }

func (a GenericJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: a.JobKind}
}

// genericWorker dispatches GenericJobArgs to the correct HandlerFunc.
type genericWorker struct {
	river.WorkerDefaults[GenericJobArgs]
	handlers map[string]HandlerFunc
}

func (w *genericWorker) Work(ctx context.Context, job *river.Job[GenericJobArgs]) error {
	handler, ok := w.handlers[job.Args.JobKind]
	if !ok {
		return fmt.Errorf("no handler registered for job kind %q", job.Args.JobKind)
	}
	return handler(ctx, job.Args.Payload)
}

// RiverQueue implements Queue using River (PostgreSQL-backed job queue).
type RiverQueue struct {
	client   *river.Client[pgx.Tx]
	handlers map[string]HandlerFunc
}

// NewRiverQueue creates a new River-backed queue.
func NewRiverQueue(pool *pgxpool.Pool, cfg config.Workers, handlers map[string]HandlerFunc) (Queue, error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &genericWorker{handlers: handlers})

	reindexWorkers := cfg.Reindex
	if reindexWorkers <= 0 {
		reindexWorkers = 2
	}
	embedWorkers := cfg.Embed
	if embedWorkers <= 0 {
		embedWorkers = 4
	}

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			"reindex": {MaxWorkers: reindexWorkers},
			"embed":   {MaxWorkers: embedWorkers},
		},
		Workers: workers,
	})
	if err != nil {
		return nil, fmt.Errorf("creating river client: %w", err)
	}

	return &RiverQueue{client: client, handlers: handlers}, nil
}

func (q *RiverQueue) Enqueue(ctx context.Context, job Job) error {
	_, err := q.client.Insert(ctx, GenericJobArgs{
		JobKind: job.Kind,
		Payload: job.Payload,
	}, nil)
	if err != nil {
		return fmt.Errorf("inserting river job: %w", err)
	}
	return nil
}

func (q *RiverQueue) Stats(ctx context.Context) (Stats, error) {
	var stats Stats

	for _, state := range []struct {
		s     rivertype.JobState
		field *int
	}{
		{rivertype.JobStateAvailable, &stats.Pending},
		{rivertype.JobStateRunning, &stats.Running},
		{rivertype.JobStateRetryable, &stats.Failed},
	} {
		result, err := q.client.JobList(ctx, river.NewJobListParams().
			Kinds("kb_job").
			States(state.s))
		if err != nil {
			return Stats{}, fmt.Errorf("listing %s jobs: %w", state.s, err)
		}
		*state.field = len(result.Jobs)
	}

	return stats, nil
}

func (q *RiverQueue) Start(ctx context.Context) error {
	return q.client.Start(ctx)
}

func (q *RiverQueue) Stop(ctx context.Context) error {
	return q.client.Stop(ctx)
}

// Client returns the underlying River client for testing purposes.
func (q *RiverQueue) Client() *river.Client[pgx.Tx] {
	return q.client
}

// MarshalJob serializes a Job to JSON for use as GenericJobArgs payload.
func MarshalJob(v any) ([]byte, error) {
	return json.Marshal(v)
}
