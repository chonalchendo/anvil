package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

func newSessionGCCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Remove session stubs that are empty and past their retention_until date",
		Long: `Remove session stubs that are empty (no handoff body) and past their
retention_until date. Sessions that carry a handoff body are never touched,
regardless of age.

Use --dry-run to report what would be pruned without removing anything.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			removed, skipped, err := pruneExpiredStubs(cmd.ErrOrStderr(), v.Root, dryRun, time.Now().UTC())
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			label := "removed"
			if dryRun {
				label = "would remove"
			}
			if len(removed) == 0 {
				fmt.Fprintf(w, "0 expired stubs to prune (checked %d sessions)\n", skipped+len(removed))
				return nil
			}
			for _, p := range removed {
				fmt.Fprintf(w, "%s expired stub: %s\n", label, p)
			}
			fmt.Fprintf(w, "%s %d expired stub(s); %d session(s) retained\n", label, len(removed), skipped)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be pruned without removing anything")
	return cmd
}

// pruneExpiredStubs scans the session directory and removes (or reports, when
// dryRun is true) files that are BOTH empty (no handoff body) AND past their
// retention_until date relative to now. Sessions with a handoff body are
// never touched. Returns the paths acted on and the count of retained sessions.
func pruneExpiredStubs(warnW io.Writer, vaultRoot string, dryRun bool, now time.Time) (removed []string, retained int, err error) {
	dir := filepath.Join(vaultRoot, core.TypeSession.Dir())
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("reading %s: %w", dir, err)
	}
	today := now.Format("2006-01-02")
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		a, err := core.LoadArtifact(path)
		if err != nil {
			// Skip unreadable files rather than aborting the whole sweep, but
			// surface them so a corrupt session is not invisible to the operator.
			fmt.Fprintf(warnW, "WARN: skipping unreadable session %s: %v\n", path, err)
			retained++
			continue
		}
		// Handoff-bearing sessions are never pruned.
		if strings.TrimSpace(a.Body) != "" {
			retained++
			continue
		}
		retentionUntil, _ := a.FrontMatter["retention_until"].(string)
		if retentionUntil == "" || retentionUntil >= today {
			// No retention date, or not yet expired.
			retained++
			continue
		}
		// Empty stub past its retention date.
		if !dryRun {
			if err := os.Remove(path); err != nil {
				return removed, retained, fmt.Errorf("removing %s: %w", path, err)
			}
		}
		removed = append(removed, path)
	}
	return removed, retained, nil
}
