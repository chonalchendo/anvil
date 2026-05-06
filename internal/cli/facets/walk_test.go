package facets_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chonalchendo/anvil/internal/cli/facets"
	"github.com/chonalchendo/anvil/internal/core"
)

func TestCollectValues_Empty(t *testing.T) {
	dir := t.TempDir()
	v := &core.Vault{Root: dir}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	got, err := facets.CollectValues(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, facet := range []string{"domain", "activity", "pattern"} {
		if _, ok := got[facet]; !ok {
			t.Errorf("CollectValues missing facet bucket %q", facet)
		}
		if len(got[facet]) != 0 {
			t.Errorf("CollectValues[%q] non-empty for fresh vault: %v", facet, got[facet])
		}
	}
}

func TestCollectValues_AggregatesAcrossTypes(t *testing.T) {
	dir := t.TempDir()
	v := &core.Vault{Root: dir}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	write := func(rel, fm string) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("---\n"+fm+"\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("70-issues/foo.a.md", "type: issue\ntitle: a\ntags: [domain/dbt, activity/testing]")
	write("20-learnings/b.md", "type: learning\ntitle: b\ntags: [domain/postgres, pattern/idempotency]")
	write("60-threads/c.md", "type: thread\ntitle: c\ntags: [domain/dbt, activity/research]")
	write("00-inbox/d.md", "type: inbox\ntitle: d\ntags: [domain/should-be-included]")

	got, err := facets.CollectValues(dir)
	if err != nil {
		t.Fatal(err)
	}
	wantDomain := map[string]bool{"dbt": true, "postgres": true, "should-be-included": true}
	for v := range wantDomain {
		if _, ok := got["domain"][v]; !ok {
			t.Errorf("missing domain value %q", v)
		}
	}
	if _, ok := got["activity"]["testing"]; !ok {
		t.Error("missing activity/testing")
	}
	if _, ok := got["activity"]["research"]; !ok {
		t.Error("missing activity/research")
	}
	if _, ok := got["pattern"]["idempotency"]; !ok {
		t.Error("missing pattern/idempotency")
	}
}
