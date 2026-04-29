// Package main is the anvil CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/chonalchendo/anvil/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
