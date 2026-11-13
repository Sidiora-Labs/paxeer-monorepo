# Jev advisory checks

This directory holds the Python client, command-line interface and advisory checks
for the Paxeer network repository. The checks evaluate qualification records, pull
requests, test output, Rust dependency advisories and documentation translations,
then write ranked JSON and Markdown reports for review.

## Layout

- [`__init__.py`](__init__.py): Python package marker.
- [`__main__.py`](__main__.py): entry point for `python3 -m tools.jev`.
- [`cli.py`](cli.py): check loading, shared arguments and report output.
- [`client.py`](client.py): typed decision questions, HTTP requests, response validation and call metadata.
- [`report.py`](report.py): findings, confidence routes, ranking and report serialization.
- [`checks/`](checks/): the `ledger`, `pr`, `failures`, `deps` and `docs` checks, their registry and focused tests.
- [`tests/`](tests/): client, report and check tests, recorded responses and input fixtures.

## Build and test

The Python package runs directly from the repository root and has no separate
compilation target. Its runtime uses the Python standard library; test modules
also import `pytest`. The root [Makefile](../../Makefile) provides the test command:

```sh
make jev-test
```

That target runs `python3 -m unittest discover -s tools/jev/tests -t .`.
Both `tests/` and `checks/` also register with the root `test-tools-python` target
through their `python-tests.mk` files. Live tests use `OPENROUTER_API_KEY` when
it is present.

The root Makefile also provides these advisory commands:

| Command | Inputs and purpose |
| --- | --- |
| `make jev-ledger` | Classify record severity, assess resolutions and group related observations. |
| `make jev-pr` | Compare pull request and commit claims with changed files. |
| `make jev-failures` | Classify and group failures from supplied test logs. |
| `make jev-deps` | Run `cargo deny` in the selected Rust workspaces and evaluate dependency advisories. |
| `make jev-docs` | Compare translated sections with their source documentation. |

Advisory calls use the configured decision service. The CLI requires `--out` for
the report directory and accepts `--model`, `--log` and `--dry-run`; the latter
prepares a decision request without sending it.

## Configuration

The client reads these environment variables:

| Name | Purpose |
| --- | --- |
| `OPENROUTER_API_KEY` | Authenticate decision requests. |
| `JEV_ENDPOINT` | Select the decision service endpoint. |
| `JEV_MODEL` | Select the model; `--model` takes precedence. |

The Makefile accepts `JEV_OUT`, `JEV_MODEL`, `JEV_BASE`, `JEV_HEAD`, `JEV_PR_BODY`,
`JEV_LOGS`, `JEV_FORMAT` and `JEV_WORKSPACES` to select output, model and inputs.

Check inputs include qualification ledgers selected with `--kvx`, with repository
discovery using `spec/*/qualification.kvx`; a pull request body selected with
`--body` and the checklist in
[`.github/pull_request_template.md`](../../.github/pull_request_template.md);
test logs selected with `--log-file`; and dependency scan captures selected with
`--input`. The dependency check reads workspace and member `Cargo.toml` manifests.
The documentation check reads the root [README.md](../../README.md) and translated
Markdown files, with `--source` and `--translations` selecting the comparison.

## Documentation

- [Workflow secrets, variables and runners](../../docs/ci/secrets-and-runners.md): configuration for the Jev pull request advisory.
- [README translations](../../docs/readme/): translated documentation consumed by the documentation check.
