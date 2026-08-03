// Package agents exposes the bundled Anvil agent definitions as an embed.FS
// so the binary can materialise them on `anvil install agents`. Each top-level
// *.md is a committed-subagent definition deployed to ~/.claude/agents/<name>.md.
package agents

import "embed"

// FS is the embedded bundle of Anvil agent definitions. The glob keeps a new
// agent file from landing unembedded.
//
//go:embed *.md
var FS embed.FS
