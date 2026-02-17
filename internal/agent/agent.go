package agent

import (
	"context"
	"errors"

	"github.com/ivy/kb/internal/config"
	"github.com/ivy/kb/internal/session"
	"github.com/ivy/kb/internal/store"
)

// Agent handles question-answering with KB context.
type Agent interface {
	Ask(ctx context.Context, question string, history []session.Turn, model string) (string, error)
}

// MRKLAgent implements Agent using a MRKL-style architecture.
type MRKLAgent struct {
	store   store.Store
	vectors store.VectorStore
	cfg     config.Agent
}

// New creates a new Agent.
func New(s store.Store, v store.VectorStore, cfg config.Agent) Agent {
	return &MRKLAgent{store: s, vectors: v, cfg: cfg}
}

func (a *MRKLAgent) Ask(ctx context.Context, question string, history []session.Turn, model string) (string, error) {
	return "", errors.New("not implemented")
}

var _ Agent = (*MRKLAgent)(nil)
