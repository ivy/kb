package agent_test

import (
	"context"

	"github.com/ivy/kb/internal/agent"
	"github.com/ivy/kb/internal/session"
)

// FakeAgent implements agent.Agent with canned responses.
type FakeAgent struct {
	Response string
	Err      error
}

func (f *FakeAgent) Ask(_ context.Context, _ string, _ []session.Turn, _ string) (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	return f.Response, nil
}

var _ agent.Agent = (*FakeAgent)(nil)
