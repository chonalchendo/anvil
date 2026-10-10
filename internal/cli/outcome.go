package cli

import (
	"fmt"

	"github.com/chonalchendo/anvil/internal/core"
)

// bumpOutcome adds one to an outcome counter. An absent counter reads as 0, so
// nothing is written until the first event.
func bumpOutcome(a *core.Artifact, key string) {
	n, _ := a.FrontMatter[key].(int)
	a.FrontMatter[key] = n + 1
}

// issueOutcome is an issue's four outcome fields; nil when unset. prLinks feeds the
// one-PR sum and stays out of the JSON.
type issueOutcome struct {
	Escalations  *int    `json:"outcome_escalations"`
	Reopens      *int    `json:"outcome_reopens"`
	Rescopes     *int    `json:"outcome_rescopes"`
	FirstVerdict *string `json:"outcome_first_verdict"`
	prLinks      int
}

func issueOutcomeFrom(fm map[string]any) issueOutcome {
	var o issueOutcome
	for k, dst := range map[string]**int{
		"outcome_escalations": &o.Escalations,
		"outcome_reopens":     &o.Reopens,
		"outcome_rescopes":    &o.Rescopes,
	} {
		if n, ok := fm[k].(int); ok {
			*dst = &n
		}
	}
	if s, ok := fm["outcome_first_verdict"].(string); ok {
		o.FirstVerdict = &s
	}
	links, _ := fm["external_links"].([]any)
	for _, raw := range links {
		if url, ok := raw.(string); ok && prURLNumber.MatchString(url) {
			o.prLinks++
		}
	}
	return o
}

// milestoneOutcome sums the issue counters; Issues excludes abandoned ones.
type milestoneOutcome struct {
	Issues         int `json:"issues"`
	OnePRNoRescope int `json:"one_pr_no_rescope"`
	FirstPass      int `json:"first_pass"`
	Escalations    int `json:"escalations"`
	Reopens        int `json:"reopens"`
	Rescopes       int `json:"rescopes"`
	Amendments     int `json:"amendments"`
}

func sumOutcome(rows []milestoneIssueRow, ms map[string]any) milestoneOutcome {
	var o milestoneOutcome
	o.Amendments, _ = ms["outcome_amendments"].(int)
	for _, r := range rows {
		if r.Status == "abandoned" {
			continue
		}
		o.Issues++
		if r.FirstVerdict != nil && *r.FirstVerdict == "pass" {
			o.FirstPass++
		}
		if r.Status == "resolved" && r.prLinks <= 1 && (r.Rescopes == nil || *r.Rescopes == 0) {
			o.OnePRNoRescope++
		}
		for _, p := range []struct {
			src *int
			dst *int
		}{{r.Escalations, &o.Escalations}, {r.Reopens, &o.Reopens}, {r.Rescopes, &o.Rescopes}} {
			if p.src != nil {
				*p.dst += *p.src
			}
		}
	}
	return o
}

func (o milestoneOutcome) line() string {
	return fmt.Sprintf("Outcome: %d/%d one-PR-no-rescope, %d first-pass, %d escalations, %d reopens, %d rescopes, %d amendments",
		o.OnePRNoRescope, o.Issues, o.FirstPass, o.Escalations, o.Reopens, o.Rescopes, o.Amendments)
}
