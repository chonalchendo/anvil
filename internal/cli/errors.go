package cli

import "errors"

// ErrArtifactNotFound is returned when the requested artifact file does not exist.
var ErrArtifactNotFound = errors.New("artifact not found")

// ErrSchemaInvalid is returned when frontmatter fails JSON Schema validation.
var ErrSchemaInvalid = errors.New("schema invalid")

// ErrUnresolvedLinks is returned when --validate detects wikilinks that do
// not resolve to vault files.
var ErrUnresolvedLinks = errors.New("unresolved links")

// ErrCreateDrift is returned when `create` finds an existing artifact whose
// content differs from the requested inputs and --update was not passed.
var ErrCreateDrift = errors.New("create drift")
