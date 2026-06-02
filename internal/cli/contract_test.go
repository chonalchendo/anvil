package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

// runArgs executes a fresh root command with args, capturing stdout.
func runArgs(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestContractKinds_AddListRoundTrip(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())

	if out, err := runArgs(t, "contract", "kinds", "add", "data", "--desc", "data pipeline boundaries"); err != nil {
		t.Fatalf("kinds add: %v\n%s", err, out)
	}
	if out, err := runArgs(t, "contract", "kinds", "add", "analytics"); err != nil {
		t.Fatalf("kinds add (no desc): %v\n%s", err, out)
	}

	out, err := runArgs(t, "contract", "kinds", "list", "--json")
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

func TestContractKinds_AddIdempotent(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())

	if _, err := runArgs(t, "contract", "kinds", "add", "data", "--desc", "x"); err != nil {
		t.Fatal(err)
	}
	// Same desc → idempotent success.
	if _, err := runArgs(t, "contract", "kinds", "add", "data", "--desc", "x"); err != nil {
		t.Fatalf("re-add same desc should be idempotent: %v", err)
	}
	// Different desc without --update → conflict error.
	if _, err := runArgs(t, "contract", "kinds", "add", "data", "--desc", "y"); err == nil {
		t.Fatal("expected conflict on differing desc without --update")
	}
}

func TestCreateContract_RoundTripAndKind(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())

	if _, err := runArgs(t, "contract", "kinds", "add", "data", "--desc", "data boundaries"); err != nil {
		t.Fatal(err)
	}

	out, err := runArgs(t, "create", "contract", "--project", "burgh",
		"--title", "Data boundaries", "--kind", "data",
		"--description", "what the pipeline does / does not", "--json")
	if err != nil {
		t.Fatalf("create contract: %v\n%s", err, out)
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
	if err := schema.Validate("contract", a.FrontMatter); err != nil {
		t.Fatalf("created contract fails schema: %v", err)
	}
	if a.FrontMatter["kind"] != "data" {
		t.Errorf("kind = %v, want data", a.FrontMatter["kind"])
	}
}

func TestCreateContract_UnregisteredKindRejected(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())

	out, err := runArgs(t, "create", "contract", "--project", "burgh",
		"--title", "Bad", "--kind", "boguskind", "--description", "y")
	if err == nil {
		t.Fatalf("expected rejection for unregistered kind\n%s", out)
	}
	if !strings.Contains(out, "anvil contract kinds add") {
		t.Errorf("error should point at the registration verb, got:\n%s", out)
	}
}

func TestCreateContract_RequiresKind(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())

	_, err := runArgs(t, "create", "contract", "--project", "burgh",
		"--title", "No kind", "--description", "y")
	if err == nil {
		t.Fatal("expected error: --kind required for contract")
	}
}
