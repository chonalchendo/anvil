package cli

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func writeMeasuredMilestone(t *testing.T, vault, id, kind, statusBody string) {
	t.Helper()
	a := &core.Artifact{
		Path: filepath.Join(vault, "85-milestones", id+".md"),
		FrontMatter: map[string]any{
			"type": "milestone", "title": id, "description": "fixture description",
			"created": "2026-01-01", "updated": "2026-01-01",
			"status": "in-progress", "project": "demo",
			"goal": "fixture milestone is done", "kind": kind,
		},
		Body: "## Objective\no\n\n## Status\n" + statusBody + "\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestList_MeasurementStale(t *testing.T) {
	vault := setupVault(t)
	writeMeasuredMilestone(t, vault, "demo.old", "scoped", "Measured: 2020-01-01")
	writeMeasuredMilestone(t, vault, "demo.bucket", "bucket", "Measured: 2020-01-01")

	out, errOut, err := runCmd(t, newRootCmd(), "list", "milestone", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	for _, it := range env.Items {
		v, has := it["measurement_stale"]
		switch it["id"] {
		case "milestone.demo.old":
			if v != true {
				t.Errorf("scoped stale: measurement_stale = %v, want true", v)
			}
		case "milestone.demo.bucket":
			if has {
				t.Errorf("bucket: measurement_stale = %v, want key absent", v)
			}
		}
	}
	if !strings.Contains(errOut, "milestone.demo.old") || strings.Contains(errOut, "milestone.demo.bucket") {
		t.Errorf("stderr warning mismatch: %q", errOut)
	}

	out, _, err = runCmd(t, newRootCmd(), "list", "milestone", "--json", "--fields", "id,measurement_stale")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"measurement_stale":true`) && !strings.Contains(out, `"measurement_stale": true`) {
		t.Errorf("projection lost measurement_stale: %s", out)
	}
}

func TestList_MeasurementStale_WarnsOnlyForReturnedItems(t *testing.T) {
	vault := setupVault(t)
	writeMeasuredMilestone(t, vault, "demo.a", "scoped", "Measured: 2020-01-01")
	writeMeasuredMilestone(t, vault, "demo.b", "scoped", "Measured: 2020-01-01")

	_, errOut, err := runCmd(t, newRootCmd(), "list", "milestone", "--limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(errOut, "Status block was measured"); n != 1 {
		t.Errorf("warnings = %d, want 1 (truncated item must not warn): %q", n, errOut)
	}
}

func TestShow_MeasurementStale(t *testing.T) {
	vault := setupVault(t)
	writeMeasuredMilestone(t, vault, "demo.old", "scoped", "Measured: 2020-01-01")
	writeMeasuredMilestone(t, vault, "demo.bucket", "bucket", "Measured: 2020-01-01")

	out, _, err := runCmd(t, newRootCmd(), "show", "milestone", "demo.old", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got["measurement_stale"] != true {
		t.Errorf("measurement_stale = %v, want true", got["measurement_stale"])
	}

	out, _, err = runCmd(t, newRootCmd(), "show", "milestone", "demo.bucket", "--json")
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if _, has := got["measurement_stale"]; has {
		t.Errorf("bucket: measurement_stale present, want absent")
	}
}

// TestListItemFields_MatchJSONTags keeps the --fields allowlist in step with
// listItem's json tags so a new field cannot ship unprojectable.
func TestListItemFields_MatchJSONTags(t *testing.T) {
	rt := reflect.TypeOf(listItem{})
	for i := range rt.NumField() {
		tag, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
		if tag == "" || tag == "-" {
			continue
		}
		if !slices.Contains(listItemFields, tag) {
			t.Errorf("json key %q missing from listItemFields", tag)
		}
	}
}
