package build

import (
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
