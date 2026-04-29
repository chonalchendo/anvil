// Package main is the anvil CLI entrypoint.
package main

import (
	"context"
	"errors"
	"os"

	"github.com/chonalchendo/anvil/internal/cli"
)

func main() {
	if err := cli.Execute(context.Background()); err != nil {
		if errors.Is(err, cli.ErrArtifactNotFound) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
