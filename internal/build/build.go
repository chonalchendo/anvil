package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/chonalchendo/anvil/internal/core"
)

// Sentinel errors returned by Build. cmd/anvil/main.go maps these to exit
// codes 1 / 2 / 130 per spec §4.
var (
	ErrBuildTaskFailed     = errors.New("build: task failed")
	ErrBuildQuotaExhausted = errors.New("build: quota exhausted")
	ErrBuildCancelled      = errors.New("build: cancelled")
)

const (
	defaultModel      = "claude-sonnet-4-6"
	defaultEffort     = "medium"
	defaultRunTimeout = 30 * time.Minute
)

// Options drives Build. Cwd is the agent working directory; Router maps
// model-name prefixes to adapter implementations; Concurrency caps in-flight
// tasks per wave.
type Options struct {
	Concurrency int
	Cwd         string
	DryRun      bool
	JSON        bool
	Stdout      io.Writer
	Stderr      io.Writer
	Router      Router
}

// Summary is what Build returns on success or any non-cancellation exit.
type Summary struct {
	Outcomes map[string]TaskOutcome
	Wall     time.Duration
}

// TaskOutcome is the per-task record held in memory. Sub-project 3 will
// persist this shape to SQLite via internal/telemetry.
type TaskOutcome struct {
	TaskID   string
	Wave     int
	Model    string
	Effort   string
	Outcome  string // "success" | "failed" | "quota_exhausted" | "cancelled" | "skipped_dry_run"
	Duration time.Duration
	Result   RunResult
	Err      error
}

// jsonRecord is the per-task line emitted to stdout in --json mode.
type jsonRecord struct {
	TaskID     string `json:"task_id"`
	Wave       int    `json:"wave"`
	Model      string `json:"model"`
	Effort     string `json:"effort"`
	Outcome    string `json:"outcome,omitempty"`
	Status     string `json:"status,omitempty"` // "skipped_dry_run" — distinct from outcome enum
	DurationMS int64  `json:"duration_ms"`
	Diagnostic string `json:"diagnostic,omitempty"`
}

// Build walks plan.Waves(), dispatching each task through a routed adapter
// under an errgroup with concurrency limit. Wave-complete-then-stop: all
// in-flight tasks finish before the loop aborts. Cancellation flows from the
// parent ctx, never from sibling failures.
func Build(ctx context.Context, p *core.Plan, opts Options) (*Summary, error) {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}

	waves, err := p.Waves()
	if err != nil {
		return nil, err
	}

	start := time.Now()
	sum := &Summary{Outcomes: map[string]TaskOutcome{}}
	var mu sync.Mutex

	for w, wave := range waves {
		if err := ctx.Err(); err != nil {
			return sum, fmt.Errorf("%w: context cancelled before wave %d", ErrBuildCancelled, w)
		}

		g := new(errgroup.Group)
		g.SetLimit(opts.Concurrency)

		for _, idx := range wave {
			task := p.Tasks[idx]
			waveNum := w
			g.Go(func() error {
				oc := dispatchTask(ctx, task, waveNum, opts)
				mu.Lock()
				sum.Outcomes[task.ID] = oc
				mu.Unlock()
				emitJSONRecord(opts, oc)
				return nil // never error → no auto-cancel of siblings
			})
		}
		_ = g.Wait()

		// Classify wave: scan our own outcomes, not the errgroup's err.
		quotaHit, anyFail := false, false
		var firstErr error
		for _, idx := range wave {
			oc := sum.Outcomes[p.Tasks[idx].ID]
			switch oc.Outcome {
			case "quota_exhausted":
				quotaHit, anyFail = true, true
				if firstErr == nil {
					firstErr = oc.Err
				}
			case "failed", "cancelled":
				anyFail = true
				if firstErr == nil {
					firstErr = oc.Err
				}
			}
		}
		if anyFail {
			sum.Wall = time.Since(start)
			// quota wins over cancel: resumption signal is more actionable.
			switch {
			case quotaHit:
				return sum, wrapSentinel(ErrBuildQuotaExhausted, firstErr)
			case ctx.Err() != nil:
				return sum, wrapSentinel(ErrBuildCancelled, firstErr)
			default:
				return sum, wrapSentinel(ErrBuildTaskFailed, firstErr)
			}
		}
	}

	sum.Wall = time.Since(start)
	return sum, nil
}

// dispatchTask runs a single task, classifies the outcome, and returns the
// TaskOutcome record. Never panics; never blocks indefinitely (the adapter
// honours its own Timeout).
func dispatchTask(ctx context.Context, t core.Task, wave int, opts Options) TaskOutcome {
	model := t.Model
	if model == "" {
		model = defaultModel
	}
	effort := t.Effort
	if effort == "" {
		effort = defaultEffort
	}
	oc := TaskOutcome{
		TaskID: t.ID, Wave: wave, Model: model, Effort: effort,
	}

	if opts.DryRun {
		oc.Outcome = "skipped_dry_run"
		return oc
	}

	adapter, err := selectAdapter(opts.Router, model)
	if err != nil {
		oc.Outcome = "failed"
		oc.Err = err
		return oc
	}

	req := RunRequest{
		Model:       model,
		Effort:      effort,
		Instruction: assembleInstruction(t),
		Skills:      t.SkillsToLoad,
		Context:     t.ContextToLoad,
		Files:       t.Files,
		Cwd:         opts.Cwd,
		Timeout:     defaultRunTimeout,
	}
	res, err := adapter.Run(ctx, req)
	oc.Result = res
	oc.Duration = res.Duration
	oc.Outcome = classify(ctx, res, err)
	oc.Err = err
	if oc.Outcome != "success" && oc.Outcome != "skipped_dry_run" && oc.Result.Diagnostic != "" {
		fmt.Fprintf(opts.Stderr, "task %s [%s]: %s\n", oc.TaskID, oc.Outcome, oc.Result.Diagnostic)
	}
	return oc
}

func classify(ctx context.Context, res RunResult, err error) string {
	switch {
	case errors.Is(err, ErrQuotaExhausted):
		return "quota_exhausted"
	case errors.Is(ctx.Err(), context.Canceled), errors.Is(err, context.Canceled):
		return "cancelled"
	case err != nil:
		return "failed"
	case res.ExitCode != 0:
		return "failed"
	default:
		return "success"
	}
}

// selectAdapter picks the longest-prefix match in router for model. Empty
// router or no match returns a typed error so build records "failed" with a
// useful message.
func selectAdapter(r Router, model string) (AgentAdapter, error) {
	keys := make([]string, 0, len(r))
	for k := range r {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, k := range keys {
		if strings.HasPrefix(model, k) {
			return r[k], nil
		}
	}
	return nil, fmt.Errorf("no adapter for model %q; registered prefixes: %v", model, keys)
}

// wrapSentinel keeps errors.Is(err, sentinel) true while surfacing the
// underlying task error message (e.g. "no adapter for model …") to callers
// that print err.Error().
func wrapSentinel(sentinel, cause error) error {
	if cause == nil {
		return sentinel
	}
	return fmt.Errorf("%w: %v", sentinel, cause)
}

func emitJSONRecord(opts Options, oc TaskOutcome) {
	if !opts.JSON {
		return
	}
	rec := jsonRecord{
		TaskID:     oc.TaskID,
		Wave:       oc.Wave,
		Model:      oc.Model,
		Effort:     oc.Effort,
		DurationMS: oc.Duration.Milliseconds(),
		Diagnostic: oc.Result.Diagnostic,
	}
	if oc.Outcome == "skipped_dry_run" {
		rec.Status = oc.Outcome
	} else {
		rec.Outcome = oc.Outcome
	}
	_ = json.NewEncoder(opts.Stdout).Encode(rec)
}
