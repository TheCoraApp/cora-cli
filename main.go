package main

import (
	"os"

	"github.com/clairitydev/cora/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
