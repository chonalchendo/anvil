package ui

import (
	"strings"
	"testing"
)

func inOrder(t *testing.T, body string, parts ...string) {
	t.Helper()
	at := 0
	for _, p := range parts {
		i := strings.Index(body[at:], p)
		if i < 0 {
			t.Fatalf("%q missing or out of order in:\n%s", p, body)
		}
		at += i + len(p)
	}
}

// Warrant: the hint bar must list only the keys its page handles, so a dead key never shows.
func TestKeyHints_PerPage(t *testing.T) {
	h, _ := seed(t)
	jk := []string{">j<", ">k<", ">o<", ">⇧O<", ">⇧C<"}
	arrows := []string{">↑<", ">↓<", ">←<", ">→<"}
	cases := []struct {
		path       string
		want, dead []string
	}{
		{"/", []string{">⌘K<"}, append(append([]string{}, jk...), arrows...)},
		{"/type/decision", []string{">⌘K<"}, append(append([]string{}, jk...), arrows...)},
		{decisionPath, append([]string{">⌘K<"}, jk...), arrows},
		{stackPath, append([]string{">⌘K<"}, jk...), arrows},
	}
	for _, c := range cases {
		_, body := do(h, "GET", c.path)
		i := strings.Index(body, `<footer class="keys">`)
		if i < 0 || strings.Index(body, "</main>") > i {
			t.Errorf("%s: footer missing or before main", c.path)
			continue
		}
		bar := body[i : i+strings.Index(body[i:], "</footer>")]
		for _, k := range c.want {
			if !strings.Contains(bar, k) {
				t.Errorf("%s: hint bar lacks %s", c.path, k)
			}
		}
		for _, k := range c.dead {
			if strings.Contains(bar, k) {
				t.Errorf("%s: hint bar lists dead key %s", c.path, k)
			}
		}
	}
}
