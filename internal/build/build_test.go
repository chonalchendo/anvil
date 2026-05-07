package build

import (
	"bytes"
	"context"
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
			{ID: "T1", Title: "Wave-0 task", Model: "claude-sonnet-4-6", Effort: "medium",
				Body: "do T1", Verify: "true"},
			{ID: "T2", Title: "Wave-1 task", Model: "claude-sonnet-4-6", Effort: "medium",
				Body: "do T2", Verify: "true", DependsOn: []string{"T1"}},
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
