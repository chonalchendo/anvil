package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// withStdin replaces os.Stdin with a pipe carrying data and a non-tty mode.
func withStdin(t *testing.T, data string) func() {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_, _ = io.WriteString(w, data)
		_ = w.Close()
	}()
	orig := os.Stdin
	os.Stdin = r
	return func() {
		os.Stdin = orig
		_ = r.Close()
	}
}

func TestReadBody_FlagWins(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	got, err := readBody(cmd, "hello body")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello body\n" {
		t.Errorf("got %q, want %q", got, "hello body\n")
	}
}

func TestReadBody_StdinPipe(t *testing.T) {
	cleanup := withStdin(t, "from stdin\nline2")
	defer cleanup()
	cmd := &cobra.Command{}
	got, err := readBody(cmd, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("body must end with newline, got %q", got)
	}
	if !strings.Contains(got, "from stdin") {
		t.Errorf("got %q", got)
	}
}

func TestReadBody_BothSupplied_Errors(t *testing.T) {
	cleanup := withStdin(t, "from stdin")
	defer cleanup()
	cmd := &cobra.Command{}
	if _, err := readBody(cmd, "from flag"); err == nil {
		t.Error("expected error when --body and stdin both supplied")
	}
}

func TestReadBody_NoFlagNoStdin(t *testing.T) {
	r, w, _ := os.Pipe()
	_ = w.Close()
	orig := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = orig; _ = r.Close() }()

	cmd := &cobra.Command{}
	got, err := readBody(cmd, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
