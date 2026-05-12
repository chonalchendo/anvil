package cli

import (
	"runtime/debug"
	"testing"
)

func TestFormatVersion_LDFlagWins(t *testing.T) {
	got := formatVersion("v1.2.3", &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
	})
	if got != "v1.2.3" {
		t.Errorf("ldflag should win, got %q", got)
	}
}

func TestFormatVersion_ModuleVersionFromTarball(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v0.2.0", Sum: "h1:abc="},
	}
	got := formatVersion("", info)
	if got != "v0.2.0" {
		t.Errorf("expected module version, got %q", got)
	}
}

func TestFormatVersion_VCSRevision(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "3299ef7abcdef1234567890"},
			{Key: "vcs.modified", Value: "false"},
		},
	}
	got := formatVersion("", info)
	if got != "dev-3299ef7" {
		t.Errorf("expected dev-<sha7>, got %q", got)
	}
}

func TestFormatVersion_VCSRevisionDirty(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "3299ef7abcdef1234567890"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	got := formatVersion("", info)
	if got != "dev-3299ef7-dirty" {
		t.Errorf("expected dirty marker, got %q", got)
	}
}

func TestFormatVersion_NoInfoFallback(t *testing.T) {
	got := formatVersion("", nil)
	if got != "dev" {
		t.Errorf("expected dev fallback, got %q", got)
	}
}

func TestFormatVersion_NoVCSFallback(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}
	got := formatVersion("", info)
	if got != "dev" {
		t.Errorf("expected dev fallback when no vcs info, got %q", got)
	}
}
