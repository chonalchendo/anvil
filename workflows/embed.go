// Package workflows exposes the embedded GitHub Actions workflow templates as a
// virtual filesystem. `anvil init` writes these *.yml files into the vault's
// .github/workflows/ directory so automated hygiene activates once the user
// adds a git remote. The package lives at module root because //go:embed paths
// are relative to the source file.
package workflows

import "embed"

// FS is the embedded bundle of vault GitHub Actions workflow templates.
//
//go:embed *.yml
var FS embed.FS
