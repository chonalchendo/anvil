package output_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/chonalchendo/anvil/internal/cli/output"
)

func TestWriteListJSON_Truncated(t *testing.T) {
	var buf bytes.Buffer
	items := []map[string]any{{"id": "a"}, {"id": "b"}}
	if err := output.WriteListJSON(&buf, items, 312, 2); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"items":     []any{map[string]any{"id": "a"}, map[string]any{"id": "b"}},
		"total":     float64(312),
		"returned":  float64(2),
		"truncated": true,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatal(diff)
	}
}

func TestWriteListJSON_Complete(t *testing.T) {
	var buf bytes.Buffer
	items := []map[string]any{{"id": "a"}}
	if err := output.WriteListJSON(&buf, items, 1, 1); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["truncated"] != false {
		t.Errorf("truncated = %v, want false", got["truncated"])
	}
}

func TestWriteListJSON_EmptyItems(t *testing.T) {
	var buf bytes.Buffer
	if err := output.WriteListJSON[map[string]any](&buf, nil, 0, 0); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"items":[]`)) {
		t.Errorf("expected empty array, got %s", buf.String())
	}
}
