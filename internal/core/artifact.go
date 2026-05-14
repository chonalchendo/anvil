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
	a, err := ParseArtifact(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	a.Path = path
	return a, nil
}

// ParseArtifact splits raw bytes into frontmatter and body. Path is left blank;
// callers that have an originating path should set Path themselves. Used by
// `anvil create --from <path|->` to ingest an authored artifact from memory.
func ParseArtifact(b []byte) (*Artifact, error) {
	rest, body, ok := splitFrontMatter(b)
	if !ok {
		return nil, fmt.Errorf("no frontmatter delimiter: %w", ErrFrontmatterParse)
	}
	a := &Artifact{FrontMatter: map[string]any{}, Body: body}
	if err := yaml.Unmarshal(rest, &a.FrontMatter); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", errors.Join(enrichYAMLError(rest, err), ErrFrontmatterParse))
	}
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
