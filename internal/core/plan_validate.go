package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
)

const taskBodyMinLen = 200

// ErrPlanDAG signals a graph-level problem: cycle, dangling dep, file conflict,
// or invalid file path. Maps to CLI exit code 2.
var ErrPlanDAG = errors.New("plan dag invalid")

// SameFileInWaveError reports two or more tasks in the same wave touching the
// same file. The wave graph requires file-isolation per wave so the executor
// can fan out without concurrent writes; the planner workaround is to split
// the task or add a depends_on edge that pushes one task into a later wave.
// Wraps ErrPlanDAG so callers' errors.Is checks still match.
type SameFileInWaveError struct {
	*errfmt.Structured
}

func newSameFileInWaveError(wave int, tasks []string, file string) *SameFileInWaveError {
	fix := fmt.Sprintf("split the task into per-file tasks, or add depends_on between %s and %s",
		tasks[0], tasks[1])
	return &SameFileInWaveError{
		Structured: errfmt.NewStructured("same_file_in_wave").
			Set("wave", wave).
			Set("tasks", tasks).
			Set("file", file).
			Set("fix", fix).
			Wrap(ErrPlanDAG),
	}
}

// ErrPlanTDD signals a TDD-discipline violation: missing body section,
// empty verify command. Maps to CLI exit code 3.
var ErrPlanTDD = errors.New("plan tdd invariant violated")

// ValidatePlan runs structural checks on a parsed plan beyond what JSON Schema
// covers (cycles, dangling deps, body sections, file-set sanity).
func ValidatePlan(p *Plan) error {
	ids := make(map[string]int, len(p.Tasks))
	for i, t := range p.Tasks {
		if _, dup := ids[t.ID]; dup {
			return fmt.Errorf("%w: duplicate task id %q", ErrPlanDAG, t.ID)
		}
		ids[t.ID] = i
	}

	for _, t := range p.Tasks {
		if strings.TrimSpace(t.Verify) == "" {
			return fmt.Errorf("%w: task %s has empty verify", ErrPlanTDD, t.ID)
		}
	}

	for _, t := range p.Tasks {
		if len(strings.TrimSpace(t.Body)) < taskBodyMinLen {
			return fmt.Errorf("%w: task %s body section missing or <%d chars",
				ErrPlanTDD, t.ID, taskBodyMinLen)
		}
	}

	for _, t := range p.Tasks {
		for _, f := range t.Files {
			if filepath.IsAbs(f) || strings.Contains(f, `\`) {
				return fmt.Errorf("%w: task %s file %q must be repo-relative posix path",
					ErrPlanDAG, t.ID, f)
			}
		}
	}

	waves, err := kahnWaves(p.Tasks, ids)
	if err != nil {
		return err
	}

	for w, wave := range waves {
		seen := map[string]string{}
		for _, idx := range wave {
			for _, f := range p.Tasks[idx].Files {
				if other, ok := seen[f]; ok {
					return newSameFileInWaveError(w, []string{other, p.Tasks[idx].ID}, f)
				}
				seen[f] = p.Tasks[idx].ID
			}
		}
	}
	return nil
}

// Waves returns task indices grouped by topological depth. Returns ErrPlanDAG
// on dangling deps or cycles. Tasks within a wave have no dependency on each
// other and may run in parallel.
func (p *Plan) Waves() ([][]int, error) {
	ids := make(map[string]int, len(p.Tasks))
	for i, t := range p.Tasks {
		ids[t.ID] = i
	}
	return kahnWaves(p.Tasks, ids)
}

func kahnWaves(tasks []Task, ids map[string]int) ([][]int, error) {
	n := len(tasks)
	indeg := make([]int, n)
	children := make([][]int, n)
	for i, t := range tasks {
		for _, dep := range t.DependsOn {
			j, ok := ids[dep]
			if !ok {
				return nil, fmt.Errorf("%w: task %s depends on unknown %s",
					ErrPlanDAG, t.ID, dep)
			}
			children[j] = append(children[j], i)
			indeg[i]++
		}
	}
	var waves [][]int
	placed := 0
	current := []int{}
	for i := 0; i < n; i++ {
		if indeg[i] == 0 {
			current = append(current, i)
		}
	}
	for len(current) > 0 {
		next := []int{}
		for _, i := range current {
			placed++
			for _, c := range children[i] {
				indeg[c]--
				if indeg[c] == 0 {
					next = append(next, c)
				}
			}
		}
		waves = append(waves, current)
		current = next
	}
	if placed != n {
		return nil, fmt.Errorf("%w: cycle detected (%d/%d tasks placed)",
			ErrPlanDAG, placed, n)
	}
	return waves, nil
}
