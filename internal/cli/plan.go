package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

// runShowPlan handles `anvil show plan <id> --validate|--waves`.
// Returns typed errors (core.ErrPlanDAG, core.ErrPlanTDD, ErrSchemaInvalid,
// ErrArtifactNotFound) so cmd/anvil/main.go can map to spec exit codes.
func runShowPlan(cmd *cobra.Command, v *core.Vault, id string, validate, waves bool) error {
	path := filepath.Join(v.Root, core.TypePlan.Dir(), id+".md")

	a, err := core.LoadArtifact(path)
	if err != nil {
		return ErrArtifactNotFound
	}
	if err := schema.Validate("plan", a.FrontMatter); err != nil {
		cmd.PrintErrln(err)
		return fmt.Errorf("%w: %w", ErrSchemaInvalid, err)
	}

	p, err := core.LoadPlan(path)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSchemaInvalid, err)
	}

	if validate {
		if err := core.ValidatePlan(p); err != nil {
			cmd.PrintErrln(err)
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "ok")
		return nil
	}

	if waves {
		ws, err := p.Waves()
		if err != nil {
			cmd.PrintErrln(err)
			return err
		}
		renderWaves(cmd, p, ws)
		return nil
	}
	return nil
}

func renderWaves(cmd *cobra.Command, p *core.Plan, waves [][]int) {
	var b strings.Builder
	b.WriteString("```mermaid\n")
	b.WriteString("flowchart TD\n")
	for w, wave := range waves {
		fmt.Fprintf(&b, "    subgraph wave%d[\"Wave %d\"]\n", w, w)
		for _, idx := range wave {
			t := p.Tasks[idx]
			fmt.Fprintf(&b, "        %s[\"%s: %s\"]\n", t.ID, t.ID, escapeMermaid(t.Title))
		}
		b.WriteString("    end\n")
	}
	for _, t := range p.Tasks {
		for _, dep := range t.DependsOn {
			fmt.Fprintf(&b, "    %s --> %s\n", dep, t.ID)
		}
	}
	b.WriteString("```\n")
	fmt.Fprint(cmd.OutOrStdout(), b.String())
}

func escapeMermaid(s string) string {
	return strings.ReplaceAll(s, `"`, `'`)
}

type taskView struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Kind            string   `json:"kind,omitempty"`
	Model           string   `json:"model,omitempty"`
	Effort          string   `json:"effort,omitempty"`
	Files           []string `json:"files,omitempty"`
	DependsOn       []string `json:"depends_on,omitempty"`
	SkillsToLoad    []string `json:"skills_to_load,omitempty"`
	ContextToLoad   []string `json:"context_to_load,omitempty"`
	Verify          string   `json:"verify,omitempty"`
	SuccessCriteria []string `json:"success_criteria,omitempty"`
}

func newTaskView(t *core.Task) taskView {
	return taskView{
		ID: t.ID, Title: t.Title, Kind: t.Kind, Model: t.Model, Effort: t.Effort,
		Files: t.Files, DependsOn: t.DependsOn,
		SkillsToLoad: t.SkillsToLoad, ContextToLoad: t.ContextToLoad,
		Verify: t.Verify, SuccessCriteria: t.SuccessCriteria,
	}
}

type planTaskOutput struct {
	PlanID string   `json:"plan_id"`
	Path   string   `json:"path"`
	Task   taskView `json:"task"`
	Body   *string  `json:"body,omitempty"`
}

// runShowPlanTask handles `anvil show plan <id> --task <task-id>`.
// Without --body, emits the task's frontmatter fields. With --body, also emits
// the corresponding "## Task: <id>" section.
func runShowPlanTask(cmd *cobra.Command, v *core.Vault, id, taskID string, asJSON, includeBody bool) error {
	path := filepath.Join(v.Root, core.TypePlan.Dir(), id+".md")
	p, err := core.LoadPlan(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrArtifactNotFound
		}
		return fmt.Errorf("loading plan: %w", err)
	}

	var task *core.Task
	known := make([]string, 0, len(p.Tasks))
	for i := range p.Tasks {
		known = append(known, p.Tasks[i].ID)
		if p.Tasks[i].ID == taskID {
			task = &p.Tasks[i]
		}
	}
	if task == nil {
		return errfmt.NewStructured("task_not_found").
			Set("plan", id).
			Set("task", taskID).
			Set("known", known).
			Set("hint", "list tasks via `anvil show plan "+id+"` (frontmatter shows tasks[].id)")
	}

	out := planTaskOutput{PlanID: id, Path: p.Path, Task: newTaskView(task)}
	if includeBody {
		body := task.Body
		out.Body = &body
	}

	w := cmd.OutOrStdout()
	if asJSON {
		b, _ := json.Marshal(out)
		fmt.Fprintln(w, string(b))
		return nil
	}

	emitTaskText(cmd, task, includeBody)
	return nil
}

func emitTaskText(cmd *cobra.Command, t *core.Task, includeBody bool) {
	w := cmd.OutOrStdout()
	enc, _ := json.MarshalIndent(newTaskView(t), "", "  ")
	fmt.Fprintln(w, "---")
	fmt.Fprintln(w, string(enc))
	fmt.Fprintln(w, "---")
	if includeBody {
		body := strings.TrimPrefix(t.Body, "## Task: "+t.ID)
		body = strings.TrimLeft(body, "\n")
		fmt.Fprintln(w, "## Task:", t.ID)
		fmt.Fprintln(w)
		fmt.Fprint(w, body)
	}
}
