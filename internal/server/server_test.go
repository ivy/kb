package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ivy/kb/internal/agent"
	"github.com/ivy/kb/internal/config"
	"github.com/ivy/kb/internal/queue"
	"github.com/ivy/kb/internal/server"
	"github.com/ivy/kb/internal/session"
)

// fakeAgent implements agent.Agent for handler tests.
type fakeAgent struct {
	response    string
	err         error
	gotQuestion string
	gotModel    string
}

func (f *fakeAgent) Ask(_ context.Context, question string, _ []session.Turn, model string) (string, error) {
	f.gotQuestion = question
	f.gotModel = model
	if f.err != nil {
		return "", f.err
	}
	return f.response, nil
}

var _ agent.Agent = (*fakeAgent)(nil)

// fakeQueue implements queue.Queue for server construction.
type fakeQueue struct{}

func (f *fakeQueue) Enqueue(_ context.Context, _ queue.Job) error   { return nil }
func (f *fakeQueue) Stats(_ context.Context) (queue.Stats, error)   { return queue.Stats{}, nil }
func (f *fakeQueue) Start(_ context.Context) error                  { return nil }
func (f *fakeQueue) Stop(_ context.Context) error                   { return nil }

var _ queue.Queue = (*fakeQueue)(nil)

// fakeSessionManager implements session.Manager for server construction.
type fakeSessionManager struct{}

func (f *fakeSessionManager) Create(_ context.Context) (session.Session, error) {
	return session.Session{}, nil
}
func (f *fakeSessionManager) Get(_ context.Context, _ string) (session.Session, error) {
	return session.Session{}, nil
}
func (f *fakeSessionManager) Append(_ context.Context, _ string, _ session.Turn) error {
	return nil
}

var _ session.Manager = (*fakeSessionManager)(nil)

var _ = Describe("POST /v1/ask", func() {
	var (
		fa  *fakeAgent
		srv *server.Server
	)

	BeforeEach(func() {
		fa = &fakeAgent{response: "The deployment uses canary releases[^1].\n\n[^1]: source"}
		srv = server.New(fa, &fakeSessionManager{}, &fakeQueue{}, config.Server{}, config.Tailscale{})
	})

	It("returns the agent answer as text/markdown", func() {
		req := httptest.NewRequest(http.MethodPost, "/v1/ask", strings.NewReader("What deployment patterns are used?"))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(rec.Header().Get("Content-Type")).To(Equal("text/markdown; charset=utf-8"))
		Expect(rec.Body.String()).To(ContainSubstring("canary"))
		Expect(fa.gotQuestion).To(Equal("What deployment patterns are used?"))
	})

	It("forwards X-Model header to the agent", func() {
		req := httptest.NewRequest(http.MethodPost, "/v1/ask", strings.NewReader("test question"))
		req.Header.Set("X-Model", "gpt-4.1")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(fa.gotModel).To(Equal("gpt-4.1"))
	})

	It("returns 400 for an empty body", func() {
		req := httptest.NewRequest(http.MethodPost, "/v1/ask", strings.NewReader(""))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusBadRequest))
	})

	It("returns 500 when the agent errors", func() {
		fa.err = errors.New("llm timeout")
		req := httptest.NewRequest(http.MethodPost, "/v1/ask", strings.NewReader("question"))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		Expect(rec.Body.String()).To(ContainSubstring("llm timeout"))
	})
})

var _ = Describe("POST /v1/sessions/{id}/ask", func() {
	var (
		fa  *fakeAgent
		srv *server.Server
	)

	BeforeEach(func() {
		fa = &fakeAgent{response: "Session answer"}
		srv = server.New(fa, &fakeSessionManager{}, &fakeQueue{}, config.Server{}, config.Tailscale{})
	})

	It("returns the agent answer", func() {
		req := httptest.NewRequest(http.MethodPost, "/v1/sessions/abc123/ask", strings.NewReader("follow-up question"))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(rec.Header().Get("Content-Type")).To(Equal("text/markdown; charset=utf-8"))
		Expect(rec.Body.String()).To(Equal("Session answer"))
	})
})
