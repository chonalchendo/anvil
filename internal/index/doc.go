// Package index owns the materialised view of vault state stored in
// <vault>/.anvil/vault.db. The DB is a derived index — never the source
// of truth. Markdown files on disk are. Reindex rebuilds it.
package index
