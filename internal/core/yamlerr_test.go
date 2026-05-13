package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadAndError plants frontmatter at path and returns the LoadArtifact error
// (or t.Fatal if the load unexpectedly succeeded).
func loadAndError(t *testing.T, frontmatter string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "bad.md")
	body := "---\n" + frontmatter + "---\n\nbody\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadArtifact(p)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	return err.Error()
}

func TestEnrichYAMLError_UnescapedInnerDoubleQuote(t *testing.T) {
	msg := loadAndError(t, "type: issue\ntitle: \"hello \"world\" foo\"\nstatus: open\n")
	for _, want := range []string{
		"near line",
		"hint:",
		`"hello "world" foo"`,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q in:\n%s", want, msg)
		}
	}
	if !strings.Contains(msg, "double-quote") {
		t.Errorf("expected double-quote hint, got:\n%s", msg)
	}
	if !strings.Contains(msg, "field: title") {
		t.Errorf("expected field: title, got:\n%s", msg)
	}
}

func TestEnrichYAMLError_ColonSpaceInUnquotedValue(t *testing.T) {
	msg := loadAndError(t, "type: issue\ntitle: foo: bar\nstatus: open\n")
	if !strings.Contains(msg, `": "`) && !strings.Contains(msg, "nested key") {
		t.Errorf("expected colon-space hint, got:\n%s", msg)
	}
	if !strings.Contains(msg, "field: title") {
		t.Errorf("expected field: title, got:\n%s", msg)
	}
}

func TestEnrichYAMLError_LeadingBacktick(t *testing.T) {
	msg := loadAndError(t, "type: issue\ntitle: `wrap me\nstatus: open\n")
	if !strings.Contains(msg, "reserved character") {
		t.Errorf("expected reserved-character hint, got:\n%s", msg)
	}
}

func TestEnrichYAMLError_PassthroughUnknownShape(t *testing.T) {
	// A genuinely-broken shape that won't match any classifier should still
	// produce a usable wrapped error (line + excerpt at minimum).
	msg := loadAndError(t, "type: issue\nfoo:\n  - one\n  bad-indent: x\n")
	if !strings.Contains(msg, "yaml:") {
		t.Errorf("expected original yaml error preserved, got:\n%s", msg)
	}
}

func TestClassifyLine(t *testing.T) {
	tests := []struct {
		line     string
		wantSub  string // substring expected in the hint; empty means "no hint"
		wantSkip bool   // expect empty hint
	}{
		{`title: "hello "world" foo"`, "double-quote", false},
		{`title: foo: bar`, "nested key", false},
		{"title: `wrap me", "reserved character", false},
		{`title: "well-formed"`, "", true},
		{`title: well formed`, "", true},
		{`# title: foo: bar`, "", true}, // comment
		{``, "", true},
	}
	for _, tt := range tests {
		got := classifyLine(tt.line)
		if tt.wantSkip {
			if got != "" {
				t.Errorf("classifyLine(%q) = %q, want empty", tt.line, got)
			}
			continue
		}
		if !strings.Contains(got, tt.wantSub) {
			t.Errorf("classifyLine(%q) = %q, want substring %q", tt.line, got, tt.wantSub)
		}
	}
}
