package core

import (
	"fmt"
	"regexp"
	"strings"
)

// lakehouseSchemaRE matches an unparameterised `lakehouse.<schema>.` table
// reference: `lakehouse.` followed directly by a bare identifier and a dot.
// The established fix is `${SCHEMA:-modelled}` parameterisation (mentat.0291
// precedent, now on eight issues) — `${` after `lakehouse.` starts with `$`,
// not an identifier character, so a parameterised reference never matches.
var lakehouseSchemaRE = regexp.MustCompile(`\blakehouse\.[A-Za-z_][A-Za-z0-9_]*\.`)

// heredocOpenRE matches a here-doc redirect opener (`<<`, optional `-`, a
// bare or quoted delimiter) so stripWrittenPayloads can skip everything up to
// the matching close line.
var heredocOpenRE = regexp.MustCompile(`<<-?\s*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)

// writeLiteralRE matches a printf/echo invocation that redirects its literal
// argument to a file (`printf -- '...' > "$F"`). Such a line assembles data,
// it does not execute it — a lakehouse-schema string inside the quoted
// payload is being written into a throwaway fixture, not queried.
var writeLiteralRE = regexp.MustCompile(`^\s*(?:printf|echo)\b.*>{1,2}\s*\S`)

// stripWrittenPayloads drops, from text, the shell constructs that write
// literal data rather than execute a command: a printf/echo line redirected
// to a file, and the body of a here-doc between its opener and closing
// delimiter. Without this, a predicate that assembles a fixture file
// containing a lakehouse-schema string (anvil.0241's own Indirect block does
// exactly this, to self-test the rule) is indistinguishable from one that
// queries that schema directly.
func stripWrittenPayloads(text string) string {
	var out strings.Builder
	heredocDelim := ""
	for _, line := range strings.Split(text, "\n") {
		if heredocDelim != "" {
			if strings.TrimSpace(line) == heredocDelim {
				heredocDelim = ""
			}
			continue
		}
		if m := heredocOpenRE.FindStringSubmatch(line); m != nil {
			heredocDelim = m[1]
			continue
		}
		if writeLiteralRE.MatchString(line) {
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}

// LakehouseSchemaMatches returns every hardcoded lakehouse.<schema>. reference
// found in text, one per matching line, in encounter order. A prod-hardcoded
// schema (mentat.0250/0252/0253's `lakehouse.prod.`) is legitimately red at
// authoring time and only bites at completion — after dispatch, review and
// responder rounds have already run — because the worker is forbidden the prod
// apply the predicate demands. Shell comments are dropped (prose, not a
// command), a per-worker `__dev` schema is exempt (that's exactly the schema
// a worker without prod-apply rights CAN satisfy), and text a command writes
// rather than executes is excluded via stripWrittenPayloads.
func LakehouseSchemaMatches(text string) []string {
	var out []string
	for _, line := range strings.Split(stripWrittenPayloads(text), "\n") {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}
		m := lakehouseSchemaRE.FindString(line)
		if m == "" || strings.Contains(m, "__dev") {
			continue
		}
		out = append(out, m)
	}
	return out
}

// ValidateIssueLakehouseSchema reports one error per hardcoded lakehouse
// schema reference found in the issue's Indirect Verification block(s). Scoped
// to Indirect only: a hardcoded schema in Direct (an in-tree unit/integration
// test) does not carry the same forbidden-prod-apply failure mode this rule
// targets. An unterminated fence is already reported by ValidateIssue's
// fence-balance check, so this returns no errors for that case rather than
// duplicating it.
func ValidateIssueLakehouseSchema(body string) []error {
	blocks, err := VerificationBlocks(body, "Indirect")
	if err != nil {
		return nil
	}
	var errs []error
	for _, block := range blocks {
		for _, m := range LakehouseSchemaMatches(block) {
			layer := strings.TrimSuffix(strings.TrimPrefix(m, "lakehouse."), ".")
			errs = append(errs, fmt.Errorf("indirect verification block hardcodes a lakehouse schema (%q) — parameterise the schema (e.g. ${SCHEMA:-%s}) so a worker without prod-apply rights can satisfy it", m, layer))
		}
	}
	return errs
}
