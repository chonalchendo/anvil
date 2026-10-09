package cli

import (
	"fmt"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// diagramFailures reports each diagrams entry with no file in the vault. It is
// a no-op for types without the slot, so validate and set call it unconditionally.
func diagramFailures(vaultRoot, path string, fm map[string]any) []*errfmt.ValidationError {
	var out []*errfmt.ValidationError
	for _, name := range core.MissingDiagrams(vaultRoot, fm) {
		out = append(out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "diagrams",
			fmt.Sprintf("diagram %q has no file", name)).
			WithFix(fmt.Sprintf("add %s or remove the name", core.DiagramPath(vaultRoot, name))))
	}
	return out
}
