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

func TestBuild_NoAdapter_ReturnsErrBuildTaskFailed(t *testing.T) {
	vault := setupVault(t)
	copyBuildSmokeFixture(t, vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"build", "anvil.build-smoke"})
	var errBuf bytes.Buffer
	cmd.SetOut(&errBuf)
	cmd.SetErr(&errBuf)
	err := cmd.Execute()
	if !errors.Is(err, build.ErrBuildTaskFailed) {
		t.Errorf("err = %v, want ErrBuildTaskFailed (no adapter registered → task failure)", err)
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
