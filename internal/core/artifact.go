package core

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

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
		return nil, fmt.Errorf("no frontmatter delimiter in %s", path)
	}
	a := &Artifact{Path: path, FrontMatter: map[string]any{}}
	if err := yaml.Unmarshal(rest, &a.FrontMatter); err != nil {
		return nil, fmt.Errorf("parse frontmatter %s: %w", path, err)
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
