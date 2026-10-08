package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/jsonc"
)

func TestAddTrailingCommas(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"multi-line object", "{\n  \"a\": 1\n}\n", "{\n  \"a\": 1,\n}\n"},
		{"multi-line array", "[\n  1,\n  2\n]\n", "[\n  1,\n  2,\n]\n"},
		{
			"nested",
			"[\n  {\n    \"a\": [\n      true\n    ]\n  }\n]\n",
			"[\n  {\n    \"a\": [\n      true,\n    ],\n  },\n]\n",
		},
		{"empty object", "{}\n", "{}\n"},
		{"empty array in object", "{\n  \"a\": []\n}\n", "{\n  \"a\": [],\n}\n"},
		{"one-line containers", `{"a": [1, 2]}`, `{"a": [1, 2]}`},
		{"comma already present", "[\n  1,\n]\n", "[\n  1,\n]\n"},
		{"brackets in strings", "[\n  \"}\\\"]\"\n]\n", "[\n  \"}\\\"]\",\n]\n"},
		{"empty input", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(addTrailingCommas([]byte(tt.in))))
		})
	}
}

func TestWriteJSONModes(t *testing.T) {
	v := map[string]any{"list": []int{1, 2}, "obj": map[string]int{"k": 1}}

	var withCommas, strict bytes.Buffer
	require.NoError(t, writeJSON(&withCommas, v, true))
	require.NoError(t, writeJSON(&strict, v, false))

	assert.True(t, json.Valid(strict.Bytes()))
	assert.False(t, json.Valid(withCommas.Bytes()))
	assert.Equal(t, strict.String(), removeTrailingCommas(withCommas.String()))
	assert.JSONEq(t, strict.String(), string(jsonc.ToJSON(withCommas.Bytes())))
}

// removeTrailingCommas turns the default output into the strict output.
func removeTrailingCommas(s string) string {
	return regexp.MustCompile(`,(\n *[}\]])`).ReplaceAllString(s, "$1")
}

var trailingBeforeCloser = regexp.MustCompile(`,\n *[}\]]`)

func TestCLIJSONOutputAddsTrailingCommasByDefault(t *testing.T) {
	stdout, _, err := runCmd("--json", "--schema", testdataPath("schema.json"), testdataPath("valid.json"))
	require.NoError(t, err)

	assert.Regexp(t, `"valid": true,\n  },\n]\n$`, stdout)
	var got []map[string]any
	require.NoError(t, json.Unmarshal(jsonc.ToJSON([]byte(stdout)), &got))
	require.Len(t, got, 1)
	assert.Equal(t, true, got[0]["valid"])
}

func TestCLIJSONOutputAddsTrailingCommasToErrors(t *testing.T) {
	stdout, _, err := runCmd("--json", "--schema", testdataPath("schema.json"), testdataPath("invalid.json"))
	require.Error(t, err)

	assert.Regexp(t, trailingBeforeCloser, stdout)
	assert.Equal(t, stdout, string(addTrailingCommas([]byte(stdout))), "a second pass adds nothing")
	var got []map[string]any
	require.NoError(t, json.Unmarshal(jsonc.ToJSON([]byte(stdout)), &got))
	assert.Equal(t, false, got[0]["valid"])
	assert.NotNil(t, got[0]["errors"])
}

func TestCLIJSONErrorsPrintInOneOrder(t *testing.T) {
	first, _, _ := runCmd("--json", "--schema", testdataPath("schema.json"), testdataPath("invalid.json"))
	for range 20 {
		again, _, _ := runCmd("--json", "--schema", testdataPath("schema.json"), testdataPath("invalid.json"))
		require.Equal(t, first, again)
	}
}

func TestCLIIneffectiveJSONWritesStrictJSON(t *testing.T) {
	for _, doc := range []string{"valid.json", "invalid.json"} {
		t.Run(doc, func(t *testing.T) {
			stdout, _, _ := runCmd("--json", "--ineffective-json", "--schema", testdataPath("schema.json"), testdataPath(doc))
			assert.True(t, json.Valid([]byte(stdout)), stdout)
			assert.NotRegexp(t, trailingBeforeCloser, stdout)

			withCommas, _, _ := runCmd("--json", "--schema", testdataPath("schema.json"), testdataPath(doc))
			assert.Equal(t, stdout, removeTrailingCommas(withCommas))
		})
	}
}

// Validation reads an input and never writes it, so a trailing comma in a
// document or a schema survives every CLI mode byte for byte.
func TestCLIKeepsTrailingCommasInInputs(t *testing.T) {
	dir := t.TempDir()
	schema := []byte("{\n  \"type\": \"object\",\n  \"properties\": {\"a\": {\"type\": \"array\",},},\n}\n")
	doc := []byte("{\n  \"a\": [1, 2, /* end */ ],\n}\n")
	schemaPath := filepath.Join(dir, "schema.json")
	docPath := filepath.Join(dir, "doc.json")
	require.NoError(t, os.WriteFile(schemaPath, schema, 0o644))
	require.NoError(t, os.WriteFile(docPath, doc, 0o644))

	modes := [][]string{
		{},
		{"--json"},
		{"--json", "--ineffective-json"},
		{"--quiet"},
	}
	for _, mode := range modes {
		args := append(append([]string{}, mode...), "--schema", schemaPath, docPath)
		_, _, err := runCmd(args...)
		require.NoError(t, err, "mode %v", mode)

		gotDoc, err := os.ReadFile(docPath)
		require.NoError(t, err)
		assert.Equal(t, doc, gotDoc, "mode %v", mode)
		gotSchema, err := os.ReadFile(schemaPath)
		require.NoError(t, err)
		assert.Equal(t, schema, gotSchema, "mode %v", mode)
	}
}
