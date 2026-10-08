package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"

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
	linked := false
	for _, l := range links {
		if u, ok := l.(string); ok {
			if m := prURLNumber.FindStringSubmatch(u); m != nil && m[1] == strconv.Itoa(pr) {
				linked = true
			}
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
		if url, uerr := prURL(pr); uerr != nil {
			fmt.Fprintf(errW, "warning: land-pr %d: pr url not linked: %v\n", pr, uerr)
		} else if url != "" {
			a.FrontMatter["external_links"] = append(links, url)
		}
	}
	a.FrontMatter["cost_rounds"] = rec.Rounds
	a.FrontMatter["cost_diff"] = rec.Diff
	a.FrontMatter["cost_files"] = rec.Files
	a.FrontMatter["cost_tokens"] = rec.Tokens
	return nil
}

func prURL(pr int) (string, error) {
	raw, err := ghPRViewJSONFn(pr, "url")
	if err != nil {
		return "", err
	}
	var view struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(raw, &view); err != nil {
		return "", err
	}
	return view.URL, nil
}
