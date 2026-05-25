package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testdataPath(name string) string {
	return filepath.Join("..", "testdata", name)
}

func runCmd(args ...string) (stdout, stderr string, err error) {
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
	stdout, stderr, err := runCmd("--schema", testdataPath("schema.json"), testdataPath("valid.json"))
	if err != nil {
		t.Fatalf("expected success, got error: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "valid") {
		t.Errorf("expected 'valid' in output, got: %s", stdout)
	}
}

func TestCLIInvalidFile(t *testing.T) {
	_, stderr, err := runCmd("--schema", testdataPath("schema.json"), testdataPath("invalid.json"))
	if err == nil {
		t.Fatal("expected error for invalid file")
	}
	if !strings.Contains(stderr, "INVALID") {
		t.Errorf("expected 'INVALID' in stderr, got: %s", stderr)
	}
}

func TestCLIMultipleFiles(t *testing.T) {
	_, _, err := runCmd("--schema", testdataPath("schema.json"),
		testdataPath("valid.json"), testdataPath("invalid.json"))
	if err == nil {
		t.Fatal("expected error when one file is invalid")
	}
}

func TestCLIJSONOutput(t *testing.T) {
	stdout, _, err := runCmd("--json", "--schema", testdataPath("schema.json"), testdataPath("valid.json"))
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !strings.Contains(stdout, `"valid"`) {
		t.Errorf("expected JSON output with 'valid' field, got: %s", stdout)
	}
}

func TestCLIQuietModeValid(t *testing.T) {
	stdout, stderr, err := runCmd("--quiet", "--schema", testdataPath("schema.json"), testdataPath("valid.json"))
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if stdout != "" || stderr != "" {
		t.Errorf("expected no output in quiet mode, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestCLIQuietModeInvalid(t *testing.T) {
	stdout, stderr, err := runCmd("--quiet", "--schema", testdataPath("schema.json"), testdataPath("invalid.json"))
	if err == nil {
		t.Fatal("expected error for invalid file in quiet mode")
	}
	if stdout != "" || stderr != "" {
		t.Errorf("expected no output in quiet mode, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestCLIJSONCFile(t *testing.T) {
	stdout, stderr, err := runCmd("--schema", testdataPath("schema.json"), testdataPath("valid.jsonc"))
	if err != nil {
		t.Fatalf("expected success for JSONC, got error: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "valid") {
		t.Errorf("expected 'valid' in output, got: %s", stdout)
	}
}

func TestCLINoSchemaFlag(t *testing.T) {
	_, _, err := runCmd(testdataPath("valid_with_schema.json"))
	if err != nil {
		t.Fatalf("expected success with $schema in document, got error: %v", err)
	}
}

func TestCLINoSchemaAnywhere(t *testing.T) {
	_, _, err := runCmd(testdataPath("no_schema.json"))
	if err == nil {
		t.Fatal("expected error when no schema available")
	}
}

func TestCLIMissingFile(t *testing.T) {
	_, _, err := runCmd("--schema", testdataPath("schema.json"), "nonexistent.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestCLIDraftFlag(t *testing.T) {
	_, _, err := runCmd("--draft", "7", "--schema", testdataPath("schema.json"), testdataPath("valid.json"))
	if err != nil {
		t.Fatalf("expected success with --draft 7, got error: %v", err)
	}
}

func TestCLINoAssertFormat(t *testing.T) {
	_, _, err := runCmd("--no-assert-format", "--schema", testdataPath("schema.json"), testdataPath("invalid.json"))
	if err == nil {
		t.Fatal("expected error even with --no-assert-format (invalid.json has other errors)")
	}
}

func TestCLIStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		w.Write([]byte(`{"name": "Test", "email": "test@example.com"}`))
		w.Close()
	}()

	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	_, _, err = runCmd("--schema", testdataPath("schema.json"))
	if err != nil {
		t.Fatalf("expected success from stdin, got error: %v", err)
	}
}

func TestCLIJSONOutputInvalid(t *testing.T) {
	stdout, _, err := runCmd("--json", "--schema", testdataPath("schema.json"), testdataPath("invalid.json"))
	if err == nil {
		t.Fatal("expected error for invalid file")
	}
	if !strings.Contains(stdout, `"valid": false`) || !strings.Contains(stdout, `"errors"`) {
		t.Errorf("expected JSON output with errors, got: %s", stdout)
	}
}
