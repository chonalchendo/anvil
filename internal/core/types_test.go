package core

import "testing"

func TestParseType_RoundTrip(t *testing.T) {
	for _, want := range AllTypes {
		got, err := ParseType(string(want))
		if err != nil {
			t.Errorf("ParseType(%q) error: %v", want, err)
			continue
		}
		if got != want {
			t.Errorf("ParseType(%q) = %q, want %q", want, got, want)
		}
	}
}

func TestParseType_Unknown(t *testing.T) {
	if _, err := ParseType("bogus"); err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestType_Dir(t *testing.T) {
	cases := map[Type]string{
		TypeInbox:     "00-inbox",
		TypeIssue:     "70-issues",
		TypePlan:      "80-plans",
		TypeMilestone: "85-milestones",
		TypeDecision:  "30-decisions",
	}
	for tp, want := range cases {
		if got := tp.Dir(); got != want {
			t.Errorf("%s.Dir() = %q, want %q", tp, got, want)
		}
	}
}

func TestType_Dir_PanicsOnUnknown(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for unknown type")
		}
	}()
	Type("bogus").Dir()
}

func TestParseType_AcceptsNewTypes(t *testing.T) {
	for _, name := range []string{"learning", "thread", "sweep", "transcript", "session"} {
		got, err := ParseType(name)
		if err != nil {
			t.Errorf("ParseType(%q): %v", name, err)
		}
		if string(got) != name {
			t.Errorf("ParseType(%q) = %q", name, got)
		}
	}
}

func TestType_Dir_NewTypes(t *testing.T) {
	cases := map[Type]string{
		TypeLearning:   "20-learnings",
		TypeThread:     "60-threads",
		TypeSweep:      "50-sweeps",
		TypeTranscript: "10-sessions/raw",
		TypeSession:    "10-sessions/distilled",
	}
	for tt, want := range cases {
		if got := tt.Dir(); got != want {
			t.Errorf("%s.Dir() = %q, want %q", tt, got, want)
		}
	}
}
