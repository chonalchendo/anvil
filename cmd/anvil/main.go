// Package main is the anvil CLI entrypoint.
package main

import (
	"context"
	"errors"
	"os"

	"github.com/chonalchendo/anvil/internal/cli"
	"github.com/chonalchendo/anvil/internal/core"
)

func main() {
	if err := cli.Execute(context.Background()); err != nil {
		switch {
		case errors.Is(err, core.ErrPlanTDD):
			os.Exit(3)
		case errors.Is(err, core.ErrPlanDAG):
			os.Exit(2)
		case errors.Is(err, cli.ErrArtifactNotFound):
			os.Exit(2)
		case errors.Is(err, cli.ErrSchemaInvalid):
			os.Exit(1)
		case errors.Is(err, cli.ErrUnresolvedLinks):
			os.Exit(1)
		default:
			os.Exit(1)
		}
	}
}
