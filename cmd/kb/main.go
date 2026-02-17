package main

import (
	"os"

	"github.com/ivy/kb/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
