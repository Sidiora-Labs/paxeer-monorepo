# Qualification

Paxeer X Network qualification combines evidence for the kernel, its `paxd`
host, chain storage, APIs and consumers. These are parts of one network:
the kernel participates in block proposal and execution, and receipts become
final when their blocks are committed. A library test or build provides only
the evidence it actually exercises.

The core library is C17 (`-std=c17` in the root `Makefile`), the Go node uses
Go 1.25.8 (`go.mod`), and the Rust toolchain is 1.91.1
(`rust-toolchain.toml`). Solidity projects pin 0.8.27 in `foundry.toml` and
`foundry.paxeer.toml`. The node links the kernel and programs sandbox through
`modules/lxphost/host.go`; see [Monorepo layout](MONOREPO.md#build-boundaries).

## Evidence levels

| Level | Evidence | Representative gate or source |
| --- | --- | --- |
| Source integrity | Generated specifications are current; publication excludes credentials and private operational evidence | `make public-audit` |
| Build and unit behavior | Strict C17 compilation, native tests, Solidity tests and the linked Go node | `make build`, `make test`, `make test-contracts`, `make paxeer-build`, `make paxeer-test` |
| Runtime safety | Applicable sanitizer suites and consensus symbol and floating-point exclusion | `make ci`, `make qualify-arith` |
| Deterministic replay | Identical published digest across optimization levels, compilers, libc implementations and architectures; a mutated root fails at the expected sequence | `make qualify-replay` |
| Fault and adversarial behavior | Interrupted durable writes, restart recovery, malformed activities, signatures and transfer sets | `make qualify-faults`, `make qualify-fuzz` |
| Integrated network behavior | Host execution, native balances, committed receipts, tenant isolation and state-proof verification through their real producers and consumers | `make test-paxd-domain`; node, RPC and consumer evidence described below |
| Deployment evidence | Reviewed binaries and configuration, upgrade rehearsal, committed-state checks and restart/recovery evidence for the selected deployment | Release runbook and evidence bound to the deployed revision |

The [network overview](paxeer-network.md) describes the chain and public API.
Qualification results apply to the exact source, configuration and environment
exercised by each suite. The commands below describe available coverage.

## Standard suites

From the repository root, the kernel and root contract entrypoints are:

```sh
make public-audit
make build
make test
make test-contracts
make ci
```

`make ci` performs the native suite, a reproducible two-build archive comparison,
consensus symbol checks, and ASan, UBSan and TSan runs. ThreadSanitizer requires
a host that permits the address-space layout used by the compiler runtime.
`make test-contracts` uses the root `foundry.toml` and `tests/solidity`;
the `contracts/src` project instead uses `foundry.paxeer.toml` as described
in [Build and test](paxeer-network.md#build-and-test).

## Replay qualification

```sh
make qualify-replay
```

[`tools/lxp_qual_replay_matrix.sh`](../tools/lxp_qual_replay_matrix.sh)
requires GCC 13, Clang 18 and Docker. Its containers supply an amd64 musl
runner and an AArch64 cross-compiler with QEMU. It generates a
10-million-activity corpus and root ledger under `build/qualification/replay/`.
Keep those large generated artifacts out of source control; the compact
published digest is `tests/vectors/qualification_replay_10m.digest`.

The script compares all runner digests with that published digest. It also
mutates the root ledger and requires rejection at the expected divergent
sequence. This establishes replay behavior for the corpus, while node commit
and recovery need integrated evidence.

## Arithmetic qualification

```sh
make qualify-arith
```

This gate combines boundary cases, checked 128-bit and 256-bit operations,
rounding and conservation assertions, CBMC/Z3 proof obligations, consensus
floating-point and libm exclusion, and sanitizer runs. It requires CBMC, Z3
and the Clang runtime selected by
[`tools/lxp_arith_proof.sh`](../tools/lxp_arith_proof.sh).

## Fault and fuzz qualification

```sh
make qualify-faults
make qualify-fuzz
```

Fault qualification interrupts processes at durable write boundaries and
verifies recovery. Fuzz qualification seeds malformed activity, signature and
transfer-set mutations from the generated replay corpus, and includes ASan
and UBSan runs. A crash, sanitizer finding, non-canonical acceptance or
partially committed transition blocks qualification.

These library suites must be complemented by integrated node recovery:
reopen committed storage and confirm that balances, receipts and idempotency
records survive, while uncommitted work does not become final. Existing
evidence paths include
[`modules/lxphost/idempotency_restart_test.go`](../modules/lxphost/idempotency_restart_test.go)
and the node's merge rehearsal in
[`node/merge_rehearsal_test.go`](../node/merge_rehearsal_test.go).

## Network commit and deployment gates

Release evidence must follow execution through the node, storage commit,
API transport and the consuming service or SDK. It must demonstrate:

- A receipt belongs to a committed block, with the correct chain identity,
  height and transaction or activity identity. An admission acknowledgement
  or pending receipt is insufficient. `node/abci.go` publishes stashed block
  notifications only after a successful storage commit. Transaction and
  receipt transport parity is exercised in `rpc/proof_client_parity_test.go`.
- State witnesses are decoded and verified against the appropriate trusted
  root, with that root bound to committed chain evidence. Native witness
  decoding and verification live in `layerxproof/codec/state.go`. Each proof
  format must be checked against its own contract.
- Restart and recovery preserve committed state and reject duplicate effects;
  malformed proofs, mismatched identities and unavailable dependencies fail
  explicitly along the producer-to-consumer path.
- Tenant-domain requests, balances, logs and subscriptions remain scoped to
  the requested domain. The main network and tenant views share the same
  consensus and commit authority.

Before a live upgrade, validator change, asset movement or contract deployment,
use the deployment's approved runbook and review the evidence for its exact
revision and configuration. Local test results alone do not authorize those
operations or establish the deployed feature set.

## Paxeer qualification

The Go node entrypoints are driven by `chain.mk` at the repository root:

```sh
make paxeer-build
make paxeer-lint
make paxeer-test
make paxeer-ci
```

`make paxeer-build` builds the native prerequisites before linking `paxd`.
`make paxd-build` also stamps the release version; the `node-build.yml`
workflow checks the resulting version and native archive digest.
`make test-paxd-domain` builds the native prerequisites and exercises
`modules/lxphost`, `node`, the EVM keeper and RPC packages.

For aggregate workspace qualification, `make monorepo-ci` runs
`workspace-ci`: source audit, inventory, workspace build, tests and lint.
Language-specific results contribute to the same network's evidence; none
alone proves the complete host, commit, storage and consumer path.
See [RELEASE.md](../RELEASE.md) for release coverage.
