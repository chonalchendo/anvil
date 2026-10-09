package core

import (
	"os"
	"path/filepath"
	"regexp"
)

// diagramName is the schema's item pattern; the existence check skips names
// that fail it so a traversal like "../x" never reaches the filesystem.
var diagramName = regexp.MustCompile(`^[a-z0-9-]+$`)

// DiagramPath is where a design's diagram name resolves in the vault.
func DiagramPath(vaultRoot, name string) string {
	return filepath.Join(vaultRoot, "_meta", "diagrams", name+".html")
}

// DiagramNames returns the string items of fm["diagrams"], in slot order.
// Non-string items are the schema's finding.
func DiagramNames(fm map[string]any) []string {
	var out []string
	items, _ := fm["diagrams"].([]any)
	for _, raw := range items {
		if name, ok := raw.(string); ok {
			out = append(out, name)
		}
	}
	return out
}

// MissingDiagrams returns the names in fm["diagrams"] with no HTML file under
// the vault's _meta/diagrams. Malformed names are the schema's finding.
func MissingDiagrams(vaultRoot string, fm map[string]any) []string {
	var missing []string
	for _, name := range DiagramNames(fm) {
		if !diagramName.MatchString(name) {
			continue
		}
		if _, err := os.Stat(DiagramPath(vaultRoot, name)); err != nil {
			missing = append(missing, name)
		}
	}
	return missing
}
