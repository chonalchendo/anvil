// Package cli holds the cobra root command and subcommand wiring.
//
// v0.0.0-dev: scaffold only. Cobra and fang land in a later spec when the
// first user-facing command is added.
package cli

import "fmt"

// Run is the CLI entrypoint. v0.0.0-dev prints a scaffold banner and exits.
//
// fmt.Println is intentional at this stage; once cobra+fang lands, output
// switches to cmd.Println per AGENTS.md. args is unused until the root
// command tree is wired up.
func Run(args []string) error {
	_ = args
	fmt.Println("anvil v0.0.0-dev (scaffold)")
	return nil
}
