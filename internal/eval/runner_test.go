package eval

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chonalchendo/anvil/internal/build"
)

// fakeAdapter scripts the two spawns RunCase makes: the skill run (req.Skills
// non-empty) and the judge (req.Skills empty). onRun observes each request.
type fakeAdapter struct {
	runResult   build.RunResult
	runErr      error
	judgeResult build.RunResult
	judgeErr    error
	onRun       func(req build.RunRequest)
	calls       int
}

func (f *fakeAdapter) Name() string { return "fake" }

func (f *fakeAdapter) Run(_ context.Context, req build.RunRequest) (build.RunResult, error) {
	f.calls++
	if f.onRun != nil {
		f.onRun(req)
	}
	if len(req.Skills) > 0 {
		return f.runResult, f.runErr
	}
	return f.judgeResult, f.judgeErr
}

func TestEvalRunCasePassSumsCost(t *testing.T) {
	fa := &fakeAdapter{
		runResult:   build.RunResult{CostUSD: 0.01, Duration: 2 * time.Second, Diagnostic: "produced a SKILL.md"},
		judgeResult: build.RunResult{CostUSD: 0.002, Diagnostic: "PASS — workflow ran end to end"},
	}
	r := &Runner{Adapter: fa, Model: "m"}
	res, err := r.RunCase(context.Background(), "some-skill", Case{ID: 0, Name: "happy"})
	if err != nil {
		t.Fatalf("RunCase: %v", err)
	}
	if !res.Pass {
		t.Errorf("Pass = false, want true (reason %q)", res.Reason)
	}
	if math.Abs(res.Cost-0.012) > 1e-9 {
		t.Errorf("Cost = %v, want 0.012 (run + judge)", res.Cost)
	}
	if res.Duration != 2*time.Second {
		t.Errorf("Duration = %v, want 2s", res.Duration)
	}
	if fa.calls != 2 {
		t.Errorf("adapter calls = %d, want 2 (run + judge)", fa.calls)
	}
}

func TestEvalRunCaseJudgeFail(t *testing.T) {
	fa := &fakeAdapter{judgeResult: build.RunResult{Diagnostic: "FAIL — produced a file it should have refused"}}
	res, err := (&Runner{Adapter: fa}).RunCase(context.Background(), "s", Case{Name: "neg"})
	if err != nil {
		t.Fatalf("RunCase: %v", err)
	}
	if res.Pass {
		t.Error("Pass = true, want false")
	}
}

func TestEvalRunCaseUnparseableJudgeIsFail(t *testing.T) {
	fa := &fakeAdapter{judgeResult: build.RunResult{Diagnostic: "hard to say either way"}}
	res, _ := (&Runner{Adapter: fa}).RunCase(context.Background(), "s", Case{})
	if res.Pass {
		t.Error("unparseable judge verdict must not pass")
	}
}

func TestEvalRunCaseSeedsFixtureFiles(t *testing.T) {
	var seen bool
	fa := &fakeAdapter{
		judgeResult: build.RunResult{Diagnostic: "PASS"},
		onRun: func(req build.RunRequest) {
			if len(req.Skills) == 0 {
				return // judge spawn has no cwd to check
			}
			if _, err := os.Stat(filepath.Join(req.Cwd, "input.md")); err == nil {
				seen = true
			}
		},
	}
	c := Case{Files: []FileSeed{{Path: "input.md", Content: "hello"}}}
	if _, err := (&Runner{Adapter: fa}).RunCase(context.Background(), "s", c); err != nil {
		t.Fatalf("RunCase: %v", err)
	}
	if !seen {
		t.Error("fixture file was not seeded into the run Cwd")
	}
}

func TestEvalRunCaseRunErrorGradedAsFail(t *testing.T) {
	fa := &fakeAdapter{runErr: errors.New("spawn boom")}
	res, err := (&Runner{Adapter: fa}).RunCase(context.Background(), "s", Case{})
	if err != nil {
		t.Fatalf("RunCase should grade a run error as a fail, got err: %v", err)
	}
	if res.Pass {
		t.Error("errored run must not pass")
	}
	if fa.calls != 1 {
		t.Errorf("judge should be skipped on run error; calls = %d, want 1", fa.calls)
	}
}

func TestEvalRunCaseQuotaPropagates(t *testing.T) {
	fa := &fakeAdapter{runErr: build.ErrQuotaExhausted}
	_, err := (&Runner{Adapter: fa}).RunCase(context.Background(), "s", Case{})
	if !errors.Is(err, build.ErrQuotaExhausted) {
		t.Errorf("err = %v, want ErrQuotaExhausted to propagate", err)
	}
}

func TestEvalRunCaseJudgeErrorGradedAsFail(t *testing.T) {
	fa := &fakeAdapter{
		runResult:   build.RunResult{CostUSD: 0.01},
		judgeResult: build.RunResult{CostUSD: 0.003}, // billed before erroring
		judgeErr:    errors.New("judge boom"),
	}
	res, err := (&Runner{Adapter: fa}).RunCase(context.Background(), "s", Case{})
	if err != nil {
		t.Fatalf("a judge error should grade as a fail, not abort the case: %v", err)
	}
	if res.Pass {
		t.Error("Pass = true, want false on judge error")
	}
	if math.Abs(res.Cost-0.013) > 1e-9 {
		t.Errorf("Cost = %v, want 0.013 (run + billed judge cost folded in)", res.Cost)
	}
}

func TestEvalRunCaseJudgeQuotaPropagates(t *testing.T) {
	fa := &fakeAdapter{judgeErr: build.ErrQuotaExhausted}
	_, err := (&Runner{Adapter: fa}).RunCase(context.Background(), "s", Case{})
	if !errors.Is(err, build.ErrQuotaExhausted) {
		t.Errorf("err = %v, want judge ErrQuotaExhausted to propagate", err)
	}
}
