package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/output"
	"github.com/chonalchendo/anvil/internal/core"
)

// linkBody is one resolved linked artifact under --links <type> --body: the
// wikilink target id, its frontmatter status (so a non-active design reads as
// advisory), and its capped body.
type linkBody struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Body   string `json:"body"`
}

// runShowLinks filters an artifact's frontmatter and body for wikilinks of the
// requested type and emits their targets (without [[ ]] brackets) one per line,
// or as a JSON array under --json. With --body each target's body is loaded and printed
// instead (see emitLinkBodies). Empty result exits 0 with no output (text) or [].
func runShowLinks(cmd *cobra.Command, vault *core.Vault, t core.Type, artifactID string, linkType core.Type, asJSON, includeBody bool) error {
	path := resolveArtifactPath(vault.Root, t, artifactID)
	a, err := core.LoadArtifact(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrArtifactNotFound, artifactID)
		}
		return fmt.Errorf("loading artifact: %w", err)
	}

	prefix := "[[" + string(linkType) + "."
	seen := make(map[string]bool)
	targets := make([]string, 0)
	for _, fmval := range a.FrontMatter {
		switch typed := fmval.(type) {
		case string:
			if target, ok := wikilinkTarget(typed, prefix); ok && !seen[target] {
				seen[target] = true
				targets = append(targets, target)
			}
		case []any:
			for _, elem := range typed {
				s, ok := elem.(string)
				if !ok {
					continue
				}
				if target, ok := wikilinkTarget(s, prefix); ok && !seen[target] {
					seen[target] = true
					targets = append(targets, target)
				}
			}
		}
	}
	// Body wikilinks are real edges too: a contract links its governing
	// conventions from `## Code design` prose, not a frontmatter slot. Surface
	// them on the same rail so loading a contract yields its conventions.
	for _, target := range core.BodyWikilinkTargetsOfType(a.Body, linkType) {
		if !seen[target] {
			seen[target] = true
			targets = append(targets, target)
		}
	}
	sort.Strings(targets)

	if includeBody {
		return emitLinkBodies(cmd, vault, linkType, targets, asJSON)
	}

	w := cmd.OutOrStdout()
	if asJSON {
		b, err := json.Marshal(targets)
		if err != nil {
			return fmt.Errorf("marshaling links output: %w", err)
		}
		fmt.Fprintln(w, string(b))
		return nil
	}
	for _, target := range targets {
		fmt.Fprintln(w, target)
	}
	return nil
}

// emitLinkBodies loads each resolved link target's body (capped at
// showBodyLineCap, same as `show <type> --body`) and emits them: a header
// carrying id + status per body in text mode, or a JSON array of linkBody under
// --json. The count goes to stderr (composability: prose off the data stream)
// so a large fan-out is visible, not silent.
func emitLinkBodies(cmd *cobra.Command, v *core.Vault, linkType core.Type, targets []string, asJSON bool) error {
	bodies := make([]linkBody, 0, len(targets))
	for _, target := range targets {
		id := canonicalArtifactID(v, linkType, target)
		a, err := core.LoadArtifact(resolveArtifactPath(v.Root, linkType, id))
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%w: %s", ErrArtifactNotFound, target)
			}
			return fmt.Errorf("loading linked artifact %s: %w", target, err)
		}
		body := strings.TrimPrefix(a.Body, "\n")
		if lines := strings.Split(body, "\n"); body != "" && len(lines) > showBodyLineCap {
			body = strings.Join(lines[:showBodyLineCap], "\n")
			cmd.PrintErrln(output.BodyClipHint(showBodyLineCap, len(lines), a.Path))
		}
		status, _ := a.FrontMatter["status"].(string)
		bodies = append(bodies, linkBody{ID: target, Status: status, Body: body})
	}

	noun := string(linkType)
	if len(bodies) != 1 {
		noun += "s"
	}
	cmd.PrintErrf("%d %s\n", len(bodies), noun)

	w := cmd.OutOrStdout()
	if asJSON {
		b, err := json.Marshal(bodies)
		if err != nil {
			return fmt.Errorf("marshaling link bodies: %w", err)
		}
		fmt.Fprintln(w, string(b))
		return nil
	}
	for _, lb := range bodies {
		status := lb.Status
		if status == "" {
			status = "unset"
		}
		fmt.Fprintf(w, "=== %s (status: %s) ===\n", lb.ID, status)
		fmt.Fprintln(w, lb.Body)
	}
	return nil
}

// wikilinkTarget returns the inner target of a wikilink if s has the form
// [[prefix<rest>]] (non-empty rest, closing ]]). Used to filter frontmatter
// fields by type prefix without re-invoking the full wikilink regex.
func wikilinkTarget(s, prefix string) (string, bool) {
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, "]]") {
		return "", false
	}
	// Strip surrounding [[ and ]] — prefix already begins with [[.
	inner := s[2 : len(s)-2]
	if inner == "" {
		return "", false
	}
	return inner, true
}
