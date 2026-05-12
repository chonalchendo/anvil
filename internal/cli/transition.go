package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

func newTransitionCmd() *cobra.Command {
	var owner, reason string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "transition <type> <id> <new-state>",
		Short: "Move an artifact through its state machine",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := core.ParseType(args[0])
			if err != nil {
				return fmt.Errorf("type: %w", err)
			}
			id, to := args[1], args[2]

			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}

			path := filepath.Join(v.Root, t.Dir(), id+".md")
			a, err := core.LoadArtifact(path)
			if err != nil {
				return ErrArtifactNotFound
			}

			from, _ := a.FrontMatter["status"].(string)
			if from == to {
				return emitTransitionJSON(cmd, asJSON, transitionResult{
					ID: id, Path: path, From: from, To: to, Status: "already_in_state",
				})
			}

			tr, err := core.LookupTransition(t, from, to)
			if err != nil {
				e := errfmt.NewIllegalTransition(string(t), id, from, to, core.LegalNext(t, from))
				return printAndReturn(cmd, e)
			}

			flagValues := map[string]string{"owner": owner, "reason": reason}
			for _, flag := range tr.Requires {
				if flagValues[flag] == "" {
					return printAndReturn(cmd, errfmt.NewTransitionFlagRequired(string(t), id, from, to, flag))
				}
			}
			if tr.Reverse && reason == "" {
				return printAndReturn(cmd, errfmt.NewTransitionFlagRequired(string(t), id, from, to, "reason"))
			}

			if t == core.TypePlan && to == "locked" {
				p, lerr := core.LoadPlan(path)
				if lerr != nil {
					return fmt.Errorf("plan validator: %w", lerr)
				}
				if verr := core.ValidatePlan(p); verr != nil {
					return fmt.Errorf("plan validator: %w", verr)
				}
			}

			a.FrontMatter["status"] = to
			if owner != "" {
				a.FrontMatter["owner"] = owner
			}
			a.FrontMatter["updated"] = time.Now().UTC().Format("2006-01-02")

			if tr.Reverse {
				stamp := time.Now().UTC().Format("2006-01-02")
				audit := fmt.Sprintf("\n> reopened %s: %s\n", stamp, reason)
				if !strings.HasSuffix(a.Body, "\n") {
					a.Body += "\n"
				}
				a.Body += audit
			}

			if err := a.Save(); err != nil {
				return fmt.Errorf("saving: %w", err)
			}
			if err := indexAfterSave(v, a); err != nil {
				return err
			}

			return emitTransitionJSON(cmd, asJSON, transitionResult{
				ID: id, Path: path, From: from, To: to, Owner: owner, Reason: reason, Status: "transitioned",
			})
		},
	}
	cmd.Flags().StringVar(&owner, "owner", "", "owner (required for claim transitions)")
	cmd.Flags().StringVar(&reason, "reason", "", "audit reason (required for reverse transitions)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON envelope")
	return cmd
}

type transitionResult struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	From   string `json:"from"`
	To     string `json:"to"`
	Owner  string `json:"owner,omitempty"`
	Reason string `json:"reason,omitempty"`
	Status string `json:"status"`
}

func emitTransitionJSON(cmd *cobra.Command, asJSON bool, r transitionResult) error {
	if asJSON {
		b, _ := json.Marshal(r)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	if r.Status == "already_in_state" {
		fmt.Fprintf(cmd.OutOrStdout(), "%s already in state %s\n", r.ID, r.To)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: %s → %s\n", r.ID, r.From, r.To)
	return nil
}

// printAndReturn renders the structured error to stderr and returns it for
// cobra's exit-code path.
func printAndReturn(cmd *cobra.Command, err error) error {
	b, _ := json.Marshal(err)
	cmd.PrintErrln(string(b))
	return err
}
