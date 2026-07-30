package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

// renderSchemaErr with --json must wrap in jsonRendered so errorHandler skips
// fang's stderr box — otherwise create/set/promote double-print (envelope on
// stdout plus ERROR box on stderr) for exactly the callers that parse stdout.
func TestRenderSchemaErrJSONWrapsJSONRendered(t *testing.T) {
	cmd := &cobra.Command{}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := renderSchemaErr(cmd, nil, "some/path.md", errors.New("boom"), true)
	if !errors.Is(err, ErrSchemaInvalid) {
		t.Fatalf("error must unwrap to ErrSchemaInvalid, got: %v", err)
	}
	var jr jsonRendered
	if !errors.As(err, &jr) {
		t.Fatalf("--json error must be jsonRendered so fang's box is skipped, got %T", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"error":"schema_invalid"`)) {
		t.Fatalf("envelope must land on stdout, got: %s", stdout.String())
	}
}

func TestRenderSchemaErrHumanPathUnwrapped(t *testing.T) {
	cmd := &cobra.Command{}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := renderSchemaErr(cmd, nil, "some/path.md", errors.New("boom"), false)
	var jr jsonRendered
	if errors.As(err, &jr) {
		t.Fatalf("human path must return the bare error so fang renders it")
	}
	if !errors.Is(err, ErrSchemaInvalid) {
		t.Fatalf("error must be ErrSchemaInvalid, got: %v", err)
	}
}
