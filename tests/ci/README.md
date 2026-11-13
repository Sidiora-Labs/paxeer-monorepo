# CI repository tests

This directory holds Python tests, suite runners and test data for the Paxeer X Network repository. The checks cover workflow wiring, dependency and toolchain consistency, container inputs, documentation, SDK contracts and native test integration, keeping the node, kernel and supporting packages aligned within the merged network.

## Layout

- [data/](data/) holds suite inventories, exclusions, artifact allowlists and runtime support metadata used by the checks and runners.
- [run_python_suites.py](run_python_suites.py) runs the command, daemon, explorer, conformance, integration and hosted Python suites selected by its suite inventory.
- [interop_workspace_test.py](interop_workspace_test.py) discovers Cargo workspace test targets and runs interop unit, integration and documentation tests.
- [retired_tests.py](retired_tests.py) provides shared assertions for absent paths and symbols, plus a helper that builds and runs C suites.
- [test_tests_ci_modules_collected.py](test_tests_ci_modules_collected.py) checks that Python modules are collected as tests, imported as helpers or invoked by tracked scripts.
- [test_workflow_runs_on_routed.py](test_workflow_runs_on_routed.py) checks runner selection in GitHub Actions workflows.
- [test_workflow_secrets_documented.py](test_workflow_secrets_documented.py) checks workflow configuration names against their documentation.
- [test_core_test_all_wired.py](test_core_test_all_wired.py) checks native suite wiring and inspects the built kernel archive.
- [test_copied_artifacts_in_sync.py](test_copied_artifacts_in_sync.py) checks packaged copies against their source artifacts.
- [test_markdown_links_resolve.py](test_markdown_links_resolve.py) resolves tracked Markdown links against the repository.
- [test_python_suites_have_runner.py](test_python_suites_have_runner.py) and [test_js_suites_have_runner.py](test_js_suites_have_runner.py) check Python and JavaScript test coverage in CI.
- [test_websearch_context.py](test_websearch_context.py) checks the container build inputs for the web-search service.

## Build and test

Run commands from the repository root. This directory has no separate package build or Makefile. The root [Makefile](../../Makefile) provides the build prerequisites and test entry points:

```sh
make test-tests-python
```

This target builds the kernel library, supplies its path to the Python tests, runs this directory alongside the other repository Python checks, and runs the CI shell tests. The [CI workflow](../../.github/workflows/ci.yml) declares the native dependencies and installs `pytest`, `PyYAML` and [tests/bridge/requirements.txt](../bridge/requirements.txt). Some checks invoke native builds and Cargo commands, so the suite also needs the toolchains declared by that workflow.

The suite runners in this directory are also called by these root targets:

```sh
make test-python-suites
make interop-test
```

`test-python-suites` builds its command and explorer prerequisites before calling `run_python_suites.py`. `interop-test` calls `interop_workspace_test.py` with the Makefile's `INTEROP_CARGO` and `INTEROP_MANIFEST` settings.

## Configuration

- `LAYERX_TEST_LIBRARY` selects the native archive inspected by the core checks; the root test target supplies it. `CC` selects the C compiler used by those checks.
- `PYTHONPATH`, `CARGO_TARGET_DIR`, `CARGO_BUILD_JOBS` and `CARGO_TERM_COLOR` are used when checks run Python or Cargo subprocesses.
- The standalone container build entry point in `test_websearch_context.py` reads `PAXEER_X_EVIDENCE_DIR`, `PAXEER_X_BUILD_WRAPPER`, `CARGO_BUILD_JOBS` and `PAXEER_X_DOCKER`.
- [data/python_suite_exclusions.txt](data/python_suite_exclusions.txt), [data/python_suite_baseline.txt](data/python_suite_baseline.txt) and [data/interop_test_baseline.json](data/interop_test_baseline.json) supply suite selection data to the runners. The interop runner accepts `--cargo`, `--manifest-path` and `--baseline-path` options.
- [data/js_suite_exclusions.txt](data/js_suite_exclusions.txt), [data/intentional_copies.txt](data/intentional_copies.txt), [data/large_file_allowlist.txt](data/large_file_allowlist.txt), [data/runtime_end_of_life.json](data/runtime_end_of_life.json) and [data/vendored_patched_crates.txt](data/vendored_patched_crates.txt) supply inventory and consistency rules to individual checks.
- The checks read configuration from [.github/workflows/](../../.github/workflows/), [.github/actionlint.yaml](../../.github/actionlint.yaml), the root [Makefile](../../Makefile), and the package manifests, lockfiles and container definitions they inspect.

## Documentation

- [Workflow secrets, variables and runners](../../docs/ci/secrets-and-runners.md) describes the configuration inventory checked by this suite.
- [Release readiness matrix](../../docs/release/7.0.0-readiness-matrix.md) lists this directory's build and test entry points alongside the other repository projects.
