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
	if err := os.WriteFile(filepath.Join(v.Root, "70-issues", "foo.bar.md"), []byte(""), 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(v.Root, "30-decisions", "auth.0001-use-jwt.md"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	id, _ = NextID(v, TypeDecision, IDInputs{Title: "rotate keys", Topic: "auth"})
	if id != "auth.0002-rotate-keys" {
		t.Errorf("got %q, want auth.0002-rotate-keys", id)
	}
}

func TestNextID_Decision_TopicScoped(t *testing.T) {
	v := newScaffolded(t)
	if err := os.WriteFile(filepath.Join(v.Root, "30-decisions", "auth.0001-x.md"), []byte(""), 0o644); err != nil {
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
	return os.WriteFile(path, []byte("---\ntitle: x\n---\n"), 0o644)
}
