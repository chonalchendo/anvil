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
