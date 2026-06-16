package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// setFixtureTags overwrites an issue fixture's tags, then saves. Callers must do
// this before the first `index` call so the bootstrap reindex picks them up.
func setFixtureTags(t *testing.T, vault, project, slug string, tags []string) {
	t.Helper()
	a, err := core.LoadArtifact(filepath.Join(vault, "70-issues", project+"."+slug+".md"))
	if err != nil {
		t.Fatal(err)
	}
	anyTags := make([]any, len(tags))
	for i, tg := range tags {
		anyTags[i] = tg
	}
	a.FrontMatter["tags"] = anyTags
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

func decodeRelated(t *testing.T, out string) []relatedOut {
	t.Helper()
	var rows []relatedOut
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	return rows
}

func TestIndex_TagsSeedRanksByMatchCount(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	writeFixtureIssue(t, vault, "foo", "b", "B")
	writeFixtureIssue(t, vault, "foo", "c", "C")
	setFixtureTags(t, vault, "foo", "a", []string{"domain/cli", "activity/issue"})
	setFixtureTags(t, vault, "foo", "b", []string{"domain/cli"})
	setFixtureTags(t, vault, "foo", "c", []string{"domain/vault"})

	out, _, err := runCmd(t, newRootCmd(), "index", "--tags", "domain/cli,activity/issue", "--json")
	if err != nil {
		t.Fatal(err)
	}
	rows := decodeRelated(t, out)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows (a, b), got %d: %v", len(rows), rows)
	}
	if rows[0].ID != "foo.a" || rows[0].Score != 2 {
		t.Errorf("rows[0] = %s score %d, want foo.a score 2", rows[0].ID, rows[0].Score)
	}
	if rows[1].ID != "foo.b" || rows[1].Score != 1 {
		t.Errorf("rows[1] = %s score %d, want foo.b score 1", rows[1].ID, rows[1].Score)
	}
}

func TestIndex_IDSeedExcludesSeed(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "seed", "Seed")
	writeFixtureIssue(t, vault, "foo", "a", "A")
	setFixtureTags(t, vault, "foo", "seed", []string{"domain/cli"})
	setFixtureTags(t, vault, "foo", "a", []string{"domain/cli"})

	out, _, err := runCmd(t, newRootCmd(), "index", "foo.seed", "--json")
	if err != nil {
		t.Fatal(err)
	}
	rows := decodeRelated(t, out)
	if len(rows) != 1 || rows[0].ID != "foo.a" {
		t.Fatalf("want only foo.a, got %v", rows)
	}
	for _, r := range rows {
		if r.ID == "foo.seed" {
			t.Fatalf("seed must be excluded from its own results")
		}
	}
}

func TestIndex_RequiresExactlyOneSeed(t *testing.T) {
	setupVault(t)
	if _, _, err := runCmd(t, newRootCmd(), "index"); err == nil {
		t.Error("no seed should error")
	}
	if _, _, err := runCmd(t, newRootCmd(), "index", "foo.x", "--tags", "domain/cli"); err == nil {
		t.Error("both seeds should error")
	}
}

func TestIndex_UnknownIDErrors(t *testing.T) {
	setupVault(t)
	_, _, err := runCmd(t, newRootCmd(), "index", "foo.nope", "--json")
	if err == nil {
		t.Fatal("unknown id should error")
	}
}
