# CI tools

This directory holds CI helpers and tests for the Paxeer X Network monorepo. They validate consensus-change coverage, render mainnet-beta reports, run wallet tests, and check build targets, workflow routing and container inputs for the merged network.

## Layout

- [beta-report.sh](beta-report.sh): validates a qualification ledger and renders its go/no-go report.
- [consensus_commit_gate.py](consensus_commit_gate.py): checks that consensus-changing commits in a Git revision range appear in the ledger stored at the head revision.
- [consensus_ledger_rows.py](consensus_ledger_rows.py): prints missing consensus-ledger rows in oldest-first topological order.
- [run_wallet_vitest.py](run_wallet_vitest.py): runs the wallet SDK's Vitest suite using its recorded test selection.
- [repair-22-33-731.kvx](repair-22-33-731.kvx): links consensus-gate behavior to its test cases.
- [tests/](tests/): Python and shell tests for CI helpers, Makefile targets, workflows, image publication and wallet checks; includes shared workflow readers and consensus-history fixtures.

## Build and test

The Python and Bash helpers run directly and have no separate build target. Run the following commands from the repository root; their targets are declared in the root [Makefile](../../Makefile).

| Command | Purpose |
| --- | --- |
| `make test-tests-python` | Builds the kernel library prerequisite, runs the repository Python tests including `tools/ci/tests`, then runs the image-publication and workflow-runner shell checks. |
| `make beta-contract-check` | Validates the report ledger with `tools/ci/beta-report.sh --check`. |
| `make beta-report` | Renders the report with `tools/ci/beta-report.sh`. |

The Python tests use pytest and PyYAML. The [CI workflow](../../.github/workflows/ci.yml) declares their dependency installation and native prerequisites. The shell checks are also callable as `bash tools/ci/tests/publish-images.test.sh` and `bash tools/ci/tests/workflow-runner-check.sh`, as written in the Makefile.

The [release-check workflow](../../.github/workflows/paxeer-release-check.yml) invokes the consensus gate from the repository root:

```sh
python3 tools/ci/consensus_commit_gate.py "$PR_BASE_SHA" "$PR_HEAD_SHA"
```

## Configuration

- `beta-report.sh` reads [spec/layerx-beta/qualification.kvx](../../spec/layerx-beta/qualification.kvx); `--ledger` selects another ledger, and `--check` validates without rendering the full report.
- Both consensus helpers take positional `base` and `head` revisions and read [docs/release/consensus-commits.md](../../docs/release/consensus-commits.md) at the head revision. Their `--list` option selects another ledger path within the repository.
- The release-check workflow supplies `PR_BASE_SHA` and `PR_HEAD_SHA` as command-line arguments to the consensus gate.
- `run_wallet_vitest.py` reads [tests/ci/data/wallet_vitest_baseline.json](../../tests/ci/data/wallet_vitest_baseline.json), runs pnpm and Vitest in [sdk/wallet/](../../sdk/wallet/), and forwards additional command-line arguments to Vitest.
- The root test target supplies `LAYERX_TEST_LIBRARY` to the Python suite. Tests inspect the root Makefile and the workflow files under [.github/workflows/](../../.github/workflows/).

## Documentation

- [Consensus-changing commits and coverage check](../../docs/release/consensus-commits.md)
- [Release readiness matrix](../../docs/release/7.0.0-readiness-matrix.md)
- [Release candidate report](../../docs/release/7.0.0-candidate-report.md)
- [Documentation site: Installation & Build](../../docs/site/paxeer-docs/src/app/installation/page.tsx)
