package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The embedding contract. Each case is something a host program needs and the
// CLI-shaped API could not give it without reaching around this package.

const memSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["name"],
  "properties": {
    "name": {"type": "string", "minLength": 1},
    "count": {"type": "integer", "minimum": 0},
    "home": {"type": "string", "format": "uri"}
  }
}`

// The zero value must work: defaults belong to the library, not to the CLI's
// flag definitions. An embedder writing Options{} used to get
// "unsupported draft version: ".
func TestZeroOptionsAreUsable(t *testing.T) {
	v, err := NewFromBytes("mem:test", []byte(memSchema), Options{})
	require.NoError(t, err)

	assert.True(t, v.ValidateBytes([]byte(`{"name":"ok"}`), "doc.json").Valid)
	assert.False(t, v.ValidateBytes([]byte(`{"count":1}`), "doc.json").Valid)
}

func TestDefaultDraftIsAppliedWhenUnset(t *testing.T) {
	_, err := NewCompiler(Options{})
	require.NoError(t, err)
	assert.Equal(t, DefaultDraft, Options{}.draft())
	assert.Equal(t, "7", Options{Draft: "7"}.draft())
}

// A schema compiled into the binary: no file, no network.
func TestNewFromBytesNeedsNoFilesystem(t *testing.T) {
	v, err := NewFromBytes("embedded:hook.schema.json", []byte(memSchema), Options{})
	require.NoError(t, err)
	require.NotNil(t, v.Schema())

	res := v.ValidateBytes([]byte(`{"name":"x","count":-1}`), "hook.json")
	assert.False(t, res.Valid)
	assert.Contains(t, res.Detail(), "count")
}

// Format assertions are on by default here too -- the deviation is the
// package's, not the CLI's.
func TestFormatAssertedByDefaultInMemory(t *testing.T) {
	v, err := NewFromBytes("mem:test", []byte(memSchema), Options{})
	require.NoError(t, err)
	assert.False(t, v.ValidateBytes([]byte(`{"name":"x","home":"not a url"}`), "d.json").Valid)

	lax, err := NewFromBytes("mem:test", []byte(memSchema), Options{NoAssertFormat: true})
	require.NoError(t, err)
	assert.True(t, lax.ValidateBytes([]byte(`{"name":"x","home":"not a url"}`), "d.json").Valid)
}

// One compile, many documents -- a server revalidating on every reload must
// not recompile a constant schema each time.
func TestValidatorIsReusable(t *testing.T) {
	v, err := NewFromBytes("mem:test", []byte(memSchema), Options{})
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		assert.True(t, v.ValidateBytes([]byte(`{"name":"ok"}`), "d.json").Valid)
		assert.False(t, v.ValidateBytes([]byte(`{}`), "d.json").Valid)
	}
}

// JSONC is accepted by the library exactly as by the CLI -- both the schema
// and the document may carry comments.
func TestJSONCEverywhere(t *testing.T) {
	v, err := NewFromBytes("mem:test", []byte("// what this accepts\n"+memSchema), Options{})
	require.NoError(t, err)
	assert.True(t, v.ValidateBytes([]byte("{\n  // a name\n  \"name\": \"ok\",\n}"), "d.json").Valid)
}

func TestValidatorFromPathAndFile(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	require.NoError(t, os.WriteFile(schemaPath, []byte(memSchema), 0o644))
	docPath := filepath.Join(dir, "doc.json")
	require.NoError(t, os.WriteFile(docPath, []byte(`{"name":"ok"}`), 0o644))

	v, err := New(Options{SchemaPath: schemaPath})
	require.NoError(t, err)
	assert.True(t, v.ValidateFile(docPath).Valid)
	assert.True(t, v.Validate(strings.NewReader(`{"name":"ok"}`), "<mem>").Valid)

	missing := v.ValidateFile(filepath.Join(dir, "nope.json"))
	assert.False(t, missing.Valid)
	require.Error(t, missing.AsError())
}

// With no schema of its own, each document's $schema decides -- the CLI's
// default mode, available to an embedder too.
func TestValidatorWithoutASchemaUsesTheDocuments(t *testing.T) {
	v, err := New(Options{})
	require.NoError(t, err)
	res := v.ValidateFile(testdataPath("valid_with_schema.json"))
	assert.True(t, res.Valid, res.Detail())

	noSchema := v.ValidateBytes([]byte(`{"name":"x"}`), "d.json")
	assert.False(t, noSchema.Valid)
	assert.Contains(t, noSchema.AsError().Error(), "no schema")
}

// Results have to be usable as errors and as one-line log messages: an
// embedder should not have to walk a nested ValidationError tree itself.
func TestResultAsErrorAndDetail(t *testing.T) {
	v, err := NewFromBytes("mem:test", []byte(memSchema), Options{})
	require.NoError(t, err)

	ok := v.ValidateBytes([]byte(`{"name":"ok"}`), "d.json")
	assert.NoError(t, ok.AsError())
	assert.Empty(t, ok.Detail())

	bad := v.ValidateBytes([]byte(`{"name":"","count":"x"}`), "d.json")
	require.Error(t, bad.AsError())
	detail := bad.Detail()
	assert.Contains(t, detail, "name")
	assert.Contains(t, detail, "count")
	assert.NotContains(t, detail, "\n", "Detail is one line -- a load error is not a tree")

	broken := Result{File: "d.json", Err: assert.AnError}
	assert.Equal(t, assert.AnError, broken.AsError())
	assert.Equal(t, assert.AnError.Error(), broken.Detail())
}

// A schema that is not JSON, or not a schema, must fail loudly at compile --
// never degrade into "no validation".
func TestBadSchemaBytesFailLoudly(t *testing.T) {
	_, err := NewFromBytes("mem:bad", []byte(`{"type":`), Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid JSON")

	_, err = NewFromBytes("mem:bad", []byte(`{"type": 12}`), Options{})
	require.Error(t, err)
}

func TestCompileBytesIsUsableDirectly(t *testing.T) {
	sch, err := CompileBytes("mem:test", []byte(memSchema), Options{})
	require.NoError(t, err)
	assert.True(t, ValidateBytes([]byte(`{"name":"ok"}`), "d.json", sch, Options{}).Valid)
}
