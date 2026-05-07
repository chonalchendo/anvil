package claude

import "testing"

func TestTranslateEffort(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"low", "minimal"},
		{"medium", "low"},
		{"high", "medium"},
		{"xhigh", "high"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := translateEffort(c.in); got != c.want {
				t.Errorf("translateEffort(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
