package cli

import (
	"errors"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/ui"
)

func newUICmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:     "ui",
		Short:   "Serve a read-only dark web view of the vault on a loopback address",
		Args:    cobra.NoArgs,
		Example: "  anvil ui\n  anvil ui --addr 127.0.0.1:7781",
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			db, err := indexForRead(v)
			if err != nil {
				return err
			}
			defer db.Close() //nolint:errcheck // close in defer; error not actionable
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			err = ui.Serve(ctx, v, db, addr, cmd.ErrOrStderr())
			var notLoopback *ui.ErrAddrNotLoopback
			if errors.As(err, &notLoopback) {
				return errfmt.NewStructured("ui_addr_not_loopback").
					Set("addr", notLoopback.Addr).
					Set("hint", "bind 127.0.0.0/8, ::1 or localhost; the view has no auth")
			}
			return err
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:7780", "loopback host:port to serve on")
	return cmd
}
