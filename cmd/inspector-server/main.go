package main

import (
	"os"

	"github.com/mandacode-labs/inspector/internal/cmd/inspector-server"
)

var version = "dev"

func main() {
	cmd := server.NewCmd()
	cmd.Version = version
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
