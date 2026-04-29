// Package main is the anvil CLI entrypoint.
package main

import (
	"context"
	"os"

	"github.com/chonalchendo/anvil/internal/cli"
)

func main() {
	if err := cli.Execute(context.Background()); err != nil {
		os.Exit(1)
	}
}
