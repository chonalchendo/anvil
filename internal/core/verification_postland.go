package core

import "strings"

// PostLandMarker, as the first non-blank line of a Verification block, declares
// that the block's condition only becomes true after the PR merges.
const PostLandMarker = "# anvil:post-land"

// IsPostLand reports whether a Verification block carries PostLandMarker as
// its first non-blank line.
func IsPostLand(block string) bool {
	for _, line := range strings.Split(block, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t == PostLandMarker
		}
	}
	return false
}
