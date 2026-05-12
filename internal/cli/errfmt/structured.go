package errfmt

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Structured is the canonical anvil error shape: a stable `code` for agent
// dispatch plus ordered key/value fields. Construct one with NewStructured,
// then chain Set/Wrap. JSON output preserves insertion order so the on-the-wire
// shape stays diffable.
type Structured struct {
	Code   string
	Fields []KV
	wrap   error
}

// KV is one labelled field on a Structured error.
type KV struct {
	Key   string
	Value any
}

// NewStructured returns an empty Structured carrying the given code.
func NewStructured(code string) *Structured {
	return &Structured{Code: code}
}

// Set appends or overwrites a field. The most recent Set for a given key wins;
// position in the field list is the position of the FIRST Set for that key, so
// updates after construction don't reorder the JSON output.
func (e *Structured) Set(key string, value any) *Structured {
	for i, kv := range e.Fields {
		if kv.Key == key {
			e.Fields[i].Value = value
			return e
		}
	}
	e.Fields = append(e.Fields, KV{Key: key, Value: value})
	return e
}

// Wrap stores a sentinel error so errors.Is keeps working. Typical use: wrap a
// package-level "category" sentinel (e.g. ErrPlanDAG) so callers can dispatch
// without depending on the concrete constructor.
func (e *Structured) Wrap(err error) *Structured {
	e.wrap = err
	return e
}

func (e *Structured) Unwrap() error { return e.wrap }

func (e *Structured) Error() string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "[%s]", e.Code)
	for _, kv := range e.Fields {
		fmt.Fprintf(&b, "\n  %s: %s", kv.Key, formatStructuredValue(kv.Value))
	}
	return b.String()
}

func (e *Structured) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	codeJSON, _ := json.Marshal(e.Code)
	b.WriteString(`"code":`)
	b.Write(codeJSON)
	for _, kv := range e.Fields {
		b.WriteByte(',')
		keyJSON, _ := json.Marshal(kv.Key)
		b.Write(keyJSON)
		b.WriteByte(':')
		valJSON, err := json.Marshal(kv.Value)
		if err != nil {
			return nil, err
		}
		b.Write(valJSON)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// formatStructuredValue renders a field for the bracket-tagged text form.
// Strings render verbatim; []string joins with ", " (preserves legacy shape
// used by IllegalTransition/UnsupportedForType); other types use %v.
func formatStructuredValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []string:
		out := ""
		for i, s := range x {
			if i > 0 {
				out += ", "
			}
			out += s
		}
		return out
	default:
		return fmt.Sprintf("%v", v)
	}
}
