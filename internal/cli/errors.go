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

// ErrIllegalTransition is returned by `transition` when no edge exists.
var ErrIllegalTransition = errors.New("illegal transition")

// ErrTransitionFlagRequired is returned when a required edge flag is absent.
var ErrTransitionFlagRequired = errors.New("transition flag required")

// ErrIndexStale is returned when vault.db is older than the vault dir mtime.
var ErrIndexStale = errors.New("vault index stale")

// ErrUnsupportedForType is returned for per-type gates (e.g. --ready).
var ErrUnsupportedForType = errors.New("unsupported for type")
