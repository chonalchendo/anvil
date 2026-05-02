package glossary

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `# Vault Glossary

## Tags

### domain/
- ` + "`domain/postgres`" + ` — relational DB engine
- ` + "`domain/typescript`" + ` — TS language

### activity/
- ` + "`activity/debugging`" + ` — investigative work

### pattern/

### type/
- ` + "`type/learning`" + ` — durable claim

## Definitions
- **thread** — live workspace for cross-session inquiry
- **learning** — durable, retrievable claim or know-how
`

func TestLoad_ParsesFacetsAndDefinitions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "glossary.md")
	if err := os.WriteFile(path, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}

	g, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	wantTags := []string{
		"domain/postgres", "domain/typescript",
		"activity/debugging",
		"type/learning",
	}
	got := g.Tags()
	if len(got) != len(wantTags) {
		t.Fatalf("tags = %v, want %v", got, wantTags)
	}
	for i, w := range wantTags {
		if got[i] != w {
			t.Errorf("tag[%d] = %q, want %q", i, got[i], w)
		}
	}

	if !g.HasTag("domain/postgres") {
		t.Error("HasTag(domain/postgres) = false")
	}
	if g.HasTag("domain/nope") {
		t.Error("HasTag(domain/nope) = true, want false")
	}

	def, ok := g.Definition("thread")
	if !ok || def != "live workspace for cross-session inquiry" {
		t.Errorf("Definition(thread) = (%q, %v)", def, ok)
	}
}

func TestLoad_MissingFile_ReturnsEmpty(t *testing.T) {
	g, err := Load(filepath.Join(t.TempDir(), "absent.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Tags()) != 0 {
		t.Errorf("expected empty glossary, got %v", g.Tags())
	}
}

func TestAddTag_AppendsToFacet(t *testing.T) {
	g := New()
	if err := g.AddTag("domain/auth", "authn/authz"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddTag("activity/research", "open-ended inquiry"); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "glossary.md")
	if err := g.Save(path); err != nil {
		t.Fatal(err)
	}

	g2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !g2.HasTag("domain/auth") || !g2.HasTag("activity/research") {
		t.Errorf("round-trip lost a tag: %v", g2.Tags())
	}
}

func TestAddTag_RejectsBadShape(t *testing.T) {
	g := New()
	for _, bad := range []string{"plain", "weird/", "/value", "unknown/x"} {
		if err := g.AddTag(bad, "x"); err == nil {
			t.Errorf("AddTag(%q) succeeded, want error", bad)
		}
	}
}
