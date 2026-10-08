// Package skills exposes the bundled Anvil skill directories as an embed.FS
// so the binary can materialise them on `anvil install skills`. Only the
// canonical user-facing skills are listed here; -workspace siblings used for
// eval iteration are intentionally excluded from the bundle.
package skills

import "embed"

// FS is the embedded bundle of canonical Anvil skill directories.
//
//go:embed capturing-inbox completing-issue distilling-learning handing-off-session opening-thread refreshing-learnings responding-to-pr-review resuming-session reviewing-pr running-milestone writing-component-design writing-convention writing-issue writing-milestone writing-product-design writing-system-design
var FS embed.FS
