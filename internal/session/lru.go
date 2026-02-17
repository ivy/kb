package session

import (
	"context"
	"errors"

	"github.com/ivy/kb/internal/config"
)

// LRUManager implements Manager with an LRU cache backed by JSONL files.
type LRUManager struct{}

// NewLRUManager creates a new LRU-based session manager.
func NewLRUManager(cfg config.Sessions) Manager {
	return &LRUManager{}
}

func (m *LRUManager) Create(ctx context.Context) (Session, error) {
	return Session{}, errors.New("not implemented")
}

func (m *LRUManager) Get(ctx context.Context, id string) (Session, error) {
	return Session{}, errors.New("not implemented")
}

func (m *LRUManager) Append(ctx context.Context, id string, turn Turn) error {
	return errors.New("not implemented")
}

var _ Manager = (*LRUManager)(nil)
