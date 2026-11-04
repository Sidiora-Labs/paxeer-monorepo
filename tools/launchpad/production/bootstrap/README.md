# Bootstrap qualification

Run commands from the repository root unless shown otherwise. Use the pins in
[toolchain/versions.txt](toolchain/versions.txt), a Docker daemon with Compose,
and Anvil on PATH for the frontend wallet fixture. These are local qualification
fixtures; production credentials are not required.

Paxeer is one network, with the kernel participating in block proposal. Its EVM
chain ID is 125 (`0x7d`), and a receipt is final when its block commits. The local
Compose EVM and in-process Hardhat chain also use 125 but do not reproduce Paxeer
consensus or establish deployment readiness. Public JSON-RPC uses the root URL,
for example `https://api1.mainnet-beta.paxeer.network/`; local fixture commands
below must use their private chains.

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

The launchpad baseline runner expects its own production-system specification
and reviewed [check registry](check-registry.json). The configured invocation is:

```sh
python3 tools/launchpad/production/qualify.py \
  --task 0.1 \
  --spec launchpad/spec/production-system/spec.kvx \
  --require-current-source
```

The configured `launchpad/spec/production-system/spec.kvx` is absent from this
checkout. The command needs that reviewed launchpad specification restored or an
explicit reviewed `--spec` file with the runner's expected task schema. Do not
substitute `spec/layerx-merge/spec.kvx`: this is bootstrap task `0.1`, not a
layerx-merge task gate.
The runner verifies tools, dependency integrity and source provenance, starts
loopback fixtures, deploys contracts on a private EVM, and executes registered
Go, SDK and web checks. At most two contiguous Go test suites overlap; individual
Go builds use one package build worker. Read the registry for the precise suite
selection rather than assuming all source tests run.

[build/manifest.json](build/manifest.json) declares the shared and protocol Go
modules under `sdk/launchpad` and service modules under `services/launchpad`.
The build checker requires its module list to cover discovered launchpad modules
exactly, and `baseline.py` still requires exactly thirteen. Additional modules
can therefore fail the baseline; an inventory is not proof that the current tree
builds. The task-0.1 registry excludes livestream from its required test targets
while retaining its build target; this does not mean livestream has no tests.

## Evidence and cleanup

The default evidence directory is
`launchpad/evidence/production-system/qualification/0.1/<run>/`.
Read `result.json` for the final result, `progress.json` for completed checks,
and the retained command logs and source snapshots for provenance. A nonzero
exit, named skip, missing expected test, stale source or incomplete check fails
qualification. Empty utility packages cannot replace a module test suite.

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
