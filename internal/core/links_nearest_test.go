package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNearestArtifactTargets(t *testing.T) {
	v := newScaffolded(t)
	dir := filepath.Join(v.Root, TypeDecision.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	for _, id := range []string{"serving.0001-local-api", "serving.0002-other", "serving.00010-x"} {
		if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte("---\ntype: decision\n---\n"), 0o644); err != nil { //nolint:gosec // 0644 is correct for data files
			t.Fatal(err)
		}
	}
	cases := []struct {
		target string
		want   []string
	}{
		{"decision.serving.0001", []string{"decision.serving.0001-local-api"}},
		{"decision.serving", []string{"decision.serving.0001-local-api", "decision.serving.00010-x", "decision.serving.0002-other"}},
		{"decision.serving.000", nil},
		{"decision.", nil},
		{"decision.serving.0001-local-api", nil},
		{"decision.ghost", nil},
		{"nope.serving", nil},
	}
	for _, tc := range cases {
		if got, more := NearestArtifactTargets(v, tc.target); !reflect.DeepEqual(got, tc.want) || more != 0 {
			t.Errorf("%s: got %v (+%d), want %v", tc.target, got, more, tc.want)
		}
	}
}

func TestNearestArtifactTargets_CapReportsOverflow(t *testing.T) {
	v := newScaffolded(t)
	dir := filepath.Join(v.Root, TypeDecision.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	for _, id := range []string{"s.0001-a", "s.0002-b", "s.0003-c", "s.0004-d", "s.0005-e"} {
		if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte("---\ntype: decision\n---\n"), 0o644); err != nil { //nolint:gosec // 0644 is correct for data files
			t.Fatal(err)
		}
	}
	got, more := NearestArtifactTargets(v, "decision.s")
	if len(got) != maxNearestIDs || more != 2 {
		t.Errorf("got %v (+%d), want %d items (+2)", got, more, maxNearestIDs)
	}
}
