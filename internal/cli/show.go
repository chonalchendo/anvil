package cli

import "github.com/spf13/cobra"

func newShowCmd() *cobra.Command { return &cobra.Command{Use: "show"} }
