// internal/server/server_test.go
package server

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	kbv1 "github.com/ivy/kb/gen/kb/v1"
)

func TestList_ReturnsKnowledgeBases(t *testing.T) {
	s := &Server{}
	resp, err := s.List(context.Background(), connect.NewRequest(&kbv1.ListRequest{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Msg.KnowledgeBases) != 2 {
		t.Errorf("expected 2 knowledge bases, got %d", len(resp.Msg.KnowledgeBases))
	}
}

func TestQuery_DefaultEffort(t *testing.T) {
	s := &Server{}
	resp, err := s.Query(context.Background(), connect.NewRequest(&kbv1.QueryRequest{
		Question: "test",
		// Effort unspecified — should default to MEDIUM
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Msg.Answer == "" {
		t.Error("expected non-empty answer")
	}
	// The stub should include EFFORT_MEDIUM in the response
	expected := "EFFORT_MEDIUM"
	if !contains(resp.Msg.Answer, expected) {
		t.Errorf("expected answer to contain %q, got %q", expected, resp.Msg.Answer)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s != "" && substr != "" &&
		   (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
