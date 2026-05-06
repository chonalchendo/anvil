package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
)

func printValidationErrors(cmd *cobra.Command, errs []*errfmt.ValidationError) {
	for i, f := range errs {
		if i > 0 {
			cmd.PrintErrln("")
		}
		cmd.PrintErrln(fmt.Sprintf("[%s] %s", f.Code, f.Path))
		if f.Field != "" {
			cmd.PrintErrln(fmt.Sprintf("  field: %s", f.Field))
		}
		if f.Got != "" {
			cmd.PrintErrln(fmt.Sprintf("  got: %s", f.Got))
		}
		if f.Suggest != "" {
			cmd.PrintErrln(fmt.Sprintf("  suggest: %s", f.Suggest))
		}
		if f.Expected != nil {
			cmd.PrintErrln(fmt.Sprintf("  expected: %v", f.Expected))
		}
		if f.Note != "" {
			cmd.PrintErrln(fmt.Sprintf("  note: %s", f.Note))
		}
		if f.Fix != "" {
			cmd.PrintErrln(fmt.Sprintf("  fix: %s", f.Fix))
		}
	}
}

func renderSchemaErr(cmd *cobra.Command, path string, err error) error {
	errs := schemaErrToValidationErrors(path, err)
	printValidationErrors(cmd, errs)
	return ErrSchemaInvalid
}
