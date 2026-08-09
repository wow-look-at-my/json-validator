# CLI-contract tests for json-validator: exit codes and the messages that
# accompany them, run against the REAL built binary by the org's dats runner
# (github.com/wow-look-at-my/dats). go-toolchain runs this suite automatically
# as its dats phase after every build -- there is nothing to wire into CI.
#
# These exist because the contract they pin was broken and nothing noticed:
# rootCmd sets SilenceErrors and main() only read the exit code, so EVERY way
# of failing to run -- a missing schema, a schema that is not JSON, an
# unresolvable $ref, a mistyped flag -- exited 1 having printed NOTHING. The
# in-process Go tests could not have caught it: they drove rootCmd.Execute()
# directly, which is not the path the binary takes. A black-box suite against
# the shipped binary is the only thing that tests what a CI job actually sees.
#
# Commands exec the freshly built binary as
# "${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/json-validator": go-toolchain's dats
# phase stages throwaway copies under $GO_TOOLCHAIN_DATS_BUILD_DIR and does NOT
# put them on PATH (a bare `json-validator` exits 127 there), while a standalone
# `dats test dats` from the repo root falls back to build/.
#
# Assertion semantics: a LIST entry is substring-contains; a MAP entry is a
# 0-based line number matched as a REGEX.

# SANDBOX OFF. dats sandboxes by default (bubblewrap), and its sandbox gives a
# command a fresh /tmp -- while the binary these tests exec lives in an
# os.MkdirTemp under /tmp, so inside the sandbox that path does not exist and
# every test exits 127. Nothing here needs isolating: offline, secret-free
# tests of our own freshly built CLI.
sandbox: false

tests:
	- desc: a valid document exits 0 and says so on stdout
	  cmd: '"${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/json-validator" --schema {inputs.schema.json} {inputs.doc.json}'
	  inputs:
		files:
			schema.json: |
				{
				  "$schema": "https://json-schema.org/draft/2020-12/schema",
				  "type": "object",
				  "properties": { "name": { "type": "string" } },
				  "required": ["name"]
				}
			doc.json: |
				{"name": "ok"}
	  exit: 0
	  outputs:
		stdout:
			- "valid"

	- desc: an invalid document exits 1 and reports the violation
	  cmd: '"${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/json-validator" --schema {inputs.schema.json} {inputs.bad.json}'
	  inputs:
		files:
			schema.json: |
				{
				  "$schema": "https://json-schema.org/draft/2020-12/schema",
				  "type": "object",
				  "properties": { "name": { "type": "string" } },
				  "required": ["name"]
				}
			bad.json: |
				{"name": 42}
	  exit: 1
	  outputs:
		stderr:
			- "INVALID"

	# An invalid document is the ordinary NEGATIVE RESULT, not a failure to run:
	# it is already reported per file, so it must not also get the generic
	# "json-validator:" failure line.
	- desc: an invalid document is not also reported as a failure to run
	  cmd: '"${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/json-validator" --schema {inputs.schema.json} {inputs.bad.json} 2>&1 | grep -c "json-validator:" || true'
	  inputs:
		files:
			schema.json: |
				{
				  "$schema": "https://json-schema.org/draft/2020-12/schema",
				  "type": "object",
				  "properties": { "name": { "type": "string" } },
				  "required": ["name"]
				}
			bad.json: |
				{"name": 42}
	  exit: 0
	  outputs:
		stdout:
			0: "^0$"

	# --quiet means EXIT CODE ONLY, for results.
	- desc: quiet mode prints nothing for an invalid document
	  cmd: '"${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/json-validator" --quiet --schema {inputs.schema.json} {inputs.bad.json} 2>&1 | wc -c'
	  inputs:
		files:
			schema.json: |
				{
				  "$schema": "https://json-schema.org/draft/2020-12/schema",
				  "type": "object",
				  "properties": { "name": { "type": "string" } },
				  "required": ["name"]
				}
			bad.json: |
				{"name": 42}
	  exit: 0
	  outputs:
		stdout:
			0: "^ *0$"

	# ---- Failures to RUN. Every one of these exited 1 printing NOTHING. ----

	- desc: a schema file that does not exist names the file, not just exit 1
	  cmd: '"${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/json-validator" --schema "$(dirname "{inputs.doc.json}")/absent.schema.json" {inputs.doc.json}'
	  inputs:
		files:
			doc.json: |
				{"name": "ok"}
	  exit: 1
	  outputs:
		stderr:
			- "json-validator:"
			- "absent.schema.json"
			- "no such file or directory"

	- desc: a schema that is not JSON says so
	  cmd: '"${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/json-validator" --schema {inputs.not-a-schema.json} {inputs.doc.json}'
	  inputs:
		files:
			not-a-schema.json: |
				this is not json
			doc.json: |
				{"name": "ok"}
	  exit: 1
	  outputs:
		stderr:
			- "json-validator:"
			- "not-a-schema.json"

	# The case that cost a real debugging detour in webhook-runner: a relative
	# $ref resolves against the document's $id, so it is fetched, and the failure
	# was invisible.
	- desc: a schema whose $ref cannot be resolved says so
	  cmd: '"${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/json-validator" --schema {inputs.dangling.schema.json} {inputs.doc.json}'
	  inputs:
		files:
			dangling.schema.json: |
				{
				  "$schema": "https://json-schema.org/draft/2020-12/schema",
				  "$id": "https://example.invalid/dangling.schema.json",
				  "allOf": [{ "$ref": "nowhere.schema.json" }]
				}
			doc.json: |
				{"name": "ok"}
	  exit: 1
	  outputs:
		stderr:
			- "json-validator:"
			- "dangling.schema.json"

	- desc: a schema invalid against its metaschema says so
	  cmd: '"${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/json-validator" --schema {inputs.badkeyword.schema.json} {inputs.doc.json}'
	  inputs:
		files:
			badkeyword.schema.json: |
				{
				  "$schema": "https://json-schema.org/draft/2020-12/schema",
				  "type": 12345
				}
			doc.json: |
				{"name": "ok"}
	  exit: 1
	  outputs:
		stderr:
			- "json-validator:"
			- "metaschema"

	# --quiet suppresses RESULTS, never the fact that no result could be
	# computed -- the way `grep -q` still reports a missing file.
	- desc: quiet mode still reports a failure to run
	  cmd: '"${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/json-validator" --quiet --schema "$(dirname "{inputs.doc.json}")/absent.schema.json" {inputs.doc.json}'
	  inputs:
		files:
			doc.json: |
				{"name": "ok"}
	  exit: 1
	  outputs:
		stderr:
			- "json-validator:"
			- "absent.schema.json"

	- desc: a mistyped flag names the flag
	  cmd: '"${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/json-validator" --frobnicate {inputs.doc.json}'
	  inputs:
		files:
			doc.json: |
				{"name": "ok"}
	  exit: 1
	  outputs:
		stderr:
			- "json-validator:"
			- "unknown flag: --frobnicate"

	- desc: mutually exclusive flags are refused out loud
	  cmd: '"${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/json-validator" --json --quiet {inputs.doc.json}'
	  inputs:
		files:
			doc.json: |
				{"name": "ok"}
	  exit: 1
	  outputs:
		stderr:
			- "json-validator:"
			- "[json quiet]"

	- desc: a document that does not exist is reported per file
	  cmd: '"${GO_TOOLCHAIN_DATS_BUILD_DIR:-build}/json-validator" --schema {inputs.schema.json} "$(dirname "{inputs.schema.json}")/absent.json"'
	  inputs:
		files:
			schema.json: |
				{
				  "$schema": "https://json-schema.org/draft/2020-12/schema",
				  "type": "object"
				}
	  exit: 1
	  outputs:
		stderr:
			- "absent.json"
			- "no such file or directory"
