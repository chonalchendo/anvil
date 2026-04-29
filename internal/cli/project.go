package cli

import "github.com/spf13/cobra"

func newProjectCmd() *cobra.Command { return &cobra.Command{Use: "project"} }
