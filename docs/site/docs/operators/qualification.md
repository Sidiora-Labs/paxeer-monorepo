# Qualification

Qualification records evidence for the merged Paxeer network. The `paxd` node
embeds the LayerX kernel and Programs runtime; native libraries, Go modules,
precompiles, and hosted clients are parts of the same execution path. A passing
component suite establishes only the behavior it exercises.

Mainnet-beta uses EVM chain ID 125 (`0x7d`). The kernel is part of the network
and proposes blocks. A transaction receipt becomes final when its block is
committed. Transaction admission, simulation, and an estimated fee are not
proof of a successful committed transaction. See the [network overview](../index.md).

## Evidence levels

| Level | Evidence | Representative gate or record |
| --- | --- | --- |
| Source integrity | Publication audit and generated-source checks | `make public-audit` |
| Native runtime | C compilation, native behavior, and arithmetic invariants | `make build`, `make test` |
| Contract behavior | Solidity state-surface check and offline Foundry suite | `make test-contracts` |
| Runtime safety | Native reproducibility, consensus-symbol checks, and sanitizer suites | `make ci`, `make qualify-arith` |
| Deterministic replay | Corpus digest agrees across optimization levels, compilers, libc implementations, and architectures | `make qualify-replay` |
| Fault and adversarial behavior | Durable-write interruption and recovery; malformed activity, signature, and transfer-set rejection | `make qualify-faults`, `make qualify-fuzz` |
| Integrated node | Native host, chain execution, and node RPC behavior | `make test-paxd-domain` |
| Deployment evidence | Correct release binary, upgrade activation, committed receipts, balance conservation, recovery, and tenant isolation on the intended deployment | Reviewed deployment runbook and retained evidence |

Requirements are defined in
[`spec/layerx-merge/spec.kvx`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/spec/layerx-merge/spec.kvx).
The commands below exercise different components and integration boundaries;
a component result establishes only the behavior that command covers.

## Standard suites

The C runtime uses C17 (`Makefile`). Rust workspaces use the pinned toolchain in
`rust-toolchain.toml` (1.91.1); the Go module declares Go 1.25.8. The default
Foundry profile pins Solidity 0.8.27 (`foundry.toml`). Use the dependency and
native-library setup required by the target before invoking it.

```sh
make public-audit
make build
make test
make test-contracts
make ci
```

`make build` produces the native library and supporting tools; it does not build
the complete node. `make test-contracts` runs the state-surface script followed
by offline Foundry tests. `make ci` depends on `public-audit`, `test`,
`reproducible`, `scan-consensus`, and `test-sanitizers`. ThreadSanitizer needs a
host compatible with its address-space requirements.

## Replay qualification

```sh
make qualify-replay
```

`tools/lxp_qual_replay_matrix.sh` requires GCC 13, Clang 18, and Docker. Its
container runners provide amd64 musl and an AArch64 cross-compiler with QEMU.
It creates the 10-million-activity corpus and root ledger in
`build/qualification/replay/` and compares runner digests with
`tests/vectors/qualification_replay_10m.digest`. Keep the large generated
corpus out of version control. This qualifies native replay; it does not by
itself establish node recovery, transaction inclusion, or RPC correctness.

## Arithmetic qualification

```sh
make qualify-arith
```

This target combines checked 128-bit and 256-bit arithmetic, boundaries,
rounding and conservation assertions, floating-point exclusion, and
`tools/lxp_arith_proof.sh`. The proof script requires CBMC, Z3, and the selected
Clang runtimes, and exercises address, undefined-behavior, thread, memory, and
leak instrumentation. Preserve failed proof and sanitizer output as evidence.

## Fault and fuzz qualification

```sh
make qualify-faults
make qualify-fuzz
```

The fault target runs `tests/test_qual_recovery.c` with the qualification fault
harness and checks durable-write interruption and restart behavior. The fuzz
target requires the replay corpus (`QUALIFICATION_CORPUS`) and runs activity,
signature, and transfer-set mutations, including ASan and UBSan variants.
A crash, sanitizer finding, noncanonical acceptance, or partially committed
transition is a qualification failure.

## Network and deployment gates

Qualify the real node path: submit a signed chain transaction, observe its
committed block and receipt, check status and logs, and reconcile the relevant
native balances and module state. A committed failure remains final but does
not establish that the requested operation succeeded.

Recovery evidence must exercise node restart and durable state, receipt lookup
after restart, and safe retry behavior. Tenant-domain evidence must show that
state, authority, and receipt queries remain scoped to the selected domain.
For an upgrade, verify the intended plan and precompile activation against the
running release. Native replay or fault harness results alone cannot establish
these deployment properties.

Public JSON-RPC uses the root of `api-mainnet-beta.paxeer.network` or the public
node names `api1` through `api14.mainnet-beta.paxeer.network`; verify chain ID
125 before submitting transactions. Hosted `px_` methods have their contract in
`services/platform/hosted/gateway/openrpc.json`.

Deployment, validator changes, upgrades, and real-value canaries follow the
approved operational runbook. Local qualification records evidence; it does
not execute those production actions.

## Paxeer node qualification

From the repository root:

```sh
make paxd-build
make paxd-version-check
make test-paxd-domain
```

`make paxd-build` builds native libraries and the release-stamped `build/paxd`;
`make paxd-version-check` compares its version with `tools/release/version.json`.
`make test-paxd-domain` runs the native host tests and the Go node, EVM keeper,
and RPC packages with CGO enabled.

The existing `make paxeer-build`, `make paxeer-lint`, `make paxeer-test`, and
`make paxeer-ci` targets delegate to `chain.mk`; they are component build and
qualification targets for this same network. `make monorepo-ci` delegates to
`workspace-ci` and is an aggregate release gate. Record exactly which target
ran, its revision, exit code, and retained log location; never infer a broader
qualification result from a narrower passing command.
