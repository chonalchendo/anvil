package cli

import "github.com/spf13/cobra"

func newListCmd() *cobra.Command { return &cobra.Command{Use: "list"} }
