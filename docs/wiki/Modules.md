# Modules

Protocol modules execute inside the Paxeer network's kernel. Paxeer mainnet-beta
is one BFT chain, with EVM chain ID `125` (`0x7d`); the kernel is part of the
network and proposes blocks. An execution receipt is final when its block is
committed. Module execution does not wait for settlement on another network.

The C module interface is declared in `include/layerx/lxp_module.h`. Its live node
host is `modules/lxphost/host.go`, which registers the interfaces, binds account
and asset state, and invokes the kernel in the node's execution context.
[Hosted webhooks](HostedWebhooks.md) describes delivery of events backed by
committed node receipts; webhook delivery is a separate application operation
from block commitment.

---

## The registered set

A declared module ID is not a promise that every deployment enables it. IDs
occupy the high 16 bits of `activity_type`; the low 16 bits select the operation.
Use the registered interface's activity list rather than assuming every ordinal
is supported.

| ID | Module | Purpose and current registration |
| --- | --- | --- |
| `0x01` | [Asset](Asset.md) | Asset registration, accounts, transfers and supply operations; registered by the live node host |
| `0x02` | [Escrow](Escrow.md) | Lock, capture and release funds; registered by the live node host |
| `0x03` | [Budget](Budget.md) | Spending ceilings and lifecycle; registered by the live node host |
| `0x04` | [Stream](Stream.md) | Metered payments and lifecycle; registered by the live node host |
| `0x05` | [Service](Service.md) | Offers, commitments, delivery, acceptance and disputes; registered by the live node host |
| `0x06` | [Perps](Perps.md) | Positions, margin, funding and liquidation; the live node host registers the oracle-transport interface |
| `0x07` | [Governance](Governance.md) | Governed protocol parameters and authority; registered by the live node host |
| `0x08` | bridge | Reserved identifier retained in the header; absent from the current genesis registration table and live node host |
| `0x09` | [Programs](Programs.md) | Deterministic guest execution; the live node host selects the storage-retirement v4 or guest-ABI-v5 registration |
| `0x0A` | [Spot](Spot.md) | Two-asset order matching and settlement; registered by the live node host |
| `0x0B` | web | Reserved identifier retained in the header; absent from the current genesis registration table and live node host |

Runtime sources live in `src/modules/`. The standalone genesis plan in
`src/protocol/lxp_genesis.c` is explicit: Programs is always selected; Asset and
Governance are selected for state-commitment protocol 3; Escrow, Budget, Stream,
Service, Perps and Spot use enable flags and default to disabled in that plan.
Programs variants and Perps oracle/time-in-force variants depend on additional
genesis flags. Genesis rejects module values for Bridge and Web with
`LXP_ERR_UNKNOWN_MODULE`. The node host's registration in
`modules/lxphost/host.go` is a separate, concrete list; do not infer it from the
standalone plan's defaults.

---

## One doorway for money

`402LXP` applies monetary transfer sets. Modules stage account and module state
and emit authorized transfer legs through the kernel; they have no general
`set_balance` capability. Transfer sets apply atomically and conserve value per
asset. Issuance and burning move units through the asset's issuance account and
update the asset supply record under the same kernel checks.

Locked funds have canonical accounts. Examples accepted by
`src/ledger/lx_account_id.c` include:

```text
agent:<did>:main
agent:<did>:asset:<lowercase hex64 asset_id>
agent:<did>:budget:<id>
agent:<did>:escrow:<id>
agent:<did>:stream:<id>
agent:<did>:margin:<position>
module:asset:value:<lowercase hex64 account-id>
module:programs:value:<lowercase hex64 account-id>
system:fees
system:paxeer-reserve
system:paxeer-withdrawals
```

These names describe account namespaces, not a list of accounts provisioned at
genesis. `lx_account` in `include/layerx/lxp_ledger.h` holds one account ID, one
asset binding, one balance and one `next_sequence`. A per-asset principal account
is `LX_ACCOUNT_AGENT_ASSET` (`14`); `agent:<did>:main` is
`LX_ACCOUNT_AGENT_MAIN` (`1`). The parser distinguishes these account kinds.
A balance lookup for the wrong bound asset returns `LXP_ERR_ASSET_MISMATCH`.

Named accounts use the `LX:ACCOUNT:v1` derivation. The issuance account ID derives
from `asset:<lowercase hex64 asset_id>:issuance`; its canonical stored name is
`module:asset:value:<issuance-account-id>`, constructed by
`lx_asset_issuance_name`. A module value name carries its derived account ID in
its final 64 hex characters. See [Assets](Assets.md) for the asset interface.

Identity admission and monetary sequencing are distinct checks. A transfer set
carries its sequence account and that account's next sequence. For example, the
asset supply path sequences a mint against the issuance account and a burn
against the debited holder account (`src/modules/asset/lx_asset_execution.h`).
Do not substitute a DID's other account counter when constructing a transfer.

---

## Not modules

| Surface | Where it lives |
| --- | --- |
| Identity and authority | Kernel admission and the native identity/authorization modules; universal checks on callers and accounts |
| Oracle intake | Authenticated external data admitted to deterministic execution; module execution does not fetch a price over HTTP |
| Hosted delivery | Application services such as [Hosted webhooks](HostedWebhooks.md); delivery success does not establish consensus finality |

Programs **is** module `0x09`. It is not an additional balance writer.
Sub-chains are tenant domains in the database; a tenant namespace does not
require a new chain or a second module execution service.

---

## What each module is for

**Asset.** The current `activity_types` list in
`src/modules/asset/lx_asset_registry.c` contains register (`1`), pause (`2`),
unpause (`3`), account_open (`4`), send (`5`), mint (`10`) and burn (`11`).
The old receive/grant_issue/grant_revoke ordinals (`6`–`8`) are
retired and are not registered activities. Do not build a receive-grant flow
from the older ordinal constants.

Register uses issuer kind `1` for native issuance and `2` for `paxeer_custody`.
The persisted `lx_asset_custody_kind` now uses the same values:
`LX_ASSET_CUSTODY_NATIVE = 1`, `LX_ASSET_CUSTODY_PAXEER = 2`
(`include/layerx/lx_asset.h`). Only the recorded issuer can pause or unpause an
asset. Their payload is `version:u16=1 || asset_id32`; a redundant change returns
`LXP_ERR_PAUSED_SCOPE`. Paused transfers and supply operations are refused with
`LXP_ERR_ASSET_PAUSED`.

**Escrow.** Lock, capture and release money under stored terms. Monetary effects
use kernel transfer sets.

**Budget.** Allocate a ceiling and enforce spend and lifecycle rules. Budget
state and monetary effects participate in the kernel transition.

**Stream.** Meter payments under stream lifecycle rules using deterministic
execution time and authorized account transfers.

**Service.** Record offers, commitments, delivery, acceptance and disputes. The
node host binds a service runtime that reads escrow links through
`lxphost_read_escrow_link`; service is not merely an unbound registration.

**Perps.** Maintain positions, margin, funding, liquidation and insurance state.
The node host registers `lx_perps_oracle_transport_module_iface`; prices must
arrive through its authenticated input path.

**Governance.** Apply governed parameter and authority transitions. Governance
state also participates in the evidence that consumers verify.

**Bridge.** The old C Bridge ID is not in the active registration set. The
network's native EVM integration is a different surface; see
[Unified Network Architecture](UnifiedNetwork.md). The former
[custody credit profile](Custody.md) and external-deposit-backed credit path
are retired.

**Spot.** Match two-asset orders and settle the fill's legs through kernel
monetary transfers. See [Spot](Spot.md).

---

## Programs: module `0x09`

Programs run guest code on the deterministic WASM runtime under the invoking
activity's authority. Runtime, registry and SDK sources live in `programs/`;
registration lives in `src/modules/programs/registration.c`.

- Program-owned value accounts derive from the program and seed. Knowing an
  account's derivation does not grant a principal authority to debit it.
- Nested spending authority must narrow the caller's authority. Guest execution
  produces effects for kernel validation and application.
- Program monetary effects use `402LXP` transfer sets and canonical account
  state, rather than a guest-maintained balance counter.
- Storage and occupancy behavior depends on the selected registration. The live
  host chooses `programs_module_registration_v4_storage_retirement()` or
  `programs_module_registration_v5()`; do not apply the older occupancy-batch
  description to both variants.

See [Programs](Programs.md) and `programs/README.md`. Match the guest ABI to the
registration selected by the node.

---

## Kernel boundary

`lxp_module_iface` declares `genesis`, `decode`, read-only `validate`, `execute`,
`epoch_begin`, `epoch_end`, `state_root` and optional `release` callbacks. The
context exposes namespaced KV, staged accounts, transfer sets, events, protocol
time and gas charging (`include/layerx/lxp_module.h`). It does not expose wall
clock reads, randomness, HTTP calls or arbitrary balance assignment to guests.

The live binding is in `modules/lxphost/host.go`: `lxphost_reset` registers the
interfaces and `lxphost_bind_assets` restores asset state and binds Asset,
Escrow, Service and Programs runtimes. Service reads escrow state from the same
kernel. `modules/lxphost/execute.go` constructs activities using the node block
time. The kernel provides `lxp_kernel_epoch_transition` for epoch callbacks;
that API alone does not prove a deployment invokes every maintenance path.

Receipts and state are produced within the node's block execution. Consumers
that claim `block-committed` must verify the corresponding commit evidence, as
[Hosted webhooks](HostedWebhooks.md#event-contracts) describes.

---

## Start here

- [Home](Home.md)
- [Protocol](Protocol.md)
- [Asset](Asset.md) · [Escrow](Escrow.md) · [Budget](Budget.md) · [Stream](Stream.md) · [Service](Service.md) · [Perps](Perps.md) · [Governance](Governance.md) · [Spot](Spot.md)
- [Programs](Programs.md)
- [Assets](Assets.md) · [Fees](Fees.md)
- [Hosted webhooks](HostedWebhooks.md) · [Hosted dashboard](HostedDashboard.md)
- [Unified Network Architecture](UnifiedNetwork.md)
