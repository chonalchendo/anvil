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

// BodyWikilinkTargetsOfType scans body (outside fenced code blocks) for
// `[[t.id]]` wikilinks and returns each distinct full target (e.g.
// "convention.python"), in first-seen order. Body wikilinks are real graph
// edges — a contract links its conventions from `## Code design` prose, not a
// frontmatter slot — so a caller surfacing an artifact's links of a type must
// include them, not just frontmatter. A trailing `|alias` and surrounding
// whitespace are stripped to match the indexer's normalization.
func BodyWikilinkTargetsOfType(body string, t Type) []string {
	prefix := string(t) + "."
	matches := wikilinkRe.FindAllStringSubmatch(StripFencedBlocks(body), -1)
	seen := make(map[string]struct{})
	var out []string
	for _, m := range matches {
		target := strings.TrimSpace(m[1])
		if bar := strings.IndexByte(target, '|'); bar >= 0 {
			target = strings.TrimSpace(target[:bar])
		}
		if !strings.HasPrefix(target, prefix) || len(target) == len(prefix) {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		out = append(out, target)
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

// wikilinkTargetPath maps a wikilink target (`type.id`, brackets already
// stripped) to the file that would hold it. ok is false when the target cannot
// name an artifact at all: id-illegal chars (<, >, whitespace) mark a
// documentation placeholder, and a missing dot or unknown type prefix is not a
// vault reference.
func wikilinkTargetPath(v *Vault, target string) (string, bool) {
	if strings.ContainsAny(target, "<> \t\n") {
		return "", false
	}
	dot := strings.IndexByte(target, '.')
	if dot < 0 {
		return "", false
	}
	t, err := ParseType(target[:dot])
	if err != nil {
		return "", false
	}
	return artifactPath(v, t, ArtifactBasename(v, t, target)), true
}

// CanonicalID maps a raw id or wikilink target — with or without its `<type>.`
// prefix — to the id shape type t registers under. Design-type and convention
// ids keep the prefix (e.g. system-design.burgh, convention.python) because the
// index keys on a global artifacts.id; every other type drops it.
func CanonicalID(t Type, raw string) string {
	bare := strings.TrimPrefix(raw, string(t)+".")
	if t == TypeProductDesign || t == TypeSystemDesign || t == TypeConvention {
		return string(t) + "." + bare
	}
	return bare
}

// ArtifactBasename maps a raw id to type t's on-disk basename in v, accepting
// either filename shape: a file whose name carries its type prefix reads back
// the same as the bare-id shape. Neither on disk → the canonical shape, so a
// not-found error names the id the type is minted under.
func ArtifactBasename(v *Vault, t Type, raw string) string {
	canonical := CanonicalID(t, raw)
	prefix := string(t) + "."
	// Design and convention ids already carry the prefix, so canonical is their
	// only shape — probing the stripped form there would also resolve a doubled
	// `convention.convention.x` onto the plain file.
	if strings.HasPrefix(canonical, prefix) {
		return canonical
	}
	if _, err := os.Stat(artifactPath(v, t, prefix+canonical)); err == nil {
		return prefix + canonical
	}
	return canonical
}

// artifactPath joins a resolved basename onto its type's vault folder.
func artifactPath(v *Vault, t Type, basename string) string {
	return filepath.Join(v.Root, t.Dir(), basename+".md")
}

// WikilinkTargetExists reports whether target names an artifact file present
// in v. It is a predicate, not a reporter: ResolveLinks returns nothing both
// for "resolves" and for "not a link at all", so a write path asking "may I
// embed this edge?" must not read its empty result as existence.
func WikilinkTargetExists(v *Vault, target string) bool {
	path, ok := wikilinkTargetPath(v, target)
	if !ok {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func checkWikilinkTarget(v *Vault, field, target string) (UnresolvedLink, bool) {
	path, ok := wikilinkTargetPath(v, target)
	if !ok {
		return UnresolvedLink{}, false
	}
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
