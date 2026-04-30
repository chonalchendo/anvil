// Package schemas exposes the embedded JSON Schema files as a virtual filesystem.
// The package lives at module root because //go:embed paths are relative to the
// source file, and the schema files live alongside this Go file rather than
// inside internal/schema/.
package schemas

import "embed"

//go:embed *.schema.json
var FS embed.FS
