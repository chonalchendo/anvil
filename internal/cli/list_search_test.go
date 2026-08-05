package cli

import (
	"bytes"
	"strings"
	"testing"
)

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

func TestListSearch_UnsupportedTypeRefusesNonZero(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"list", "convention", "--search", "anything", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected non-nil error so the CLI exits non-zero")
	}
	if !strings.Contains(stdout.String(), "unsupported_for_type") {
		t.Fatalf("expected unsupported_for_type payload; got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "decision") {
		t.Fatalf("expected the supported set to name decision; got: %s", stdout.String())
	}
}
