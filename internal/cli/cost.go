package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// costRecord is what one landed PR cost. Wave 2's land stamps it, so the shape
// is shared between the verb and its callers.
type costRecord struct {
	Issue     string         `json:"issue"`
	PR        int            `json:"pr"`
	Rounds    int            `json:"rounds"`
	Additions int            `json:"additions"`
	Deletions int            `json:"deletions"`
	Diff      int            `json:"diff"`
	Files     int            `json:"files"`
	Tokens    int            `json:"tokens"`
	ByAgent   map[string]int `json:"by_agent"`
	Session   string         `json:"session"`
	notice    string
}

var prURLNumber = regexp.MustCompile(`/pull/(\d+)`)

func newCostCmd() *cobra.Command {
	var flagJSON bool
	var flagProjects string
	cmd := &cobra.Command{
		Use:   "cost <issue-id>",
		Short: "Print an issue's review rounds, PR diff size and subagent tokens",
		Long: "Derive what one issue's PR cost: review rounds (the `## Review findings — PR <n>, round <k>` sections), " +
			"diff size from `gh pr view`, and subagent tokens from the claim session's transcripts under --projects-dir, deduped by message id. " +
			"Read-only. Missing transcripts give tokens 0 and a stderr notice.",
		Example: "  anvil cost issue.anvil.0330.anvil-cost-derives-rounds-diff-and --json | jq .diff",
		Args:    namedArgs("anvil cost <issue-id>", []string{"<issue-id>"}, 1, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			id, path, err := core.ResolveArtifact(v, core.TypeIssue, args[0])
			if err != nil {
				return err
			}
			a, err := loadIssueForVerify(path, id, args[0])
			if err != nil {
				return err
			}
			rec, err := issueCost(a, id, flagProjects)
			if err != nil {
				return printAndReturn(cmd, err)
			}
			if rec.notice != "" {
				cmd.PrintErrln(rec.notice)
			}
			if flagJSON {
				b, _ := json.Marshal(rec)
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "issue:   %s\npr:      %d\nrounds:  %d\ndiff:    %d (+%d -%d) in %d files\ntokens:  %d\n",
				rec.Issue, rec.PR, rec.Rounds, rec.Diff, rec.Additions, rec.Deletions, rec.Files, rec.Tokens)
			agents := make([]string, 0, len(rec.ByAgent))
			for k := range rec.ByAgent {
				agents = append(agents, k)
			}
			sort.Strings(agents)
			for _, k := range agents {
				fmt.Fprintf(out, "  %s: %d\n", k, rec.ByAgent[k])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "print the record as one JSON line")
	cmd.Flags().StringVar(&flagProjects, "projects-dir", "", "Claude Code projects root (default ~/.claude/projects)")
	return cmd
}

// issueCost derives the record for an issue that has a PR link.
func issueCost(a *core.Artifact, id, projectsDir string) (costRecord, error) {
	links := prLinks(a)
	if len(links) == 0 {
		return costRecord{}, errfmt.NewStructured("cost_no_pr").Set("issue", id).
			Set("fix_hint", "anvil link issue "+id+" --external <pr-url>")
	}
	num, _ := strconv.Atoi(prURLNumber.FindStringSubmatch(links[len(links)-1])[1])
	raw, err := ghPRViewJSONFn(num, "additions,deletions,changedFiles")
	var view struct {
		Additions    int `json:"additions"`
		Deletions    int `json:"deletions"`
		ChangedFiles int `json:"changedFiles"`
	}
	if err == nil {
		err = json.Unmarshal(raw, &view)
	}
	if err != nil {
		return costRecord{}, errfmt.NewStructured("cost_pr_view_failed").Set("issue", id).Set("pr", num).Set("error", err.Error())
	}
	rec := costRecord{
		Issue: id, PR: num, Rounds: countRounds(a.Body, num),
		Additions: view.Additions, Deletions: view.Deletions, Diff: view.Additions + view.Deletions, Files: view.ChangedFiles,
		ByAgent: map[string]int{},
	}
	rec.Session, _ = a.FrontMatter["claim_session"].(string)
	rec.tokensFromTranscripts(id, projectsDir)
	return rec, nil
}

func countRounds(body string, pr int) int {
	prefix := fmt.Sprintf("## Review findings — PR %d, round ", pr)
	n := 0
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, prefix) {
			n++
		}
	}
	return n
}

// tokensFromTranscripts fills Tokens and ByAgent. A missing session or
// directory is a notice, not a refusal: the harness garbage-collects transcripts.
func (r *costRecord) tokensFromTranscripts(id, projectsDir string) {
	if projectsDir == "" {
		home, err := userHomeFn()
		if err == nil {
			projectsDir = filepath.Join(home, ".claude", "projects")
		}
	}
	root, terr := gitToplevelFn()
	if r.Session == "" || projectsDir == "" || terr != nil {
		r.notice = "cost: no transcripts to read (session, projects dir or repo root unknown); tokens 0"
		return
	}
	dir := filepath.Join(projectsDir, strings.ReplaceAll(root, "/", "-"), r.Session, "subagents")
	files, _ := filepath.Glob(filepath.Join(dir, "agent-*.jsonl"))
	matched := 0
	for _, f := range files {
		agent, total, ok := transcriptTokens(f, id)
		if !ok {
			continue
		}
		matched++
		r.Tokens += total
		r.ByAgent[agent] += total
	}
	if matched == 0 {
		r.notice = "cost: no subagent transcript for " + id + " under " + dir + "; tokens 0"
	}
}

type transcriptLine struct {
	Type             string `json:"type"`
	AttributionAgent string `json:"attributionAgent"`
	Message          struct {
		ID      string          `json:"id"`
		Content json.RawMessage `json:"content"`
		Usage   map[string]int  `json:"usage"`
	} `json:"message"`
}

// transcriptTokens sums one transcript's usage deduped by message id. ok is
// false when the dispatch prompt does not name the issue.
func transcriptTokens(path, id string) (agent string, total int, ok bool) {
	f, err := os.Open(path) //nolint:gosec // path comes from a Glob under the projects dir
	if err != nil {
		return "", 0, false
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	seen := map[string]bool{}
	sawUser := false
	for sc.Scan() {
		var l transcriptLine
		if json.Unmarshal(sc.Bytes(), &l) != nil {
			continue
		}
		if agent == "" && l.AttributionAgent != "" {
			agent = l.AttributionAgent
		}
		switch {
		case l.Type == "user" && !sawUser:
			sawUser = true
			if !strings.Contains(promptText(l.Message.Content), id) {
				return "", 0, false
			}
		case l.Type == "assistant" && !seen[l.Message.ID]:
			seen[l.Message.ID] = true
			u := l.Message.Usage
			total += u["input_tokens"] + u["cache_creation_input_tokens"] + u["cache_read_input_tokens"] + u["output_tokens"]
		}
	}
	if !sawUser {
		return "", 0, false
	}
	if agent == "" {
		agent = "subagent-unknown"
	}
	return agent, total, true
}

// promptText flattens message.content: a string, or an array of text parts.
func promptText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.Text)
		b.WriteByte('\n')
	}
	return b.String()
}
