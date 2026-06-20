package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

// ErrArtifactNotFound is returned when the requested artifact file does not exist.
var ErrArtifactNotFound = errors.New("artifact not found")

// errArtifactNotFound wraps ErrArtifactNotFound with the missing id so the
// error message names the offending id verbatim ("artifact not found: <id>").
// The fixed prefix ("artifact") is first so fang's title-case transform
// capitalises a real word rather than the id.
type errArtifactNotFound struct{ id string }

func (e *errArtifactNotFound) Error() string { return "artifact not found: " + e.id }
func (e *errArtifactNotFound) Is(target error) bool {
	return target == ErrArtifactNotFound
}

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

// namedArgs returns a cobra.PositionalArgs validator that replaces cobra's
// bare "Accepts N arg(s)" message with one that names the missing positional(s)
// drawn from the command's Use string. When the first missing arg is <type>,
// the error also lists the valid type values so the caller doesn't need a
// separate --help round-trip.
//
// positionals lists all expected arg placeholders in order (e.g. ["<type>",
// "<id>"]). minCount is the minimum number required. maxCount is the most
// accepted; pass -1 for an unbounded trailing variadic (e.g. set's
// [<value>...]). For ExactArgs-style commands minCount == maxCount ==
// len(positionals).
func namedArgs(use string, positionals []string, minCount, maxCount int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) < minCount {
			// Identify the first missing positional.
			missing := positionals[len(args)]
			msg := fmt.Sprintf("missing required argument %s — expected: %s", missing, use)
			if missing == "<type>" {
				types := make([]string, len(core.AllTypes))
				for i, t := range core.AllTypes {
					types[i] = string(t)
				}
				msg += "\nvalid types: " + strings.Join(types, "|")
			}
			return errors.New(msg)
		}
		if maxCount >= 0 && len(args) > maxCount {
			return fmt.Errorf("too many arguments: %q — expected: %s", strings.Join(args[maxCount:], " "), use)
		}
		return nil
	}
}
