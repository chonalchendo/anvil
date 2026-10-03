package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunPostLandBlocks(t *testing.T) {
	body := "## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\nfalse\n```\n```bash\n# anvil:post-land\nfalse\n```\n```bash\n# anvil:post-land\ntrue\n```\n"
	var buf bytes.Buffer
	runPostLandBlocks(&buf, body)
	out := buf.String()
	if strings.Contains(out, "Indirect#1") {
		t.Errorf("unmarked block ran post-land: %q", out)
	}
	if !strings.Contains(out, "Indirect#2 is RED") || !strings.Contains(out, "Indirect#3 passed") {
		t.Errorf("unexpected output: %q", out)
	}
}
