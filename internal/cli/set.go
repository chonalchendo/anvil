package cli

import "github.com/spf13/cobra"

func newSetCmd() *cobra.Command { return &cobra.Command{Use: "set"} }
