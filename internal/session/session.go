package session

import (
	"context"
	"time"
)

// Manager manages conversation sessions.
type Manager interface {
	Create(ctx context.Context) (Session, error)
	Get(ctx context.Context, id string) (Session, error)
	Append(ctx context.Context, id string, turn Turn) error
}

// Session represents a conversation session.
type Session struct {
	ID        string
	CreatedAt time.Time
	Turns     []Turn
}

// Turn represents a single exchange in a session.
type Turn struct {
	Role    string
	Content string
	Time    time.Time
}
