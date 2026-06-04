// Package bases exposes the embedded Obsidian Bases dashboards as a virtual
// filesystem. `anvil init` writes these *.base files into the vault's 90-bases/
// directory so a fresh vault renders per-type table views out of the box; the
// user then edits them freely. The package lives at module root because
// //go:embed paths are relative to the source file.
package bases

import "embed"

// FS is the embedded bundle of project-agnostic Bases dashboards.
//
//go:embed *.base
var FS embed.FS
