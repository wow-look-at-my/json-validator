package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testdataPath(name string) string {
	return filepath.Join("..", "..", "testdata", name)
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
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDraft(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
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
			got := silentFailureAllowed(tt.feature)
			if got != tt.want {
				t.Errorf("silentFailureAllowed(%q) with env=%q: got %v, want %v", tt.feature, tt.env, got, tt.want)
			}
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
			if len(got) != tt.want {
				t.Errorf("splitMulti(%q) = %v (len %d), want len %d", tt.input, got, len(got), tt.want)
			}
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
			if (err != nil) != tt.wantErr {
				t.Errorf("extractSchemaURI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractSchemaURI() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewCompiler(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		c, err := NewCompiler(Options{Draft: "2020"})
		if err != nil {
			t.Fatalf("NewCompiler() error = %v", err)
		}
		if c == nil {
			t.Fatal("NewCompiler() returned nil")
		}
	})

	t.Run("invalid draft", func(t *testing.T) {
		_, err := NewCompiler(Options{Draft: "invalid"})
		if err == nil {
			t.Fatal("NewCompiler() expected error for invalid draft")
		}
	})

	t.Run("no assert format", func(t *testing.T) {
		c, err := NewCompiler(Options{Draft: "2020", NoAssertFormat: true})
		if err != nil {
			t.Fatalf("NewCompiler() error = %v", err)
		}
		if c == nil {
			t.Fatal("NewCompiler() returned nil")
		}
	})

	t.Run("no assert format via env", func(t *testing.T) {
		t.Setenv("JSON_VALIDATION_ALLOW_SILENT_FAILURES", "assert-format")
		c, err := NewCompiler(Options{Draft: "2020"})
		if err != nil {
			t.Fatalf("NewCompiler() error = %v", err)
		}
		if c == nil {
			t.Fatal("NewCompiler() returned nil")
		}
	})
}

func TestValidateFile(t *testing.T) {
	schemaPath := testdataPath("schema.json")

	t.Run("valid document", func(t *testing.T) {
		opts := Options{SchemaPath: schemaPath, Draft: "2020"}
		c, err := NewCompiler(opts)
		if err != nil {
			t.Fatal(err)
		}
		sch, err := CompileSchema(c, schemaPath)
		if err != nil {
			t.Fatal(err)
		}
		r := ValidateFile(testdataPath("valid.json"), sch, opts)
		if r.Err != nil {
			t.Fatalf("unexpected error: %v", r.Err)
		}
		if !r.Valid {
			t.Errorf("expected valid, got invalid: %v", r.Error)
		}
	})

	t.Run("invalid document", func(t *testing.T) {
		opts := Options{SchemaPath: schemaPath, Draft: "2020"}
		c, err := NewCompiler(opts)
		if err != nil {
			t.Fatal(err)
		}
		sch, err := CompileSchema(c, schemaPath)
		if err != nil {
			t.Fatal(err)
		}
		r := ValidateFile(testdataPath("invalid.json"), sch, opts)
		if r.Err != nil {
			t.Fatalf("unexpected error: %v", r.Err)
		}
		if r.Valid {
			t.Error("expected invalid, got valid")
		}
		if r.Error == nil {
			t.Error("expected validation error, got nil")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		r := ValidateFile("nonexistent.json", nil, Options{Draft: "2020"})
		if r.Err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("JSONC input", func(t *testing.T) {
		opts := Options{SchemaPath: schemaPath, Draft: "2020"}
		c, err := NewCompiler(opts)
		if err != nil {
			t.Fatal(err)
		}
		sch, err := CompileSchema(c, schemaPath)
		if err != nil {
			t.Fatal(err)
		}
		r := ValidateFile(testdataPath("valid.jsonc"), sch, opts)
		if r.Err != nil {
			t.Fatalf("unexpected error: %v", r.Err)
		}
		if !r.Valid {
			t.Errorf("expected valid JSONC, got invalid: %v", r.Error)
		}
	})
}

func TestValidateReader(t *testing.T) {
	schemaPath := testdataPath("schema.json")

	t.Run("valid from reader", func(t *testing.T) {
		opts := Options{SchemaPath: schemaPath, Draft: "2020"}
		c, err := NewCompiler(opts)
		if err != nil {
			t.Fatal(err)
		}
		sch, err := CompileSchema(c, schemaPath)
		if err != nil {
			t.Fatal(err)
		}
		input := strings.NewReader(`{"name": "Test", "email": "test@example.com"}`)
		r := Validate(input, "<test>", sch, opts)
		if r.Err != nil {
			t.Fatalf("unexpected error: %v", r.Err)
		}
		if !r.Valid {
			t.Errorf("expected valid, got invalid: %v", r.Error)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		r := Validate(strings.NewReader("{invalid json}"), "<test>", nil, Options{Draft: "2020"})
		if r.Err == nil {
			t.Error("expected parse error for invalid JSON")
		}
	})

	t.Run("JSONC from reader", func(t *testing.T) {
		opts := Options{SchemaPath: schemaPath, Draft: "2020"}
		c, err := NewCompiler(opts)
		if err != nil {
			t.Fatal(err)
		}
		sch, err := CompileSchema(c, schemaPath)
		if err != nil {
			t.Fatal(err)
		}
		input := strings.NewReader(`{
			// comment
			"name": "Test",
			"email": "test@example.com",
		}`)
		r := Validate(input, "<test>", sch, opts)
		if r.Err != nil {
			t.Fatalf("unexpected error: %v", r.Err)
		}
		if !r.Valid {
			t.Errorf("expected valid JSONC, got invalid: %v", r.Error)
		}
	})
}

func TestValidateWithSchemaFromDocument(t *testing.T) {
	t.Run("document with $schema", func(t *testing.T) {
		r := ValidateFile(testdataPath("valid_with_schema.json"), nil, Options{Draft: "2020"})
		if r.Err != nil {
			t.Fatalf("unexpected error: %v", r.Err)
		}
		if !r.Valid {
			t.Errorf("expected valid, got invalid: %v", r.Error)
		}
	})

	t.Run("document without $schema and no --schema", func(t *testing.T) {
		r := ValidateFile(testdataPath("no_schema.json"), nil, Options{Draft: "2020"})
		if r.Err == nil {
			t.Error("expected error when no schema available")
		}
	})
}

func TestFormatAssertions(t *testing.T) {
	schemaPath := testdataPath("schema.json")

	t.Run("format enforced by default", func(t *testing.T) {
		opts := Options{SchemaPath: schemaPath, Draft: "2020"}
		c, err := NewCompiler(opts)
		if err != nil {
			t.Fatal(err)
		}
		sch, err := CompileSchema(c, schemaPath)
		if err != nil {
			t.Fatal(err)
		}
		input := strings.NewReader(`{"name": "Test", "email": "not-an-email"}`)
		r := Validate(input, "<test>", sch, opts)
		if r.Valid {
			t.Error("expected invalid due to format assertion, got valid")
		}
	})

	t.Run("format not enforced with NoAssertFormat", func(t *testing.T) {
		opts := Options{SchemaPath: schemaPath, Draft: "2020", NoAssertFormat: true}
		c, err := NewCompiler(opts)
		if err != nil {
			t.Fatal(err)
		}
		sch, err := CompileSchema(c, schemaPath)
		if err != nil {
			t.Fatal(err)
		}
		input := strings.NewReader(`{"name": "Test", "email": "not-an-email"}`)
		r := Validate(input, "<test>", sch, opts)
		if r.Err != nil {
			t.Fatalf("unexpected error: %v", r.Err)
		}
		if !r.Valid {
			t.Error("expected valid with format assertions disabled, got invalid")
		}
	})

	t.Run("format not enforced via env var", func(t *testing.T) {
		t.Setenv("JSON_VALIDATION_ALLOW_SILENT_FAILURES", "assert-format")
		opts := Options{SchemaPath: schemaPath, Draft: "2020"}
		c, err := NewCompiler(opts)
		if err != nil {
			t.Fatal(err)
		}
		sch, err := CompileSchema(c, schemaPath)
		if err != nil {
			t.Fatal(err)
		}
		input := strings.NewReader(`{"name": "Test", "email": "not-an-email"}`)
		r := Validate(input, "<test>", sch, opts)
		if r.Err != nil {
			t.Fatalf("unexpected error: %v", r.Err)
		}
		if !r.Valid {
			t.Error("expected valid with env-disabled format assertions, got invalid")
		}
	})
}

func TestHTTPLoader(t *testing.T) {
	loader := httpLoader{client: nil}
	_ = loader
}

func TestCompileSchema(t *testing.T) {
	t.Run("valid schema file", func(t *testing.T) {
		c, err := NewCompiler(Options{Draft: "2020"})
		if err != nil {
			t.Fatal(err)
		}
		sch, err := CompileSchema(c, testdataPath("schema.json"))
		if err != nil {
			t.Fatalf("CompileSchema() error = %v", err)
		}
		if sch == nil {
			t.Fatal("CompileSchema() returned nil")
		}
	})

	t.Run("nonexistent schema", func(t *testing.T) {
		c, err := NewCompiler(Options{Draft: "2020"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = CompileSchema(c, "nonexistent.json")
		if err == nil {
			t.Error("expected error for nonexistent schema")
		}
	})
}

func TestValidateFilePermissions(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "unreadable.json")
	if err := os.WriteFile(path, []byte(`{"x":1}`), 0o000); err != nil {
		t.Fatal(err)
	}
	r := ValidateFile(path, nil, Options{Draft: "2020"})
	if r.Err == nil {
		t.Error("expected error for unreadable file")
	}
}
