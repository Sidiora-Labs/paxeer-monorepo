# Protocol

Paxeer is one BFT network, run by `paxd`, with EVM chain ID `125` (`0x7d`). The kernel is part of the network and participates in block proposal. Agent activities, program execution, accounts and EVM transactions belong to that network's committed history. A receipt is final when its block is committed.

The merged implementation connects the C protocol kernel in `src/protocol/` to the node through `modules/lxphost/`. Keepers and EVM precompiles invoke that host under authenticated authority. Execution and accounting do not wait for a second network to settle a checkpoint.

Normative encodings and interfaces live in `spec/`, `include/layerx/` and the implementation. This page introduces the boundaries; [Modules](modules.md) describes dispatch and [Programs workspace gates](../programs/workspace.md) describes the runtime, ABI and qualification recipes.

---

## One network and its interfaces

Public JSON-RPC uses HTTPS at the root of `api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`, or `api-mainnet-beta.paxeer.network`. For example:

```sh
curl https://api-mainnet-beta.paxeer.network/ \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected chain ID is `0x7d`. Node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`. Hosted `px_` methods have their own contract in `services/platform/hosted/gateway/openrpc.json`; their presence in that contract does not make them node methods. See [Public RPC](../reference/public-rpc.md).

`node.hyperpaxeer.com` is only the hpx library mirror used by build scripts; it is not an RPC endpoint. Sub-chains are tenant domains represented in the database, rather than additional validator deployments; their node-side implementation lives in `modules/layerxchains/`.

---

## The activity (LXC/1)

A native protocol activity is a canonical binary envelope. Fixed-width integers use big-endian encoding; the decoder rejects noncanonical encodings and trailing bytes. The source contract is `include/layerx/lxp_activity.h`, with encoding and signing logic in `src/protocol/lxp_activity.c`.

| Field | Role |
| --- | --- |
| `protocol_version` | Explicit wire version. The header defines 1 (legacy), 2 (occupancy) and 3 (state commitment); the merged host constructs protocol-3 activities. |
| `network_id` | Domain binding checked against the configured network. It is distinct from the EVM chain ID field. |
| `activity_type` | High 16 bits identify the module; low 16 bits identify the operation. |
| `actor_did` and `authority` | Acting identity and its authenticated authority. |
| `account_sequence` | Identity sequence checked by the protocol; transfer-account sequences have a separate role. |
| Timestamp bounds | Validity window checked against the execution timestamp. The merged host derives time from the block context. |
| `idempotency_key` | Binds a repeated request to its original execution evidence. |
| `fee_limit` | Upper bound checked against the deterministic fee calculation. |
| `payload` and `payload_hash` | Module body and its verified digest. |
| `signature` | Authenticates the canonical signing preimage on the native signed-envelope path. |

Do not infer the active wire version from the C header's `LXP_PROTOCOL_VERSION` alias: it still names occupancy version 2. `modules/lxphost/execute.go` explicitly selects state-commitment version 3. Guest WASM ABI versions are a different version space; see [Frozen ABI](../programs/workspace.md#frozen-abi).

EVM callers use the precompile ABI and EVM authentication. The node resolves that authority and invokes the same kernel; clients do not supply a trusted receipt or assert that their own key has been verified. `modules/lxphost/execute.go` and `modules/lxphost/host.go` implement the activity and authority handoff.

---

## Three design rules

1. **One committed history.** Activities have deterministic ordering and state-root evidence inside the network. Finality comes from the BFT commit of the containing block. A signature on an execution result alone is insufficient evidence of that commit.
2. **One financial doorway.** `402LXP` applies validated atomic transfer sets to canonical accounts. Economic modules and programs request monetary effects through the kernel; they do not keep an independent writable balance table. See [One doorway for money](modules.md#one-doorway-for-money).
3. **One reproducible result.** Consensus execution uses committed inputs. Local clocks, outbound HTTP, nondeterministic iteration and floating-point arithmetic must not determine a transition. External observations enter through authenticated, ordered data and bounded host interfaces.

---

## Identity and authority (kernel)

Identity and authority are kernel services shared by modules: DIDs, primary and session keys, scoped grants, revocation and sequence checks. Identity is not an economic module ID. A nested call can narrow authority; it cannot enlarge it. Spending authority and ownership of an account are checked independently of knowing the account's public identifier.

Oracle observations and web responses must be verified inputs to execution. The Programs ABI exposes versioned `oracle_read` and `web_read` host functions; those names do not authorize arbitrary network access from guest WASM. The versioned host tables live in `programs/crates/layerx-programs-runtime/src/abi/manifest.rs`.

---

## Programs: module `0x09`

Programs execute untrusted WASM under the invoking activity's authority. `LXP_MODULE_PROGRAMS` is kernel module ID `9`; this is distinct from the EVM Programs precompile at `0x000000000000000000000000000000000000101D`, registered by `precompiles/layerxprograms/`. Runtime, registry, SDKs and porting kits live under `programs/`.

- **Program-owned accounts.** Accounts derive from `(program_id, seed)` in a domain disjoint from principal accounts. Derivation alone grants no debit authority. A principal can fund a program account without acquiring control over it.
- **Downward-only spending grants.** Nested program calls may convey bounded authority over the caller's program accounts. Source, destination, asset and amount constraints cannot widen. A refusal cannot leave a partial transfer set committed.
- **Occupancy accounting.** Persistent namespace bytes are metered using committed fee parameters. Occupancy charges are ordinary ledger transfers with receipt evidence, rather than a second balance system.
- **Protocol-backed balances.** Registry balance views are backed by canonical account state and verified proofs. Programs emit transfer sets; `402LXP` applies them.

The runtime's default ABI is v4, frozen manifests cover v1 through v5, and AI market uses the ABI-v5 host route. Host registration, validation and deployment metadata must agree on the selected guest ABI. See [Programs workspace gates](../programs/workspace.md), including the AI guest's domain and deployment-size requirements.

---

## From activity to receipt

The protocol decodes the canonical activity, resolves the enabled module and authority, checks network, version, sequence, time, idempotency and fees, then validates and executes a journaled transition. Module effects and transfer legs are committed atomically on success. Execution failure rolls back module effects; admission errors and executed failures are distinct outcomes, so a refusal must not be treated as evidence that a payment occurred.

In the merged host, `modules/lxphost/ctx.go` loads canonical module KV, accounts, identity sequences and idempotency records from the node store. The block context supplies timestamp, height and ordered sequence. `modules/lxphost/host.go` executes the C kernel and publishes the resulting state through that context. `modules/lxphost/simulate.go` supplies the simulation path; a simulated result does not establish a committed transition.

Verify a receipt against the intended chain, operation, authority and resulting state. For committed evidence, also verify inclusion and the containing block's consensus commit under independently trusted chain authority. The stored signed vectors described in [Stored receipt and commit verification](../programs/workspace.md#stored-receipt-and-commit-verification) exercise those checks. SDK consumers must preserve the original signed bytes and refuse mismatched proofs.

Fees are deterministic execution inputs. Their schedule is stored in protocol state; a caller must not substitute an estimate for the committed fee or invent unavailable execution inputs.

---

## Start here

- [Home](../index.md)
- [Public RPC](../reference/public-rpc.md): node endpoints and chain ID
- [Modules](modules.md): declared IDs, host registration and monetary boundaries
- [Programs workspace gates](../programs/workspace.md): crate inventory, ABI versions and build recipes
- [Programs](../programs/index.md): guest development and program operations
- [Assets](../concepts/assets.md): asset and account encodings
