# json-validator

JSON Schema validator CLI tool in Go.

## Build & Test

```sh
go-toolchain
```

This handles mod tidy, vet, test, coverage, and build. Never use bare `go` commands.

The built binary is output to `build/json-validator`.

## Project Structure

- `main.go` -- entry point
- `cmd/root.go` -- cobra root command (CLI flags, arg handling, output formatting)
- `internal/validator/validator.go` -- schema loading, compilation, validation logic
- `testdata/` -- test fixture JSON/JSONC files and schemas
- `action.yml` -- composite GitHub Action (build with caching + run)

## Key Design Decisions

- Format assertions are **on by default** (unlike the JSON Schema 2020-12 spec default). Disable with `--no-assert-format` or `JSON_VALIDATION_ALLOW_SILENT_FAILURES=assert-format`.
- JSONC support is transparent -- all input runs through `jsonc.ToJSON()` before parsing.
- `$schema` in the document is used to auto-detect the schema when `--schema` flag is not provided.

## Dependencies

- `github.com/santhosh-tekuri/jsonschema/v6` -- JSON Schema validation engine
- `github.com/tidwall/jsonc` -- JSONC comment/trailing comma stripping
- `github.com/spf13/cobra` -- CLI framework
