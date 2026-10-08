package cli

import (
	"regexp"
	"strconv"
	"strings"
)

// reviewRound is one persisted `reviewing-pr` round for a PR.
type reviewRound struct {
	round   int
	sha     string
	blocked bool
}

var (
	reviewHeadingRe = regexp.MustCompile(`^## Review findings\s+\S+\s+PR (\d+), round (\d+) @ ([0-9a-f]{7,40})`)
	blockingBandRe  = regexp.MustCompile(`^\[(blocker|high|medium)\]`)
)

// latestReviewRound returns the highest-numbered round for pr. A round is
// blocked when a line in its own section starts with a blocking band; prose
// that merely quotes a band mid-line does not count.
func latestReviewRound(body string, pr int) (reviewRound, bool) {
	var best reviewRound
	found, in := false, false
	cur := -1
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## ") {
			in = false
			m := reviewHeadingRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			p, _ := strconv.Atoi(m[1])
			k, _ := strconv.Atoi(m[2])
			if p != pr {
				continue
			}
			in = true
			if !found || k > best.round {
				best = reviewRound{round: k, sha: m[3]}
				found = true
			}
			cur = k
			continue
		}
		if in && cur == best.round && blockingBandRe.MatchString(line) {
			best.blocked = true
		}
	}
	return best, found
}
