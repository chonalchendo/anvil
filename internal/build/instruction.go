package build

import (
	"fmt"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// assembleInstruction produces the prompt body delivered to the agent CLI.
// Skills / Context / Model / Effort travel as RunRequest fields, not in the
// prompt body — the adapter handles their delivery.
func assembleInstruction(t core.Task) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(t.Body))
	b.WriteByte('\n')
	if len(t.SuccessCriteria) > 0 {
		b.WriteString("\n## Success criteria\n")
		for _, c := range t.SuccessCriteria {
			b.WriteString("- ")
			b.WriteString(c)
			b.WriteByte('\n')
		}
	}
	if len(t.Files) > 0 {
		b.WriteString("\n## Files most relevant\n")
		for _, f := range t.Files {
			b.WriteString("- ")
			b.WriteString(f)
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n## Verification\nBefore declaring done, run: ")
	b.WriteString(t.Verify)
	b.WriteByte('\n')
	return b.String()
}

// noDiffNudge is Channel B (agent-fetched contents on respawn): prose appended
// to the instruction on a retry attempt after the advance-gate found no
// landable diff on the prior try. It escalates by attempt so the last try
// before the cap reads as the final warning it is — distinct from Channel A
// (harness-enforced caps/timeouts on the spawn itself).
func noDiffNudge(attempt, maxAttempts int) string {
	severity := "CRITICAL"
	if attempt == maxAttempts-1 {
		severity = "FINAL ATTEMPT — CRITICAL"
	}
	return fmt.Sprintf(
		"\n## Retry %d/%d\n%s: your previous attempt produced no verified diff — no commit, no PR. "+
			"You MUST make the change, verify it, commit, push, and open a PR before finishing. "+
			"Do not report done without a landable diff.\n",
		attempt, maxAttempts-1, severity,
	)
}
