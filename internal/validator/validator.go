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

type Result struct {
	File  string
	Valid bool
	Error *jsonschema.ValidationError
	Err   error
}

type Options struct {
	SchemaPath     string
	Draft          string
	NoAssertFormat bool
}

func NewCompiler(opts Options) (*jsonschema.Compiler, error) {
	c := jsonschema.NewCompiler()

	draft, err := parseDraft(opts.Draft)
	if err != nil {
		return nil, err
	}
	c.DefaultDraft(draft)

	if !opts.NoAssertFormat && !silentFailureAllowed("assert-format") {
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

func Validate(r io.Reader, filename string, compiled *jsonschema.Schema, opts Options) Result {
	res := Result{File: filename}

	raw, err := io.ReadAll(r)
	if err != nil {
		res.Err = fmt.Errorf("reading input: %w", err)
		return res
	}

	cleaned := jsonc.ToJSON(raw)
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

func silentFailureAllowed(feature string) bool {
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
