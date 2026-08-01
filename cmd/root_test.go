package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testdataPath(name string) string {
	return filepath.Join("..", "testdata", name)
}

func runCmd(args ...string) (stdout, stderr string, err error) {
	schemaFlag = ""
	jsonOutputFlag = false
	quietFlag = false
	draftFlag = "2020"
	noAssertFormat = false
	rootCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs(args)

	// Execute(), not rootCmd.Execute(): the error reporting lives there, so
	// going straight to cobra would test a path main() never takes.
	err = Execute()

	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)

	return outBuf.String(), errBuf.String(), err
}

func TestCLIValidFile(t *testing.T) {
	stdout, _, err := runCmd("--schema", testdataPath("schema.json"), testdataPath("valid.json"))
	require.Nil(t, err)

	assert.Contains(t, stdout, "valid")

}

func TestCLIInvalidFile(t *testing.T) {
	_, stderr, err := runCmd("--schema", testdataPath("schema.json"), testdataPath("invalid.json"))
	require.NotNil(t, err)

	assert.Contains(t, stderr, "INVALID")

}

func TestCLIMultipleFiles(t *testing.T) {
	_, _, err := runCmd("--schema", testdataPath("schema.json"),
		testdataPath("valid.json"), testdataPath("invalid.json"))
	require.NotNil(t, err)

}

func TestCLIJSONOutput(t *testing.T) {
	stdout, _, err := runCmd("--json", "--schema", testdataPath("schema.json"), testdataPath("valid.json"))
	require.Nil(t, err)

	assert.Contains(t, stdout, `"valid"`)

}

func TestCLIQuietModeValid(t *testing.T) {
	stdout, stderr, err := runCmd("--quiet", "--schema", testdataPath("schema.json"), testdataPath("valid.json"))
	require.Nil(t, err)

	assert.False(t, stdout != "" || stderr != "")

}

func TestCLIQuietModeInvalid(t *testing.T) {
	stdout, stderr, err := runCmd("--quiet", "--schema", testdataPath("schema.json"), testdataPath("invalid.json"))
	require.NotNil(t, err)

	assert.False(t, stdout != "" || stderr != "")

}

func TestCLIJSONCFile(t *testing.T) {
	stdout, _, err := runCmd("--schema", testdataPath("schema.json"), testdataPath("valid.jsonc"))
	require.Nil(t, err)

	assert.Contains(t, stdout, "valid")

}

func TestCLINoSchemaFlag(t *testing.T) {
	_, _, err := runCmd(testdataPath("valid_with_schema.json"))
	require.Nil(t, err)

}

func TestCLINoSchemaAnywhere(t *testing.T) {
	_, _, err := runCmd(testdataPath("no_schema.json"))
	require.NotNil(t, err)

}

func TestCLIMissingFile(t *testing.T) {
	_, _, err := runCmd("--schema", testdataPath("schema.json"), "nonexistent.json")
	require.NotNil(t, err)

}

func TestCLIDraftFlag(t *testing.T) {
	_, _, err := runCmd("--draft", "7", "--schema", testdataPath("schema.json"), testdataPath("valid.json"))
	require.Nil(t, err)

}

func TestCLINoAssertFormat(t *testing.T) {
	_, _, err := runCmd("--no-assert-format", "--schema", testdataPath("schema.json"), testdataPath("invalid.json"))
	require.NotNil(t, err)

}

func TestCLIStdin(t *testing.T) {
	r, w, err := os.Pipe()
	require.Nil(t, err)

	go func() {
		w.Write([]byte(`{"name": "Test", "email": "test@example.com"}`))
		w.Close()
	}()

	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	_, _, err = runCmd("--schema", testdataPath("schema.json"))
	require.Nil(t, err)

}

func TestCLIJSONOutputInvalid(t *testing.T) {
	stdout, _, err := runCmd("--json", "--schema", testdataPath("schema.json"), testdataPath("invalid.json"))
	require.NotNil(t, err)

	assert.False(t, !strings.Contains(stdout, `"valid": false`) || !strings.Contains(stdout, `"errors"`))

}

// The documented env escape hatch (JSON_VALIDATION_ALLOW_SILENT_FAILURES)
// lives at the CLI boundary: the library reads no environment, so the CLI is
// what turns the variable into the option it stands for. Asserted end to end,
// because "the flag exists" is not the same claim.
func TestCLIEnvDisablesFormatAssertions(t *testing.T) {
	doc := filepath.Join(t.TempDir(), "bad-format.json")
	require.NoError(t, os.WriteFile(doc, []byte(`{"name":"Test","email":"not-an-email"}`), 0o644))

	_, _, err := runCmd("--schema", testdataPath("schema.json"), doc)
	require.Error(t, err, "format assertions are on by default")

	t.Setenv("JSON_VALIDATION_ALLOW_SILENT_FAILURES", "assert-format")
	_, _, err = runCmd("--schema", testdataPath("schema.json"), doc)
	assert.NoError(t, err, "the env var must still disable format assertions on the CLI")
}

// Every way of failing to RUN must say why. Each of these exited 1 printing
// NOTHING before -- cobra was silenced and main() only read the exit code --
// which in CI means a red gate with no reason and nothing to search for.
func TestCLIFailuresToRunAreReported(t *testing.T) {
	dir := t.TempDir()
	notJSON := filepath.Join(dir, "not-a-schema.json")
	require.NoError(t, os.WriteFile(notJSON, []byte("this is not json"), 0o644))
	danglingRef := filepath.Join(dir, "dangling.schema.json")
	require.NoError(t, os.WriteFile(danglingRef, []byte(
		`{"$schema":"https://json-schema.org/draft/2020-12/schema",`+
			`"$id":"https://example.test/dangling.schema.json",`+
			`"allOf":[{"$ref":"nowhere.schema.json"}]}`), 0o644))

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"schema file missing", []string{"--schema", filepath.Join(dir, "nope.json"), testdataPath("valid.json")}, "nope.json"},
		{"schema is not JSON", []string{"--schema", notJSON, testdataPath("valid.json")}, "not-a-schema.json"},
		{"schema $ref unresolvable", []string{"--schema", danglingRef, testdataPath("valid.json")}, "dangling.schema.json"},
		{"unknown flag", []string{"--bogus", testdataPath("valid.json")}, "bogus"},
		{"mutually exclusive flags", []string{"--json", "--quiet", testdataPath("valid.json")}, "quiet"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, stderr, err := runCmd(c.args...)
			require.Error(t, err)
			assert.Contains(t, stderr, "json-validator:", "the failure must reach stderr, not just the exit code")
			assert.Contains(t, stderr, c.want, "the message must name what went wrong")
		})
	}
}

// --quiet governs RESULTS, not the tool's ability to run: `grep -q` still
// reports a missing file. A silent quiet-mode compile failure is the same
// invisible red this fix exists to remove.
func TestCLIQuietStillReportsAFailureToRun(t *testing.T) {
	_, stderr, err := runCmd("--quiet", "--schema", filepath.Join(t.TempDir(), "absent.json"), testdataPath("valid.json"))
	require.Error(t, err)
	assert.Contains(t, stderr, "json-validator:")
	assert.Contains(t, stderr, "absent.json")
}

// The other half of the contract: an INVALID document is the ordinary negative
// result, already reported per file, so Execute must not bolt a second
// "validation failed" line onto it -- and --quiet must stay silent.
func TestCLIInvalidDocumentIsNotDoubleReported(t *testing.T) {
	_, stderr, err := runCmd("--schema", testdataPath("schema.json"), testdataPath("invalid.json"))
	require.Error(t, err)
	assert.Contains(t, stderr, "INVALID")
	assert.NotContains(t, stderr, "json-validator:", "an invalid document is a result, not a failure to run")

	_, quietErr, err := runCmd("--quiet", "--schema", testdataPath("schema.json"), testdataPath("invalid.json"))
	require.Error(t, err)
	assert.Empty(t, quietErr, "--quiet means exit code only for results")
}
