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
- `validator/validator.go` -- schema loading, compilation, validation logic;
  `Options`, `Result` (+ `AsError`/`Detail`), `NewCompiler`, `CompileSchema`,
  `Validate`, `ValidateFile`.
- `validator/embed.go` -- the EMBEDDING surface: `Validator` (compile once,
  validate many, concurrency-safe), `New`, `NewFromBytes`, `CompileBytes`,
  `ValidateBytes`.

**This package is consumed two ways and both are first-class**: the CLI/action
here, and as a library inside other Go programs (webhook-runner validates every
hook.json/manager.json against the published schema at LOAD through it, so the
runtime gate and CI are one implementation). That makes its API a real
contract -- a signature change breaks a consumer in another repo -- and it
imposes rules the CLI alone would not:

  - The ZERO VALUE of `Options` must stay usable (draft 2020-12, format
    assertions on). Defaults belong here, never in the flag definitions.
  - The library reads NO environment. `JSON_VALIDATION_ALLOW_SILENT_FAILURES`
    is resolved by `cmd/root.go` into `Options.NoAssertFormat`
    (`SilentFailureAllowed` is exported for anyone wanting it explicitly). Never
    reintroduce an env read into validation itself: a host program's strictness
    must not depend on its ambient environment.
  - Nothing in `validator/` prints or exits -- results are values.
  - The CLI's mirror of that rule: **`cmd.Execute()` is what prints a failure
    to RUN**, and it must keep doing so. `rootCmd` sets `SilenceErrors` so
    cobra prints nothing, and `main()` only reads the exit code -- so until
    this existed, a schema that did not exist, was not JSON, or carried an
    unresolvable `$ref`, plus any mistyped flag, exited 1 having printed
    NOTHING. Tests drive `Execute()`, never `rootCmd.Execute()`, or they
    exercise a path `main()` never takes. An INVALID document is the one
    exception (`errValidationFailed`): already reported per file, so it gets
    no second line. `--quiet` suppresses RESULTS only -- a failure to run
    still reaches stderr, the way `grep -q` reports a missing file.
- `dats/` -- **CLI-contract tests: exit codes and the messages that go with
  them.** Black-box, run by the org's [dats](https://github.com/wow-look-at-my/dats)
  runner against the REAL built binary; go-toolchain runs them automatically as
  its dats phase after every build, so there is nothing to wire into CI. Suites
  declare `sandbox: false` and exec the binary as
  `"${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/json-validator"` (the phase stages
  throwaway copies under that dir and does NOT put them on PATH; the `:-build`
  fallback keeps a standalone `dats test dats` working). New exit-code or
  stderr-contract behavior belongs HERE, not in `cmd/*_test.go`: the in-process
  tests drive cobra and cannot see what the shipped binary prints, which is
  exactly how every failure-to-run went silent unnoticed.
- `testdata/` -- test fixture JSON/JSONC files and schemas
- `action.yml` -- composite GitHub Action. Downloads the prebuilt binary from
  pazer.build for the runner's OS/arch (with a build-from-source fallback if the
  download fails), then runs it. Logic is tsc-checked TypeScript steps via
  `wow-look-at-my/actions@typescript#latest`, mirroring the xml-validator pattern.

## Key Design Decisions

- Format assertions are **on by default** (unlike the JSON Schema 2020-12 spec default). Disable with `--no-assert-format` or `JSON_VALIDATION_ALLOW_SILENT_FAILURES=assert-format`.
- JSONC support is transparent -- all input runs through `jsonc.ToJSON()` before parsing.
- `$schema` in the document is used to auto-detect the schema when `--schema` flag is not provided.

## Dependencies

- `github.com/santhosh-tekuri/jsonschema/v6` -- JSON Schema validation engine
- `github.com/tidwall/jsonc` -- JSONC comment/trailing comma stripping
- `github.com/spf13/cobra` -- CLI framework
