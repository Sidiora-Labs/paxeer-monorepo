# Jev decisions

Implementation and test paths on this page are relative to `tools/hyperfide`.

Jev is TypeSafe AI's System One decision model, reached through OpenRouter as
`typesafe/jev-1.13`. It answers typed questions about a supplied state.
Hyperfide's local graph, memory, spec and snapshots run locally. Jev-backed
features use the decision endpoint; changelog summaries and recap have separate
remote configuration.

Direct decisions and memory classification require `OPENROUTER_API_KEY`.
Annotations on qualification, guard findings and pull requests are advisory:
qualification and command exit status come from deterministic checks.

The implementation is `core/native/src/jev.c`; the integration suite is
`tests/integration/28_jev.sh`. Examples use the canonical `hyperfide` command;
`cg` is its deprecated compatibility alias.

## Question types

| Type | Answer | Use |
| --- | --- | --- |
| `noul` | Probability that a statement is true, with confidence | Yes/no judgments |
| `choice` | One of 2–255 labelled options, with probabilities and confidence | Classification |
| `score` | An ordinal level, with probabilities and confidence | Ranking |

A request carries questions about one state. Canonical serialization sorts
question names and criteria keys, uses compact JSON and preserves the state
text. The request hash identifies those bytes in the call log.

## Transport

The transport runs `curl` through `popen` with a private configuration file.
The key stays out of the command arguments, and a separate file supplies the
request body. Both files have mode `0600` and live under `.codegraph/`, or
`/tmp/codify-<uid>` outside a project. They are removed after the call.

HTTP `429`, HTTP `529` and connection failures are retried. The defaults are
four attempts with a 500 ms initial backoff, doubled for each retry. Other
HTTP errors return the status and a bounded response excerpt.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `OPENROUTER_API_KEY` | — | Required to send a decision request |
| `HYPERFIDE_JEV_MODEL` | `typesafe/jev-1.13` | Model identifier |
| `HYPERFIDE_JEV_ENDPOINT` | `https://openrouter.ai/api/alpha/decisions` | Decision endpoint |
| `HYPERFIDE_JEV_CURL` | `curl` | Executable path or name resolved on PATH |
| `HYPERFIDE_JEV_TIMEOUT` | `30` | Seconds per attempt; `jev_ask_at`, used by recap, defaults to 180 when unset |
| `HYPERFIDE_JEV_ATTEMPTS` | `4` | Attempts including the first |
| `HYPERFIDE_JEV_BACKOFF_MS` | `500` | Initial retry delay in milliseconds |

The `HYPERFIDE_JEV_*` variables accept their `CG_JEV_*` aliases. Conflicting
values return exit 78 with values redacted. `OPENROUTER_API_KEY` has no alias.
A request refused before transport does not create a call-log entry.

## Commands

### `hyperfide jev doctor`

`hyperfide jev doctor` reports key presence, curl, endpoint, model and log
health. Without `--probe`, success requires a key and a curl executable.
`--probe` also requires a successful small decision request.

Keys longer than 14 characters are displayed as the first six and last four
characters with an ellipsis between them; shorter keys are shown as `set`.
`--json` reports `ok`, `model`, `endpoint`, `key`, `key_hint`, `curl`,
`curl_wanted`, `curl_version`, `timeout_s`, `attempts`, `log`, `calls`, `last_call` and
`probe`. Outside a project there is no project log.

### `hyperfide jev ask`

Pass questions on the command line:

```sh
hyperfide jev ask --state "Choose the documentation category" \
  --choice category "Which category fits this state?" \
  --option user="User guidance" --option developer="Developer reference"
```

`--noul NAME INSTRUCTIONS` adds a probability question.
`--score NAME INSTRUCTIONS` with repeated `--level TEXT` adds an ordinal
question. A complete request can instead come from `request.json` or stdin:

```sh
hyperfide jev ask request.json
hyperfide jev ask -
```

An omitted model in the request body uses the configured model. Text output
includes the answers, served model, token usage, cost and latency. `--json`
returns `model`, `requested_model`, `id`, `usage`, `attempts`, `ms` and
`answers`.

### `hyperfide jev log`

Calls made in a project append one JSON line to `.codegraph/jev.log`.
`hyperfide jev log [-n N]` renders recent entries. Entries record the request
identity, served model, token counts, cost, latency and attempt count, or the
transport error. `hyperfide jev doctor` reports the call count and last-call
time.

## Jev inside deterministic commands

| Command | Question | Annotation |
| --- | --- | --- |
| `hyperfide spec done` after an unsuccessful `verify_cmd` | Category and next action | `jev triage:` line and outcome-memory annotation |
| `hyperfide guard` | Severity per finding | `jev_score`, `jev_level` and ordering |
| `hyperfide fleet pr` | Readiness of the observed feature state | `Jev readiness:` line in the pull request body |

An unavailable key or unusable response produces an advisory stderr message.
The deterministic verdict and exit code remain in effect.

### Failure triage

Triage sends the last 40 lines, capped at 4 KiB, of combined verification
output. Its category choices are `test_failure`, `build_error`,
`missing_dependency`, `flaky`, `environment` and `spec_mismatch`. Its action
choices are `fix_code`, `fix_test`, `rerun`, `install_dependency`,
`revise_spec` and `ask_human`.

A successful response is appended to the task's outcome memory. Qualification
still requires the task's verification and graph checks to pass.

### Guard ranking

Guard ranking uses one request for up to 50 findings and orders ranked
findings by severity. JSON includes `jev_ranked` and per-finding
`jev_score` and `jev_level`; unavailable scores are `null`.
`hyperfide guard --strict` uses its own deterministic findings for exit status.

### Pull request readiness

Readiness uses one `noul` question about the feature, branch, base, commits
ahead and qualified task inventory. The answer annotates the pull request
body. Without a usable response the annotation is omitted.

## Memory classification and skills

`hyperfide memory classify` requires a decision response. Each memory produces
one call with a `choice` among `skill`, `decision`, `constraint`, `fact` and
`noise`, and a `noul` question about reuse beyond its task. The result is
stored in `memories.class` and `memories.confidence`.

With no argument the command selects unclassified memories. `--all`
reclassifies the inventory, an ID selects one memory, and `-n N` caps the
batch. Recall displays the class and confidence; brief uses labels such as
`[decision/skill]`. Both expose `class` and `confidence` in JSON.

### `hyperfide skills`

| Command | Behavior |
| --- | --- |
| `hyperfide skills list` | Lists candidates, promoted copies and stale copies |
| `hyperfide skills promote <id>` | Writes `.agents/skills/<slug>/SKILL.md` from a memory |
| `hyperfide skills render` | Refreshes generated copies and reports orphans |

Generated skill files carry `generated_by: codify`, `source_memory` and the
`codify-owned: memory-skill v1` marker. Promotion refuses to overwrite a file
without that marker; rendering leaves such files alone. Reclassification
keeps an existing promoted file and can make it stale. Removing its source
memory makes it an orphan, reported by rendering and integration diagnostics.
Edit the memory and render again to update a generated skill.

MCP exposes `memory_classify`, `skills_list` and `skills_promote`.
Classification errors are returned as `isError` results.

## Testing without the network

`HYPERFIDE_JEV_CURL` accepts an executable implementing curl's `-K CONFIG`
interface. The integration suite uses `tests/fixtures/jev/fake-curl.sh` to
exercise request serialization, file permissions, retries and log records.
Its `JEV_FAKE_MODE` values are `ok`, `429-once`, `529-once`, `500`, `down`,
`garbage` and `noanswers`.

## Limitations

Jev requests require a network connection and a key. Classification makes one
call per selected memory. The call log is an append-only diagnostic record;
it is read by `hyperfide jev log` and `hyperfide jev doctor`.

The decision transport uses the curl executable. The native binary's
`Makefile` links SQLite, libcurl, libcrypto, libm and pthreads.

For a distributed authority descriptor, the source routes guard ranking,
pull request readiness, memory classification and recap statement selection
through `decision.legacy.<purpose>` with a two-second budget. The top-level CLI
refuses most local commands in that mode with exit 78. See
[authority mode](integrate.md#authority-mode) for command routing.
