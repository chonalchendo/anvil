// Package anvil embeds the text the session hooks print: the session guide and
// the shared Prose style block.
package anvil

import _ "embed"

// SessionGuide teaches anvil to sessions whose repo instructions do not.
//
//go:embed session-guide.md
var SessionGuide string

// WritingBlock is the standard Prose style rule; check-skills.sh holds every
// shipped skill and agent to it byte for byte.
//
//go:embed writing-block.md
var WritingBlock string
