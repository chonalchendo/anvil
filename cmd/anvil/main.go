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
		switch {
		case errors.Is(err, cli.ErrArtifactNotFound):
			os.Exit(2)
		case errors.Is(err, cli.ErrSchemaInvalid):
			os.Exit(3)
		default:
			os.Exit(1)
		}
	}
}
