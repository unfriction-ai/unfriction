package main

import (
	"fmt"
	"os"

	"github.com/unfriction-ai/unfriction/internal/cli"
)

// version is set by release builds using -ldflags "-X main.version=<version>".
var version = "dev"

func main() {
	if err := cli.NewCommand(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
