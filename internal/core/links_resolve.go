package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// UnresolvedLink names a frontmatter field whose wikilink target cannot be
// found in the vault. Field uses `name` for scalars and `name[i]` for arrays.
type UnresolvedLink struct {
	Field  string `json:"field"`
	Target string `json:"target"`
}

var wikilinkRe = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// fencedBlockRe matches a triple-backtick fenced code block (opening fence,
// any content, closing fence) including the fence lines themselves. The
// non-greedy [\s\S]*? prevents the first closing fence from consuming
// subsequent fenced blocks.
var fencedBlockRe = regexp.MustCompile("(?m)^```[^\n]*\n[\\s\\S]*?^```[ \t]*$")

// StripFencedBlocks replaces the content of every triple-backtick fenced code
// block with an empty placeholder, so that wikilink scanners won't mistake
// illustrative [[links]] inside code samples for live vault references.
// Only standard triple-backtick fences are stripped; tilde fences and
// indented code blocks are out of scope per the issue non-goals.
func StripFencedBlocks(body string) string {
	return fencedBlockRe.ReplaceAllString(body, "```\n```")
}

// ResolveLinks walks fm and returns every wikilink (`[[type.id]]`) whose
// target file is missing from v. Tokens whose type prefix is not a known
// Anvil type are ignored — they are treated as non-vault references.
//
// Field iteration is sorted by name to keep output deterministic.
func ResolveLinks(v *Vault, fm map[string]any) []UnresolvedLink {
	keys := make([]string, 0, len(fm))
	for k := range fm {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out []UnresolvedLink
	for _, k := range keys {
		switch val := fm[k].(type) {
		case string:
			if u, ok := checkWikilink(v, k, val); ok {
				out = append(out, u)
			}
		case []any:
			for i, e := range val {
				s, ok := e.(string)
				if !ok {
					continue
				}
				field := fmt.Sprintf("%s[%d]", k, i)
				if u, ok := checkWikilink(v, field, s); ok {
					out = append(out, u)
				}
			}
		}
	}
	return out
}

// checkWikilink returns (link, true) if s contains a wikilink whose target
// resolves to a known type but the file is missing. Strings without a
// wikilink, and wikilinks with unknown type prefixes, return (_, false).
func checkWikilink(v *Vault, field, s string) (UnresolvedLink, bool) {
	m := wikilinkRe.FindStringSubmatch(s)
	if m == nil {
		return UnresolvedLink{}, false
	}
	return checkWikilinkTarget(v, field, m[1])
}

func checkWikilinkTarget(v *Vault, field, target string) (UnresolvedLink, bool) {
	// A target containing id-illegal chars (<, >, or whitespace) is a
	// documentation placeholder — it can never be a real artifact id, so
	// treat it as literal text rather than a resolvable link.
	if strings.ContainsAny(target, "<> \t\n") {
		return UnresolvedLink{}, false
	}
	dot := strings.IndexByte(target, '.')
	if dot < 0 {
		return UnresolvedLink{}, false
	}
	prefix, id := target[:dot], target[dot+1:]
	t, err := ParseType(prefix)
	if err != nil {
		return UnresolvedLink{}, false
	}
	// Design-type ids keep the type prefix (e.g. system-design.burgh) for global
	// uniqueness, so the on-disk id is the full wikilink target, not the portion
	// after the type prefix.
	fileID := id
	if t == TypeProductDesign || t == TypeSystemDesign {
		fileID = target
	}
	path := filepath.Join(v.Root, t.Dir(), fileID+".md")
	if _, err := os.Stat(path); err == nil {
		return UnresolvedLink{}, false
	}
	return UnresolvedLink{Field: field, Target: target}, true
}

// ResolveBodyLinks scans body text for every `[[type.id]]` wikilink and
// returns the targets that don't resolve in v. Mirrors ResolveLinks but
// walks free-form markdown instead of typed frontmatter slots — duplicate
// targets in the body are reported once. Field is "body" for every entry.
//
// Unlike frontmatter links, body wikilinks whose type prefix is not a known
// Anvil type are flagged as unresolved rather than ignored. The link-indexer
// only indexes type-prefixed wikilinks, so a bare `project.slug` form would
// silently produce a graph orphan; rejecting it at create time surfaces the
// error before the artifact is written.
func ResolveBodyLinks(v *Vault, body string) []UnresolvedLink {
	matches := wikilinkRe.FindAllStringSubmatch(StripFencedBlocks(body), -1)
	if matches == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	var out []UnresolvedLink
	for _, m := range matches {
		// Normalize the token exactly as the indexer's LinkRowsFromBody does —
		// trim surrounding whitespace, then drop a trailing `|alias` — so the
		// two paths agree on which body wikilinks are real references. Without
		// this, `[[ anvil.foo ]]` or `[[type.x|Alias]]` would survive create yet
		// the indexer would treat them differently, re-opening the orphan hole.
		target := strings.TrimSpace(m[1])
		if bar := strings.IndexByte(target, '|'); bar >= 0 {
			target = strings.TrimSpace(target[:bar])
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		// Angle-bracket targets (e.g. [[milestone.<project>.<slug>]]) are
		// documentation placeholders, not real artifact ids — skip them.
		if strings.ContainsAny(target, "<>") {
			continue
		}
		dot := strings.IndexByte(target, '.')
		if dot < 0 {
			// No dot → cannot be a vault reference; skip.
			continue
		}
		prefix := target[:dot]
		if _, err := ParseType(prefix); err != nil {
			// Unknown type prefix: the indexer will silently drop this link, so
			// flag it now so the author can use the full type.project.slug form.
			out = append(out, UnresolvedLink{Field: "body", Target: target})
			continue
		}
		if u, ok := checkWikilinkTarget(v, "body", target); ok {
			out = append(out, u)
		}
	}
	return out
}
