// cmd/kb-server/main.go
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/ivy/kb/internal/server"
	"github.com/ivy/kb/internal/socket"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	socketPath, err := socket.ResolveSocketPath()
	if err != nil {
		log.Fatalf("failed to resolve socket path: %v", err)
	}

	srv := server.New(socketPath)
	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
