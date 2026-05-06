package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/chonalchendo/anvil/internal/glossary"
)

// writeArtifact drops a frontmatter-only markdown file into the vault for the test.
func writeArtifact(t *testing.T, root, rel string, fm string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\n" + fm + "---\n\n# body\n"
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTagsList_Aggregates(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANVIL_VAULT", root)

	writeArtifact(t, root, "20-learnings/anvil.a.md",
		"type: learning\ntitle: A\ntags: [domain/dev-tools, activity/research]\n")
	writeArtifact(t, root, "20-learnings/anvil.b.md",
		"type: learning\ntitle: B\ntags: [domain/dev-tools]\n")
	writeArtifact(t, root, "70-issues/anvil.c.md",
		"type: issue\ntitle: C\ntags: [domain/dev-tools, type/issue]\n")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"tags", "list", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var got []struct {
		Tag   string `json:"tag"`
		Count int    `json:"count"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("parse json: %v\nraw: %s", err, out.String())
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Tag < got[j].Tag })

	want := []struct {
		Tag   string `json:"tag"`
		Count int    `json:"count"`
	}{
		{Tag: "activity/research", Count: 1},
		{Tag: "domain/dev-tools", Count: 3},
		{Tag: "type/issue", Count: 1},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("tags mismatch (-want +got):\n%s", diff)
	}
}

func TestTagsList_FilterByType(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANVIL_VAULT", root)

	writeArtifact(t, root, "20-learnings/anvil.a.md",
		"type: learning\ntitle: A\ntags: [foo, bar]\n")
	writeArtifact(t, root, "70-issues/anvil.b.md",
		"type: issue\ntitle: B\ntags: [foo, baz]\n")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"tags", "list", "--type", "learning", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(out.String(), `"bar"`) || strings.Contains(out.String(), `"baz"`) {
		t.Errorf("filter by --type learning leaked issue tags: %s", out.String())
	}
}

func TestTagsList_FilterByPrefix(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANVIL_VAULT", root)

	writeArtifact(t, root, "20-learnings/anvil.a.md",
		"type: learning\ntitle: A\ntags: [domain/x, activity/y, type/learning]\n")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"tags", "list", "--prefix", "domain/", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(out.String(), `"domain/x"`) ||
		strings.Contains(out.String(), `"activity/y"`) {
		t.Errorf("prefix filter wrong: %s", out.String())
	}
}

func TestTagsList_TextOutput(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANVIL_VAULT", root)

	writeArtifact(t, root, "20-learnings/anvil.a.md",
		"type: learning\ntitle: A\ntags: [foo, foo-extra]\n")
	writeArtifact(t, root, "20-learnings/anvil.b.md",
		"type: learning\ntitle: B\ntags: [foo]\n")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"tags", "list"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Format: "<count>\t<tag>\n", sorted by tag.
	want := "1\tfoo-extra\n2\tfoo\n"
	got := out.String()
	// Order may be by descending count then tag — accept either documented order.
	if got != want && got != "2\tfoo\n1\tfoo-extra\n" {
		t.Errorf("text output unexpected:\nwant one of:\n%q\n%q\ngot:\n%q",
			want, "2\tfoo\n1\tfoo-extra\n", got)
	}
}

func TestTagsList_SourceDefined_FromGlossary(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANVIL_VAULT", root)

	g := glossary.New()
	_ = g.AddTag("domain/postgres", "rdbms")
	_ = g.AddTag("activity/research", "investigative work")
	if err := g.Save(glossary.Path(root)); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"tags", "list", "--source", "defined", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %v", len(got), got)
	}
	for _, e := range got {
		if e["defined"] != true {
			t.Errorf("entry %v missing defined:true", e)
		}
		if _, hasCount := e["count"]; hasCount {
			t.Errorf("defined source must not include count: %v", e)
		}
	}
}

func TestTagsList_SourceAll_UnionShape(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANVIL_VAULT", root)

	writeArtifact(t, root, "20-learnings/anvil.a.md",
		"type: learning\ntitle: A\ntags: [domain/used-only]\n")
	g := glossary.New()
	_ = g.AddTag("domain/defined-only", "x")
	if err := g.Save(glossary.Path(root)); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"tags", "list", "--source", "all", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("parse: %v", err)
	}
	byTag := map[string]map[string]any{}
	for _, r := range rows {
		byTag[r["tag"].(string)] = r
	}
	if u := byTag["domain/used-only"]; u == nil || u["defined"] != false || u["count"].(float64) != 1 {
		t.Errorf("used-only row wrong: %v", u)
	}
	if d := byTag["domain/defined-only"]; d == nil || d["defined"] != true {
		t.Errorf("defined-only row wrong: %v", d)
	}
}

func TestTagsList_LimitEmitsTruncationHint(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANVIL_VAULT", root)
	for i := 0; i < 3; i++ {
		writeArtifact(t, root, fmt.Sprintf("20-learnings/anvil.%d.md", i),
			fmt.Sprintf("type: learning\ntitle: A\ntags: [t/%d]\n", i))
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"tags", "list", "--limit", "1"})
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(errBuf.String(), "showing 1 of 3") {
		t.Errorf("missing truncation hint, got stderr: %q", errBuf.String())
	}
}

func TestTagsAdd_Idempotent(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())

	first := newRootCmd()
	first.SetArgs([]string{"tags", "add", "domain/postgres", "--desc", "rdbms"})
	if err := first.Execute(); err != nil {
		t.Fatalf("first add: %v", err)
	}

	second := newRootCmd()
	second.SetArgs([]string{"tags", "add", "domain/postgres", "--desc", "rdbms"})
	var out bytes.Buffer
	second.SetOut(&out)
	if err := second.Execute(); err != nil {
		t.Fatalf("second add (same desc) should be no-op: %v", err)
	}

	g, err := glossary.Load(glossary.Path(vault))
	if err != nil {
		t.Fatal(err)
	}
	if !g.HasTag("domain/postgres") {
		t.Error("tag missing after add")
	}
}

func TestTagsAdd_DriftErrorsWithoutUpdate(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())

	first := newRootCmd()
	first.SetArgs([]string{"tags", "add", "domain/postgres", "--desc", "rdbms"})
	if err := first.Execute(); err != nil {
		t.Fatalf("first add: %v", err)
	}

	second := newRootCmd()
	second.SetArgs([]string{"tags", "add", "domain/postgres", "--desc", "different"})
	err := second.Execute()
	if err == nil {
		t.Fatal("expected drift error")
	}
	if !strings.Contains(err.Error(), "--update") {
		t.Errorf("error must suggest --update: %q", err.Error())
	}
}

func TestTagsAdd_UpdateRewrites(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())

	first := newRootCmd()
	first.SetArgs([]string{"tags", "add", "domain/postgres", "--desc", "rdbms"})
	if err := first.Execute(); err != nil {
		t.Fatalf("first add: %v", err)
	}

	upd := newRootCmd()
	upd.SetArgs([]string{"tags", "add", "domain/postgres", "--desc", "relational engine", "--update"})
	if err := upd.Execute(); err != nil {
		t.Fatalf("update: %v", err)
	}

	body, err := os.ReadFile(glossary.Path(vault))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "relational engine") {
		t.Errorf("description not rewritten:\n%s", body)
	}
	if strings.Contains(string(body), "— rdbms\n") {
		t.Errorf("old description still present:\n%s", body)
	}
}

func TestTagsAdd_RejectsUnknownFacet(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"tags", "add", "bogus/foo", "--desc", "x"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected facet error")
	}
	if !strings.Contains(err.Error(), "valid values:") || !strings.Contains(err.Error(), "domain") {
		t.Errorf("error must list valid facets: %q", err.Error())
	}
}

func TestTagsDefine_KnownAndMissing(t *testing.T) {
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
	cmd.SetArgs([]string{"tags", "define", "thread"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "live workspace") {
		t.Errorf("define thread: got %q", out.String())
	}

	missing := newRootCmd()
	missing.SetArgs([]string{"tags", "define", "no-such"})
	if err := missing.Execute(); err == nil {
		t.Error("expected error for missing term")
	}
}

func TestTagsParent_UnknownSubcommandErrors(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())

	bad := newRootCmd()
	bad.SetArgs([]string{"tags", "show"})
	var buf bytes.Buffer
	bad.SetOut(&buf)
	bad.SetErr(&buf)
	err := bad.Execute()
	if err == nil {
		t.Fatal("expected error for `tags show`")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("error must mention 'unknown command': %q", err.Error())
	}

	ok := newRootCmd()
	ok.SetArgs([]string{"tags", "add", "domain/foo", "--desc", "x"})
	if err := ok.Execute(); err != nil {
		t.Fatalf("valid subcommand regressed: %v", err)
	}
}

// TestTagsList_DataGoesToStdout pins the cobra footgun: cmd.Println /
// cmd.Printf default to OutOrStderr() unless SetOut is called, which silently
// breaks `anvil tags list --json | jq ...` for agent pipelines. Other tests
// in this file call SetOut(&buf) which masks the footgun by aliasing both
// streams to the same buffer; this test redirects os.Stdout/os.Stderr at the
// FD level and asserts data lands on stdout.
func TestTagsList_DataGoesToStdout(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANVIL_VAULT", root)
	writeArtifact(t, root, "20-learnings/anvil.a.md",
		"type: learning\ntitle: A\ntags: [domain/dev-tools]\n")

	// Swap os.Stdout/os.Stderr for pipes so we can tell them apart even when
	// cobra falls back to its default writers (cmd.Println without SetOut goes
	// to the real os.Stderr).
	origOut, origErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	os.Stdout, os.Stderr = outW, errW
	t.Cleanup(func() { os.Stdout, os.Stderr = origOut, origErr })

	done := make(chan struct {
		out, err string
	}, 1)
	go func() {
		var ob, eb bytes.Buffer
		bufCh := make(chan struct{}, 2)
		go func() { _, _ = ob.ReadFrom(outR); bufCh <- struct{}{} }()
		go func() { _, _ = eb.ReadFrom(errR); bufCh <- struct{}{} }()
		<-bufCh
		<-bufCh
		done <- struct {
			out, err string
		}{ob.String(), eb.String()}
	}()

	cmd := newRootCmd()
	cmd.SetArgs([]string{"tags", "list", "--json"})
	// Deliberately NOT calling SetOut/SetErr — the bug only shows up when
	// cobra falls back to its default writers.
	if execErr := cmd.Execute(); execErr != nil {
		t.Fatalf("execute: %v", execErr)
	}
	_ = outW.Close()
	_ = errW.Close()
	r := <-done
	stdout, stderr := r.out, r.err

	if !strings.Contains(stdout, `"domain/dev-tools"`) {
		t.Errorf("--json data missing from stdout\nstdout: %q\nstderr: %q", stdout, stderr)
	}
	if strings.Contains(stderr, `"domain/dev-tools"`) {
		t.Errorf("--json data leaked to stderr (cobra cmd.Println footgun)\nstderr: %q", stderr)
	}
}
