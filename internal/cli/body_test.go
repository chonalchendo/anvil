package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// withStdin replaces os.Stdin with a pipe whose data is fully written and the
// write-end closed before returning. This ensures fi.Size()>0 is visible to
// stdinIsPipe() without a race, since stdinIsPipe now requires bytes on the
// pipe to treat it as a body source.
func withStdin(t *testing.T, data string) func() {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	// Write synchronously; small test payloads fit in the kernel pipe buffer
	// (typically 64 KiB) so this never blocks. Close write-end before returning
	// so Size()>0 is already stable when the caller calls readBody.
	if _, err := io.WriteString(w, data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
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
	got, err := readBody(cmd, "hello body", "")
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
	got, err := readBody(cmd, "", "")
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
	if _, err := readBody(cmd, "from flag", ""); err == nil {
		t.Error("expected error when --body and stdin both supplied")
	}
}

// withSocketStdin replaces os.Stdin with one half of a connected unix-socket
// pair. The other half is held by the test and never written to — mirroring
// the shape the Claude Code harness presents to subprocess stdin. Returns a
// cleanup that restores os.Stdin and closes both fds.
func withSocketStdin(t *testing.T) func() {
	t.Helper()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	stdin := os.NewFile(uintptr(fds[0]), "test-stdin-socket")
	peer := os.NewFile(uintptr(fds[1]), "test-stdin-socket-peer")
	orig := os.Stdin
	os.Stdin = stdin
	return func() {
		os.Stdin = orig
		_ = stdin.Close()
		_ = peer.Close()
	}
}

// TestReadBody_SocketStdinDoesNotHang is the regression test for the bug
// where a unix-socket stdin (attached by an agent harness with no writer)
// caused readBody to block forever inside io.ReadAll. The fix narrows
// stdinIsPipe to named pipes and non-empty regular files; sockets should
// resolve as "no piped data" and return immediately.
func TestReadBody_SocketStdinDoesNotHang(t *testing.T) {
	cleanup := withSocketStdin(t)
	defer cleanup()

	cmd := &cobra.Command{}
	done := make(chan struct{})
	var got string
	var err error
	go func() {
		got, err = readBody(cmd, "", "")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("readBody hung on socket stdin")
	}
	if err != nil {
		t.Fatalf("readBody: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty (socket stdin should not be read)", got)
	}
}

// Same shape, but with --body set. The earlier bug discarded stdin before
// returning the error, which itself blocked on the socket.
func TestReadBody_SocketStdinWithFlagDoesNotHang(t *testing.T) {
	cleanup := withSocketStdin(t)
	defer cleanup()

	cmd := &cobra.Command{}
	done := make(chan struct{})
	var got string
	var err error
	go func() {
		got, err = readBody(cmd, "from flag", "")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("readBody hung on socket stdin with --body set")
	}
	if err != nil {
		t.Fatalf("readBody: %v", err)
	}
	if got != "from flag\n" {
		t.Errorf("got %q, want %q", got, "from flag\n")
	}
}

func TestReadBody_NoFlagNoStdin(t *testing.T) {
	r, w, _ := os.Pipe()
	_ = w.Close()
	orig := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = orig; _ = r.Close() }()

	cmd := &cobra.Command{}
	got, err := readBody(cmd, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// TestReadBody_EmptyPipeWithBodyFileDoesNotCollide is the regression test for
// the bug where an empty named pipe (e.g. `printf '' | cmd`) was misread as a
// competing body source and rejected with "mutually exclusive". The fix
// requires fi.Size()>0 before treating a named pipe as carrying data.
func TestReadBody_EmptyPipeWithBodyFileDoesNotCollide(t *testing.T) {
	// Simulate `printf '' | cmd --body-file f`: a named pipe (r) with the
	// write end closed immediately, so Size()==0.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Close() // write end closed; pipe is empty

	orig := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = orig; _ = r.Close() }()

	// Write a temp body file for --body-file to read.
	f := t.TempDir() + "/body.md"
	if err := os.WriteFile(f, []byte("body content\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	got, err := readBody(cmd, "", f)
	if err != nil {
		t.Fatalf("unexpected error (collision guard still firing?): %v", err)
	}
	if got != "body content\n" {
		t.Errorf("got %q, want %q", got, "body content\n")
	}
}
