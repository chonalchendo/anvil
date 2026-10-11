package cli

import (
	"bytes"
	"errors"
	"os/exec"
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

// layerPatchIDFn is a seam: it returns the stable patch-id of the layer diff
// (merge-base of base and commit, to commit), run from dir.
var layerPatchIDFn = layerPatchIDReal

func layerPatchIDReal(dir, base, commit string) (string, error) {
	diff := exec.Command("git", "diff", base+"..."+commit) //nolint:gosec // refs come from gh and the vault, passed as one argv entry
	diff.Dir = dir
	raw, err := diff.Output()
	if err != nil {
		return "", err
	}
	pid := exec.Command("git", "patch-id", "--stable")
	pid.Dir = dir
	pid.Stdin = bytes.NewReader(raw)
	out, err := pid.Output()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", errors.New("empty layer diff")
	}
	return fields[0], nil
}

// samePatch reports whether the layer diff is identical at the round's sha and
// at the head. Any failure, such as an unreachable sha, reads as "not the same".
func samePatch(dir, base, roundSha, headOid string) bool {
	a, err := layerPatchIDFn(dir, base, roundSha)
	if err != nil {
		return false
	}
	b, err := layerPatchIDFn(dir, base, headOid)
	return err == nil && a == b
}
