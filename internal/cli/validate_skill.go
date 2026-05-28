package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/anvil/skills"
	"github.com/chonalchendo/anvil/schemas"
)

// CodeSkillSchemaDrift is the error code emitted when a skill prescribes a
// frontmatter field that its target type's schema rejects.
const CodeSkillSchemaDrift = "skill_schema_drift"

// skillTypeTargets maps each authoring skill name to its target artifact type.
// Only skills that prescribe typed frontmatter can drift from a schema; the
// remaining skills (workflow-only) are omitted.
var skillTypeTargets = map[string]string{
	"writing-issue":          "issue",
	"writing-milestone":      "milestone",
	"writing-plan":           "plan",
	"writing-product-design": "product-design",
	"writing-system-design":  "system-design",
}

// prescriptivePrefixes are the patterns that identify a directive telling the
// author to write a field into frontmatter. Each prefix must include enough
// context to distinguish a frontmatter authoring directive from other uses of
// backtick-quoted tokens (e.g. "Capture `id` from JSON output").
var prescriptivePrefixes = []string{
	"Capture frontmatter `",
	"Populate frontmatter `",
	"Add to frontmatter `",
	"frontmatter field `",
}

// backtickFieldRE extracts the first backtick-quoted token after a prefix.
var backtickFieldRE = regexp.MustCompile("`([^`]+)`")

func newValidateSkillCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "skill [name]",
		Short: "Check authoring skill frontmatter directives against schemas",
		Long: "Scan each authoring skill's SKILL.md for prescriptive frontmatter\n" +
			"directives and report any field that its target type's schema rejects.\n" +
			"Omit [name] to check all authoring skills.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targets := skillTypeTargets
			if len(args) == 1 {
				name := args[0]
				typ, ok := skillTypeTargets[name]
				if !ok {
					return fmt.Errorf("unknown authoring skill %q; known: %s",
						name, knownSkillNames())
				}
				targets = map[string]string{name: typ}
			}

			var drifts []*skillDrift
			for skillName, typeName := range targets {
				found, err := checkSkillAlignment(skillName, typeName)
				if err != nil {
					return err
				}
				drifts = append(drifts, found...)
			}

			if asJSON {
				b, _ := json.Marshal(driftJSON(drifts))
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
			} else {
				for _, d := range drifts {
					cmd.PrintErrln(d.String())
				}
				if len(drifts) == 0 {
					cmd.Println("skill alignment: ok")
				}
			}

			if len(drifts) > 0 {
				return ErrSchemaInvalid
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON array of drift findings")
	return cmd
}

// skillDrift records one prescriptive-field violation.
type skillDrift struct {
	Skill string
	Type  string
	Field string
}

func (d *skillDrift) String() string {
	return fmt.Sprintf("[%s] skill=%s type=%s field=%s",
		CodeSkillSchemaDrift, d.Skill, d.Type, d.Field)
}

// driftJSON converts drifts to the structured JSON shape used by --json.
func driftJSON(drifts []*skillDrift) []map[string]string {
	out := make([]map[string]string, 0, len(drifts))
	for _, d := range drifts {
		out = append(out, map[string]string{
			"code":  CodeSkillSchemaDrift,
			"skill": d.Skill,
			"type":  d.Type,
			"field": d.Field,
		})
	}
	return out
}

// checkSkillAlignment reads skillName's SKILL.md from the embedded bundle,
// loads the target type's schema properties, and returns one skillDrift per
// field prescribed by the skill but absent from the schema.
func checkSkillAlignment(skillName, typeName string) ([]*skillDrift, error) {
	body, err := skills.FS.ReadFile(skillName + "/SKILL.md")
	if err != nil {
		return nil, fmt.Errorf("read embedded skill %s: %w", skillName, err)
	}
	allowed, err := schemaProperties(typeName)
	if err != nil {
		return nil, err
	}

	var drifts []*skillDrift
	text := string(body)
	for _, prefix := range prescriptivePrefixes {
		idx := 0
		for {
			pos := strings.Index(text[idx:], prefix)
			if pos < 0 {
				break
			}
			pos += idx
			after := text[pos+len(prefix)-1:] // -1 to include the opening backtick
			m := backtickFieldRE.FindString(after)
			if m != "" {
				field := strings.Trim(m, "`")
				if _, ok := allowed[field]; !ok {
					drifts = append(drifts, &skillDrift{
						Skill: skillName,
						Type:  typeName,
						Field: field,
					})
				}
			}
			idx = pos + len(prefix)
		}
	}
	return drifts, nil
}

// schemaProperties returns the set of property names defined in the JSON
// Schema for typeName (e.g. "issue", "product-design").
func schemaProperties(typeName string) (map[string]struct{}, error) {
	filename := typeName + ".schema.json"
	raw, err := schemas.FS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read schema %s: %w", filename, err)
	}
	var s struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parse schema %s: %w", filename, err)
	}
	out := make(map[string]struct{}, len(s.Properties))
	for k := range s.Properties {
		out[k] = struct{}{}
	}
	return out, nil
}

// knownSkillNames returns a comma-separated list of authoring skill names for
// error messages.
func knownSkillNames() string {
	names := make([]string, 0, len(skillTypeTargets))
	for name := range skillTypeTargets {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}
