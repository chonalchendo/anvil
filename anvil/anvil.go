// Package anvil exposes the shipped text that is not a skill or agent: the
// session guide and the Prose style block every skill and agent also carries.
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
