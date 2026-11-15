# Paxeer X tools

This directory holds candidate inventory, release verification, route catalogues,
governance proposal rendering and SID holder export tools for Paxeer X. Paxeer uses the LayerX kernel inside `paxd`, which executes and orders each
transaction and builds each block; validators check and commit it. A receipt is
final when its block is committed. These tools describe and check network components and prepare
upgrade inputs.

## Layout

- [build/](build/) — Makefile fragments for native builds and proof tests.
- [builders/](builders/) — production web settings artifact builder.
- [commitfixture/](commitfixture/) — Go tests that generate and validate CometBFT commit fixtures.
- [gates/](gates/) — executable verification scripts, shared helpers and the gate registry.
- [gov/](gov/) — governance proposal templates, renderer and offline transaction tests.
- [sid-snapshot/](sid-snapshot/) — SID holder exporter and reconciliation tests.
- [tests/](tests/) — candidate, dispatcher, route, catalogue and release evidence tests.
- [candidate.py](candidate.py) — creates and validates candidate manifests and selects runtime observations.
- [candidate_contract.go](candidate_contract.go) — candidate policy, dependency resolution and network foundation checks invoked by the Python tool.
- [ai_evidence.py](ai_evidence.py) — validates AI admission and supporting evidence records.
- [runtime-inventory.sh](runtime-inventory.sh) — bounded wrapper for runtime inventory selection.
- [verify-task.sh](verify-task.sh) — dispatches registered verification scripts and validates their evidence against the checkout and candidate.
- [release.sh](release.sh) — resolves release selectors, runs distinct registered gates and writes a release evidence record.
- [route-catalogue.json](route-catalogue.json) — route declarations used by the gateway and candidate policy.
- [retired_callers.json](retired_callers.json) — caller inventory used by retirement checks.
- [candidate_contract_test.go](candidate_contract_test.go) — Go tests for candidate policy and dependency decisions.

## Build and test

Run commands from the repository root. Python and shell entry points run directly;
`candidate.py` invokes `go run` for `candidate_contract.go`, so candidate operations
also require Go. There is no standalone Makefile or package manifest in this
directory. The [root Makefile](../../Makefile) includes its build fragments and
[Python suite registration](tests/python-tests.mk).

| Command | Purpose and source |
| --- | --- |
| `make paxeer-x-arbiter-prestate-build` | Builds arbiter prestate artifacts using [build/104.35.11.mk](build/104.35.11.mk). |
| `make test-tools-python` | Runs the registered tools Python suites, including `tools/paxeer-x/tests`, through the root Makefile. |
| `timeout 5m python3 tools/paxeer-x/tests/gate_dispatch_test.py` | Tests dispatcher and release behavior; command from [gates/0.2.sh](gates/0.2.sh). |
| `bash tools/paxeer-x/gov/test.sh` | Builds `paxd` unless supplied through `PAXD`, then checks rendered proposals and offline unsigned transactions. |

The native build uses the root Makefile's compiler and Programs toolchain
settings. Python suites require `pytest`; governance tests also require Bash,
`jq`, Go and a compatible `paxd`. The
[governance kit](gov/README.md) and [SID exporter](sid-snapshot/README.md)
describe their additional commands and integration prerequisites.

`verify-task.sh` accepts a registered selector and an optional `--manifest` path.
`release.sh` accepts selectors, scope files or requirement selectors and an
optional `--manifest` path. Both bind evidence to the source checkout and
candidate manifest. Runtime inventory inputs are sanitized private JSON records;
candidate inventory does not authorize runtime changes.

## Configuration

The main entry points read these environment variables; no values are specified
here:

- `PAXEER_X_CANDIDATE_MANIFEST` — candidate manifest consumed by verification.
- `PAXEER_X_EVIDENCE_DIR` — private evidence output directory outside the checkout.
- `PAXEER_X_MAINLINE` — mainline reference used for candidate validation.
- `PAXEER_X_ROUTE_CASES` — route test plan consumed by the routed contract suite.
- `PAXEER_X_QUALIFICATION_BUILD_RECORD` — build record consumed by durable release checks.
- `PAXAI_ADMISSION_RECORD` and `PAXAI_APPROVAL_RECORD` — AI evidence input records.
- `PAXD` — governance renderer and test binary selection.
- `GOV_TEST_OUT` — governance test output directory.

Candidate commands take the service catalogue through `--spec` and private JSON
inputs through `--inventory`, `--input` or the manifest argument. Verification
reads [gates/registry.kvx](gates/registry.kvx) and the service declarations in
[spec/layerx-merge/spec.kvx](../../spec/layerx-merge/spec.kvx). Candidate policy
embeds [route-catalogue.json](route-catalogue.json).

Governance rendering reads [gov/templates/](gov/templates/) and, for bridge
proposals, the manifest [services/bridge/deploy/attestors.json](../../services/bridge/deploy/attestors.json)
or the file selected by `--bridge-manifest`, plus each `--bridge-config` file.
The SID exporter takes CSV or Postgres sources and RPC settings as command-line
arguments. Individual [gates/](gates/) and [build/](build/) recipes declare their
component-specific artifact inputs and toolchain settings.

## Documentation

- [System routes](../../docs/system/routes.md)
- [Hosted gateway](../../docs/wiki/HostedGateway.md)
- [Candidate release report](../../docs/release/7.0.0-candidate-report.md)
- [Release readiness checklist](../../docs/release/7.0.0-readiness-checklist.md)
- [Governance kit](gov/README.md)
- [SID holder exporter](sid-snapshot/README.md)
- [Documentation site: unified network](../../docs/site/paxeer-docs/src/app/unified-network/page.tsx)
- [Documentation site: governance](../../docs/site/paxeer-docs/src/app/governance/page.tsx)
