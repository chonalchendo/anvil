package cli

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestTransitionMilestoneDoneStampsDone(t *testing.T) {
	vault := finishLineVault(t, "true")
	stubBranches(t, "base")
	execCmd(t, "transition", "milestone", "demo.line", "done")
	m, err := core.LoadArtifact(filepath.Join(vault, "85-milestones", "demo.line.md"))
	if err != nil {
		t.Fatal(err)
	}
	done, _ := m.FrontMatter["done"].(string)
	block := regexp.MustCompile(`Measured: (\d{4}-\d{2}-\d{2})`).FindStringSubmatch(m.Body)
	if done == "" || block == nil || block[1] != done {
		t.Fatalf("frontmatter done = %q, Status block = %v", done, block)
	}
}
