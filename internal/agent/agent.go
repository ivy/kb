package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/tools"

	agenttools "github.com/ivy/kb/internal/agent/tools"
	"github.com/ivy/kb/internal/config"
	"github.com/ivy/kb/internal/embed"
	"github.com/ivy/kb/internal/session"
	"github.com/ivy/kb/internal/store"
)

// Agent handles question-answering with KB context.
type Agent interface {
	Ask(ctx context.Context, question string, history []session.Turn, model string) (string, error)
}

// MRKLAgent implements Agent using a MRKL-style architecture with LangChainGo.
type MRKLAgent struct {
	store    store.Store
	vectors  store.VectorStore
	embedder *embed.Client
	cfg      config.Agent
}

// New creates a new Agent.
func New(s store.Store, v store.VectorStore, embedder *embed.Client, cfg config.Agent) Agent {
	return &MRKLAgent{store: s, vectors: v, embedder: embedder, cfg: cfg}
}

func (a *MRKLAgent) Ask(ctx context.Context, question string, history []session.Turn, model string) (string, error) {
	llm, err := a.newLLM(model)
	if err != nil {
		return "", fmt.Errorf("creating LLM: %w", err)
	}

	agentTools := []tools.Tool{
		agenttools.NewSearchTool(a.embedder, a.vectors),
		agenttools.NewLookupTool(a.store),
		agenttools.NewReadTool(a.store),
		agenttools.NewListTool(a.store),
	}

	ag := agents.NewOneShotAgent(llm, agentTools,
		agents.WithPromptPrefix(systemPrompt),
	)

	maxIter := a.cfg.MaxIterations
	if maxIter == 0 {
		maxIter = 15
	}

	executor := agents.NewExecutor(ag,
		agents.WithMaxIterations(maxIter),
	)

	input := a.buildInput(question, history)

	result, err := chains.Run(ctx, executor, input)
	if err != nil {
		return "", fmt.Errorf("agent execution: %w", err)
	}

	return result, nil
}

func (a *MRKLAgent) newLLM(model string) (llms.Model, error) {
	if model == "" {
		model = a.cfg.DefaultModel
	}

	if model == "" {
		switch a.cfg.Provider {
		case "openai":
			model = "gpt-4.1"
		case "anthropic":
			model = "claude-sonnet-4-5"
		}
	}

	switch a.cfg.Provider {
	case "openai":
		return openai.New(
			openai.WithToken(a.cfg.APIKey),
			openai.WithModel(model),
		)
	case "anthropic":
		return anthropic.New(
			anthropic.WithToken(a.cfg.APIKey),
			anthropic.WithModel(model),
		)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %q", a.cfg.Provider)
	}
}

func (a *MRKLAgent) buildInput(question string, history []session.Turn) string {
	if len(history) == 0 {
		return question
	}

	var sb strings.Builder
	sb.WriteString("Previous conversation:\n")
	for _, turn := range history {
		role := "User"
		if turn.Role == "agent" || turn.Role == "assistant" {
			role = "Agent"
		}
		fmt.Fprintf(&sb, "%s: %s\n", role, turn.Content)
	}
	sb.WriteString("\nCurrent question:\n")
	sb.WriteString(question)
	return sb.String()
}

var _ Agent = (*MRKLAgent)(nil)
