package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/json-validator/validator"
)

var (
	schemaFlag      string
	jsonOutputFlag  bool
	quietFlag       bool
	draftFlag       string
	noAssertFormat  bool
	ineffectiveJSON bool
)

func init() {
	rootCmd.Flags().StringVarP(&schemaFlag, "schema", "s", "", "path or URL to JSON Schema (overrides $schema in document)")
	rootCmd.Flags().BoolVar(&jsonOutputFlag, "json", false, "output errors as JSON, with a trailing comma after the last member of every multi-line object and array")
	rootCmd.Flags().BoolVar(&ineffectiveJSON, "ineffective-json", false, "with --json, write strict JSON: no trailing commas")
	rootCmd.Flags().BoolVarP(&quietFlag, "quiet", "q", false, "suppress output; exit code only")
	rootCmd.Flags().StringVarP(&draftFlag, "draft", "d", "2020", "default draft version when schema has no $schema (4, 6, 7, 2019, 2020)")
	rootCmd.Flags().BoolVar(&noAssertFormat, "no-assert-format", false, "disable format assertions (format becomes annotation-only per spec)")

	rootCmd.MarkFlagsMutuallyExclusive("json", "quiet")
}

var rootCmd = &cobra.Command{
	Use:   "json-validator [flags] [files...]",
	Short: "Validate JSON/JSONC files against JSON Schema",
	Long: `Validate JSON and JSONC files against JSON Schema (2020-12 by default).

By default, the schema is determined from the $schema field in each document.
Use --schema to override with a local file path or URL.

Supports JSON with Comments (JSONC): // line comments, /* block comments */,
and trailing commas are accepted in every document and schema. Validation
never rewrites an input file, so a trailing comma in it stays.

--json output writes a trailing comma after the last member of every
multi-line object and array. Add --ineffective-json for strict JSON.

Format assertions (email, date-time, uri, etc.) are enforced by default.
Use --no-assert-format to disable, or set the environment variable
JSON_VALIDATION_ALLOW_SILENT_FAILURES=assert-format`,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          run,
}

// errValidationFailed marks the ordinary negative result -- documents were read and found invalid.
var errValidationFailed = errors.New("validation failed")

// Execute runs the CLI and reports any failure that is NOT an ordinary invalid
// document on stderr.
func Execute() error {
	err := rootCmd.Execute()
	if err != nil && !errors.Is(err, errValidationFailed) {
		fmt.Fprintf(rootCmd.ErrOrStderr(), "json-validator: %v\n", err)
	}
	return err
}

func run(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return cmd.Help()
		}
	}

	// The library reads no environment: behavior must follow from Options alone
	// for the programs that embed it.
	opts := validator.Options{
		SchemaPath:     schemaFlag,
		Draft:          draftFlag,
		NoAssertFormat: noAssertFormat || validator.SilentFailureAllowed("assert-format"),
	}

	var compiled *jsonschema.Schema
	if schemaFlag != "" {
		var err error
		compiled, err = compileSharedSchema(opts)
		if err != nil {
			return err
		}
	}

	var results []validator.Result
	if len(args) == 0 {
		results = append(results, validator.Validate(os.Stdin, "<stdin>", compiled, opts))
	} else {
		for _, path := range args {
			results = append(results, validator.ValidateFile(path, compiled, opts))
		}
	}

	if !quietFlag {
		if err := printResults(cmd, results); err != nil {
			return err
		}
	}

	for _, r := range results {
		if !r.Valid || r.Err != nil {
			return errValidationFailed
		}
	}
	return nil
}

func compileSharedSchema(opts validator.Options) (*jsonschema.Schema, error) {
	c, err := validator.NewCompiler(opts)
	if err != nil {
		return nil, err
	}
	sch, err := validator.CompileSchema(c, opts.SchemaPath)
	if err != nil {
		return nil, fmt.Errorf("compiling schema %s: %w", opts.SchemaPath, err)
	}
	return sch, nil
}

func printResults(cmd *cobra.Command, results []validator.Result) error {
	if jsonOutputFlag {
		return printJSON(cmd, results)
	}
	return printHuman(cmd, results)
}

func printHuman(cmd *cobra.Command, results []validator.Result) error {
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(errOut, "%s: error: %v\n", r.File, r.Err)
			continue
		}
		if r.Error != nil {
			fmt.Fprintf(errOut, "%s: INVALID\n", r.File)
			fmt.Fprintln(errOut, r.Error)
			continue
		}
		fmt.Fprintf(out, "%s: valid\n", r.File)
	}
	return nil
}

type jsonFileResult struct {
	File   string `json:"file"`
	Valid  bool   `json:"valid"`
	Error  string `json:"error,omitempty"`
	Errors any    `json:"errors,omitempty"`
}

func printJSON(cmd *cobra.Command, results []validator.Result) error {
	out := make([]jsonFileResult, len(results))
	for i, r := range results {
		out[i] = jsonFileResult{
			File:  r.File,
			Valid: r.Valid && r.Err == nil,
		}
		if r.Err != nil {
			out[i].Error = r.Err.Error()
		}
		if r.Error != nil {
			out[i].Errors = r.Error.BasicOutput()
		}
	}
	return writeJSON(cmd.OutOrStdout(), out, !ineffectiveJSON)
}
