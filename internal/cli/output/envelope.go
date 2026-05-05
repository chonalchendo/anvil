// Package output centralises JSON envelopes and stderr hints used across read verbs.
package output

import (
	"encoding/json"
	"io"
)

// WriteListJSON emits the canonical bounded-list envelope. items may be any
// slice serialisable to JSON; total is the unfiltered match count, returned
// is len(items) after limit. truncated is derived (returned < total).
func WriteListJSON[T any](w io.Writer, items []T, total, returned int) error {
	if items == nil {
		items = []T{}
	}
	env := struct {
		Items     []T  `json:"items"`
		Total     int  `json:"total"`
		Returned  int  `json:"returned"`
		Truncated bool `json:"truncated"`
	}{items, total, returned, returned < total}
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}
