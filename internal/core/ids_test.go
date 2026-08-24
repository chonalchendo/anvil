package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	if id != "issue.foo.bar" {
		t.Errorf("got %q, want issue.foo.bar", id)
	}
	if err := os.WriteFile(filepath.Join(v.Root, "70-issues", "issue.foo.bar.md"), []byte(""), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	id, err = NextID(v, TypeIssue, IDInputs{Title: "bar", Project: "foo"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "issue.foo.bar-2" {
		t.Errorf("got %q, want issue.foo.bar-2", id)
	}
}

func TestNextID_PlanSameAsIssue(t *testing.T) {
	v := newScaffolded(t)
	id, err := NextID(v, TypePlan, IDInputs{Title: "Q2 cleanup", Project: "foo"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "plan.foo.q2-cleanup" {
		t.Errorf("got %q", id)
	}
}

func TestNextID_Milestone_SlugOnly(t *testing.T) {
	v := newScaffolded(t)
	got, err := NextID(v, TypeMilestone, IDInputs{Title: "CLI substrate", Project: "anvil"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "milestone.anvil.cli-substrate" {
		t.Errorf("got %q, want milestone.anvil.cli-substrate", got)
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
		{"issue", TypeIssue, IDInputs{Title: "Fix Login Bug", Project: "foo"}, "issue.foo.fix-login-bug"},
		{"plan", TypePlan, IDInputs{Title: "Add OAuth", Project: "foo"}, "plan.foo.add-oauth"},
		{"milestone", TypeMilestone, IDInputs{Title: "v0.1 GA", Project: "foo"}, "milestone.foo.v0-1-ga"},
		{"learning", TypeLearning, IDInputs{Title: "Slogger gotcha"}, "slogger-gotcha"},
		{"sweep", TypeSweep, IDInputs{Title: "Drop python2"}, "drop-python2"},
		// Design ids are bare project slugs — the index (IndexKey)
		// disambiguates a bare id shared across the two design types.
		{"product-design", TypeProductDesign, IDInputs{Project: "foo"}, "foo"},
		{"system-design singleton", TypeSystemDesign, IDInputs{Project: "foo"}, "foo"},
		{"system-design shard", TypeSystemDesign, IDInputs{Project: "foo", Slug: "build"}, "foo.build"},
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

func TestDeterministicID_TopicOrdinalTypes_Error(t *testing.T) {
	for _, typ := range []Type{TypeDecision, TypeThread} {
		if _, err := DeterministicID(typ, IDInputs{Title: "pick db"}); err == nil {
			t.Errorf("expected error for %s (topic-ordinal, non-deterministic)", typ)
		}
	}
}

func TestDeterministicID_EmptyTitle(t *testing.T) {
	if _, err := DeterministicID(TypeIssue, IDInputs{Project: "foo"}); err == nil {
		t.Errorf("expected error for empty title")
	}
}

func TestNextID_FallsBackToSuffixOnCollision(t *testing.T) {
	v := newScaffolded(t)
	dir := filepath.Join(v.Root, TypeLearning.Dir())
	if err := writeStub(filepath.Join(dir, "auth-retries.md")); err != nil {
		t.Fatal(err)
	}
	got, err := NextID(v, TypeLearning, IDInputs{Title: "auth retries"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "auth-retries-2" {
		t.Errorf("got %q, want auth-retries-2", got)
	}
}

// Threads mint the decision-style topic-ordinal id: the ordinal is scoped to the
// topic, so a second topic restarts at 0001 and a same-topic title collision
// advances the ordinal rather than taking a -2 suffix.
func TestNextID_Thread_TopicScopedOrdinal(t *testing.T) {
	v := newScaffolded(t)
	dir := filepath.Join(v.Root, TypeThread.Dir())

	mint := func(topic, title string) string {
		t.Helper()
		id, err := NextID(v, TypeThread, IDInputs{Title: title, Topic: topic})
		if err != nil {
			t.Fatal(err)
		}
		if err := writeStub(filepath.Join(dir, id+".md")); err != nil {
			t.Fatal(err)
		}
		return id
	}

	if got, want := mint("ducklake", "Which catalog backend"), "ducklake.0001-which-catalog-backend"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if got, want := mint("ducklake", "Which catalog backend"), "ducklake.0002-which-catalog-backend"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if got, want := mint("neon", "Quota headroom"), "neon.0001-quota-headroom"; got != want {
		t.Errorf("got %q want %q", got, want)
	}

	// A bare-slug back-catalogue file in the folder is not a topic-ordinal
	// filename and must not perturb allocation.
	if err := writeStub(filepath.Join(dir, "how-should-we-shard.md")); err != nil {
		t.Fatal(err)
	}
	if got, want := mint("ducklake", "Third question"), "ducklake.0003-third-question"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestNextID_Thread_RequiresTopic(t *testing.T) {
	v := newScaffolded(t)
	if _, err := NextID(v, TypeThread, IDInputs{Title: "auth retries"}); err == nil {
		t.Error("expected error when --topic is absent for thread")
	}
}

// A dotted topic ("v0.2") would split as topic "v0" with an unparseable
// ordinal, so nextTopicOrdinal would go blind to its own files: every create
// mints 0001 and a repeated title overwrites the prior artifact. Reject it at
// allocation instead.
func TestNextID_TopicOrdinalTypes_RejectNonSlugTopic(t *testing.T) {
	v := newScaffolded(t)
	for _, typ := range []Type{TypeDecision, TypeThread} {
		for _, topic := range []string{"v0.2", "Ducklake", "has space"} {
			if _, err := NextID(v, typ, IDInputs{Title: "which backend", Topic: topic}); err == nil {
				t.Errorf("%s: expected error for topic %q", typ, topic)
			}
		}
	}
}

func TestSplitTopicOrdinal(t *testing.T) {
	cases := []struct {
		id      string
		topic   string
		ordinal int
		slug    string
		ok      bool
	}{
		{"ducklake.0001-which-backend", "ducklake", 1, "which-backend", true},
		{"anvil.0042-go-rewrite", "anvil", 42, "go-rewrite", true},
		// Bare back-catalogue slugs and prefixed spine ids are not topic-ordinal.
		{"how-should-we-shard", "", 0, "", false},
		{"anvil.0042.fix-thing", "", 0, "", false},
		{"topic.notanumber-slug", "", 0, "", false},
	}
	for _, tc := range cases {
		topic, ordinal, slug, ok := SplitTopicOrdinal(tc.id)
		if ok != tc.ok || topic != tc.topic || ordinal != tc.ordinal || slug != tc.slug {
			t.Errorf("SplitTopicOrdinal(%q) = (%q, %d, %q, %v), want (%q, %d, %q, %v)",
				tc.id, topic, ordinal, slug, ok, tc.topic, tc.ordinal, tc.slug, tc.ok)
		}
	}
}

func writeStub(path string) error {
	return os.WriteFile(path, []byte("---\ntitle: x\n---\n"), 0o644) //nolint:gosec // 0644 is correct for config/data files readable by owner and group
}

func TestAllocateIssueID_OrdinalAndIdempotency(t *testing.T) {
	v := newScaffolded(t)
	// AllocateIssueID only reserves the ordinal; callers persist the real file.
	// Mirror that here so later scans see prior allocations.
	persist := func(path string, release func()) {
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
			t.Fatal(err)
		}
		release()
	}

	id1, path1, rel1, err := AllocateIssueID(v, "foo", "Fix the bug", "")
	if err != nil {
		t.Fatal(err)
	}
	if id1 != "issue.foo.0001.fix-the-bug" {
		t.Errorf("first allocation = %q, want issue.foo.0001.fix-the-bug", id1)
	}
	persist(path1, rel1)

	// A distinct slug gets the next ordinal.
	id2, path2, rel2, _ := AllocateIssueID(v, "foo", "Another thing", "")
	if id2 != "issue.foo.0002.another-thing" {
		t.Errorf("distinct slug = %q, want issue.foo.0002.another-thing", id2)
	}
	persist(path2, rel2)

	// Same slug → idempotent: resolves to the existing id/path, no new ordinal.
	idDup, pathDup, _, _ := AllocateIssueID(v, "foo", "Fix the bug", "")
	if idDup != id1 || pathDup != path1 {
		t.Errorf("same-slug re-allocation = (%q,%q), want existing (%q,%q)", idDup, pathDup, id1, path1)
	}

	// Ordinals are per-project: a different project starts at 0001.
	idBar, _, _, _ := AllocateIssueID(v, "bar", "Hello", "")
	if idBar != "issue.bar.0001.hello" {
		t.Errorf("per-project ordinal = %q, want issue.bar.0001.hello", idBar)
	}
}

// TestAllocateIssueID_ConcurrentCreatesGetDistinctOrdinals is the regression for
// the observed collision: two sessions creating different issues at once both
// minted issue.mentat.0291. The second allocation happens while the first has
// only reserved its ordinal — no file written yet — which is the window the
// old slug-keyed probe left open.
func TestAllocateIssueID_ConcurrentCreatesGetDistinctOrdinals(t *testing.T) {
	v := newScaffolded(t)

	id1, _, rel1, err := AllocateIssueID(v, "foo", "First in flight", "")
	if err != nil {
		t.Fatal(err)
	}
	defer rel1()
	id2, _, rel2, err := AllocateIssueID(v, "foo", "Second in flight", "")
	if err != nil {
		t.Fatal(err)
	}
	defer rel2()

	if id1 != "issue.foo.0001.first-in-flight" || id2 != "issue.foo.0002.second-in-flight" {
		t.Errorf("in-flight allocations = (%q, %q), want distinct ordinals 0001/0002", id1, id2)
	}
}

// TestAllocateIssueID_GoroutineFanOut exercises the EEXIST retry branch, which
// sequential calls never reach — they win the marker on attempt 1. Only genuine
// scan/reserve interleaving makes two allocations pick the same ordinal, so this
// is the test that covers the retry. Run under -race.
func TestAllocateIssueID_GoroutineFanOut(t *testing.T) {
	v := newScaffolded(t)

	const n = 8
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		ids      []string
		releases []func()
		errs     []error
	)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, _, release, err := AllocateIssueID(v, "foo", fmt.Sprintf("Concurrent create %d", i), "")
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			ids = append(ids, id)
			releases = append(releases, release)
		}()
	}
	wg.Wait()
	for _, release := range releases {
		defer release()
	}
	for _, err := range errs {
		t.Errorf("allocation failed: %v", err)
	}

	ordinals := map[string]string{}
	for _, id := range ids {
		parts := strings.Split(id, ".")
		ord := parts[2]
		if prev, dup := ordinals[ord]; dup {
			t.Errorf("ordinal %s minted twice: %q and %q", ord, prev, id)
		}
		ordinals[ord] = id
	}
	if len(ordinals) != n {
		t.Errorf("distinct ordinals = %d, want %d (got %v)", len(ordinals), n, ids)
	}
}

// A create that never writes its file frees the ordinal on release, so an
// abandoned create doesn't burn ordinals.
func TestAllocateIssueID_ReleaseFreesOrdinal(t *testing.T) {
	v := newScaffolded(t)

	_, _, release, err := AllocateIssueID(v, "foo", "Abandoned create", "")
	if err != nil {
		t.Fatal(err)
	}
	release()

	id, _, rel, err := AllocateIssueID(v, "foo", "Next create", "")
	if err != nil {
		t.Fatal(err)
	}
	defer rel()
	if id != "issue.foo.0001.next-create" {
		t.Errorf("after release = %q, want issue.foo.0001.next-create", id)
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
	id, found, err := ResolveIssueOrdinal(v, project, ordinal)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("ResolveIssueOrdinal returned not-found for anvil.0019")
	}
	if id != "issue.anvil.0019.some-slug" {
		t.Errorf("ResolveIssueOrdinal = %q, want issue.anvil.0019.some-slug", id)
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

// TestIndexKey pins the one asymmetry in the index key space: design types
// mint a bare CanonicalID but key the index on the type-qualified form; every
// other type's index key is its CanonicalID unchanged.
func TestIndexKey(t *testing.T) {
	cases := []struct {
		t    Type
		id   string
		want string
	}{
		{TypeProductDesign, "burgh", "product-design.burgh"},
		{TypeProductDesign, "product-design.burgh", "product-design.burgh"},
		{TypeSystemDesign, "burgh.api", "system-design.burgh.api"},
		{TypeIssue, "demo.0001.x", "issue.demo.0001.x"},
		{TypeIssue, "issue.demo.0001.x", "issue.demo.0001.x"},
		{TypeLearning, "sqlmesh-audits", "sqlmesh-audits"},
		{TypeConvention, "python", "convention.python"},
	}
	for _, tc := range cases {
		if got := IndexKey(tc.t, tc.id); got != tc.want {
			t.Errorf("IndexKey(%s, %q) = %q, want %q", tc.t, tc.id, got, tc.want)
		}
	}
}

// TestResolveIssueArg_AmbiguousOrdinalRefuses pins the synced-clone collision:
// two files on one ordinal must refuse the shorthand and name both, while the
// full id of either still resolves.
func TestResolveIssueArg_AmbiguousOrdinalRefuses(t *testing.T) {
	root := t.TempDir()
	v := &Vault{Root: root}
	dir := filepath.Join(root, TypeIssue.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // test dir
		t.Fatal(err)
	}
	for _, name := range []string{"issue.demo.0001.alpha.md", "issue.demo.0001.beta.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("---\ntype: issue\n---\n"), 0o644); err != nil { //nolint:gosec // test file
			t.Fatal(err)
		}
	}

	_, err := ResolveIssueArg(v, "demo.0001")
	var amb *AmbiguousOrdinalError
	if !errors.As(err, &amb) {
		t.Fatalf("err = %v, want *AmbiguousOrdinalError", err)
	}
	if amb.Ordinal != "demo.0001" {
		t.Errorf("Ordinal = %q, want demo.0001", amb.Ordinal)
	}
	want := []string{"issue.demo.0001.alpha", "issue.demo.0001.beta"}
	if strings.Join(amb.Candidates, ",") != strings.Join(want, ",") {
		t.Errorf("Candidates = %v, want %v", amb.Candidates, want)
	}

	id, err := ResolveIssueArg(v, "issue.demo.0001.beta")
	if err != nil || id != "issue.demo.0001.beta" {
		t.Errorf("full id resolved to (%q, %v), want (issue.demo.0001.beta, nil)", id, err)
	}
}

// TestReserveIssueOrdinal_WantTakenRefuses: --to on an occupied ordinal must
// refuse rather than mint a third duplicate.
func TestReserveIssueOrdinal_WantTakenRefuses(t *testing.T) {
	root := t.TempDir()
	v := &Vault{Root: root}
	dir := filepath.Join(root, TypeIssue.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // test dir
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "issue.demo.0002.taken.md"), []byte("---\ntype: issue\n---\n"), 0o644); err != nil { //nolint:gosec // test file
		t.Fatal(err)
	}
	if _, _, _, err := ReserveIssueOrdinal(v, "demo", "beta", 2); err == nil || !strings.Contains(err.Error(), "issue.demo.0002.taken") {
		t.Fatalf("err = %v, want taken-by issue.demo.0002.taken", err)
	}
	id, _, release, err := ReserveIssueOrdinal(v, "demo", "beta", 3)
	defer release()
	if err != nil || id != "issue.demo.0003.beta" {
		t.Fatalf("want=3 → (%q, %v), want issue.demo.0003.beta", id, err)
	}
}
