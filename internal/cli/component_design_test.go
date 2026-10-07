package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

// runArgs executes a fresh root command with args, capturing stdout and
// stderr merged into one buffer. Callers that parse the result as JSON must
// use runArgsJSON instead: a stderr diagnostic on an otherwise-successful
// command corrupts this merged buffer's JSON.
func runArgs(t *testing.T, args ...string) (string, error) {
	t.Helper()
	stdout, stderr, err := runCmd(t, newRootCmd(), args...)
	return stdout + stderr, err
}

// runArgsJSON is like runArgs but returns stdout only, so a stderr
// diagnostic can never land inside the JSON a caller unmarshals.
func runArgsJSON(t *testing.T, args ...string) (string, error) {
	t.Helper()
	stdout, _, err := runCmd(t, newRootCmd(), args...)
	return stdout, err
}

func TestComponentDesignKinds_AddListRoundTrip(t *testing.T) {
	setupVault(t)

	if out, err := runArgs(t, "component-design", "kinds", "add", "data", "--desc", "data pipeline boundaries"); err != nil {
		t.Fatalf("kinds add: %v\n%s", err, out)
	}
	if out, err := runArgs(t, "component-design", "kinds", "add", "analytics"); err != nil {
		t.Fatalf("kinds add (no desc): %v\n%s", err, out)
	}

	out, err := runArgsJSON(t, "component-design", "kinds", "list", "--json")
	if err != nil {
		t.Fatalf("kinds list: %v\n%s", err, out)
	}
	var kinds []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &kinds); err != nil {
		t.Fatalf("parse kinds json: %v\n%s", err, out)
	}
	if len(kinds) != 2 || kinds[0] != "analytics" || kinds[1] != "data" {
		t.Fatalf("kinds = %v, want [analytics data] (sorted)", kinds)
	}
}

func TestComponentDesignKinds_AddIdempotent(t *testing.T) {
	setupVault(t)

	if _, err := runArgs(t, "component-design", "kinds", "add", "data", "--desc", "x"); err != nil {
		t.Fatal(err)
	}
	// Same desc → idempotent success.
	if _, err := runArgs(t, "component-design", "kinds", "add", "data", "--desc", "x"); err != nil {
		t.Fatalf("re-add same desc should be idempotent: %v", err)
	}
	// Different desc without --update → conflict error.
	if _, err := runArgs(t, "component-design", "kinds", "add", "data", "--desc", "y"); err == nil {
		t.Fatal("expected conflict on differing desc without --update")
	}
}

func TestCreateComponentDesign_RoundTripAndKind(t *testing.T) {
	setupVault(t)

	if _, err := runArgs(t, "component-design", "kinds", "add", "data", "--desc", "data boundaries"); err != nil {
		t.Fatal(err)
	}

	out, err := runArgsJSON(t, "create", "component-design", "--project", "burgh",
		"--title", "Data boundaries", "--kind", "data",
		"--description", "what the pipeline does / does not", "--json")
	if err != nil {
		t.Fatalf("create component design: %v\n%s", err, out)
	}
	var res map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &res); err != nil {
		t.Fatalf("parse create json: %v\n%s", err, out)
	}
	path := res["path"]
	if path == "" {
		t.Fatalf("create result missing path: %s", out)
	}

	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate("component-design", a.FrontMatter); err != nil {
		t.Fatalf("created component design fails schema: %v", err)
	}
	if a.FrontMatter["kind"] != "data" {
		t.Errorf("kind = %v, want data", a.FrontMatter["kind"])
	}
}

func TestCreateComponentDesign_UnregisteredKindRejected(t *testing.T) {
	setupVault(t)

	out, err := runArgs(t, "create", "component-design", "--project", "burgh",
		"--title", "Bad", "--kind", "boguskind", "--description", "y")
	if err == nil {
		t.Fatalf("expected rejection for unregistered kind\n%s", out)
	}
	if !strings.Contains(out, "anvil component-design kinds add") {
		t.Errorf("error should point at the registration verb, got:\n%s", out)
	}
}

func TestCreateComponentDesign_RequiresKind(t *testing.T) {
	setupVault(t)

	_, err := runArgs(t, "create", "component-design", "--project", "burgh",
		"--title", "No kind", "--description", "y")
	if err == nil {
		t.Fatal("expected error: --kind required for component design")
	}
}

// TestTagsAdd_RejectsKindFacet pins the single-registration-path invariant:
// `kind/` is glossary storage for component design kinds, but the only public way to
// register one is `anvil component-design kinds add`, not the generic `tags add`.
func TestTagsAdd_RejectsKindFacet(t *testing.T) {
	setupVault(t)

	out, err := runArgs(t, "tags", "add", "kind/data", "--desc", "x")
	if err == nil {
		t.Fatalf("expected rejection of tags add kind/...\n%s", out)
	}
	if !strings.Contains(err.Error(), "component-design kinds add") {
		t.Errorf("error should redirect to the dedicated verb, got: %v", err)
	}
}

func TestCreateComponentDesign_BodyRequiresCore(t *testing.T) {
	setupVault(t)
	if _, err := runArgs(t, "component-design", "kinds", "add", "data"); err != nil {
		t.Fatal(err)
	}
	create := func(body string) error {
		_, err := runArgs(t, "create", "component-design", "--project", "burgh",
			"--title", "Probe", "--kind", "data", "--description", "d", "--body", body)
		return err
	}
	if err := create("## Does\n\n- a\n\n## Interfaces\n\n- x\n"); err == nil {
		t.Error("body missing required sections must be rejected")
	}
	full := "## Does\n\n- a\n\n## Does not\n\n- b\n\n## Interfaces\n\ni\n\n## Invariants\n\n- v\n\n## Verification\n\n### Direct\n\nx\n\n### Indirect\n\ny\n"
	if err := create(full); err != nil {
		t.Errorf("core-only body must be accepted: %v", err)
	}
}

func TestCreateComponentDesign_NoBodyGetsCoreSkeleton(t *testing.T) {
	setupVault(t)
	if _, err := runArgs(t, "component-design", "kinds", "add", "data"); err != nil {
		t.Fatal(err)
	}
	out, err := runArgsJSON(t, "create", "component-design", "--project", "burgh",
		"--title", "Skeleton", "--kind", "data", "--description", "d", "--json")
	if err != nil {
		t.Fatalf("bodiless create: %v\n%s", err, out)
	}
	var res map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &res); err != nil {
		t.Fatalf("parse create json: %v\n%s", err, out)
	}
	a, err := core.LoadArtifact(res["path"])
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range core.RequiredComponentDesignSections {
		if !strings.Contains(a.Body, h) {
			t.Errorf("scaffold body missing %q:\n%s", h, a.Body)
		}
	}
	tmpl, err := runArgs(t, "create", "component-design", "--show-template")
	if err != nil {
		t.Fatalf("--show-template: %v", err)
	}
	if !strings.Contains(tmpl, "## Interfaces") {
		t.Errorf("--show-template missing core skeleton:\n%s", tmpl)
	}
}

// TestComponentDesign_ValidateEnforcesCore pins that a design which
// lost a core heading after create is refused by validate, and that a
// plain append onto a valid design still lands.
func TestComponentDesign_ValidateEnforcesCore(t *testing.T) {
	setupVault(t)
	if _, err := runArgs(t, "component-design", "kinds", "add", "data"); err != nil {
		t.Fatal(err)
	}
	out, err := runArgsJSON(t, "create", "component-design", "--project", "burgh",
		"--title", "Probe", "--kind", "data", "--description", "d", "--json")
	if err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	var res map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &res); err != nil {
		t.Fatal(err)
	}
	if out, err := runArgs(t, "validate", res["path"]); err != nil {
		t.Fatalf("scaffold must validate: %v\n%s", err, out)
	}

	out, err = runArgs(t, "append", "component-design", strings.TrimSuffix(filepath.Base(res["path"]), ".md"), "--body", "- PR #1: precedent", "--json")
	if err != nil || !strings.Contains(out, `"status":"appended"`) {
		t.Fatalf("append onto a valid design must succeed: err=%v\n%s", err, out)
	}

	a, err := core.LoadArtifact(res["path"])
	if err != nil {
		t.Fatal(err)
	}
	a.Body = strings.Replace(a.Body, "## Does not", "## Dropped", 1)
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	out, err = runArgs(t, "validate", res["path"])
	if err == nil || !strings.Contains(out+err.Error(), "Does not") {
		t.Errorf("validate must name the missing heading: err=%v\n%s", err, out)
	}
}
