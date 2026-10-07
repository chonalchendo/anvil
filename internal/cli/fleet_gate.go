package cli

import "github.com/chonalchendo/anvil/internal/core"

// fleetGate is one issue's loop state, read from the vault and the PR. Every
// key is always present so a consumer branches on empty values, not absence.
type fleetGate struct {
	Claim  gateClaim  `json:"claim"`
	PR     gatePR     `json:"pr"`
	Review gateReview `json:"review"`
	CI     gateCI     `json:"ci"`
}

type gateClaim struct {
	Owner     string `json:"owner"`
	ClaimedAt string `json:"claimed_at"`
}

type gatePR struct {
	Number int    `json:"number"`
	State  string `json:"state"`
}

type gateReview struct {
	State        string `json:"state"`
	OpenComments int    `json:"open_comments"`
}

type gateCI struct {
	Conclusion string `json:"conclusion"`
}

func gateFromRow(a *core.Artifact, r fleetRow) fleetGate {
	claimedAt, _ := a.FrontMatter["claimed_at"].(string)
	return fleetGate{
		Claim:  gateClaim{Owner: r.Owner, ClaimedAt: claimedAt},
		PR:     gatePR{Number: r.PRNumber, State: r.prState},
		Review: gateReview{State: r.ReviewerState, OpenComments: r.OpenInlineComments},
		CI:     gateCI{Conclusion: r.CIConclusion},
	}
}
