package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestShowTemplateDesigns(t *testing.T) {
	cases := map[string][]string{
		"product-design": core.RequiredProductDesignSections,
		"system-design":  core.RequiredSystemDesignSections,
	}
	for typ, headings := range cases {
		t.Run(typ, func(t *testing.T) {
			v := setupVault(t)
			out, err := runArgs(t, "create", typ, "--show-template")
			if err != nil {
				t.Fatalf("--show-template: %v\n%s", err, out)
			}
			rest := out
			for _, h := range headings {
				i := strings.Index(rest, h)
				if i < 0 {
					t.Fatalf("heading %q missing or out of order:\n%s", h, out)
				}
				rest = rest[i+len(h):]
			}
			var files []string
			_ = filepath.WalkDir(v, func(p string, d os.DirEntry, _ error) error {
				if d != nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
					files = append(files, p)
				}
				return nil
			})
			if len(files) != 0 {
				t.Errorf("--show-template wrote files: %v", files)
			}
		})
	}
}
