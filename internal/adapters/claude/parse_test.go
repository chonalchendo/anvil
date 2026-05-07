package claude

import "testing"

func TestParseEvent_ResultWithUsage(t *testing.T) {
	line := []byte(`{"type":"result","subtype":"success","duration_ms":12345,"duration_api_ms":11000,"total_cost_usd":0.0123,"usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":7,"cache_read_input_tokens":3},"is_error":false,"result":"done."}`)
	ev, ok := parseEvent(line)
	if !ok {
		t.Fatalf("parseEvent returned ok=false for valid result line")
	}
	if ev.Type != "result" || ev.Subtype != "success" {
		t.Errorf("type/subtype = %q/%q, want result/success", ev.Type, ev.Subtype)
	}
	if ev.DurationMS != 12345 || ev.APIDurationMS != 11000 {
		t.Errorf("duration_ms = %d / api = %d, want 12345 / 11000", ev.DurationMS, ev.APIDurationMS)
	}
	if ev.CostUSD != 0.0123 {
		t.Errorf("CostUSD = %v, want 0.0123", ev.CostUSD)
	}
	if ev.Usage.InputTokens != 100 || ev.Usage.OutputTokens != 50 ||
		ev.Usage.CacheCreate != 7 || ev.Usage.CacheRead != 3 {
		t.Errorf("usage = %+v, want {100 50 7 3}", ev.Usage)
	}
	if ev.Text != "done." {
		t.Errorf("Text = %q, want \"done.\"", ev.Text)
	}
	if ev.IsError {
		t.Errorf("IsError = true, want false")
	}
}

func TestParseEvent_AssistantWithText(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"},{"type":"tool_use","id":"abc","name":"Bash"}]}}`)
	ev, ok := parseEvent(line)
	if !ok {
		t.Fatalf("parseEvent returned ok=false for assistant line")
	}
	if ev.Type != "assistant" {
		t.Errorf("type = %q, want assistant", ev.Type)
	}
	if ev.Text != "Hello" {
		t.Errorf("Text = %q, want \"Hello\" (first text block)", ev.Text)
	}
}

func TestParseEvent_SystemInitIgnoredFields(t *testing.T) {
	line := []byte(`{"type":"system","subtype":"init","model":"claude-sonnet-4-6"}`)
	ev, ok := parseEvent(line)
	if !ok {
		t.Fatalf("parseEvent returned ok=false for system line")
	}
	if ev.Type != "system" {
		t.Errorf("type = %q, want system", ev.Type)
	}
}

func TestParseEvent_NonJSONLine(t *testing.T) {
	if _, ok := parseEvent([]byte("not json at all")); ok {
		t.Errorf("parseEvent returned ok=true for non-JSON line")
	}
}

func TestParseEvent_EmptyLine(t *testing.T) {
	if _, ok := parseEvent([]byte("")); ok {
		t.Errorf("parseEvent returned ok=true for empty line")
	}
}

func TestIsQuotaMessage(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"Claude AI usage limit reached. Try again at 5pm UTC.", true},
		{"claude ai USAGE LIMIT REACHED — try again later", true},
		{"Anthropic API usage limit reached", true},
		{"all good, no problems here", false},
		{"", false},
		{"rate limit", false}, // we anchor on the specific phrase, not "rate"
	}
	for _, c := range cases {
		if got := isQuotaMessage(c.in); got != c.want {
			t.Errorf("isQuotaMessage(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
