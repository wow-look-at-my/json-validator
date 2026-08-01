package validator

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testdataPath(name string) string {
	return filepath.Join("..", "testdata", name)
}

func TestParseDraft(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"4", false},
		{"6", false},
		{"7", false},
		{"2019", false},
		{"2020", false},
		{"invalid", true},
		{"", true},
		{"3", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := parseDraft(tt.input)
			assert.Equal(t, tt.wantErr, (err != nil))

		})
	}
}

func TestSilentFailureAllowed(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		feature string
		want    bool
	}{
		{"empty env", "", "assert-format", false},
		{"comma separated match", "assert-format,other", "assert-format", true},
		{"semicolon separated match", "other;assert-format", "assert-format", true},
		{"space separated match", "other assert-format", "assert-format", true},
		{"no match", "other,stuff", "assert-format", false},
		{"case insensitive", "Assert-Format", "assert-format", true},
		{"mixed delimiters", "a,b;assert-format d", "assert-format", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("JSON_VALIDATION_ALLOW_SILENT_FAILURES", tt.env)
			got := SilentFailureAllowed(tt.feature)
			assert.Equal(t, tt.want, got)

		})
	}
}

func TestSplitMulti(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"a", 1},
		{"a,b", 2},
		{"a;b;c", 3},
		{"a b c", 3},
		{"a,b;c d", 4},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitMulti(tt.input)
			assert.Equal(t, tt.want, len(got))

		})
	}
}

func TestExtractSchemaURI(t *testing.T) {
	tests := []struct {
		name    string
		doc     any
		want    string
		wantErr bool
	}{
		{
			name: "valid $schema",
			doc:  map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema"},
			want: "https://json-schema.org/draft/2020-12/schema",
		},
		{
			name:    "missing $schema",
			doc:     map[string]any{"name": "test"},
			wantErr: true,
		},
		{
			name:    "non-object document",
			doc:     []any{1, 2, 3},
			wantErr: true,
		},
		{
			name:    "$schema is not a string",
			doc:     map[string]any{"$schema": 42},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractSchemaURI(tt.doc)
			assert.Equal(t, tt.wantErr, (err != nil))

			assert.Equal(t, tt.want, got)

		})
	}
}

func TestNewCompiler(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		c, err := NewCompiler(Options{Draft: "2020"})
		require.Nil(t, err)

		require.NotNil(t, c)

	})

	t.Run("invalid draft", func(t *testing.T) {
		_, err := NewCompiler(Options{Draft: "invalid"})
		require.NotNil(t, err)

	})

	t.Run("no assert format", func(t *testing.T) {
		c, err := NewCompiler(Options{Draft: "2020", NoAssertFormat: true})
		require.Nil(t, err)

		require.NotNil(t, c)

	})

	t.Run("no assert format via env", func(t *testing.T) {
		t.Setenv("JSON_VALIDATION_ALLOW_SILENT_FAILURES", "assert-format")
		c, err := NewCompiler(Options{Draft: "2020"})
		require.Nil(t, err)

		require.NotNil(t, c)

	})
}

func TestValidateFile(t *testing.T) {
	schemaPath := testdataPath("schema.json")

	t.Run("valid document", func(t *testing.T) {
		opts := Options{SchemaPath: schemaPath, Draft: "2020"}
		c, err := NewCompiler(opts)
		require.Nil(t, err)

		sch, err := CompileSchema(c, schemaPath)
		require.Nil(t, err)

		r := ValidateFile(testdataPath("valid.json"), sch, opts)
		require.Nil(t, r.Err)

		assert.True(t, r.Valid)

	})

	t.Run("invalid document", func(t *testing.T) {
		opts := Options{SchemaPath: schemaPath, Draft: "2020"}
		c, err := NewCompiler(opts)
		require.Nil(t, err)

		sch, err := CompileSchema(c, schemaPath)
		require.Nil(t, err)

		r := ValidateFile(testdataPath("invalid.json"), sch, opts)
		require.Nil(t, r.Err)

		assert.False(t, r.Valid)

		assert.NotNil(t, r.Error)

	})

	t.Run("file not found", func(t *testing.T) {
		r := ValidateFile("nonexistent.json", nil, Options{Draft: "2020"})
		assert.NotNil(t, r.Err)

	})

	t.Run("JSONC input", func(t *testing.T) {
		opts := Options{SchemaPath: schemaPath, Draft: "2020"}
		c, err := NewCompiler(opts)
		require.Nil(t, err)

		sch, err := CompileSchema(c, schemaPath)
		require.Nil(t, err)

		r := ValidateFile(testdataPath("valid.jsonc"), sch, opts)
		require.Nil(t, r.Err)

		assert.True(t, r.Valid)

	})
}

func TestValidateReader(t *testing.T) {
	schemaPath := testdataPath("schema.json")

	t.Run("valid from reader", func(t *testing.T) {
		opts := Options{SchemaPath: schemaPath, Draft: "2020"}
		c, err := NewCompiler(opts)
		require.Nil(t, err)

		sch, err := CompileSchema(c, schemaPath)
		require.Nil(t, err)

		input := strings.NewReader(`{"name": "Test", "email": "test@example.com"}`)
		r := Validate(input, "<test>", sch, opts)
		require.Nil(t, r.Err)

		assert.True(t, r.Valid)

	})

	t.Run("invalid JSON", func(t *testing.T) {
		r := Validate(strings.NewReader("{invalid json}"), "<test>", nil, Options{Draft: "2020"})
		assert.NotNil(t, r.Err)

	})

	t.Run("JSONC from reader", func(t *testing.T) {
		opts := Options{SchemaPath: schemaPath, Draft: "2020"}
		c, err := NewCompiler(opts)
		require.Nil(t, err)

		sch, err := CompileSchema(c, schemaPath)
		require.Nil(t, err)

		input := strings.NewReader(`{
			// comment
			"name": "Test",
			"email": "test@example.com",
		}`)
		r := Validate(input, "<test>", sch, opts)
		require.Nil(t, r.Err)

		assert.True(t, r.Valid)

	})
}

func TestValidateWithSchemaFromDocument(t *testing.T) {
	t.Run("document with $schema", func(t *testing.T) {
		r := ValidateFile(testdataPath("valid_with_schema.json"), nil, Options{Draft: "2020"})
		require.Nil(t, r.Err)

		assert.True(t, r.Valid)

	})

	t.Run("document without $schema and no --schema", func(t *testing.T) {
		r := ValidateFile(testdataPath("no_schema.json"), nil, Options{Draft: "2020"})
		assert.NotNil(t, r.Err)

	})
}

func TestFormatAssertions(t *testing.T) {
	schemaPath := testdataPath("schema.json")

	t.Run("format enforced by default", func(t *testing.T) {
		opts := Options{SchemaPath: schemaPath, Draft: "2020"}
		c, err := NewCompiler(opts)
		require.Nil(t, err)

		sch, err := CompileSchema(c, schemaPath)
		require.Nil(t, err)

		input := strings.NewReader(`{"name": "Test", "email": "not-an-email"}`)
		r := Validate(input, "<test>", sch, opts)
		assert.False(t, r.Valid)

	})

	t.Run("format not enforced with NoAssertFormat", func(t *testing.T) {
		opts := Options{SchemaPath: schemaPath, Draft: "2020", NoAssertFormat: true}
		c, err := NewCompiler(opts)
		require.Nil(t, err)

		sch, err := CompileSchema(c, schemaPath)
		require.Nil(t, err)

		input := strings.NewReader(`{"name": "Test", "email": "not-an-email"}`)
		r := Validate(input, "<test>", sch, opts)
		require.Nil(t, r.Err)

		assert.True(t, r.Valid)

	})

	// The env escape hatch is the CLI's, NOT the library's: an embedded
	// validator whose strictness silently depends on the host process's
	// environment is a trap. cmd/root_test.go covers the CLI end of it.
	t.Run("the env var does NOT reach the library", func(t *testing.T) {
		t.Setenv("JSON_VALIDATION_ALLOW_SILENT_FAILURES", "assert-format")
		opts := Options{SchemaPath: schemaPath}
		c, err := NewCompiler(opts)
		require.Nil(t, err)

		sch, err := CompileSchema(c, schemaPath)
		require.Nil(t, err)

		input := strings.NewReader(`{"name": "Test", "email": "not-an-email"}`)
		r := Validate(input, "<test>", sch, opts)
		require.Nil(t, r.Err)

		assert.False(t, r.Valid, "Options alone decides: format assertions stay on")
	})
}

func TestHTTPLoader(t *testing.T) {
	loader := httpLoader{client: nil}
	_ = loader
}

func TestCompileSchema(t *testing.T) {
	t.Run("valid schema file", func(t *testing.T) {
		c, err := NewCompiler(Options{Draft: "2020"})
		require.Nil(t, err)

		sch, err := CompileSchema(c, testdataPath("schema.json"))
		require.Nil(t, err)

		require.NotNil(t, sch)

	})

	t.Run("nonexistent schema", func(t *testing.T) {
		c, err := NewCompiler(Options{Draft: "2020"})
		require.Nil(t, err)

		_, err = CompileSchema(c, "nonexistent.json")
		assert.NotNil(t, err)

	})
}

func TestValidateFilePermissions(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "unreadable.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"x":1}`), 0o000))

	r := ValidateFile(path, nil, Options{Draft: "2020"})
	assert.NotNil(t, r.Err)

}
