// Package validator validates JSON and JSONC documents against JSON Schema.
//
// It is consumed TWO ways, and both are first-class:
//
//   - the json-validator CLI / GitHub Action in this repo, and
//   - as a LIBRARY embedded in other Go programs -- webhook-runner validates
//     every hook.json against its published schema at load through this
//     package, so a manifest is checked by one implementation in CI and at
//     runtime rather than by two that drift.
//
// What being embeddable requires, and what this package therefore guarantees:
//
//   - The ZERO VALUE of Options works. Defaults live here, not in the CLI's
//     flag definitions, so `validator.Options{}` means draft 2020-12 with
//     format assertions on.
//   - Behavior is a pure function of Options. This package reads NO
//     environment variables; the CLI resolves
//     JSON_VALIDATION_ALLOW_SILENT_FAILURES into Options itself, because a
//     library that changes behavior on ambient env is a trap for its host.
//   - Schemas can come from memory, not just a path or URL (CompileBytes,
//     NewFromBytes) -- an embedder usually has its schema compiled in.
//   - Compile once, validate many (the Validator type): a server revalidating
//     on every reload must not recompile a constant schema each time.
//   - Nothing here writes to stdout/stderr or exits. Results are values.
package validator

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/tidwall/jsonc"
)

// DefaultDraft is the draft assumed when a schema does not name one and
// Options.Draft is empty.
const DefaultDraft = "2020"

type Result struct {
	File  string
	Valid bool
	Error *jsonschema.ValidationError
	Err   error
}

// AsError renders the result as a single error: nil when valid, the load /
// parse error when there was one, else the validation failure. Embedders
// usually want an error, not a struct to re-inspect.
func (r Result) AsError() error {
	if r.Err != nil {
		return r.Err
	}
	if r.Valid {
		return nil
	}
	return fmt.Errorf("%s", r.Detail())
}

// Detail renders a validation failure as one actionable line per failing
// location ("path/to/field: cause"), instead of the nested tree the CLI
// prints. A log line or a load error needs this shape.
func (r Result) Detail() string {
	if r.Err != nil {
		return r.Err.Error()
	}
	if r.Valid {
		return ""
	}
	if r.Error == nil {
		return "unknown validation error"
	}
	var lines []string
	var walk func(*jsonschema.ValidationError)
	walk = func(cur *jsonschema.ValidationError) {
		if len(cur.Causes) == 0 {
			loc := strings.Join(cur.InstanceLocation, "/")
			if loc == "" {
				loc = "(root)"
			}
			lines = append(lines, fmt.Sprintf("%s: %v", loc, cur.ErrorKind))
			return
		}
		for _, c := range cur.Causes {
			walk(c)
		}
	}
	walk(r.Error)
	if len(lines) == 0 {
		return r.Error.Error()
	}
	return strings.Join(lines, "; ")
}

// Options configures validation. The zero value is valid: draft 2020-12,
// format assertions ON (this package's deliberate deviation from the 2020-12
// spec default -- a `format` in the schema is meant to be enforced).
type Options struct {
	// SchemaPath is a file path or http(s) URL. Empty means each document's
	// own $schema decides.
	SchemaPath string
	// Draft is "4", "6", "7", "2019" or "2020"; empty means DefaultDraft.
	Draft string
	// NoAssertFormat turns format assertions off.
	NoAssertFormat bool
}

func (o Options) draft() string {
	if o.Draft == "" {
		return DefaultDraft
	}
	return o.Draft
}

func NewCompiler(opts Options) (*jsonschema.Compiler, error) {
	c := jsonschema.NewCompiler()

	draft, err := parseDraft(opts.draft())
	if err != nil {
		return nil, err
	}
	c.DefaultDraft(draft)

	if !opts.NoAssertFormat {
		c.AssertFormat()
	}

	c.UseLoader(jsonschema.SchemeURLLoader{
		"file":  jsonschema.FileLoader{},
		"http":  httpLoader{client: http.DefaultClient},
		"https": httpLoader{client: http.DefaultClient},
	})

	return c, nil
}

func CompileSchema(c *jsonschema.Compiler, schemaRef string) (*jsonschema.Schema, error) {
	return c.Compile(schemaRef)
}

// Validate checks one document against `compiled`, or against the schema its
// own $schema names when compiled is nil.
func Validate(r io.Reader, filename string, compiled *jsonschema.Schema, opts Options) Result {
	return doValidate(r, filename, compiled, opts)
}

// stripJSONC removes comments and trailing commas -- every input this package
// accepts is JSONC, on the CLI and in a host program alike.
func stripJSONC(raw []byte) []byte { return jsonc.ToJSON(raw) }

func doValidate(r io.Reader, filename string, compiled *jsonschema.Schema, opts Options) Result {
	res := Result{File: filename}

	raw, err := io.ReadAll(r)
	if err != nil {
		res.Err = fmt.Errorf("reading input: %w", err)
		return res
	}

	cleaned := stripJSONC(raw)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(cleaned))
	if err != nil {
		res.Err = fmt.Errorf("parsing JSON: %w", err)
		return res
	}

	schema := compiled
	if schema == nil {
		schemaURI, err := extractSchemaURI(doc)
		if err != nil {
			res.Err = err
			return res
		}

		c, err := NewCompiler(opts)
		if err != nil {
			res.Err = err
			return res
		}

		schema, err = c.Compile(schemaURI)
		if err != nil {
			res.Err = fmt.Errorf("compiling schema: %w", err)
			return res
		}
	}

	err = schema.Validate(doc)
	if err != nil {
		if vErr, ok := err.(*jsonschema.ValidationError); ok {
			res.Error = vErr
			return res
		}
		res.Err = err
		return res
	}

	res.Valid = true
	return res
}

func ValidateFile(path string, compiled *jsonschema.Schema, opts Options) Result {
	f, err := os.Open(path)
	if err != nil {
		return Result{File: path, Err: err}
	}
	defer f.Close()
	return Validate(f, path, compiled, opts)
}

func extractSchemaURI(doc any) (string, error) {
	obj, ok := doc.(map[string]any)
	if !ok {
		return "", fmt.Errorf("no schema: document is not a JSON object; use --schema flag")
	}
	v, exists := obj["$schema"]
	if !exists {
		return "", fmt.Errorf("no schema: document has no $schema field; use --schema flag")
	}
	uri, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("no schema: $schema is not a string; use --schema flag")
	}
	return uri, nil
}

func parseDraft(s string) (*jsonschema.Draft, error) {
	switch s {
	case "4":
		return jsonschema.Draft4, nil
	case "6":
		return jsonschema.Draft6, nil
	case "7":
		return jsonschema.Draft7, nil
	case "2019":
		return jsonschema.Draft2019, nil
	case "2020":
		return jsonschema.Draft2020, nil
	default:
		return nil, fmt.Errorf("unsupported draft version: %s (valid: 4, 6, 7, 2019, 2020)", s)
	}
}

// SilentFailureAllowed reports whether `feature` appears in the value of
// JSON_VALIDATION_ALLOW_SILENT_FAILURES (comma, semicolon or space
// delimited).
//
// It is a HELPER for the CLI, never consulted by validation itself: the
// library's behavior must follow from Options alone, so the CLI calls this and
// sets the corresponding Option. An embedder that wants the same env-driven
// escape hatch can call it too -- explicitly.
func SilentFailureAllowed(feature string) bool {
	env := os.Getenv("JSON_VALIDATION_ALLOW_SILENT_FAILURES")
	if env == "" {
		return false
	}
	for _, f := range splitMulti(env) {
		if strings.EqualFold(f, feature) {
			return true
		}
	}
	return false
}

func splitMulti(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})
}

type httpLoader struct {
	client *http.Client
}

func (l httpLoader) Load(url string) (any, error) {
	resp, err := l.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}

	return jsonschema.UnmarshalJSON(resp.Body)
}
