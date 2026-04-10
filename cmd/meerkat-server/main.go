package main

import (
	"os"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server"
)

var version = "dev"

func main() {
	cmd := server.NewCmd()
	cmd.Version = version
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
