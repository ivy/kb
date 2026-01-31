// cmd/kb-mcp/main.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	kbmcp "github.com/ivy/kb/internal/mcp"
)

func main() {
	// MCP uses stdout for JSON-RPC transport; all logging must go to stderr.
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	server := kbmcp.NewServer(socketPath())

	if err := server.Run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func socketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir + "/kb.sock"
	}
	return "/tmp/kb.sock"
}
