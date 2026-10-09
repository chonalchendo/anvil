package ui

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func cssTokens(t *testing.T) map[string]string {
	t.Helper()
	b, err := staticFS.ReadFile("static/anvil.css")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, m := range regexp.MustCompile(`--([a-z0-9-]+):\s*(#[0-9a-fA-F]{6})`).FindAllStringSubmatch(string(b), -1) {
		out[m[1]] = m[2]
	}
	return out
}

func luminance(t *testing.T, hex string) float64 {
	t.Helper()
	var l float64
	for i, w := range []float64{0.2126, 0.7152, 0.0722} {
		n, err := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
		if err != nil {
			t.Fatal(err)
		}
		c := float64(n) / 255
		if c <= 0.03928 {
			c /= 12.92
		} else {
			c = math.Pow((c+0.055)/1.055, 2.4)
		}
		l += w * c
	}
	return l
}

func TestCSS_TextContrast(t *testing.T) {
	tok := cssTokens(t)
	texts := []string{"text", "text-2", "muted", "accent"}
	for name := range tok {
		if strings.HasPrefix(name, "status-") || strings.HasPrefix(name, "type-") {
			texts = append(texts, name)
		}
	}
	for _, text := range texts {
		for _, surface := range []string{"bg", "panel", "raise", "sel"} {
			a, b := luminance(t, tok[text]), luminance(t, tok[surface])
			if a < b {
				a, b = b, a
			}
			if ratio := (a + 0.05) / (b + 0.05); ratio < 4.5 {
				t.Errorf("%s on %s = %.2f, want >= 4.5", text, surface, ratio)
			}
		}
	}
}

func TestCSS_StatusTokensAndFullWidthMain(t *testing.T) {
	tok := cssTokens(t)
	n := 0
	for name := range tok {
		if strings.HasPrefix(name, "status-") {
			n++
		}
	}
	if n < 4 {
		t.Errorf("%d --status- tokens, want >= 4", n)
	}
	b, _ := staticFS.ReadFile("static/anvil.css")
	m := regexp.MustCompile(`(?m)^main\s*\{[^}]*\}`).FindString(string(b))
	if m == "" || strings.Contains(m, "max-width") {
		t.Errorf("main rule = %q, want present with no max-width", m)
	}
}

func TestStatusGlyphCarriesHueClass(t *testing.T) {
	h, _ := seed(t)
	for _, p := range []string{"/type/decision", "/"} {
		_, body := do(h, "GET", p)
		if !strings.Contains(body, `class="status status-`) {
			t.Errorf("%s lacks a status-<value> class on the glyph span", p)
		}
	}
}
