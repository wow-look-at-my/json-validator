package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
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

	err = rootCmd.Execute()

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
