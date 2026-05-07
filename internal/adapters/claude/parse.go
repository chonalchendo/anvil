package claude

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Event is the minimal slice of a Claude Code stream-json line we care about.
// Fields absent in the input default to zero values; unknown event types are
// returned with Type set so callers can ignore them by Type-switching.
type Event struct {
	Type          string
	Subtype       string
	DurationMS    int64
	APIDurationMS int64
	CostUSD       float64
	Usage         Usage
	Text          string
	IsError       bool
}

// Usage mirrors the `usage` field of a result event.
type Usage struct {
	InputTokens, OutputTokens int64
	CacheCreate, CacheRead    int64
}

// rawLine is the on-the-wire shape; only fields used downstream are mapped.
type rawLine struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	// result-event fields
	DurationMS    int64   `json:"duration_ms"`
	APIDurationMS int64   `json:"duration_api_ms"`
	CostUSD       float64 `json:"total_cost_usd"`
	Usage         struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
		CacheCreate  int64 `json:"cache_creation_input_tokens"`
		CacheRead    int64 `json:"cache_read_input_tokens"`
	} `json:"usage"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`
	// assistant-event fields
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

// parseEvent decodes one NDJSON line. Returns ok=false for empty / non-JSON
// input or input lacking a non-empty `type` field.
func parseEvent(line []byte) (Event, bool) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 || line[0] != '{' {
		return Event{}, false
	}
	var r rawLine
	if err := json.Unmarshal(line, &r); err != nil {
		return Event{}, false
	}
	if r.Type == "" {
		return Event{}, false
	}
	ev := Event{
		Type:          r.Type,
		Subtype:       r.Subtype,
		DurationMS:    r.DurationMS,
		APIDurationMS: r.APIDurationMS,
		CostUSD:       r.CostUSD,
		Usage: Usage{
			InputTokens:  r.Usage.InputTokens,
			OutputTokens: r.Usage.OutputTokens,
			CacheCreate:  r.Usage.CacheCreate,
			CacheRead:    r.Usage.CacheRead,
		},
		IsError: r.IsError,
	}
	switch r.Type {
	case "result":
		ev.Text = r.Result
	case "assistant":
		for _, c := range r.Message.Content {
			if c.Type == "text" {
				ev.Text = c.Text
				break
			}
		}
	}
	return ev, true
}

// isQuotaMessage reports whether s contains the canonical Claude usage-cap
// phrase. Anchored on "usage limit reached" so wording variants like
// "Claude AI usage limit reached" and "Anthropic API usage limit reached"
// both match.
func isQuotaMessage(s string) bool {
	return strings.Contains(strings.ToLower(s), "usage limit reached")
}
