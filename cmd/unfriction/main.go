package main

import (
	"fmt"
	"os"

	"github.com/unfriction-ai/unfriction/internal/cli"
)

func main() {
	if err := cli.NewCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
