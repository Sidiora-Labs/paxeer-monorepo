# Bootstrap qualification

Run commands from the repository root unless shown otherwise. Use the pins in
[toolchain/versions.txt](toolchain/versions.txt), a Docker daemon with Compose,
and Anvil on PATH for the frontend wallet fixture. These are local qualification
fixtures; production credentials are not required.

Paxeer is one network, with the kernel participating in block proposal. Its EVM
chain ID is 125 (`0x7d`), and a receipt is final when its block commits. The local
Compose EVM and in-process Hardhat chain also use 125 but do not reproduce Paxeer
consensus or establish deployment readiness. Public node JSON-RPC uses the root
path. Local fixture commands below use their private chains.

## Source and dependency prerequisites

The web application is `apps/launchpad/web`; its workspace manifest and lockfile
are under `apps/launchpad`. Install with the pinned package manager:

```sh
cd apps/launchpad
npm exec --yes --package=pnpm@9.15.0 -- pnpm install --frozen-lockfile
cd ../..
```

The default contract source is `contracts/launchpad`, inside this repository.
The contract fixture consumes that directory's installed Hardhat dependencies,
compiled `artifacts/contracts`, debug files and build-info. It checks compiled
source contents against current sources and uses an isolated in-process Hardhat
network with `--no-compile`; it does not compile missing or stale artifacts.
The qualifier's `--contract-root` option explicitly selects a different reviewed
contract root. Standalone `contracts/run.sh` requires `CONTRACT_SOURCE_ROOT` and
`FIXTURE_OUTPUT`; the registered qualifier supplies them.

## Current-source baseline gate

The launchpad baseline runner requires a production-system specification and
matching execution plan admitted by the [check registry](check-registry.json).
The specification contains task `0.1`. Set `LAUNCHPAD_SPEC_FILE` to the
specification named by the registry and `QUALIFICATION_OUTPUT_DIR` to an output base
inside the repository's `launchpad/evidence` directory before running. The runner
creates a private task/run subdirectory below that base:

```sh
python3 tools/launchpad/production/qualify.py \
  --task 0.1 \
  --spec "$LAUNCHPAD_SPEC_FILE" \
  --registry tools/launchpad/production/bootstrap/check-registry.json \
  --contract-root contracts/launchpad \
  --evidence-dir "$QUALIFICATION_OUTPUT_DIR" \
  --require-current-source
```

The runner verifies tools, dependency integrity and source provenance, starts
loopback fixtures, deploys contracts on a private EVM, and executes registered
Go, SDK and web checks. At most two contiguous Go test suites overlap; individual
Go builds use one package build worker. Read the registry for the precise suite
selection rather than assuming all source tests run.

`services/launchpad/go.work` is the canonical Go workspace. `baseline.py` pins
its seventeen reviewed production members (fifteen service modules, including
security/control, and the shared/protocol SDK modules) and reconciles the
workspace against them. Recovery-broker and load-eventdriver workspace members,
and discovered test fixtures, are reported separately as auxiliary Go modules.
Every selected module still requires its checksum file and dependency verification;
the collector never repairs manifests or checksums. `baseline.json` records
production modules in `go_modules` and test/tool modules in `auxiliary_go_modules`.

JavaScript membership comes from `apps/launchpad/pnpm-workspace.yaml`: `web`,
`mobile` and `../../sdk/launchpad/packages/*`, plus the workspace root manifest.
The baseline checks workspace names and file/link targets in all four dependency
sections, and records immutable hashes in `npm_workspace_manifests`. Package
manifests in unrelated qualification fixtures are not workspace members.

[build/manifest.json](build/manifest.json) is a source command inventory covering
thirteen modules. Its checker requires exact discovered-module coverage, matching
module identities and Go versions. Registered builds run `go list ./...` and
`go build -p 1 ./...` with `GOWORK=off` and `GOFLAGS=-mod=readonly`. The task-0.1
registry selects required build/test targets independently of dependency
membership. It includes livestream as a build target.

## Evidence and cleanup

The default evidence directory is
`launchpad/evidence/production-system/qualification/0.1/<run>/`.
Read `result.json` for the final result, `progress.json` for completed checks,
`source-before.json` and `source-after.json` for provenance, and `<check-id>.log`
for each registered check. The baseline additionally writes `baseline.json`,
`baseline-command-<index>.log` and `abi-baseline.json` in that run directory.
A nonzero exit, named skip, missing expected test, stale source or incomplete
check fails qualification. Empty utility packages cannot replace a module test suite.

The frontend fixture starts and terminates two ephemeral Anvil instances: a
wallet chain using 125 and a dispatch chain initially using 1. The reusable
Compose fixture persists between checks. Stop it while
preserving its volumes with:

```sh
docker compose -f deploy/launchpad/production/bootstrap/compose.yml down
```

A passing bootstrap establishes the registered existing-suite baseline. Browser
journeys, deployment readiness and 500,000 daily active users require separate
evidence. Continue with the [security prerequisite map](security/GAPS.md),
[worker dispatcher](../dispatcher/README.md),
[integration assurance](../integration/README.md) and
[load workload](../load/README.md).
