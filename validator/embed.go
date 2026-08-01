package validator

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// The embedding surface: compile a schema once, validate many documents
// against it, without touching the underlying jsonschema compiler.
//
// The CLI's shape (a schema path plus a list of files, one process, exit code)
// is a poor fit for a server: its schema is usually compiled into the binary
// rather than sitting at a path, it validates on every reload, and it wants an
// error value instead of printed output. Everything below exists so a host
// program does not have to reach around this package to get that.

// Validator is a compiled schema plus the options it was compiled with, ready
// to validate documents repeatedly. Safe for concurrent use: compilation
// happens up front and validation does not mutate it.
type Validator struct {
	schema *jsonschema.Schema
	opts   Options
}

// New compiles the schema named by Options.SchemaPath (a file path or an
// http(s) URL).
//
// With an empty SchemaPath the Validator has no schema of its own and each
// document's `$schema` decides -- the CLI's default mode, which costs a
// compile per document, so prefer a fixed schema when you have one.
func New(opts Options) (*Validator, error) {
	if opts.SchemaPath == "" {
		return &Validator{opts: opts}, nil
	}
	c, err := NewCompiler(opts)
	if err != nil {
		return nil, err
	}
	sch, err := CompileSchema(c, opts.SchemaPath)
	if err != nil {
		return nil, fmt.Errorf("compiling schema %s: %w", opts.SchemaPath, err)
	}
	return &Validator{schema: sch, opts: opts}, nil
}

// NewFromBytes compiles a schema held IN MEMORY -- the embedded case: a host
// program that ships its schema with go:embed has no path to hand over, and
// must not depend on a file or a network fetch at validation time.
//
// `name` identifies the schema in error messages and resolves any relative
// $ref inside it; it is not fetched. Use something stable and recognizable
// (e.g. "embedded:hook.schema.json").
func NewFromBytes(name string, schema []byte, opts Options) (*Validator, error) {
	sch, err := CompileBytes(name, schema, opts)
	if err != nil {
		return nil, err
	}
	return &Validator{schema: sch, opts: opts}, nil
}

// CompileBytes compiles an in-memory schema document (JSONC allowed, like
// every other input here).
func CompileBytes(name string, schema []byte, opts Options) (*jsonschema.Schema, error) {
	c, err := NewCompiler(opts)
	if err != nil {
		return nil, err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(stripJSONC(schema)))
	if err != nil {
		return nil, fmt.Errorf("schema %s is not valid JSON: %w", name, err)
	}
	if err := c.AddResource(name, doc); err != nil {
		return nil, fmt.Errorf("schema %s is not a valid JSON Schema: %w", name, err)
	}
	sch, err := c.Compile(name)
	if err != nil {
		return nil, fmt.Errorf("compiling schema %s: %w", name, err)
	}
	return sch, nil
}

// Schema exposes the compiled schema, for a caller that needs the underlying
// jsonschema API. Nil when the Validator resolves a schema per document.
func (v *Validator) Schema() *jsonschema.Schema { return v.schema }

// Validate checks one document read from r. `filename` is used only to label
// the Result.
func (v *Validator) Validate(r io.Reader, filename string) Result {
	return doValidate(r, filename, v.schema, v.opts)
}

// ValidateBytes checks a document already in memory -- the common case for a
// host program that just read a manifest.
func (v *Validator) ValidateBytes(doc []byte, filename string) Result {
	return doValidate(bytes.NewReader(doc), filename, v.schema, v.opts)
}

// ValidateFile checks a document on disk.
func (v *Validator) ValidateFile(path string) Result {
	f, err := os.Open(path)
	if err != nil {
		return Result{File: path, Err: err}
	}
	defer f.Close()
	return doValidate(f, path, v.schema, v.opts)
}

// ValidateBytes is the one-shot form for callers that hold both the document
// and a compiled schema.
func ValidateBytes(doc []byte, filename string, compiled *jsonschema.Schema, opts Options) Result {
	return doValidate(bytes.NewReader(doc), filename, compiled, opts)
}
