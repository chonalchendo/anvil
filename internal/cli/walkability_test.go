package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWalkability(t *testing.T) {
	t.Run("reports walkable, broken, and empty edges across the vault", func(t *testing.T) {
		vault := setupVault(t)

		// walkable: milestone and design both resolve with non-stub bodies.
		writeHydrateIssue(t, vault, "foo.walkable", map[string]any{"milestone": "[[milestone.foo.mw]]"})
		writeHydrateMilestone(t, vault, "foo.mw",
			map[string]any{"product_design": "[[product-design.foo]]"},
			"## Why now\n\nreal milestone body.\n")
		writeHydrateDesign(t, vault, "foo", "product-design", nil, "## Vision\n\nreal design body.\n")

		// broken: milestone link dangles.
		writeHydrateIssue(t, vault, "foo.broken", map[string]any{"milestone": "[[milestone.foo.ghost]]"})

		// transitive-broken: issue→milestone resolves, but milestone→design dangles;
		// the issue must report NOT walkable and name that depth>1 edge.
		writeHydrateIssue(t, vault, "foo.transitive", map[string]any{"milestone": "[[milestone.foo.mt]]"})
		writeHydrateMilestone(t, vault, "foo.mt",
			map[string]any{"product_design": "[[product-design.foo.ghost]]"},
			"## Why now\n\nreal milestone body.\n")

		// empty: milestone resolves but its body is a stub (blank).
		writeHydrateIssue(t, vault, "foo.empty", map[string]any{"milestone": "[[milestone.foo.me]]"})
		writeHydrateMilestone(t, vault, "foo.me", nil, "")

		cmd := newRootCmd()
		out, stderr, err := runCmd(t, cmd, "walkability")
		if err != nil {
			t.Fatalf("walkability: %v\nstderr: %s", err, stderr)
		}

		if !strings.Contains(out, "foo.walkable walkable") {
			t.Errorf("expected foo.walkable to be walkable, got:\n%s", out)
		}
		if !strings.Contains(out, "foo.broken NOT walkable") || !strings.Contains(out, "milestone.foo.ghost") {
			t.Errorf("expected foo.broken to name the dangling milestone edge, got:\n%s", out)
		}
		if !strings.Contains(out, "foo.empty NOT walkable") || !strings.Contains(out, "milestone milestone.foo.me (stub body)") {
			t.Errorf("expected foo.empty to name the empty milestone edge, got:\n%s", out)
		}
		if !strings.Contains(out, "foo.transitive NOT walkable") || !strings.Contains(out, "product-design.foo.ghost") {
			t.Errorf("expected foo.transitive to name the dangling transitive design edge, got:\n%s", out)
		}
	})

	t.Run("--json emits the bounded-list envelope with per-issue verdicts", func(t *testing.T) {
		vault := setupVault(t)
		writeHydrateIssue(t, vault, "foo.i1", nil)

		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "walkability", "--json")
		if err != nil {
			t.Fatalf("walkability --json: %v", err)
		}

		var env struct {
			Items []issueWalkability `json:"items"`
			Total int                `json:"total"`
		}
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("unmarshal: %v\noutput: %s", err, out)
		}
		if env.Total != 1 || len(env.Items) != 1 {
			t.Fatalf("expected 1 item, got total=%d items=%d", env.Total, len(env.Items))
		}
		if env.Items[0].ID != "issue.foo.i1" || !env.Items[0].Walkable {
			t.Errorf("expected issue.foo.i1 walkable=true, got %+v", env.Items[0])
		}
	})

	t.Run("--project filters the report to one project's issues", func(t *testing.T) {
		vault := setupVault(t)
		writeHydrateIssue(t, vault, "foo.i1", nil)
		writeHydrateIssue(t, vault, "bar.i1", map[string]any{"project": "bar"})

		cmd := newRootCmd()
		out, _, err := runCmd(t, cmd, "walkability", "--project", "foo")
		if err != nil {
			t.Fatalf("walkability --project: %v", err)
		}
		if !strings.Contains(out, "foo.i1") {
			t.Errorf("expected foo.i1 in filtered report, got:\n%s", out)
		}
		if strings.Contains(out, "bar.i1") {
			t.Errorf("expected bar.i1 excluded by --project foo, got:\n%s", out)
		}
	})
}
