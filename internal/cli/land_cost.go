package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// stampLandCost records the landed PR's cost on the issue. The merge is
// already confirmed, so any failure warns and leaves the fields unset.
func stampLandCost(errW io.Writer, a *core.Artifact, id string, pr int) {
	if err := stampLandCostErr(errW, a, id, pr); err != nil {
		fmt.Fprintf(errW, "warning: land-pr %d: cost not stamped: %v\n", pr, err)
	}
}

func stampLandCostErr(errW io.Writer, a *core.Artifact, id string, pr int) error {
	home, err := userHomeFn()
	if err != nil {
		return fmt.Errorf("resolving home for the projects dir: %w", err)
	}
	links, _ := a.FrontMatter["external_links"].([]any)
	needle := fmt.Sprintf("/pull/%d", pr)
	linked := false
	for _, l := range links {
		if s, ok := l.(string); ok && strings.HasSuffix(s, needle) {
			linked = true
		}
	}
	rec, err := issueCost(a, id, pr, filepath.Join(home, ".claude", "projects"))
	if err != nil {
		return err
	}
	for _, n := range rec.notices {
		fmt.Fprintln(errW, n)
	}
	if !linked {
		raw, verr := ghPRViewJSONFn(pr, "url")
		var view struct {
			URL string `json:"url"`
		}
		if verr == nil {
			verr = json.Unmarshal(raw, &view)
		}
		if verr != nil {
			return verr
		}
		if view.URL != "" {
			a.FrontMatter["external_links"] = append(links, view.URL)
		}
	}
	a.FrontMatter["cost_rounds"] = rec.Rounds
	a.FrontMatter["cost_diff"] = rec.Diff
	a.FrontMatter["cost_files"] = rec.Files
	a.FrontMatter["cost_tokens"] = rec.Tokens
	return nil
}
