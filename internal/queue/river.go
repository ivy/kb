package queue

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ivy/kb/internal/config"
)

// RiverQueue implements Queue using River (PostgreSQL-backed job queue).
type RiverQueue struct{}

// NewRiverQueue creates a new River-backed queue.
func NewRiverQueue(pool *pgxpool.Pool, cfg config.Workers, handlers map[string]HandlerFunc) (Queue, error) {
	return nil, errors.New("not implemented")
}
