package core

import (
	"strings"
	"testing"
)

func TestValidateIssueLakehouseSchema_HardcodedSchema_Rejected(t *testing.T) {
	cases := []string{
		`uv run mentat lakehouse query --explore "SELECT count(*) FROM lakehouse.modelled.met_fundamentals"`,
		`uv run mentat lakehouse query --explore "SELECT * FROM lakehouse.prod.orders"`,
	}
	for _, line := range cases {
		body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\n" + line + "\n```\n\n## Links\n"
		errs := ValidateIssueLakehouseSchema(body)
		if len(errs) == 0 {
			t.Errorf("line %q: expected a lakehouse-schema error, got none", line)
			continue
		}
		if !strings.Contains(errs[0].Error(), "lakehouse schema") {
			t.Errorf("line %q: error %q does not mention 'lakehouse schema'", line, errs[0].Error())
		}
	}
}

func TestValidateIssueLakehouseSchema_Parameterised_Accepted(t *testing.T) {
	line := `uv run mentat lakehouse query --explore "SELECT count(*) FROM lakehouse.${SCHEMA:-modelled}.met_fundamentals"`
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\n" + line + "\n```\n\n## Links\n"
	if errs := ValidateIssueLakehouseSchema(body); len(errs) != 0 {
		t.Errorf("parameterised schema must be accepted, got: %v", errs)
	}
}

func TestValidateIssueLakehouseSchema_HardcodedInDirectOnly_Accepted(t *testing.T) {
	// Scoped to Indirect only — a Direct (in-tree) test doesn't carry the
	// forbidden-prod-apply failure mode this rule targets.
	line := `SELECT count(*) FROM lakehouse.prod.orders`
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\n" + line + "\n```\n\n### Indirect\n```bash\ntrue\n```\n\n## Links\n"
	if errs := ValidateIssueLakehouseSchema(body); len(errs) != 0 {
		t.Errorf("hardcoded schema in Direct only must be accepted, got: %v", errs)
	}
}

func TestLakehouseSchemaMatches_NoHeadingRequired(t *testing.T) {
	// anvil validate --verification-stdin feeds raw predicate text with no
	// "## Verification" wrapper — the matcher must not require one.
	if m := LakehouseSchemaMatches("true"); len(m) != 0 {
		t.Errorf("LakehouseSchemaMatches(%q) = %v, want none", "true", m)
	}
	if m := LakehouseSchemaMatches("SELECT * FROM lakehouse.modelled.foo"); len(m) != 1 {
		t.Errorf("LakehouseSchemaMatches with hardcoded schema = %v, want 1 match", m)
	}
	if m := LakehouseSchemaMatches("SELECT * FROM lakehouse.${SCHEMA:-modelled}.foo"); len(m) != 0 {
		t.Errorf("LakehouseSchemaMatches with parameterised schema = %v, want none", m)
	}
}
