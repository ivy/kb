// internal/mcp/server.go
package mcp

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	kbv1 "github.com/ivy/kb/gen/kb/v1"
	"github.com/ivy/kb/gen/kb/v1/kbv1connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	mcpServer  *mcp.Server
	socketPath string
	httpClient *http.Client
}

func NewServer(socketPath string) *Server {
	s := &Server{
		socketPath: socketPath,
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		},
	}

	s.mcpServer = mcp.NewServer(
		&mcp.Implementation{
			Name:    "kb",
			Version: "0.1.0",
		},
		nil,
	)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_knowledge_bases",
		Description: "List available knowledge bases.",
	}, s.handleList)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ask_knowledge_base",
		Description: "Search knowledge bases using natural language. Returns relevant context with source citations.",
	}, s.handleAsk)

	return s
}

func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

func (s *Server) client() kbv1connect.KnowledgeBaseServiceClient {
	// URL host is ignored when using Unix socket transport
	return kbv1connect.NewKnowledgeBaseServiceClient(s.httpClient, "http://localhost")
}

func (s *Server) handleList(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ListKnowledgeBasesInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("list_knowledge_bases: called")

	resp, err := s.client().List(ctx, connect.NewRequest(&kbv1.ListRequest{}))
	if err != nil {
		return errorResult("failed to contact kb-server: " + err.Error()), nil, nil
	}

	// Format response
	var sb strings.Builder
	sb.WriteString("Available knowledge bases:\n")
	for _, kb := range resp.Msg.KnowledgeBases {
		fmt.Fprintf(&sb, "- %s: %s\n", kb.Name, kb.Description)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func (s *Server) handleAsk(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input AskKnowledgeBaseInput,
) (*mcp.CallToolResult, any, error) {
	effort := parseEffort(input.Effort)

	log.Printf("ask_knowledge_base: question=%q effort=%s knowledge_bases=%v",
		input.Question, effort, input.KnowledgeBases)

	resp, err := s.client().Query(ctx, connect.NewRequest(&kbv1.QueryRequest{
		Question:       input.Question,
		Effort:         effort,
		KnowledgeBases: input.KnowledgeBases,
	}))
	if err != nil {
		return errorResult("query failed: " + err.Error()), nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: resp.Msg.Answer}},
	}, nil, nil
}

func parseEffort(s string) kbv1.Effort {
	switch s {
	case "low":
		return kbv1.Effort_EFFORT_LOW
	case "high":
		return kbv1.Effort_EFFORT_HIGH
	default:
		return kbv1.Effort_EFFORT_MEDIUM
	}
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}
}
