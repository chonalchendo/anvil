// Package glossary owns _meta/glossary.md — the vault's tag vocabulary plus
// a small definitions section. Parsing is intentionally regex-light: the file
// has a fixed shape produced exclusively by Save, so we walk it line by line.
package glossary

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Facets is the closed set of tag facets. Order matches Save's output.
var Facets = []string{"domain", "activity", "pattern", "type"}

// Path returns the canonical location of the glossary file inside vaultRoot.
func Path(vaultRoot string) string {
	return filepath.Join(vaultRoot, "_meta", "glossary.md")
}

// Glossary is the parsed glossary; tag entries live under their facet.
type Glossary struct {
	tags        map[string][]Entry
	definitions []Entry
}

// Entry is a single bullet: a key (tag without facet, or term) plus a description.
type Entry struct {
	Key  string
	Desc string
}

// New returns an empty glossary with all facets initialised.
func New() *Glossary {
	g := &Glossary{tags: map[string][]Entry{}}
	for _, f := range Facets {
		g.tags[f] = nil
	}
	return g
}

// Load parses path. If path does not exist, returns an empty Glossary, nil.
func Load(path string) (*Glossary, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading glossary: %w", err)
	}
	return parse(b)
}

// Tags returns every tag in canonical (facet) order, then insertion order within facet.
func (g *Glossary) Tags() []string {
	var out []string
	for _, f := range Facets {
		for _, e := range g.tags[f] {
			out = append(out, f+"/"+e.Key)
		}
	}
	return out
}

// HasTag reports whether tag (full "<facet>/<name>") is present.
func (g *Glossary) HasTag(tag string) bool {
	facet, name, ok := splitTag(tag)
	if !ok {
		return false
	}
	for _, e := range g.tags[facet] {
		if e.Key == name {
			return true
		}
	}
	return false
}

// AddTag appends a tag entry under the inferred facet. No-op if the tag exists.
func (g *Glossary) AddTag(tag, desc string) error {
	facet, name, ok := splitTag(tag)
	if !ok {
		return fmt.Errorf("tag %q must have shape <facet>/<name> with facet in %v", tag, Facets)
	}
	if !knownFacet(facet) {
		return fmt.Errorf("unknown facet %q (want one of %v)", facet, Facets)
	}
	for _, e := range g.tags[facet] {
		if e.Key == name {
			return nil
		}
	}
	g.tags[facet] = append(g.tags[facet], Entry{Key: name, Desc: desc})
	return nil
}

// FindTagDesc returns the description for tag (full <facet>/<name>) or "", false.
func (g *Glossary) FindTagDesc(tag string) (string, bool) {
	facet, name, ok := splitTag(tag)
	if !ok {
		return "", false
	}
	for _, e := range g.tags[facet] {
		if e.Key == name {
			return e.Desc, true
		}
	}
	return "", false
}

// UpdateTagDesc rewrites tag's description in place.
// Returns false if the tag is absent.
func (g *Glossary) UpdateTagDesc(tag, desc string) bool {
	facet, name, ok := splitTag(tag)
	if !ok {
		return false
	}
	for i, e := range g.tags[facet] {
		if e.Key == name {
			g.tags[facet][i].Desc = desc
			return true
		}
	}
	return false
}

// Definition returns the description for term, or "", false if absent.
func (g *Glossary) Definition(term string) (string, bool) {
	for _, e := range g.definitions {
		if e.Key == term {
			return e.Desc, true
		}
	}
	return "", false
}

// Save writes g to path in canonical form, creating the parent directory.
func (g *Glossary) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	var b bytes.Buffer
	b.WriteString("# Vault Glossary\n\n## Tags\n\n")
	for _, f := range Facets {
		fmt.Fprintf(&b, "### %s/\n", f)
		for _, e := range g.tags[f] {
			fmt.Fprintf(&b, "- `%s/%s` — %s\n", f, e.Key, e.Desc)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Definitions\n")
	for _, e := range g.definitions {
		fmt.Fprintf(&b, "- **%s** — %s\n", e.Key, e.Desc)
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

func splitTag(tag string) (facet, name string, ok bool) {
	i := strings.IndexByte(tag, '/')
	if i <= 0 || i == len(tag)-1 {
		return "", "", false
	}
	return tag[:i], tag[i+1:], true
}

func knownFacet(f string) bool {
	for _, k := range Facets {
		if k == f {
			return true
		}
	}
	return false
}

func parse(b []byte) (*Glossary, error) {
	g := New()
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)

	const (
		modeNone = iota
		modeTags
		modeDefs
	)
	mode := modeNone
	currentFacet := ""

	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), " \t")
		switch {
		case line == "## Tags":
			mode = modeTags
			currentFacet = ""
		case line == "## Definitions":
			mode = modeDefs
			currentFacet = ""
		case strings.HasPrefix(line, "### ") && mode == modeTags:
			facet := strings.TrimSuffix(strings.TrimPrefix(line, "### "), "/")
			if knownFacet(facet) {
				currentFacet = facet
			} else {
				currentFacet = ""
			}
		case strings.HasPrefix(line, "- ") && mode == modeTags && currentFacet != "":
			key, desc, ok := parseTagBullet(line, currentFacet)
			if !ok {
				continue
			}
			g.tags[currentFacet] = append(g.tags[currentFacet], Entry{Key: key, Desc: desc})
		case strings.HasPrefix(line, "- **") && mode == modeDefs:
			key, desc, ok := parseDefBullet(line)
			if !ok {
				continue
			}
			g.definitions = append(g.definitions, Entry{Key: key, Desc: desc})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scanning glossary: %w", err)
	}
	return g, nil
}

func parseTagBullet(line, facet string) (key, desc string, ok bool) {
	prefix := "- `" + facet + "/"
	if !strings.HasPrefix(line, prefix) {
		return "", "", false
	}
	rest := line[len(prefix):]
	end := strings.Index(rest, "`")
	if end < 0 {
		return "", "", false
	}
	key = rest[:end]
	rest = strings.TrimSpace(rest[end+1:])
	rest = strings.TrimPrefix(rest, "—")
	return key, strings.TrimSpace(rest), true
}

func parseDefBullet(line string) (key, desc string, ok bool) {
	rest := strings.TrimPrefix(line, "- **")
	end := strings.Index(rest, "**")
	if end < 0 {
		return "", "", false
	}
	key = rest[:end]
	rest = strings.TrimSpace(rest[end+2:])
	rest = strings.TrimPrefix(rest, "—")
	return key, strings.TrimSpace(rest), true
}
