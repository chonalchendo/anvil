package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrFrontmatterParse is the sentinel wrapped into every LoadArtifact error
// that originates from malformed YAML or a missing frontmatter delimiter.
// Callers that tolerate corrupt files (e.g. the facet walk) test for this via
// errors.Is; other load failures (OS/permission errors) are not wrapped.
var ErrFrontmatterParse = errors.New("frontmatter parse error")

// Artifact is a parsed vault file: YAML frontmatter delimited by `---`,
// followed by a Markdown body.
type Artifact struct {
	Path        string
	FrontMatter map[string]any
	Body        string
}

const fmDelim = "---"

// LoadArtifact reads path and splits YAML frontmatter from the body.
func LoadArtifact(path string) (*Artifact, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rest, body, ok := splitFrontMatter(b)
	if !ok {
		return nil, fmt.Errorf("no frontmatter delimiter in %s: %w", path, ErrFrontmatterParse)
	}
	a := &Artifact{Path: path, FrontMatter: map[string]any{}}
	if err := yaml.Unmarshal(rest, &a.FrontMatter); err != nil {
		return nil, fmt.Errorf("parse frontmatter %s: %w", path, errors.Join(enrichYAMLError(rest, err), ErrFrontmatterParse))
	}
	a.Body = body
	return a, nil
}

func splitFrontMatter(b []byte) ([]byte, string, bool) {
	if !bytes.HasPrefix(b, []byte(fmDelim+"\n")) {
		return nil, "", false
	}
	rest := b[len(fmDelim)+1:]
	end := bytes.Index(rest, []byte("\n"+fmDelim+"\n"))
	if end < 0 {
		return nil, "", false
	}
	return rest[:end], string(rest[end+len("\n"+fmDelim+"\n"):]), true
}

// Save writes a back to disk: frontmatter delimited by `---`, then Body.
func (a *Artifact) Save() error {
	var out bytes.Buffer
	out.WriteString(fmDelim + "\n")
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(a.FrontMatter); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	out.WriteString(fmDelim + "\n")
	if !strings.HasPrefix(a.Body, "\n") {
		out.WriteString("\n")
	}
	out.WriteString(a.Body)
	return os.WriteFile(a.Path, out.Bytes(), 0o644)
}
