package cli

import "testing"

func TestListSearch_DecisionMatchesTitleAndDescription(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	writeArtifact(t, vault, "30-decisions/spdji-fetch-venue.0001.md",
		"type: decision\ntitle: spdji fetch venue stays Cloud Run\nstatus: accepted\ncreated: 2026-08-01\n")
	writeArtifact(t, vault, "30-decisions/other.0001.md",
		"type: decision\ntitle: Harlequin is the exploration surface\ndescription: TUI over notebooks\nstatus: accepted\ncreated: 2026-08-02\n")

	out, _, err := runCmd(t, newRootCmd(), "list", "decision", "--search", "spdji fetch venue", "--json")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	env := unmarshalListEnvelope(t, out)
	if env.Total != 1 || env.Items[0].ID != "spdji-fetch-venue.0001" {
		t.Fatalf("want the spdji decision only, got %+v", env.Items)
	}

	// Terms are ANDed and matched over description too, case-insensitively.
	out, _, err = runCmd(t, newRootCmd(), "list", "decision", "--search", "HARLEQUIN notebooks", "--json")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if env := unmarshalListEnvelope(t, out); env.Total != 1 {
		t.Fatalf("want 1 hit on title+description terms, got %+v", env.Items)
	}

	out, _, err = runCmd(t, newRootCmd(), "list", "decision", "--search", "spdji harlequin", "--json")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if env := unmarshalListEnvelope(t, out); env.Total != 0 {
		t.Fatalf("terms must AND, got %+v", env.Items)
	}
}

// Every walk type searches — there is no allowlist to fall off, so a flag the
// CLI accepts can never be silently discarded.
func TestListSearch_AppliesToAnyWalkType(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	writeArtifact(t, vault, "35-conventions/cli-tooling.md",
		"type: convention\ntitle: CLI tooling\ndescription: flags and exit codes\nstatus: active\ncreated: 2026-08-01\n")
	writeArtifact(t, vault, "35-conventions/prose.md",
		"type: convention\ntitle: Prose\ndescription: house voice\nstatus: active\ncreated: 2026-08-02\n")

	out, _, err := runCmd(t, newRootCmd(), "list", "convention", "--search", "exit codes", "--json")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	env := unmarshalListEnvelope(t, out)
	if env.Total != 1 || env.Items[0].ID != "convention.cli-tooling" {
		t.Fatalf("want the cli-tooling convention only, got %+v", env.Items)
	}
}

// --search must narrow the --ready queue rather than being accepted and
// discarded (the flag reaches the indexed path, not just the walk).
func TestListSearch_NarrowsReadyQueue(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeFixtureIssueDated(t, vault, "demo", "fix-login-flake", "fix login flake", "2026-01-01")
	writeFixtureIssueDated(t, vault, "demo", "add-export", "add csv export", "2026-01-02")
	execCmd(t, "reindex")

	out, _, err := runCmd(t, newRootCmd(), "list", "issue", "--ready", "--search", "login flake", "--json")
	if err != nil {
		t.Fatalf("ready search: %v", err)
	}
	env := unmarshalListEnvelope(t, out)
	if env.Total != 1 || env.Items[0].ID != "issue.demo.fix-login-flake" {
		t.Fatalf("want the login-flake issue only, got %+v", env.Items)
	}

	out, _, err = runCmd(t, newRootCmd(), "list", "issue", "--ready", "--search", "zzzznonexistentterm", "--json")
	if err != nil {
		t.Fatalf("ready search: %v", err)
	}
	if env := unmarshalListEnvelope(t, out); env.Total != 0 {
		t.Fatalf("a no-match search must empty the ready queue, got %+v", env.Items)
	}
}
