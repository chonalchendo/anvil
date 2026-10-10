package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

func TestCheckFieldCaps_JSONEnvelope(t *testing.T) {
	long := strings.Repeat("x", maxDescriptionChars+5)
	tests := []struct {
		name, desc, goal, code string
		got                    int
	}{
		{"description", long, "g", "description_too_long", len(long)},
		{"goal", "d", long, "goal_too_long", len(long)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkFieldCaps(core.TypeIssue, tt.desc, tt.goal, true)
			var s *errfmt.Structured
			if !errors.As(err, &s) {
				t.Fatalf("want *Structured, got %T", err)
			}
			b, _ := json.Marshal(s)
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			if m["code"] != tt.code || int(m["got"].(float64)) != tt.got || int(m["max"].(float64)) != 120 {
				t.Errorf("envelope = %s", b)
			}
		})
	}
}

func TestCheckFieldCaps_TextUnchanged(t *testing.T) {
	err := checkFieldCaps(core.TypeIssue, strings.Repeat("x", 130), "g", false)
	if err == nil || !strings.Contains(err.Error(), "--description too long: 130 chars (max 120)") {
		t.Errorf("err = %v", err)
	}
}
