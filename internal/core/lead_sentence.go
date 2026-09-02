package core

import (
	"fmt"
	"regexp"
	"strings"
)

// LeadSentenceMaxWords is the word budget writing-issue and writing-milestone
// hold the lead sentence of `## Problem` (issue) / `## Objective` (milestone)
// to. Both skills prescribe it in prose; ValidateLeadSentence is the
// deterministic check that prose alone doesn't hold under context pressure.
const LeadSentenceMaxWords = 25

// sentenceEndRE splits on a sentence terminator followed by whitespace.
// Under-counts on abbreviations ("e.g. foo") — accepted per the issue's
// direction: a hard refusal on this heuristic would teach agents to work
// around it, so the finding stays warning-level regardless.
var sentenceEndRE = regexp.MustCompile(`[.?!]\s`)

// ValidateLeadSentence reports a warning when the first sentence after
// `## <heading>` is over LeadSentenceMaxWords words or contains a backtick.
// The scan stops at the section's first fenced code block, so a fence quoted
// early in the section (e.g. a Problem citing a code sample) never leaks into
// the sentence text. Returns nil when the heading is absent or its section is
// empty — RequiredIssueSections/RequiredMilestoneSections own presence.
func ValidateLeadSentence(body, heading string) []error {
	section := Section(body, heading)
	if section == "" {
		return nil
	}
	if i := strings.Index(section, "```"); i >= 0 {
		section = section[:i]
	}
	section = strings.TrimSpace(section)
	if section == "" {
		return nil
	}

	sentence := section
	if loc := sentenceEndRE.FindStringIndex(section); loc != nil {
		sentence = section[:loc[0]+1]
	}

	words := strings.Fields(sentence)
	overLength := len(words) > LeadSentenceMaxWords
	hasBacktick := strings.Contains(sentence, "`")
	if !overLength && !hasBacktick {
		return nil
	}

	var reasons []string
	if overLength {
		reasons = append(reasons, fmt.Sprintf("%d words (max %d)", len(words), LeadSentenceMaxWords))
	}
	if hasBacktick {
		reasons = append(reasons, "contains a code span")
	}
	return []error{fmt.Errorf(
		"## %s lead sentence is %s — plain words, no identifiers, for a cold-reading triager: %q",
		heading, strings.Join(reasons, "; "), truncateForError(sentence, 120),
	)}
}

// truncateForError bounds the quoted sentence in ValidateLeadSentence's
// message so a genuinely long lead sentence doesn't blow out the finding.
func truncateForError(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "…"
}
