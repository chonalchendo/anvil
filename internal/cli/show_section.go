package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

// runShowSection prints one section via core.ScanSection. A bare name like
// "Approach" is read as an H2; "### X" selects a deeper level. With --json it
// emits {"id","heading","section"}.
func runShowSection(cmd *cobra.Command, v *core.Vault, t core.Type, basename, rawID, want string, asJSON bool) error {
	a, err := core.LoadArtifact(resolveArtifactPath(v.Root, t, basename))
	if err != nil {
		if os.IsNotExist(err) {
			return notFoundErr(core.CanonicalID(t, basename), rawID)
		}
		return fmt.Errorf("loading artifact: %w", err)
	}
	want = strings.TrimSpace(want)
	text, h2s, ok := core.ScanSection(a.Body, want)
	if !ok {
		id := core.CanonicalID(t, basename)
		if len(h2s) == 0 {
			return fmt.Errorf("%w: %q in %s (the body has no H2 headings)", ErrSectionNotFound, want, id)
		}
		return fmt.Errorf("%w: %q in %s; available:\n%s\ntry: anvil show %s %s --section %q", ErrSectionNotFound, want, id, strings.Join(h2s, "\n"), t, id, h2s[0])
	}
	if asJSON {
		heading, _, _ := strings.Cut(text, "\n")
		enc, err := json.MarshalIndent(map[string]string{"id": core.CanonicalID(t, basename), "heading": heading, "section": text}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(enc))
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), text)
	return nil
}
