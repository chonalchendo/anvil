package cli

import "errors"

// ErrArtifactNotFound is returned when the requested artifact file does not exist.
var ErrArtifactNotFound = errors.New("artifact not found")

// ErrSchemaInvalid is returned when frontmatter fails JSON Schema validation.
var ErrSchemaInvalid = errors.New("schema invalid")
