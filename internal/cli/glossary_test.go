package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/glossary"
)

func TestGlossary_AddTag_PersistsToFile(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"glossary", "add", "tag", "domain/postgres", "--desc", "relational DB"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("add tag: %v\nout: %s", err, out.String())
	}

	g, err := glossary.Load(glossary.Path(vault))
	if err != nil {
		t.Fatal(err)
	}
	if !g.HasTag("domain/postgres") {
		t.Errorf("tag not persisted; got %v", g.Tags())
	}
}

func TestGlossary_Tags_FiltersByPrefix(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())

	g := glossary.New()
	_ = g.AddTag("domain/postgres", "x")
	_ = g.AddTag("activity/debugging", "y")
	if err := g.Save(glossary.Path(vault)); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"glossary", "tags", "--prefix", "domain/"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "domain/postgres") || strings.Contains(got, "activity/debugging") {
		t.Errorf("tags --prefix domain/: got %q", got)
	}
}

func TestGlossary_Define_KnownAndMissing(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())

	body := "# Vault Glossary\n\n## Tags\n\n## Definitions\n- **thread** — live workspace\n"
	path := glossary.Path(vault)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"glossary", "define", "thread"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "live workspace") {
		t.Errorf("define thread: got %q", out.String())
	}

	cmd = newRootCmd()
	cmd.SetArgs([]string{"glossary", "define", "no-such-term"})
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing term")
	}
}

func TestGlossary_Show_Empty(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"glossary", "show"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Vault Glossary") {
		t.Errorf("show empty: got %q", out.String())
	}
}
