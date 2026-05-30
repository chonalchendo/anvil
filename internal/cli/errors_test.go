package cli

import (
	"strings"
	"testing"
)

func TestNamedArgs_ExactArity(t *testing.T) {
	validate := namedArgs("anvil show <type> <id>", []string{"<type>", "<id>"}, 2, 2)

	tests := []struct {
		name       string
		args       []string
		wantErr    bool
		wantSubstr string
	}{
		{name: "exact accepted", args: []string{"issue", "x"}, wantErr: false},
		{name: "under-arity names missing positional", args: []string{"issue"}, wantErr: true, wantSubstr: "missing required argument <id>"},
		{name: "over-arity rejects surplus", args: []string{"issue", "x", "y"}, wantErr: true, wantSubstr: "too many arguments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(nil, tt.args)
			if tt.wantErr != (err != nil) {
				t.Fatalf("args=%v: wantErr=%v got err=%v", tt.args, tt.wantErr, err)
			}
			if tt.wantSubstr != "" && !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Fatalf("args=%v: error %q missing %q", tt.args, err.Error(), tt.wantSubstr)
			}
		})
	}
}

func TestNamedArgs_TypePositionalListsValidTypes(t *testing.T) {
	validate := namedArgs("anvil list <type>", []string{"<type>"}, 1, 1)
	err := validate(nil, nil)
	if err == nil {
		t.Fatal("expected error for missing <type>")
	}
	if !strings.Contains(err.Error(), "missing required argument <type>") || !strings.Contains(err.Error(), "valid types:") {
		t.Fatalf("error %q missing named positional or valid-types listing", err.Error())
	}
}

func TestNamedArgs_MinimumOnlyAcceptsSurplus(t *testing.T) {
	// set is minimum-only (maxCount=-1): surplus values are the [<value>...] variadic.
	validate := namedArgs("anvil set <type> <id> <field> [<value>...]", []string{"<type>", "<id>", "<field>"}, 3, -1)

	if err := validate(nil, []string{"issue", "x", "tags", "a", "b", "c"}); err != nil {
		t.Fatalf("minimum-only should accept surplus, got %v", err)
	}
	if err := validate(nil, []string{"issue", "x"}); err == nil {
		t.Fatal("expected under-arity error for set with 2 args")
	} else if !strings.Contains(err.Error(), "missing required argument <field>") {
		t.Fatalf("error %q missing named positional <field>", err.Error())
	}
}
