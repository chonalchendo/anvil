package core

import "strings"

// PostLandMarker is the first non-blank line that marks a Verification block as post-land: its condition only becomes true after the PR merges.
const PostLandMarker = "# anvil:post-land"

// IsPostLand reports whether a Verification block carries PostLandMarker as
// its first non-blank line.
func IsPostLand(block string) bool {
	for _, line := range strings.Split(block, "\n") {
		if t := strings.Trim(line, " \t\r\v\f"); t != "" {
			return t == PostLandMarker
		}
	}
	return false
}
