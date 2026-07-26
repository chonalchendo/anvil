package cli

import (
	"fmt"
	"strings"

	"github.com/chonalchendo/anvil/internal/build"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/index"
)

// readyUnitsToTasks maps the priority-ordered ready frontier to dispatch tasks.
// Each unit becomes a completing-issue task whose body carries the assembled
// start-context (goal, severity, milestone, governing contracts, path) — the
// same context `anvil next` hands an interactive agent, so a dispatched agent
// starts from the unit-with-context rather than a bare id. Milestone and
// contracts lines are omitted when empty so the body carries no blank scaffolding.
func readyUnitsToTasks(units []readyUnit) []core.Task {
	tasks := make([]core.Task, 0, len(units))
	for _, u := range units {
		var b strings.Builder
		fmt.Fprintf(&b, "Complete anvil issue %s end-to-end to a verified tree using the completing-issue skill; the driver lands it. The human owns the merge.\n\n", u.ID)
		fmt.Fprintf(&b, "Goal: %s\n", u.Goal)
		fmt.Fprintf(&b, "Severity: %s\n", u.Severity)
		if u.Milestone != "" {
			fmt.Fprintf(&b, "Milestone: %s\n", u.Milestone)
		}
		if len(u.Contracts) > 0 {
			fmt.Fprintf(&b, "Governing contracts: %s\n", strings.Join(u.Contracts, ", "))
		}
		fmt.Fprintf(&b, "Issue path: %s\n", u.Path)

		tasks = append(tasks, core.Task{
			ID:           u.ID,
			SkillsToLoad: []string{"completing-issue"},
			Body:         b.String(),
		})
	}
	return tasks
}

// injectHydratedContext folds each dispatch task's spine-closure bodies into its
// prompt — the milestone objectives, its design bodies, the issue's
// contracts→conventions, and prior learnings — the same box `anvil hydrate`
// opens for an interactive agent in completing-issue Phase 1. Without it, the
// headless worker starts from the bare identifiers readyUnitsToTasks carries and
// the milestone→designs edge never opens (anvil.0154). The driver owns the vault
// read (build-orchestration-contract); the engine never sees it. A broken spine
// edge does not abort the build — the worker gets what resolved, tagged
// structurally unhydratable so it treats the box as partial. A load error
// anywhere on the spine (the issue or a linked node) skips the fold — the task
// keeps readyUnitsToTasks' bare start-context, which the worker re-hydrates at
// completion.
func injectHydratedContext(v *core.Vault, tasks []core.Task) {
	for i := range tasks {
		h, err := assembleHydration(v, tasks[i].ID)
		if err != nil {
			continue
		}
		var b strings.Builder
		for _, n := range h.nodes {
			b.WriteString(closureHeader(n))
			b.WriteByte('\n')
			body, _, clipped := clipBody(n.Body)
			if clipped {
				body += "\n… (body clipped)"
			}
			b.WriteString(body)
			b.WriteByte('\n')
		}
		tasks[i].Body += "\n## Hydrated context\n" +
			"The issue's assembled spine closure — the box `anvil hydrate` opens. Weigh it as the grounding for this change.\n\n" +
			b.String()
		if len(h.broken) > 0 {
			tasks[i].Body += "\nThe box is structurally unhydratable at these edges — treat the closure as partial:\n" +
				brokenSpineError(h.broken).Error() + "\n"
		}
	}
}

// learningInjectionLimit caps how many related learnings ride into one complete
// task body — the load-bearing few the researcher would surface, not every facet
// hit, so the spawn's context stays lean.
const learningInjectionLimit = 5

// injectLearnings folds each complete task's most-related vault learnings into
// its body, so a headless worker — which cannot sub-dispatch the
// anvil-learnings-researcher — still starts from what the vault knows. The driver
// owns the vault read (build-orchestration-contract) and queries the same
// relatedness index `anvil index --type learning` uses (RelatedByID). Learnings
// are deliberately not project-scoped: the index ranks by shared facets, so a
// cross-cutting learning surfaces by relevance regardless of project — matching
// the researcher's own unscoped query. Best-effort: a query error or unloadable
// learning is skipped, never fatal — missing learnings must not abort a build.
func injectLearnings(db *index.DB, tasks []core.Task) {
	for i := range tasks {
		rows, err := db.RelatedByID(tasks[i].ID, index.QueryFilters{})
		if err != nil {
			continue
		}
		var b strings.Builder
		n := 0
		for _, r := range rows {
			// Skip only retracted (known-false) learnings; stale and draft still
			// ride along under the weigh-against-present-code disclaimer below.
			if r.Type != string(core.TypeLearning) || r.Status == "retracted" {
				continue
			}
			a, err := core.LoadArtifact(r.Path)
			if err != nil {
				continue
			}
			title, _ := a.FrontMatter["title"].(string)
			conf, _ := a.FrontMatter["confidence"].(string)
			updated, _ := a.FrontMatter["updated"].(string)
			// The schema requires a "## TL;DR"; collapse it to one line. Absence
			// means a malformed learning — surfaced as a bare title, not a failure.
			tldr := ""
			if k := strings.Index(a.Body, "## TL;DR"); k >= 0 {
				rest := a.Body[k+len("## TL;DR"):]
				if j := strings.Index(rest, "\n## "); j >= 0 {
					rest = rest[:j]
				}
				tldr = strings.Join(strings.Fields(rest), " ")
			}
			fmt.Fprintf(&b, "\n- %s · confidence:%s · updated:%s\n  %s\n  Source: %s\n",
				title, conf, updated, tldr, r.ID)
			if n++; n >= learningInjectionLimit {
				break
			}
		}
		if n == 0 {
			continue
		}
		tasks[i].Body += "\n## Prior learnings\n" +
			"Vault learnings related to this issue (relevance-ranked). Each was true when written; weigh it against the present code before acting.\n" +
			b.String()
	}
}

// reviewTasksFromTasks builds the review-phase wave: one reviewing-pr task per
// complete-phase task the advance-gate passed (outcome "success" → a verified PR
// exists on its branch). Each review task reuses the issue's cut worktree and
// branch so the reviewing-pr skill discovers the PR via `gh pr list --head
// <branch>` — the engine threads no data between the complete and review spawns;
// they decouple through gh state (build-orchestration-contract). A task the gate
// failed gets no review: there is no PR to review.
func reviewTasksFromTasks(completeTasks []core.Task, sum *build.Summary) []core.Task {
	reviews := make([]core.Task, 0, len(completeTasks))
	for _, t := range completeTasks {
		if sum.Outcomes[t.ID].Outcome != "success" {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Review the open PR for anvil issue %s using the reviewing-pr skill, then record the structured review verdict. The human owns the merge.\n\n", t.ID)
		fmt.Fprintf(&b, "Find the PR on its branch: gh pr list --head %s --state open.\n", t.Branch)
		fmt.Fprintf(&b, "Issue branch: %s\n", t.Branch)

		reviews = append(reviews, core.Task{
			ID:           t.ID,
			SkillsToLoad: []string{"reviewing-pr"},
			Body:         b.String(),
			Cwd:          t.Cwd,
			Branch:       t.Branch,
			// The review box judges a diff; the wall makes "cannot mutate the code
			// it reviews" a harness guarantee, not a prompt hope (anvil.0117).
			// complete and respond keep the full set — both must edit code.
			DisallowedTools: []string{"Edit", "Write"},
		})
	}
	return reviews
}

// respondTasksFromTasks builds the respond-phase wave: one responding-to-pr-review
// task per review-phase task that ran (outcome "success"). Each respond task reuses
// the issue's cut worktree and branch so the responding-to-pr-review skill discovers
// the PR and its review findings via `gh pr list --head <branch>` — the engine
// threads no data between the review and respond spawns; they decouple through gh
// state (build-orchestration-contract). The spawn drives each finding to an outcome
// (fix / skip-with-reason / push-back) and CI to green, then halts review-green
// awaiting the human merge. A review task that did not succeed gets no respond task.
func respondTasksFromTasks(reviewTasks []core.Task, sum *build.Summary) []core.Task {
	responds := make([]core.Task, 0, len(reviewTasks))
	for _, t := range reviewTasks {
		if sum.Outcomes[t.ID].Outcome != "success" {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Address the review findings on the open PR for anvil issue %s using the responding-to-pr-review skill, driving each finding to an outcome (fix / skip-with-reason / push-back) and CI to green. Halt at review-green; the human owns the merge.\n\n", t.ID)
		fmt.Fprintf(&b, "Find the PR on its branch: gh pr list --head %s --state open.\n", t.Branch)
		fmt.Fprintf(&b, "Issue branch: %s\n", t.Branch)

		responds = append(responds, core.Task{
			ID:           t.ID,
			SkillsToLoad: []string{"responding-to-pr-review"},
			Body:         b.String(),
			Cwd:          t.Cwd,
			Branch:       t.Branch,
		})
	}
	return responds
}
