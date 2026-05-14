package build

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chonalchendo/anvil/internal/core"
)

// fakeAdapter is a configurable in-memory adapter for build_test. resp is
// keyed by an instruction-body prefix; the first matching prefix wins. Missing
// entries default to a 0-exit-code success.
type fakeAdapter struct {
	name string
	mu   sync.Mutex
	resp map[string]fakeResp
	// callOrder records the body-prefix of each Run call in dispatch order.
	callOrder     []string
	inflight, max atomic.Int32
}

type fakeResp struct {
	res  RunResult
	err  error
	hold time.Duration // delay before returning, for concurrency / ordering tests
}

func (f *fakeAdapter) Name() string { return f.name }

func (f *fakeAdapter) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	cur := f.inflight.Add(1)
	for {
		old := f.max.Load()
		if cur <= old || f.max.CompareAndSwap(old, cur) {
			break
		}
	}
	defer f.inflight.Add(-1)

	f.mu.Lock()
	f.callOrder = append(f.callOrder, req.Instruction)
	var r fakeResp
	for k, v := range f.resp {
		if strings.HasPrefix(req.Instruction, k) {
			r = v
			break
		}
	}
	f.mu.Unlock()

	if r.hold > 0 {
		select {
		case <-time.After(r.hold):
		case <-ctx.Done():
			return RunResult{}, ctx.Err()
		}
	}
	return r.res, r.err
}

func twoTaskPlan() *core.Plan {
	return &core.Plan{
		ID: "anvil.demo", Slug: "demo", Status: "ready",
		Tasks: []core.Task{
			{
				ID: "T1", Title: "Wave-0 task", Model: "claude-sonnet-4-6", Effort: "medium",
				Body: "do T1", Verify: "true",
			},
			{
				ID: "T2", Title: "Wave-1 task", Model: "claude-sonnet-4-6", Effort: "medium",
				Body: "do T2", Verify: "true", DependsOn: []string{"T1"},
			},
		},
	}
}

func TestBuild_WaveOrder_T1BeforeT2(t *testing.T) {
	fa := &fakeAdapter{name: "fake", resp: map[string]fakeResp{}}
	opts := Options{
		Concurrency: 4,
		Cwd:         t.TempDir(),
		Stdout:      io.Discard,
		Stderr:      io.Discard,
		Router:      Router{"claude-": fa},
	}
	sum, err := Build(context.Background(), twoTaskPlan(), opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := len(fa.callOrder); got != 2 {
		t.Fatalf("call count = %d, want 2", got)
	}
	if !strings.HasPrefix(fa.callOrder[0], "do T1") || !strings.HasPrefix(fa.callOrder[1], "do T2") {
		t.Errorf("call order = %v, want [T1 first, T2 second]", fa.callOrder)
	}
	if sum.Outcomes["T1"].Outcome != "success" || sum.Outcomes["T2"].Outcome != "success" {
		t.Errorf("outcomes = %+v, want all success", sum.Outcomes)
	}
}

func TestBuild_ConcurrencyLimit_HoldsAtCap(t *testing.T) {
	// 3 independent wave-0 tasks; concurrency=2; assert max in-flight ≤ 2.
	plan := &core.Plan{
		ID: "anvil.fan", Slug: "fan", Status: "ready",
		Tasks: []core.Task{
			{ID: "T1", Title: "a", Model: "claude-sonnet-4-6", Body: "a", Verify: "true"},
			{ID: "T2", Title: "b", Model: "claude-sonnet-4-6", Body: "b", Verify: "true"},
			{ID: "T3", Title: "c", Model: "claude-sonnet-4-6", Body: "c", Verify: "true"},
		},
	}
	fa := &fakeAdapter{name: "fake", resp: map[string]fakeResp{
		"a": {hold: 50 * time.Millisecond},
		"b": {hold: 50 * time.Millisecond},
		"c": {hold: 50 * time.Millisecond},
	}}
	opts := Options{
		Concurrency: 2, Cwd: t.TempDir(),
		Stdout: io.Discard, Stderr: io.Discard,
		Router: Router{"claude-": fa},
	}
	if _, err := Build(context.Background(), plan, opts); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := fa.max.Load(); got > 2 {
		t.Errorf("max in-flight = %d, want ≤ 2", got)
	}
}

func TestBuild_TaskFailure_ReturnsErrBuildTaskFailed(t *testing.T) {
	fa := &fakeAdapter{name: "fake", resp: map[string]fakeResp{
		"do T1": {res: RunResult{ExitCode: 1}},
	}}
	opts := Options{
		Concurrency: 1, Cwd: t.TempDir(),
		Stdout: io.Discard, Stderr: io.Discard,
		Router: Router{"claude-": fa},
	}
	_, err := Build(context.Background(), twoTaskPlan(), opts)
	if !errors.Is(err, ErrBuildTaskFailed) {
		t.Errorf("err = %v, want ErrBuildTaskFailed", err)
	}
}

func TestBuild_QuotaExhausted_AbortsWaveAndReturnsSentinel(t *testing.T) {
	fa := &fakeAdapter{name: "fake", resp: map[string]fakeResp{
		"do T1": {err: ErrQuotaExhausted},
	}}
	opts := Options{
		Concurrency: 1, Cwd: t.TempDir(),
		Stdout: io.Discard, Stderr: io.Discard,
		Router: Router{"claude-": fa},
	}
	sum, err := Build(context.Background(), twoTaskPlan(), opts)
	if !errors.Is(err, ErrBuildQuotaExhausted) {
		t.Errorf("err = %v, want ErrBuildQuotaExhausted", err)
	}
	// T2 must never be dispatched: wave 0 failed, wave 1 is aborted.
	if _, ran := sum.Outcomes["T2"]; ran {
		t.Errorf("T2 was dispatched after wave-0 quota exhaustion: %+v", sum.Outcomes["T2"])
	}
}

func TestBuild_DryRun_SkipsAdapterAndSucceeds(t *testing.T) {
	// Empty router: only --dry-run can succeed.
	opts := Options{
		Concurrency: 1, Cwd: t.TempDir(), DryRun: true,
		Stdout: io.Discard, Stderr: io.Discard,
		Router: Router{},
	}
	sum, err := Build(context.Background(), twoTaskPlan(), opts)
	if err != nil {
		t.Fatalf("dry-run Build: %v", err)
	}
	for _, id := range []string{"T1", "T2"} {
		if got := sum.Outcomes[id].Outcome; got != "skipped_dry_run" {
			t.Errorf("%s outcome = %q, want skipped_dry_run", id, got)
		}
	}
}

func TestBuild_NoAdapterRegistered_ErrorsLoud(t *testing.T) {
	opts := Options{
		Concurrency: 1, Cwd: t.TempDir(),
		Stdout: io.Discard, Stderr: io.Discard,
		Router: Router{}, // nothing registered
	}
	_, err := Build(context.Background(), twoTaskPlan(), opts)
	if err == nil || !strings.Contains(err.Error(), "no adapter for model") {
		t.Errorf("err = %v, want 'no adapter for model …'", err)
	}
}

func TestBuild_DiagnosticOnFailure_InJSONAndStderr(t *testing.T) {
	// Adapter returns ExitCode=1 with a diagnostic message.
	fa := &fakeAdapter{name: "fake", resp: map[string]fakeResp{
		"do T1": {res: RunResult{ExitCode: 1, Diagnostic: "boom"}},
	}}
	var stdout, stderr bytes.Buffer
	opts := Options{
		Concurrency: 1, Cwd: t.TempDir(),
		JSON:   true,
		Stdout: &stdout,
		Stderr: &stderr,
		Router: Router{"claude-": fa},
	}
	_, _ = Build(context.Background(), twoTaskPlan(), opts)

	// JSON record must carry the diagnostic field.
	if !strings.Contains(stdout.String(), `"diagnostic":"boom"`) {
		t.Errorf("JSON stdout missing diagnostic; got:\n%s", stdout.String())
	}
	// Human-readable line must appear on stderr.
	wantLine := "task T1 [failed]: boom"
	if !strings.Contains(stderr.String(), wantLine) {
		t.Errorf("stderr missing %q; got:\n%s", wantLine, stderr.String())
	}
}

func TestBuild_JSONRecord_IncludesTokensAndCost(t *testing.T) {
	fa := &fakeAdapter{name: "fake", resp: map[string]fakeResp{
		"do T1": {res: RunResult{
			ExitCode:  0,
			Duration:  200 * time.Millisecond,
			AgentTime: 150 * time.Millisecond,
			Tokens:    TokenUsage{Input: 100, Output: 50, CacheRead: 3, CacheWrite: 7},
			CostUSD:   0.0123,
		}},
		"do T2": {res: RunResult{
			ExitCode:  0,
			Duration:  100 * time.Millisecond,
			AgentTime: 80 * time.Millisecond,
			Tokens:    TokenUsage{Input: 60, Output: 40, CacheRead: 2, CacheWrite: 5},
			CostUSD:   0.0044,
		}},
	}}
	var stdout, stderr strings.Builder
	opts := Options{
		Concurrency: 1, Cwd: t.TempDir(), JSON: true,
		Stdout: &stdout, Stderr: &stderr,
		Router: Router{"claude-": fa},
	}
	if _, err := Build(context.Background(), twoTaskPlan(), opts); err != nil {
		t.Fatalf("Build: %v", err)
	}
	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d JSON lines, want 2: %q", len(lines), stdout.String())
	}
	for i, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %d: unmarshal: %v (line=%q)", i, err, line)
		}
		for _, k := range []string{"tokens", "cost_usd", "agent_time_ms"} {
			if _, ok := rec[k]; !ok {
				t.Errorf("line %d missing key %q: %v", i, k, rec)
			}
		}
		toks, ok := rec["tokens"].(map[string]any)
		if !ok {
			t.Fatalf("line %d tokens not an object: %v", i, rec["tokens"])
		}
		for _, k := range []string{"input", "output", "cache_read", "cache_write"} {
			if _, ok := toks[k]; !ok {
				t.Errorf("line %d tokens missing %q: %v", i, k, toks)
			}
		}
	}
	// Spot-check the T1 row's specific values.
	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("spot-check unmarshal: %v (line=%q)", err, lines[0])
	}
	if got := rec["cost_usd"]; got != 0.0123 {
		t.Errorf("T1 cost_usd = %v, want 0.0123", got)
	}
	toks := rec["tokens"].(map[string]any)
	if got := toks["input"]; got != float64(100) {
		t.Errorf("T1 tokens.input = %v, want 100", got)
	}
}

func TestBuild_JSONRecord_DryRunOmitsCostFields(t *testing.T) {
	var stdout, stderr strings.Builder
	opts := Options{
		Concurrency: 1, Cwd: t.TempDir(), DryRun: true, JSON: true,
		Stdout: &stdout, Stderr: &stderr,
		Router: Router{},
	}
	if _, err := Build(context.Background(), twoTaskPlan(), opts); err != nil {
		t.Fatalf("Build: %v", err)
	}
	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d JSON lines, want 2: %q", len(lines), stdout.String())
	}
	for i, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %d: unmarshal: %v (line=%q)", i, err, line)
		}
		for _, k := range []string{"tokens", "cost_usd", "agent_time_ms"} {
			if _, present := rec[k]; present {
				t.Errorf("line %d unexpectedly contains %q: %v", i, k, rec)
			}
		}
		if rec["status"] != "skipped_dry_run" {
			t.Errorf("line %d status = %v, want skipped_dry_run", i, rec["status"])
		}
	}
}

func TestBuild_Summary_AggregatesAcrossTasks(t *testing.T) {
	fa := &fakeAdapter{name: "fake", resp: map[string]fakeResp{
		"do T1": {res: RunResult{
			ExitCode:  0,
			Duration:  200 * time.Millisecond,
			AgentTime: 150 * time.Millisecond,
			Tokens:    TokenUsage{Input: 100, Output: 50, CacheRead: 3, CacheWrite: 7},
			CostUSD:   0.0123,
		}},
		"do T2": {res: RunResult{
			ExitCode:  0,
			Duration:  100 * time.Millisecond,
			AgentTime: 80 * time.Millisecond,
			Tokens:    TokenUsage{Input: 60, Output: 40, CacheRead: 2, CacheWrite: 5},
			CostUSD:   0.0044,
		}},
	}}
	var stderr strings.Builder
	opts := Options{
		Concurrency: 1, Cwd: t.TempDir(),
		Stdout: io.Discard, Stderr: &stderr,
		Router: Router{"claude-": fa},
	}
	if _, err := Build(context.Background(), twoTaskPlan(), opts); err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := stderr.String()
	// Aggregated values: 2 tasks; 160→90 tokens; (cache: 5r/12w); $0.0167; agent 0.2s.
	wantSubstrings := []string{
		"build summary: 2 tasks,",
		"$0.0167 cost,",
		"160→90 tokens",
		"(cache: 5r/12w)",
		"0.2s agent,",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(got, want) {
			t.Errorf("stderr missing %q\nfull stderr: %q", want, got)
		}
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("stderr should end with newline, got %q", got)
	}
}

func TestBuild_Summary_OmittedOnDryRun(t *testing.T) {
	var stderr strings.Builder
	opts := Options{
		Concurrency: 1, Cwd: t.TempDir(), DryRun: true,
		Stdout: io.Discard, Stderr: &stderr,
		Router: Router{},
	}
	if _, err := Build(context.Background(), twoTaskPlan(), opts); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := stderr.String(); got != "" {
		t.Errorf("dry-run stderr = %q, want empty", got)
	}
}

func TestBuild_Summary_PrintedOnPartialFail(t *testing.T) {
	fa := &fakeAdapter{name: "fake", resp: map[string]fakeResp{
		"do T1": {res: RunResult{
			ExitCode:  1, // task fails
			Duration:  200 * time.Millisecond,
			AgentTime: 150 * time.Millisecond,
			Tokens:    TokenUsage{Input: 80, Output: 40, CacheRead: 2, CacheWrite: 5},
			CostUSD:   0.0099,
		}},
	}}
	var stderr strings.Builder
	opts := Options{
		Concurrency: 1, Cwd: t.TempDir(),
		Stdout: io.Discard, Stderr: &stderr,
		Router: Router{"claude-": fa},
	}
	_, err := Build(context.Background(), twoTaskPlan(), opts)
	if !errors.Is(err, ErrBuildTaskFailed) {
		t.Fatalf("err = %v, want ErrBuildTaskFailed", err)
	}
	got := stderr.String()
	wantSubstrings := []string{
		"build summary: 1 tasks,",
		"$0.0099 cost,",
		"80→40 tokens",
		"(cache: 2r/5w)",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(got, want) {
			t.Errorf("stderr missing %q\nfull stderr: %q", want, got)
		}
	}
}

func TestBuild_Summary_PrintedOnQuotaExhausted(t *testing.T) {
	fa := &fakeAdapter{name: "fake", resp: map[string]fakeResp{
		"do T1": {
			res: RunResult{
				Duration:  200 * time.Millisecond,
				AgentTime: 150 * time.Millisecond,
				Tokens:    TokenUsage{Input: 100, Output: 50, CacheRead: 3, CacheWrite: 7},
				CostUSD:   0.0123,
			},
			err: ErrQuotaExhausted,
		},
	}}
	var stderr strings.Builder
	opts := Options{
		Concurrency: 1, Cwd: t.TempDir(),
		Stdout: io.Discard, Stderr: &stderr,
		Router: Router{"claude-": fa},
	}
	_, err := Build(context.Background(), twoTaskPlan(), opts)
	if !errors.Is(err, ErrBuildQuotaExhausted) {
		t.Fatalf("err = %v, want ErrBuildQuotaExhausted", err)
	}
	got := stderr.String()
	if !strings.Contains(got, "build summary: 1 tasks,") {
		t.Errorf("stderr missing summary header; got %q", got)
	}
	if !strings.Contains(got, "$0.0123 cost,") {
		t.Errorf("stderr missing cost; got %q", got)
	}
}

// concurrencyDetectingWriter records the maximum number of in-flight Write
// calls. Used to verify emitJSONRecord writes are serialized.
type concurrencyDetectingWriter struct {
	hold     time.Duration
	inflight atomic.Int32
	max      atomic.Int32
}

func (c *concurrencyDetectingWriter) Write(p []byte) (int, error) {
	cur := c.inflight.Add(1)
	defer c.inflight.Add(-1)
	for {
		old := c.max.Load()
		if cur <= old || c.max.CompareAndSwap(old, cur) {
			break
		}
	}
	if c.hold > 0 {
		time.Sleep(c.hold)
	}
	return len(p), nil
}

func TestBuild_JSONRecord_WritesAreSerialized(t *testing.T) {
	plan := &core.Plan{
		ID: "anvil.fanout", Slug: "fanout", Status: "ready",
		Tasks: []core.Task{
			{ID: "T1", Title: "a", Model: "claude-sonnet-4-6", Body: "a", Verify: "true"},
			{ID: "T2", Title: "b", Model: "claude-sonnet-4-6", Body: "b", Verify: "true"},
			{ID: "T3", Title: "c", Model: "claude-sonnet-4-6", Body: "c", Verify: "true"},
			{ID: "T4", Title: "d", Model: "claude-sonnet-4-6", Body: "d", Verify: "true"},
		},
	}
	fa := &fakeAdapter{name: "fake", resp: map[string]fakeResp{
		"a": {hold: 5 * time.Millisecond},
		"b": {hold: 5 * time.Millisecond},
		"c": {hold: 5 * time.Millisecond},
		"d": {hold: 5 * time.Millisecond},
	}}
	stdout := &concurrencyDetectingWriter{hold: 2 * time.Millisecond}
	opts := Options{
		Concurrency: 4, Cwd: t.TempDir(), JSON: true,
		Stdout: stdout, Stderr: io.Discard,
		Router: Router{"claude-": fa},
	}
	if _, err := Build(context.Background(), plan, opts); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := stdout.max.Load(); got > 1 {
		t.Errorf("max concurrent writes to stdout = %d, want 1 (records must not interleave)", got)
	}
}
