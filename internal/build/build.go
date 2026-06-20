package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// TaskOutcome is the per-task record held in memory. The driver projects it
// onto vault.db build_tasks rows after a run; see build-orchestration-contract.
type TaskOutcome struct {
	TaskID    string
	Wave      int
	Model     string
	Effort    string
	Outcome   string // "success" | "failed" | "quota_exhausted" | "cancelled" | "skipped_dry_run"
	Duration  time.Duration
	Result    RunResult
	Err       error
	ConfigDir string // per-spawn CLAUDE_CONFIG_DIR; the planned path on dry-run
}

// jsonRecord is the per-task line emitted to stdout in --json mode.
type jsonRecord struct {
	TaskID      string      `json:"task_id"`
	Wave        int         `json:"wave"`
	Model       string      `json:"model"`
	Effort      string      `json:"effort"`
	Outcome     string      `json:"outcome,omitempty"`
	Status      string      `json:"status,omitempty"` // "skipped_dry_run" — distinct from outcome enum
	DurationMS  int64       `json:"duration_ms"`
	AgentTimeMS int64       `json:"agent_time_ms,omitempty"`
	CostUSD     float64     `json:"cost_usd,omitempty"`
	Tokens      *tokensJSON `json:"tokens,omitempty"`
	Diagnostic  string      `json:"diagnostic,omitempty"`
	ConfigDir   string      `json:"config_dir,omitempty"`
	// AutoMerge is a literal false: the human owns the merge button. Emitted
	// so downstream readers (telemetry, dry-run plan) can assert the invariant
	// without it being a control-flow field carried through the engine.
	AutoMerge bool `json:"auto_merge"`
}

// tokensJSON mirrors RunResult.Tokens for the JSON record. Pointer in
// jsonRecord so omitempty actually drops the whole sub-object when no
// token data was reported.
type tokensJSON struct {
	Input      int64 `json:"input,omitempty"`
	Output     int64 `json:"output,omitempty"`
	CacheRead  int64 `json:"cache_read,omitempty"`
	CacheWrite int64 `json:"cache_write,omitempty"`
}

// Build dispatches pre-computed task waves through a routed adapter under an
// errgroup with concurrency limit. Wave-complete-then-stop: all in-flight tasks
// finish before the loop aborts. Cancellation flows from the parent ctx, never
// from sibling failures. The engine owns dispatch only — the caller (driver)
// owns work-selection and hands it dependency-ordered waves; see
// contract.anvil.build-orchestration-contract.
func Build(ctx context.Context, waves [][]core.Task, opts Options) (*Summary, error) {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}

	start := time.Now()
	sum := &Summary{Outcomes: map[string]TaskOutcome{}}
	var mu sync.Mutex

	for w, wave := range waves {
		if err := ctx.Err(); err != nil {
			sum.Wall = time.Since(start)
			emitSummary(opts.Stderr, sum)
			return sum, fmt.Errorf("%w: context cancelled before wave %d", ErrBuildCancelled, w)
		}

		g := new(errgroup.Group)
		g.SetLimit(opts.Concurrency)

		for _, task := range wave {
			waveNum := w
			g.Go(func() error {
				oc := dispatchTask(ctx, task, waveNum, opts)
				// Hold mu across the JSON write so concurrent records can't
				// interleave on opts.Stdout. Encode is microseconds; the LLM
				// call is the parallel work, so contention here is noise.
				mu.Lock()
				sum.Outcomes[task.ID] = oc
				emitJSONRecord(opts, oc)
				mu.Unlock()
				return nil // never error → no auto-cancel of siblings
			})
		}
		_ = g.Wait()

		// Classify wave: scan our own outcomes, not the errgroup's err.
		quotaHit, anyFail := false, false
		var firstErr error
		for _, task := range wave {
			oc := sum.Outcomes[task.ID]
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
			emitSummary(opts.Stderr, sum)
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
	emitSummary(opts.Stderr, sum)
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
		// No spawn happens, so no dir is minted. Surface the path the adapter
		// would isolate to, derived from the task ID so it is deterministic and
		// unique per task — making the per-spawn isolation guarantee observable
		// in the plan without fabricating throwaway directories.
		oc.ConfigDir = plannedConfigDir(t.ID)
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
	oc.ConfigDir = res.ConfigDir
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
	return fmt.Errorf("%w: %w", sentinel, cause)
}

// emitSummary writes a one-line build summary to w aggregated across all
// non-skipped TaskOutcomes. No-ops when sum has no real-run outcomes
// (dry-run, pre-wave-0 cancel). Output errors are dropped — consistent with
// emitJSONRecord; the summary is best-effort, not load-bearing.
func emitSummary(w io.Writer, sum *Summary) {
	var (
		nReal                   int
		agent                   time.Duration
		cost                    float64
		in, out, cacheR, cacheW int64
	)
	for _, oc := range sum.Outcomes {
		if oc.Outcome == "skipped_dry_run" {
			continue
		}
		nReal++
		agent += oc.Result.AgentTime
		cost += oc.Result.CostUSD
		in += oc.Result.Tokens.Input
		out += oc.Result.Tokens.Output
		cacheR += oc.Result.Tokens.CacheRead
		cacheW += oc.Result.Tokens.CacheWrite
	}
	if nReal == 0 {
		return
	}
	_, _ = fmt.Fprintf(w,
		"build summary: %d tasks, %.1fs wall, %.1fs agent, $%.4f cost, %d→%d tokens (cache: %dr/%dw)\n",
		nReal, sum.Wall.Seconds(), agent.Seconds(), cost, in, out, cacheR, cacheW,
	)
}

func emitJSONRecord(opts Options, oc TaskOutcome) {
	if !opts.JSON {
		return
	}
	_ = json.NewEncoder(opts.Stdout).Encode(toJSONRecord(oc))
}

// toJSONRecord projects a TaskOutcome onto the wire shape. The single source of
// truth for the per-task JSON shape — both the live --json stream and the
// dry-run plan envelope (built by the driver) go through here.
func toJSONRecord(oc TaskOutcome) jsonRecord {
	rec := jsonRecord{
		TaskID:     oc.TaskID,
		Wave:       oc.Wave,
		Model:      oc.Model,
		Effort:     oc.Effort,
		DurationMS: oc.Duration.Milliseconds(),
		Diagnostic: oc.Result.Diagnostic,
		ConfigDir:  oc.ConfigDir,
		AutoMerge:  false, // literal invariant: the human owns the merge button
	}
	if oc.Outcome == "skipped_dry_run" {
		rec.Status = oc.Outcome
	} else {
		rec.Outcome = oc.Outcome
		rec.AgentTimeMS = oc.Result.AgentTime.Milliseconds()
		rec.CostUSD = oc.Result.CostUSD
		rec.Tokens = &tokensJSON{
			Input:      oc.Result.Tokens.Input,
			Output:     oc.Result.Tokens.Output,
			CacheRead:  oc.Result.Tokens.CacheRead,
			CacheWrite: oc.Result.Tokens.CacheWrite,
		}
	}
	return rec
}

// PlanJSON writes the dry-run plan envelope to w: the run id plus one engine
// jsonRecord per task wrapped in {"run_id": ..., "tasks": [...]}. The envelope
// (vs the live --json NDJSON stream) lets callers assert per-task fields with a
// plain jq path; the run id ties the plan to its persisted build_tasks rows. The
// per-task shape stays the engine's jsonRecord so the driver never redefines it
// (build-orchestration-contract: engine is the single source of the outcome shape).
func PlanJSON(w io.Writer, runID string, waves [][]core.Task) error {
	recs := []jsonRecord{}
	for wave, tasks := range waves {
		for _, t := range tasks {
			recs = append(recs, toJSONRecord(dispatchTask(context.Background(), t, wave, Options{DryRun: true})))
		}
	}
	return json.NewEncoder(w).Encode(struct {
		RunID string       `json:"run_id"`
		Tasks []jsonRecord `json:"tasks"`
	}{RunID: runID, Tasks: recs})
}

// plannedConfigDir is the per-spawn CLAUDE_CONFIG_DIR the adapter would isolate
// to for a task, derived deterministically from the task ID. Used only to make
// the isolation guarantee observable in the dry-run plan; the live adapter mints
// its own dir via os.MkdirTemp.
func plannedConfigDir(taskID string) string {
	return filepath.Join(os.TempDir(), "anvil-claude-"+sanitizeID(taskID))
}

// sanitizeID replaces path separators in a task ID so it is safe as a single
// path segment.
func sanitizeID(id string) string {
	return strings.NewReplacer("/", "-", string(filepath.Separator), "-").Replace(id)
}
