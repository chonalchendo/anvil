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
	Field  string
	Target string
}

var wikilinkRe = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

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
	target := m[1]
	dot := strings.IndexByte(target, '.')
	if dot < 0 {
		return UnresolvedLink{}, false
	}
	prefix, id := target[:dot], target[dot+1:]
	t, err := ParseType(prefix)
	if err != nil {
		return UnresolvedLink{}, false
	}
	path := filepath.Join(v.Root, t.Dir(), id+".md")
	if _, err := os.Stat(path); err == nil {
		return UnresolvedLink{}, false
	}
	return UnresolvedLink{Field: field, Target: target}, true
}
