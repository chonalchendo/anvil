package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/build"
	"github.com/chonalchendo/anvil/internal/core"
)

func copyBuildSmokeFixture(t *testing.T, vault string) {
	t.Helper()
	src := filepath.Join("testdata", "plan_build_smoke.md")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(vault, core.TypePlan.Dir(), "anvil.build-smoke.md")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuild_DryRun_EmitsJSONPerTask(t *testing.T) {
	vault := setupVault(t)
	copyBuildSmokeFixture(t, vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"build", "anvil.build-smoke", "--dry-run", "--json"})
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run build: %v\nstderr: %s", err, errBuf.String())
	}

	got := out.String()
	for _, want := range []string{`"task_id":"T1"`, `"task_id":"T2"`, `"status":"skipped_dry_run"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in stdout:\n%s", want, got)
		}
	}
}

func TestBuild_ClaudeBinaryMissing_ReturnsErrBuildTaskFailed(t *testing.T) {
	vault := setupVault(t)
	copyBuildSmokeFixture(t, vault)

	// Force ANVIL_CLAUDE_BIN to a path that does not exist; PATH-fallback
	// is irrelevant because the adapter consults the env first.
	t.Setenv("ANVIL_CLAUDE_BIN", filepath.Join(t.TempDir(), "no-such-claude"))

	cmd := newRootCmd()
	cmd.SetArgs([]string{"build", "anvil.build-smoke"})
	var errBuf bytes.Buffer
	cmd.SetOut(&errBuf)
	cmd.SetErr(&errBuf)
	err := cmd.Execute()
	if !errors.Is(err, build.ErrBuildTaskFailed) {
		t.Errorf("err = %v, want ErrBuildTaskFailed (claude binary missing → task failure)", err)
	}
}

func TestBuild_ClaudeAdapterReachedViaShim(t *testing.T) {
	vault := setupVault(t)
	copyBuildSmokeFixture(t, vault)

	// Stub claude on PATH via ANVIL_CLAUDE_BIN. Reuses the adapter's
	// happy-path shim so we know it emits valid stream-json.
	shim, err := filepath.Abs(filepath.Join("..", "adapters", "claude", "testdata", "shim_success.sh"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANVIL_CLAUDE_BIN", shim)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"build", "anvil.build-smoke", "--json"})
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("build via shim: %v\nstderr: %s", err, errBuf.String())
	}
	for _, want := range []string{`"task_id":"T1"`, `"task_id":"T2"`, `"outcome":"success"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in stdout:\n%s", want, out.String())
		}
	}
}

func TestBuild_PlanNotFound_ReturnsErrArtifactNotFound(t *testing.T) {
	setupVault(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"build", "anvil.no-such-plan", "--dry-run"})
	var errBuf bytes.Buffer
	cmd.SetOut(&errBuf)
	cmd.SetErr(&errBuf)
	err := cmd.Execute()
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Errorf("err = %v, want ErrArtifactNotFound", err)
	}
}
