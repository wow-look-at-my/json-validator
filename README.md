# json-validator

Validate JSON and JSONC files against [JSON Schema](https://json-schema.org/) (2020-12 by default).

## Installation

```sh
go install github.com/wow-look-at-my/json-validator@latest
```

## Usage

```sh
# Validate using $schema from the document
json-validator config.json

# Validate multiple files
json-validator config.json data.json settings.jsonc

# Override schema with a local file
json-validator --schema schema.json config.json

# Override schema with a URL
json-validator --schema https://example.com/schema.json config.json

# Read from stdin
cat config.json | json-validator --schema schema.json

# JSON output
json-validator --json config.json

# Quiet mode (exit code only)
json-validator --quiet config.json && echo "valid"
```

## JSONC Support

JSONC (JSON with Comments) is supported transparently. Line comments (`//`), block comments (`/* */`), and trailing commas are stripped before validation. Both `.json` and `.jsonc` files work.

## Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--schema` | `-s` | | Path or URL to JSON Schema (overrides `$schema` in document) |
| `--json` | | `false` | Output errors as JSON |
| `--quiet` | `-q` | `false` | Suppress output; exit code only |
| `--draft` | `-d` | `2020` | Default draft version when schema has no `$schema` (4, 6, 7, 2019, 2020) |
| `--no-assert-format` | | `false` | Disable format assertions |

## Format Assertions

Unlike the JSON Schema 2020-12 spec default, format assertions are **enabled by default**. The `format` keyword (`email`, `date-time`, `uri`, etc.) will reject invalid values.

To disable format assertions:

```sh
json-validator --no-assert-format config.json
```

Or via environment variable:

```sh
export JSON_VALIDATION_ALLOW_SILENT_FAILURES=assert-format
json-validator config.json
```

The environment variable accepts a comma, semicolon, or space delimited list of features to disable.

## Schema Resolution

1. If `--schema` is provided, that schema is used for all files.
2. Otherwise, the `$schema` field in each JSON document determines the schema.
3. If neither is available, validation fails with an error.

Schemas can be local file paths or HTTP/HTTPS URLs. Well-known meta-schema URIs (e.g., `https://json-schema.org/draft/2020-12/schema`) are built in.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All inputs valid |
| 1 | Validation failure or load error |
| 2 | CLI usage error |

## GitHub Action

Use `wow-look-at-my/json-validator` as a GitHub Action to validate JSON/JSONC files in CI. The action builds the tool with caching (subsequent runs skip the build) and runs it.

### Zero-config (recommended)

With no inputs, the action auto-discovers all `*.json` and `*.jsonc` files containing a `$schema` field and validates them:

```yaml
- uses: wow-look-at-my/json-validator@v1
```

### Explicit files

```yaml
- uses: wow-look-at-my/json-validator@v1
  with:
    files: 'config.json settings.jsonc'
    schema: 'schema.json'
```

### Advanced: extra CLI flags

```yaml
- uses: wow-look-at-my/json-validator@v1
  with:
    files: 'config.json'
    args: '--draft 7 --no-assert-format --json'
```

### Inputs

| Input | Required | Description |
|-------|----------|-------------|
| `files` | No | Space-separated files or glob patterns to validate. When omitted, auto-discovers all `*.json`/`*.jsonc` files containing `$schema`. |
| `schema` | No | Path or URL to JSON Schema (overrides `$schema` in documents) |
| `args` | No | Additional CLI arguments (e.g. `--draft 7 --no-assert-format --json`). When provided without `files`, skips auto-discovery. |


## Supported Drafts

- JSON Schema Draft 4
- JSON Schema Draft 6
- JSON Schema Draft 7
- JSON Schema 2019-09
- JSON Schema 2020-12 (default)

## License

MIT
