package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSlugify_BasicCases(t *testing.T) {
	cases := map[string]string{
		"Hello World":             "hello-world",
		"Fix login bug!":          "fix-login-bug",
		"  trimmed  ":             "trimmed",
		"naïve café":              "naive-cafe",
		"---multiple---dashes---": "multiple-dashes",
		"":                        "",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSlugify_PreservesConnectiveTokens locks in the no-stopword-stripping
// contract. Connective words like "with"/"and"/"of"/"the" are kept verbatim;
// dropping them produces near-identical slugs across linked artifacts which
// breaks the graph (see issue
// anvil.slug-derivation-silently-drops-connective-tokens-causing-dri).
func TestSlugify_PreservesConnectiveTokens(t *testing.T) {
	cases := map[string]string{
		"foo with bar":            "foo-with-bar",
		"validate with pre parse": "validate-with-pre-parse",
		"alpha and omega":         "alpha-and-omega",
		"king of the hill":        "king-of-the-hill",
		"to be or not to be":      "to-be-or-not-to-be",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlugify_TruncatesTo60(t *testing.T) {
	long := ""
	for i := 0; i < 80; i++ {
		long += "a"
	}
	got := Slugify(long)
	if len(got) > 60 {
		t.Errorf("len(Slugify) = %d, want <= 60", len(got))
	}
}

func TestNextID_IssueIncrementsByProject(t *testing.T) {
	v := newScaffolded(t)
	id, err := NextID(v, TypeIssue, IDInputs{Title: "bar", Project: "foo"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "foo.bar" {
		t.Errorf("got %q, want foo.bar", id)
	}
	if err := os.WriteFile(filepath.Join(v.Root, "70-issues", "foo.bar.md"), []byte(""), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	id, err = NextID(v, TypeIssue, IDInputs{Title: "bar", Project: "foo"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "foo.bar-2" {
		t.Errorf("got %q, want foo.bar-2", id)
	}
}

func TestNextID_PlanSameAsIssue(t *testing.T) {
	v := newScaffolded(t)
	id, err := NextID(v, TypePlan, IDInputs{Title: "Q2 cleanup", Project: "foo"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "foo.q2-cleanup" {
		t.Errorf("got %q", id)
	}
}

func TestNextID_Milestone_SlugOnly(t *testing.T) {
	v := newScaffolded(t)
	got, err := NextID(v, TypeMilestone, IDInputs{Title: "CLI substrate", Project: "anvil"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "anvil.cli-substrate" {
		t.Errorf("got %q, want anvil.cli-substrate", got)
	}
}

func TestNextID_Decision_AutoIncrementsTopic(t *testing.T) {
	v := newScaffolded(t)
	id, err := NextID(v, TypeDecision, IDInputs{Title: "use jwt", Topic: "auth"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "auth.0001-use-jwt" {
		t.Errorf("got %q, want auth.0001-use-jwt", id)
	}
	if err := os.WriteFile(filepath.Join(v.Root, "30-decisions", "auth.0001-use-jwt.md"), []byte(""), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	id, _ = NextID(v, TypeDecision, IDInputs{Title: "rotate keys", Topic: "auth"})
	if id != "auth.0002-rotate-keys" {
		t.Errorf("got %q, want auth.0002-rotate-keys", id)
	}
}

func TestNextID_Decision_TopicScoped(t *testing.T) {
	v := newScaffolded(t)
	if err := os.WriteFile(filepath.Join(v.Root, "30-decisions", "auth.0001-x.md"), []byte(""), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	id, err := NextID(v, TypeDecision, IDInputs{Title: "schema", Topic: "data"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "data.0001-schema" {
		t.Errorf("got %q, want data.0001-schema (different topic resets counter)", id)
	}
}

func TestNextID_Inbox_DatePrefix(t *testing.T) {
	v := newScaffolded(t)
	id, err := NextID(v, TypeInbox, IDInputs{Title: "Streaming feels laggy"})
	if err != nil {
		t.Fatal(err)
	}
	// id is `<today>-streaming-feels-laggy`; assert suffix only.
	if got, want := id[len(id)-len("-streaming-feels-laggy"):], "-streaming-feels-laggy"; got != want {
		t.Errorf("got %q, want suffix %q", id, want)
	}
}

func newScaffolded(t *testing.T) *Vault {
	t.Helper()
	v := &Vault{Root: t.TempDir()}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestDeterministicID(t *testing.T) {
	cases := []struct {
		name string
		typ  Type
		in   IDInputs
		want string
	}{
		{"issue", TypeIssue, IDInputs{Title: "Fix Login Bug", Project: "foo"}, "foo.fix-login-bug"},
		{"plan", TypePlan, IDInputs{Title: "Add OAuth", Project: "foo"}, "foo.add-oauth"},
		{"milestone", TypeMilestone, IDInputs{Title: "v0.1 GA", Project: "foo"}, "foo.v0-1-ga"},
		{"thread", TypeThread, IDInputs{Title: "auth retries"}, "auth-retries"},
		{"learning", TypeLearning, IDInputs{Title: "Slogger gotcha"}, "slogger-gotcha"},
		{"sweep", TypeSweep, IDInputs{Title: "Drop python2"}, "drop-python2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DeterministicID(tc.typ, tc.in)
			if err != nil {
				t.Fatalf("DeterministicID: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestDeterministicID_Inbox_DateScoped(t *testing.T) {
	got, err := DeterministicID(TypeInbox, IDInputs{Title: "random thought"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "-random-thought") {
		t.Errorf("got %q, want suffix -random-thought", got)
	}
	if !strings.HasPrefix(got, time.Now().UTC().Format("2006-01-02")) {
		t.Errorf("got %q, want today's UTC date prefix", got)
	}
}

func TestDeterministicID_Decision_Errors(t *testing.T) {
	if _, err := DeterministicID(TypeDecision, IDInputs{Title: "pick db"}); err == nil {
		t.Errorf("expected error for decision (non-deterministic)")
	}
}

func TestDeterministicID_EmptyTitle(t *testing.T) {
	if _, err := DeterministicID(TypeIssue, IDInputs{Project: "foo"}); err == nil {
		t.Errorf("expected error for empty title")
	}
}

func TestNextID_FallsBackToSuffixOnCollision(t *testing.T) {
	v := newScaffolded(t)
	dir := filepath.Join(v.Root, TypeThread.Dir())
	if err := writeStub(filepath.Join(dir, "auth-retries.md")); err != nil {
		t.Fatal(err)
	}
	got, err := NextID(v, TypeThread, IDInputs{Title: "auth retries"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "auth-retries-2" {
		t.Errorf("got %q, want auth-retries-2", got)
	}
}

func writeStub(path string) error {
	return os.WriteFile(path, []byte("---\ntitle: x\n---\n"), 0o644) //nolint:gosec // 0644 is correct for config/data files readable by owner and group
}

func TestAllocateIssueID_OrdinalAndIdempotency(t *testing.T) {
	v := newScaffolded(t)
	// AllocateIssueID removes its probe file, so callers persist the real file;
	// mirror that here so later scans see prior allocations.
	persist := func(path string) {
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
			t.Fatal(err)
		}
	}

	id1, path1, err := AllocateIssueID(v, "foo", "Fix the bug", "")
	if err != nil {
		t.Fatal(err)
	}
	if id1 != "foo.0001.fix-the-bug" {
		t.Errorf("first allocation = %q, want foo.0001.fix-the-bug", id1)
	}
	persist(path1)

	// A distinct slug gets the next ordinal.
	id2, path2, _ := AllocateIssueID(v, "foo", "Another thing", "")
	if id2 != "foo.0002.another-thing" {
		t.Errorf("distinct slug = %q, want foo.0002.another-thing", id2)
	}
	persist(path2)

	// Same slug → idempotent: resolves to the existing id/path, no new ordinal.
	idDup, pathDup, _ := AllocateIssueID(v, "foo", "Fix the bug", "")
	if idDup != id1 || pathDup != path1 {
		t.Errorf("same-slug re-allocation = (%q,%q), want existing (%q,%q)", idDup, pathDup, id1, path1)
	}

	// Ordinals are per-project: a different project starts at 0001.
	idBar, _, _ := AllocateIssueID(v, "bar", "Hello", "")
	if idBar != "bar.0001.hello" {
		t.Errorf("per-project ordinal = %q, want bar.0001.hello", idBar)
	}
}

func TestNextIssueOrdinal_GapAndLegacyMix(t *testing.T) {
	v := newScaffolded(t)
	dir := filepath.Join(v.Root, TypeIssue.Dir())
	for _, name := range []string{"foo.legacy-untouched.md", "foo.0001.a.md", "foo.0005.b.md", "bar.0009.c.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
			t.Fatal(err)
		}
	}
	// Legacy (no ordinal) ignored; other projects ignored; max(0001,0005)+1 = 6.
	got, err := nextIssueOrdinal(v, "foo")
	if err != nil {
		t.Fatal(err)
	}
	if got != 6 {
		t.Errorf("nextIssueOrdinal(foo) = %d, want 6", got)
	}
}

func TestResolveIssueOrdinal_ProjectQualified(t *testing.T) {
	v := newScaffolded(t)
	dir := filepath.Join(v.Root, TypeIssue.Dir())
	if err := os.WriteFile(filepath.Join(dir, "anvil.0019.some-slug.md"), []byte(""), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}

	// ParseProjectQualifiedOrdinal must parse "anvil.0019".
	project, ordinal, ok := ParseProjectQualifiedOrdinal("anvil.0019")
	if !ok {
		t.Fatal("ParseProjectQualifiedOrdinal(\"anvil.0019\") returned ok=false")
	}
	if project != "anvil" || ordinal != "0019" {
		t.Fatalf("got project=%q ordinal=%q, want anvil/0019", project, ordinal)
	}

	// Resolving via extracted project+ordinal must find the file.
	id, found := ResolveIssueOrdinal(v, project, ordinal)
	if !found {
		t.Fatal("ResolveIssueOrdinal returned not-found for anvil.0019")
	}
	if id != "anvil.0019.some-slug" {
		t.Errorf("ResolveIssueOrdinal = %q, want anvil.0019.some-slug", id)
	}

	// Non-matching inputs must return false.
	if _, _, ok := ParseProjectQualifiedOrdinal("0019"); ok {
		t.Error("ParseProjectQualifiedOrdinal(\"0019\") should return ok=false (bare ordinal, no project)")
	}
	if _, _, ok := ParseProjectQualifiedOrdinal("anvil.0019.some-slug"); ok {
		t.Error("ParseProjectQualifiedOrdinal(\"anvil.0019.some-slug\") should return ok=false (full id, not project-qualified ordinal)")
	}
}

func TestSlugifyIssue_CapsAt40OnHyphenBoundary(t *testing.T) {
	if got := slugifyIssue("Short title"); got != "short-title" {
		t.Errorf("short slug = %q, want short-title", got)
	}
	got := slugifyIssue("this is a very long issue title that definitely exceeds forty characters")
	if len(got) > 40 {
		t.Errorf("len = %d, want <= 40: %q", len(got), got)
	}
	if strings.HasSuffix(got, "-") {
		t.Errorf("slug ends mid-break with trailing hyphen: %q", got)
	}
	if !strings.HasPrefix(Slugify("this is a very long issue title that definitely exceeds forty characters"), got) {
		t.Errorf("capped slug %q is not a hyphen-boundary prefix of the full slug", got)
	}
}
