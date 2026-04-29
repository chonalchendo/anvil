package cli

import "github.com/spf13/cobra"

func newLinkCmd() *cobra.Command { return &cobra.Command{Use: "link"} }
