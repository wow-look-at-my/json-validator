package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/json-validator/validator"
)

var (
	schemaFlag     string
	jsonOutputFlag bool
	quietFlag      bool
	draftFlag      string
	noAssertFormat bool
)

func init() {
	rootCmd.Flags().StringVarP(&schemaFlag, "schema", "s", "", "path or URL to JSON Schema (overrides $schema in document)")
	rootCmd.Flags().BoolVar(&jsonOutputFlag, "json", false, "output errors as JSON")
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
and trailing commas are stripped before validation.

Format assertions (email, date-time, uri, etc.) are enforced by default.
Use --no-assert-format to disable, or set the environment variable
JSON_VALIDATION_ALLOW_SILENT_FAILURES=assert-format`,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          run,
}

// errValidationFailed marks the ordinary negative result -- documents were
// read and found invalid -- as distinct from a failure to run at all. Each one
// has already been reported per file (or --quiet asked for silence), so
// Execute prints nothing further for it; the exit code carries it.
var errValidationFailed = errors.New("validation failed")

// Execute runs the CLI and reports any failure that is NOT an ordinary invalid
// document on stderr.
//
// This is load-bearing. rootCmd sets SilenceErrors, so cobra prints nothing
// itself, and main() only reads the exit code -- which left every way of
// failing to RUN silent: a schema that does not exist, one that is not valid
// JSON, one whose $ref cannot be resolved, a mistyped flag. All of them exited
// 1 having printed NOTHING, the worst failure mode for a gate CI depends on:
// red with no reason, and nothing to search for.
//
// --quiet does not suppress this. It means "exit code only" about RESULTS, the
// way `grep -q` still prints "No such file or directory" for a missing file:
// suppressing the answer is not the same as hiding the fact that no answer
// could be computed.
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

	// The library reads no environment: behavior must follow from Options
	// alone for the programs that embed it. So the documented env escape
	// hatch is resolved HERE and folded into the option it stands for.
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
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
