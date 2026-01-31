// cmd/kb-server/main.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ivy/kb/internal/server"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := server.New(socketPath())
	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func socketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir + "/kb.sock"
	}
	return "/tmp/kb.sock"
}
