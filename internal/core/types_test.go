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
		TypeContract:  "75-contracts",
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
	for _, name := range []string{"learning", "thread", "sweep", "session"} {
		got, err := ParseType(name)
		if err != nil {
			t.Errorf("ParseType(%q): %v", name, err)
		}
		if string(got) != name {
			t.Errorf("ParseType(%q) = %q", name, got)
		}
	}
}

func TestParseType_RejectsTranscript(t *testing.T) {
	if _, err := ParseType("transcript"); err == nil {
		t.Error("expected error for retired type \"transcript\"")
	}
}

func TestType_Dir_NewTypes(t *testing.T) {
	cases := map[Type]string{
		TypeLearning: "20-learnings",
		TypeThread:   "60-threads",
		TypeSweep:    "50-sweeps",
		TypeSession:  "10-sessions",
	}
	for tt, want := range cases {
		if got := tt.Dir(); got != want {
			t.Errorf("%s.Dir() = %q, want %q", tt, got, want)
		}
	}
}

func TestParseType_AcceptsDesignTypes(t *testing.T) {
	for _, name := range []string{"product-design", "system-design"} {
		got, err := ParseType(name)
		if err != nil {
			t.Errorf("ParseType(%q): %v", name, err)
		}
		if string(got) != name {
			t.Errorf("ParseType(%q) = %q", name, got)
		}
	}
}

func TestType_Dir_DesignTypes(t *testing.T) {
	cases := map[Type]string{
		TypeProductDesign: "05-product-designs",
		TypeSystemDesign:  "06-system-designs",
	}
	for tp, want := range cases {
		if got := tp.Dir(); got != want {
			t.Errorf("%s.Dir() = %q, want %q", tp, got, want)
		}
	}
}

func TestType_SupportsProject(t *testing.T) {
	cases := map[Type]bool{
		TypeIssue:         true,
		TypePlan:          true,
		TypeMilestone:     true,
		TypeProductDesign: true,
		TypeSystemDesign:  true,
		TypeLearning:      true,
		TypeDecision:      true,
		TypeInbox:         false,
		TypeSession:       false,
		TypeSweep:         false,
		TypeThread:        false,
	}
	for tp, want := range cases {
		if got := tp.SupportsProject(); got != want {
			t.Errorf("%s.SupportsProject() = %v, want %v", tp, got, want)
		}
	}
}

func TestTypesSupportingProject_IncludesLearningAndDecision(t *testing.T) {
	got := TypesSupportingProject()
	have := make(map[string]bool, len(got))
	for _, s := range got {
		have[s] = true
	}
	for _, want := range []string{"learning", "decision", "issue", "plan", "milestone", "product-design", "system-design"} {
		if !have[want] {
			t.Errorf("TypesSupportingProject() missing %q; got %v", want, got)
		}
	}
}

func TestType_Path(t *testing.T) {
	root := "/v"
	cases := []struct {
		tp   Type
		id   string
		want string
	}{
		{TypeProductDesign, "product-design.anvil", "/v/05-product-designs/product-design.anvil.md"},
		{TypeSystemDesign, "system-design.anvil", "/v/06-system-designs/system-design.anvil.md"},
		{TypeSystemDesign, "system-design.anvil.build", "/v/06-system-designs/system-design.anvil.build.md"},
		{TypeIssue, "anvil.foo", "/v/70-issues/anvil.foo.md"},
		{TypeSweep, "0001-cli", "/v/50-sweeps/0001-cli.md"},
		{TypeInbox, "2026-05-04T12-00-00-x", "/v/00-inbox/2026-05-04T12-00-00-x.md"},
	}
	for _, c := range cases {
		if got := c.tp.Path(root, c.id); got != c.want {
			t.Errorf("%s.Path(%q,%q) = %q, want %q", c.tp, root, c.id, got, c.want)
		}
	}
}
