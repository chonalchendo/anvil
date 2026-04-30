package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

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
		return fmt.Errorf("%w: %v", ErrSchemaInvalid, err)
	}

	p, err := core.LoadPlan(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSchemaInvalid, err)
	}

	if validate {
		if err := core.ValidatePlan(p); err != nil {
			cmd.PrintErrln(err)
			return err
		}
		cmd.Println("ok")
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
	cmd.Print(b.String())
}

func escapeMermaid(s string) string {
	return strings.ReplaceAll(s, `"`, `'`)
}
