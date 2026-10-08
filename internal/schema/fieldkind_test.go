package schema

import "testing"

func TestFieldKind_IssueScalars(t *testing.T) {
	for _, f := range []string{"title", "status", "project", "milestone"} {
		k, err := FieldKind("issue", f)
		if err != nil {
			t.Fatalf("FieldKind(issue, %s): %v", f, err)
		}
		if k != KindScalar {
			t.Errorf("FieldKind(issue, %s) = %v, want KindScalar", f, k)
		}
	}
}

func TestFieldKind_IssueArrays(t *testing.T) {
	for _, f := range []string{"tags", "aliases", "acceptance", "related"} {
		k, err := FieldKind("issue", f)
		if err != nil {
			t.Fatalf("FieldKind(issue, %s): %v", f, err)
		}
		if k != KindArray {
			t.Errorf("FieldKind(issue, %s) = %v, want KindArray", f, k)
		}
	}
}

func TestFieldKind_UnknownField(t *testing.T) {
	k, err := FieldKind("issue", "not_a_real_field")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if k != KindUnknown {
		t.Errorf("FieldKind(issue, not_a_real_field) = %v, want KindUnknown", k)
	}
}

func TestFieldKind_UnknownType(t *testing.T) {
	if _, err := FieldKind("nope", "title"); err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestFieldIsInteger(t *testing.T) {
	for _, tc := range []struct {
		field string
		want  bool
	}{{"cost_rounds", true}, {"title", false}, {"no_such_field", false}} {
		got, err := FieldIsInteger("issue", tc.field)
		if err != nil || got != tc.want {
			t.Errorf("FieldIsInteger(issue, %s) = %v, %v; want %v", tc.field, got, err, tc.want)
		}
	}
}

func TestFieldEnum(t *testing.T) {
	for _, tc := range []struct{ field, first string }{
		{"status", "open"},
		{"severity", "low"},
	} {
		got, err := FieldEnum("issue", tc.field)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) == 0 || got[0] != tc.first {
			t.Errorf("FieldEnum(issue, %s) = %v, want first value %s", tc.field, got, tc.first)
		}
	}
}
