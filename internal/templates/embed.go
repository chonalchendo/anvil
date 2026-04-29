// Package templates ships frontmatter templates for the create verb.
package templates

import "embed"

// FS holds the embedded *.tmpl files for all artifact types.
//
//go:embed *.tmpl
var FS embed.FS
