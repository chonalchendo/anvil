package cli

import "github.com/spf13/cobra"

func newCreateCmd() *cobra.Command { return &cobra.Command{Use: "create"} }
