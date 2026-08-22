package index

import "path/filepath"

// walkRoot returns the path a vault walk must start from. filepath.WalkDir does
// not descend a symlinked root — it reports the root as a symlink entry and
// stops — so a vault reached through a link indexes as empty, silently and with
// exit 0. Resolving here rather than in core.ResolveVault keeps every other
// caller seeing the path the operator configured.
//
// An unresolvable root (a vault not created yet) is returned unchanged; the
// walk then fails or finds nothing on its own terms, as it did before.
func walkRoot(vaultRoot string) string {
	if resolved, err := filepath.EvalSymlinks(vaultRoot); err == nil {
		return resolved
	}
	return vaultRoot
}
