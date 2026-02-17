package session_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ivy/kb/internal/session"
)

// FakeManager implements session.Manager with an in-memory map.
type FakeManager struct {
	mu       sync.Mutex
	sessions map[string]*session.Session
	counter  int
}

func NewFakeManager() *FakeManager {
	return &FakeManager{sessions: make(map[string]*session.Session)}
}

func (f *FakeManager) Create(_ context.Context) (session.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counter++
	s := session.Session{
		ID:        fmt.Sprintf("session-%d", f.counter),
		CreatedAt: time.Now(),
	}
	f.sessions[s.ID] = &s
	return s, nil
}

func (f *FakeManager) Get(_ context.Context, id string) (session.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[id]
	if !ok {
		return session.Session{}, errors.New("session not found")
	}
	return *s, nil
}

func (f *FakeManager) Append(_ context.Context, id string, turn session.Turn) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[id]
	if !ok {
		return errors.New("session not found")
	}
	s.Turns = append(s.Turns, turn)
	return nil
}

var _ session.Manager = (*FakeManager)(nil)
