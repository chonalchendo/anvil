package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify lowercases s, transliterates via NFD + ASCII-filter, collapses
// non-alnum runs to "-", trims leading/trailing "-", and clips to 60 chars.
func Slugify(s string) string {
	decomposed := norm.NFD.String(s)
	var asciiBuf strings.Builder
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue // strip combining marks
		}
		if r > 127 {
			continue
		}
		asciiBuf.WriteRune(unicode.ToLower(r))
	}
	slug := nonAlnum.ReplaceAllString(asciiBuf.String(), "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 60 {
		slug = strings.TrimRight(slug[:60], "-")
	}
	return slug
}

// IDInputs carries optional fields used by some artifact types.
type IDInputs struct {
	Title   string // required — slug source
	Project string // required for issue/plan/milestone
	Topic   string // required for decision
}

// NextID returns the next available ID for type t under v.
// It enforces the per-type format from D1 and disambiguates collisions by
// scanning existing .md filenames in the type's directory.
func NextID(v *Vault, t Type, in IDInputs) (string, error) {
	slug := Slugify(in.Title)
	if slug == "" {
		return "", fmt.Errorf("title required (produced empty slug)")
	}
	switch t {
	case TypeInbox:
		date := time.Now().UTC().Format("2006-01-02")
		return uniqueID(v, t, fmt.Sprintf("%s-%s", date, slug))
	case TypeIssue, TypePlan:
		if in.Project == "" {
			return "", fmt.Errorf("project required for %s", t)
		}
		return uniqueID(v, t, fmt.Sprintf("%s.%s", in.Project, slug))
	case TypeMilestone:
		if in.Project == "" {
			return "", fmt.Errorf("project required for milestone")
		}
		return uniqueID(v, t, fmt.Sprintf("%s.%s", in.Project, slug))
	case TypeThread, TypeLearning:
		return uniqueID(v, t, slug)
	case TypeDecision:
		if in.Topic == "" {
			return "", fmt.Errorf("topic required for decision")
		}
		n, err := nextDecisionOrdinal(v, in.Topic)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.%04d-%s", in.Topic, n, slug), nil
	}
	return "", fmt.Errorf("unknown type %q", t)
}

// uniqueID returns base, or base-2, base-3, ... whichever does not yet exist as <dir>/<id>.md.
func uniqueID(v *Vault, t Type, base string) (string, error) {
	dir := filepath.Join(v.Root, t.Dir())
	if !fileExists(filepath.Join(dir, base+".md")) {
		return base, nil
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !fileExists(filepath.Join(dir, candidate+".md")) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unable to allocate unique ID for %s after 1000 attempts", base)
}

// nextDecisionOrdinal scans the decisions directory for files matching
// <topic>.NNNN-*.md and returns the next ordinal scoped to that topic.
func nextDecisionOrdinal(v *Vault, topic string) (int, error) {
	dir := filepath.Join(v.Root, TypeDecision.Dir())
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("reading decisions dir: %w", err)
	}
	prefix := topic + "."
	max := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".md") {
			continue
		}
		// Format: <topic>.NNNN-<slug>.md
		rest := strings.TrimSuffix(name[len(prefix):], ".md")
		dash := strings.IndexByte(rest, '-')
		if dash < 0 {
			continue
		}
		n, err := strconv.Atoi(rest[:dash])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
