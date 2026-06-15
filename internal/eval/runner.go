package eval

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chonalchendo/anvil/internal/build"
)

// Runner executes eval cases through an AgentAdapter and grades the result.
// Both the skill run and the judge call go through the same adapter — no API
// keys, all routed via the agent CLI subprocess.
type Runner struct {
	Adapter build.AgentAdapter
	Model   string
	Timeout time.Duration
}

// Result is one graded eval run. Reason is the judge's one-line verdict; it is
// surfaced live but not persisted (the eval_runs table stays lean).
type Result struct {
	Skill    string        `json:"skill"`
	EvalID   int           `json:"eval_id"`
	Name     string        `json:"name"`
	Pass     bool          `json:"pass"`
	Reason   string        `json:"reason"`
	Cost     float64       `json:"cost"`
	Duration time.Duration `json:"duration"`
	Model    string        `json:"model"`
	Date     string        `json:"date"`
}

// RunCase runs one eval end-to-end: seed a fresh fixture dir, spawn the skill
// under test, then judge the outcome off the filesystem side-effects and the
// agent's diagnostic. Isolation is Cwd-scoped (a per-case temp dir) rather than
// CLAUDE_CONFIG_DIR — the thinnest cut given RunRequest carries no env override.
// ErrQuotaExhausted (from either the skill run or the judge) propagates so the
// caller can stop; any other spawn error is graded as a fail so one bad case
// does not abort the suite.
func (r *Runner) RunCase(ctx context.Context, skill string, c Case) (Result, error) {
	res := Result{Skill: skill, EvalID: c.ID, Name: c.Name, Model: r.Model, Date: time.Now().UTC().Format(time.RFC3339)}

	cwd, err := os.MkdirTemp("", "anvil-eval-")
	if err != nil {
		return res, fmt.Errorf("creating fixture dir: %w", err)
	}
	defer os.RemoveAll(cwd) //nolint:errcheck // temp cleanup; error not actionable
	if err := seedFiles(cwd, c.Files); err != nil {
		return res, err
	}

	runRes, runErr := r.Adapter.Run(ctx, build.RunRequest{
		Model:       r.Model,
		Instruction: c.Prompt,
		Skills:      []string{skill},
		Cwd:         cwd,
		Timeout:     r.Timeout,
	})
	res.Duration = runRes.Duration
	res.Cost = runRes.CostUSD
	if runErr != nil {
		if errors.Is(runErr, build.ErrQuotaExhausted) {
			return res, runErr
		}
		res.Reason = "agent run errored: " + runErr.Error()
		return res, nil
	}

	pass, reason, judgeCost, err := r.judge(ctx, c, cwd, runRes.Diagnostic)
	res.Cost += judgeCost // billed even when the judge spawn then errors
	if err != nil {
		if errors.Is(err, build.ErrQuotaExhausted) {
			return res, err
		}
		res.Reason = "judge spawn errored: " + err.Error()
		return res, nil
	}
	res.Pass, res.Reason = pass, reason
	return res, nil
}

// judge runs a second adapter spawn — no skills loaded — that grades the run
// off the side-effects (the fixture file tree) plus the agent's diagnostic, and
// returns its verdict. A verdict that does not start with PASS is treated as a
// fail, so an unparseable judge response never silently passes.
func (r *Runner) judge(ctx context.Context, c Case, cwd, diagnostic string) (bool, string, float64, error) {
	tree := fileTree(cwd)
	res, err := r.Adapter.Run(ctx, build.RunRequest{
		Model:       r.Model,
		Instruction: judgePrompt(c, diagnostic, tree),
		Timeout:     r.Timeout,
	})
	if err != nil {
		return false, "", res.CostUSD, fmt.Errorf("judge spawn: %w", err)
	}
	verdict := strings.TrimSpace(res.Diagnostic)
	pass := strings.HasPrefix(strings.ToUpper(verdict), "PASS")
	reason := verdict
	if i := strings.IndexByte(verdict, '\n'); i >= 0 {
		reason = strings.TrimSpace(verdict[:i])
	}
	return pass, reason, res.CostUSD, nil
}

func judgePrompt(c Case, diagnostic, tree string) string {
	var b strings.Builder
	b.WriteString("You are grading whether an AI agent's behaviour satisfies an evaluation case. ")
	b.WriteString("Be strict and decide from the evidence below only.\n\n")
	b.WriteString("## What the agent was asked\n")
	b.WriteString(c.Prompt)
	b.WriteString("\n\n## Expected behaviour\n")
	b.WriteString(c.ExpectedOutput)
	for _, e := range c.Expectations {
		b.WriteString("\n- ")
		b.WriteString(e)
	}
	b.WriteString("\n\n## The agent's final response\n")
	if strings.TrimSpace(diagnostic) == "" {
		b.WriteString("(no response captured)")
	} else {
		b.WriteString(diagnostic)
	}
	b.WriteString("\n\n## Files the agent created or modified in its working directory\n")
	b.WriteString(tree)
	b.WriteString("\n\nMany cases are NEGATIVE assertions — the agent should have refused, redirected, ")
	b.WriteString("or produced no file; for those an empty file list is the pass condition.\n\n")
	b.WriteString("Reply with a single line: PASS or FAIL, then a one-sentence reason.")
	return b.String()
}

// seedFiles writes each fixture file under root, rejecting paths that escape it.
func seedFiles(root string, files []FileSeed) error {
	for _, f := range files {
		dest := filepath.Join(root, f.Path)
		if !strings.HasPrefix(dest, root+string(os.PathSeparator)) {
			return fmt.Errorf("fixture file %q escapes the working directory", f.Path)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil { //nolint:gosec // 0755 is correct for traversable fixture dirs
			return fmt.Errorf("seeding %s: %w", f.Path, err)
		}
		if err := os.WriteFile(dest, []byte(f.Content), 0o644); err != nil { //nolint:gosec // 0644 is correct for fixture data files
			return fmt.Errorf("writing %s: %w", f.Path, err)
		}
	}
	return nil
}

// fileTree lists the relative paths of every file under root, sorted, as the
// observable side-effect surface the judge grades against. Returns "(none)"
// when the agent left the directory empty.
func fileTree(root string) string {
	var paths []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // a walk error on one entry should not abort grading
		}
		if rel, err := filepath.Rel(root, p); err == nil {
			paths = append(paths, rel)
		}
		return nil
	})
	if len(paths) == 0 {
		return "(none)"
	}
	sort.Strings(paths)
	return strings.Join(paths, "\n")
}
