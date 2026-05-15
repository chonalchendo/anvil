// Package skills exposes the bundled Anvil skill directories as an embed.FS
// so the binary can materialise them on `anvil install skills`. Only the
// canonical user-facing skills are listed here; -workspace siblings used for
// eval iteration are intentionally excluded from the bundle.
package skills

import "embed"

// FS is the embedded bundle of canonical Anvil skill directories.
//
//go:embed capturing-inbox dispatching-issue-fleet distilling-learning extracting-skill-from-session handing-off-session implementing-plan opening-thread researching responding-to-pr-review resuming-session reviewing-pr writing-issue writing-milestone writing-plan writing-product-design writing-system-design
var FS embed.FS
