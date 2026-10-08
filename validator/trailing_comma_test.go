package validator

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// trailingCommaCases pairs a document that holds a comma before a closing
// bracket with the strict JSON it must parse to.
var trailingCommaCases = []struct {
	name string
	in   string
	want string
}{
	{"object", `{"a": 1,}`, `{"a": 1}`},
	{"array", `[1, 2,]`, `[1, 2]`},
	{"array of one null", `[null,]`, `[null]`},
	{"object in array", `[{"a": 1,},]`, `[{"a": 1}]`},
	{"array in object", `{"a": [1, 2,],}`, `{"a": [1, 2]}`},
	{"deep nesting", `{"a": {"b": [{"c": [true,],},],},}`, `{"a": {"b": [{"c": [true]}]}}`},
	{"space before bracket", `{"a": 1,   }`, `{"a": 1}`},
	{"tabs and newlines before bracket", "[1,\n\t\n]", `[1]`},
	{"CRLF before bracket", "{\r\n  \"a\": 1,\r\n}", `{"a": 1}`},
	{"line comment before bracket", "[1, // last\n]", `[1]`},
	{"block comment before bracket", `{"a": 1, /* end */ }`, `{"a": 1}`},
	{"block comment spanning lines", "[1, /* one\n two */\n]", `[1]`},
	{"comments and whitespace mixed", "{\"a\": [1, /* x */ // y\n ], // z\n /* w */ }", `{"a": [1]}`},
	{"comma inside a string stays", `["a,]",]`, `["a,]"]`},
	{"escaped quote before the comma", `{"k": "x\"",}`, `{"k": "x\""}`},
	{"escaped backslash before the comma", `["x\\",]`, `["x\\"]`},
}

func TestTrailingCommasParse(t *testing.T) {
	for _, tc := range trailingCommaCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseJSONC([]byte(tc.in))
			require.NoError(t, err)
			want, err := jsonschema.UnmarshalJSON(strings.NewReader(tc.want))
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestTrailingCommasValidateAsDocuments(t *testing.T) {
	v, err := NewFromBytes("mem:any", []byte(`{}`), Options{})
	require.NoError(t, err)
	for _, tc := range trailingCommaCases {
		t.Run(tc.name, func(t *testing.T) {
			res := v.ValidateBytes([]byte(tc.in), "doc.json")
			require.NoError(t, res.Err)
			assert.True(t, res.Valid, res.Detail())
		})
	}
}

// A trailing comma adds no item: an array that ends in one still meets
// maxItems for the items it holds.
func TestTrailingCommaAddsNoItem(t *testing.T) {
	v, err := NewFromBytes("mem:pair", []byte(`{"type": "array", "maxItems": 2}`), Options{})
	require.NoError(t, err)
	assert.True(t, v.ValidateBytes([]byte(`[1, 2,]`), "doc.json").Valid)
	assert.False(t, v.ValidateBytes([]byte(`[1, 2, 3,]`), "doc.json").Valid)
}

func TestCommaThatIsNotTrailingIsStillAnError(t *testing.T) {
	v, err := NewFromBytes("mem:any", []byte(`{}`), Options{})
	require.NoError(t, err)
	for _, in := range []string{`[1,,2]`, `[1,,]`, `{"a": 1,,}`} {
		t.Run(in, func(t *testing.T) {
			res := v.ValidateBytes([]byte(in), "doc.json")
			require.Error(t, res.Err)
			assert.Contains(t, res.Err.Error(), "parsing JSON")
		})
	}
}

// trailingCommaSchema holds a comma before every closing bracket it can.
const trailingCommaSchema = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "object",
	"required": ["name",],
	"properties": {
		"name": {"type": "string",},
		"tags": {"type": "array", "items": {"type": "string",},}, // last property
	},
	/* end of schema */
}`

const trailingCommaDoc = "{\n\t\"name\": \"ok\",\n\t\"tags\": [\"a\", \"b\",],\n}"

func TestTrailingCommasInAnInMemorySchema(t *testing.T) {
	v, err := NewFromBytes("mem:trailing", []byte(trailingCommaSchema), Options{})
	require.NoError(t, err)
	assert.True(t, v.ValidateBytes([]byte(trailingCommaDoc), "doc.json").Valid)
	assert.False(t, v.ValidateBytes([]byte(`{"tags": [1,],}`), "doc.json").Valid)
}

func TestTrailingCommasInASchemaFile(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	require.NoError(t, os.WriteFile(schemaPath, []byte(trailingCommaSchema), 0o644))

	v, err := New(Options{SchemaPath: schemaPath})
	require.NoError(t, err)
	assert.True(t, v.ValidateBytes([]byte(trailingCommaDoc), "doc.json").Valid)
	assert.False(t, v.ValidateBytes([]byte(`{"tags": [1,],}`), "doc.json").Valid)
}

// A document's $schema that names a local file loads that file as JSONC too.
func TestTrailingCommasInASchemaTheDocumentNames(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	require.NoError(t, os.WriteFile(schemaPath, []byte(trailingCommaSchema), 0o644))

	v, err := New(Options{})
	require.NoError(t, err)
	doc := `{"$schema": "file://` + filepath.ToSlash(schemaPath) + `", "name": "ok",}`
	res := v.ValidateBytes([]byte(doc), "doc.json")
	require.NoError(t, res.Err)
	assert.True(t, res.Valid, res.Detail())
}

func TestTrailingCommasInAFetchedSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(trailingCommaSchema))
	}))
	defer srv.Close()

	v, err := New(Options{SchemaPath: srv.URL + "/schema.json"})
	require.NoError(t, err)
	assert.True(t, v.ValidateBytes([]byte(trailingCommaDoc), "doc.json").Valid)
	assert.False(t, v.ValidateBytes([]byte(`{"tags": [1,],}`), "doc.json").Valid)
}

func TestFileLoaderErrors(t *testing.T) {
	_, err := fileLoader{}.Load("http://example.invalid/schema.json")
	require.Error(t, err)

	_, err = fileLoader{}.Load("file://" + filepath.ToSlash(filepath.Join(t.TempDir(), "absent.json")))
	require.Error(t, err)
}
