package main

import (
	"os"
	_ "time/tzdata"

	"github.com/ivy/kb/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
