package main

import (
	"os"

	"github.com/rahulvramesh/justup/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
