# Modules

The merged network's kernel dispatches activities by module ID and operation ordinal. Identity, authority, sequencing, state journals and receipts are shared kernel services. Modules use those services inside the same network; [Protocol](index.md) describes the execution and finality model.

## The declared set (0x01–0x0b)

`include/layerx/lxp_module.h` declares these stable IDs. They occupy the high 16 bits of `activity_type`; the low 16 bits carry the operation ordinal. A declared ID alone does not prove that a particular host has registered or enabled it.

| ID | Module | Role and current host boundary |
| --- | --- | --- |
| `0x01` | asset | Asset registration, per-asset accounts and validated ledger transfers. |
| `0x02` | escrow | Lock, capture and release under escrow terms. |
| `0x03` | budget | Bounded spending and budget lifecycle. |
| `0x04` | stream | Metered payments and stream lifecycle. |
| `0x05` | service | Service agreements, delivery and escrow-linked outcomes. |
| `0x06` | perps | Perpetual-market operations; the merged host registers the oracle-transport interface. |
| `0x07` | governance | Protocol parameters and governed transitions. |
| `0x08` | bridge | Declared bridge ID; not registered by the current `lxphost` reset path. |
| `0x09` | programs | Deterministic WASM execution, program accounts and lifecycle. |
| `0x0a` | spot | Spot-market transitions. |
| `0x0b` | web | Declared Web ID; not registered by the current `lxphost` reset path. Web keeper and Programs host interfaces are separate integration points. |

`modules/lxphost/host.go` registers governance, Programs, asset, budget, escrow, the perps oracle-transport interface, spot, stream and service when resetting the host. Programs registration selects `programs_module_registration_v5()` for guest ABI 5 and `programs_module_registration_v4_storage_retirement()` otherwise. This registration policy is separate from the immutable guest ABI manifests.

C module sources live in `src/modules/`; node keepers live in `modules/`. Kernel module IDs are not EVM precompile addresses. In particular, Programs ID `0x09` is reached through the EVM Programs precompile at `0x000000000000000000000000000000000000101D`, whose implementation and ABI live in `precompiles/layerxprograms/`.

---

## One doorway for money

`402LXP` applies monetary transitions to the canonical account registry. Modules and guests emit validated transfer sets through the kernel. Transfer legs commit atomically and conserve value per asset; asset issuance and burning use the issuance account under their own authorization rules.

Locked value is held in real accounts. Canonical names include:

```text
agent:<did>:main
agent:<did>:asset:<lowercase hex64 asset_id>
agent:<did>:budget:<id>
agent:<did>:escrow:<id>
agent:<did>:stream:<id>
agent:<did>:margin:<position>
module:asset:value:<lowercase hex64 issuance_account_id>
module:programs:value:<account_id>
system:fees
system:paxeer-reserve
system:paxeer-withdrawals
```

`src/ledger/lx_account_id.c` implements account-name parsing and derivation. The issuance account ID is derived from the `asset:<lowercase hex64 asset_id>:issuance` seed, while `lx_asset_issuance_name` produces its `module:asset:value:` stored name. Program account derivation is domain-separated from principal accounts.

**One record per account and asset.** An `lx_account` holds an account ID, an asset binding, a balance and its account sequence. `agent:<did>:main` is `LX_ACCOUNT_AGENT_MAIN`; `agent:<did>:asset:<hex64>` is `LX_ACCOUNT_AGENT_ASSET` (14). The parser, not an old comment about main accounts, defines that distinction. An account's asset cannot silently rebind after it has been established. The enum is in `include/layerx/lxp_ledger.h`.

The activity's identity sequence and a transfer set's account sequence serve different checks. Do not substitute an identity's sequence for the sequence of a different asset account. Opening another per-asset account does not copy the owner's native-account counter.

---

## What each module is for

### Asset

The header `include/layerx/lx_asset.h` retains ordinal names 1 through 11. The actual dispatch table in `src/modules/asset/lx_asset_registry.c` registers:

| Ordinal | Operation | Current C transition |
| --- | --- | --- |
| 1 | register | Register an asset and its issuance account. |
| 2 / 3 | pause / unpause | Issuer-authorized change of the asset's paused flag. Payload: `version:u16=1 || asset_id32`. |
| 4 | account_open | Open the actor's per-asset account. |
| 5 | send | Transfer from an authorized main or per-asset account. |
| 9 | withdraw | Execute the C withdrawal transition, with the account and custody checks described below. |
| 10 / 11 | mint / burn | Authorized supply changes compiled to ledger legs. |

Ordinals 6 (`receive`), 7 (`grant_issue`) and 8 (`grant_revoke`) are retired payer-grant operations, not active C module transitions. The decoder rejects them with `LXP_ERR_UNKNOWN_ACTIVITY`. The merged ledger keeper's `Draw`, `IssueGrant` and `RevokeGrant` methods also refuse the retired route with `ErrNoPayerGrant` after their applicable pause and authority checks (`modules/layerxledger/keeper/draw.go` and `grants.go`). Account funding must follow an authorized current transfer path.

Register uses `issuer_kind = 1` for native assets and `2` for Paxeer custody. The current persisted enum is `LX_ASSET_CUSTODY_NATIVE = 1`, `LX_ASSET_CUSTODY_PAXEER = 2`; use the definitions and record validation, rather than the outdated numbering in the header's introductory comment. The canonical record binds issuance authority, custody data and supply.

WITHDRAW has a C transition; it is not evidence that every submission API exposes a withdrawal operation. Its payload is `asset_id32 || amount16 || payout_recipient20 || checkpoint_id32 || fee8` (108 bytes). `fee8` must agree with the activity fee limit. The transition debits the admitted principal account into `system:paxeer-withdrawals`, stages a `withdrawal:` record and rejects an already recorded request with `LXP_ERR_WITHDRAWAL_ALREADY_SETTLED`. `lxp_kernel_withdraw_execution_sequence` checks admission facts and refuses an account mismatch with `LXP_ERR_CONTEXT_MISMATCH`. The presence of the legacy `checkpoint_id` field in this encoding does not add a second finality stage to the merged network. Sources: `src/modules/asset/lx_asset_registry.c`, `lx_asset_custody.c` and `src/protocol/lxp_kernel.c`.

Pause and unpause affect canonical asset state. Repeating a transition in the same state returns `LXP_ERR_PAUSED_SCOPE`; paused assets refuse affected economic operations with `LXP_ERR_ASSET_PAUSED`. Register, account-open and supply encodings are defined by `lx_asset.h` and `src/modules/asset/lx_asset_decode.c`.

### Escrow, budget, stream and service

Escrow records terms for locking, capturing and releasing funds. Budget bounds spending through its lifecycle and authority checks. Stream implements metered payments. Service records agreements and delivery outcomes, with escrow links read through the host. Their money movements remain ledger transfer sets; module records do not create independent balances.

### Perps, spot and governance

Perps defines positions, margin and oracle-dependent market operations; the merged host specifically registers `lx_perps_oracle_transport_module_iface()`. Spot has its own module ID and registration. Governance owns governed protocol settings. Interface availability must be checked against the actual registered implementation rather than inferred from the module's name.

### Bridge and external assets

Bridge ID `0x08` remains allocated, but the merged host registration above does not install a C bridge runtime. External-asset and bank integration belongs to the node keepers and authenticated precompile paths that implement it. The old opt-in custody-credit profile is retired; it is not an additional funding prerequisite for the merged network.

---

## Programs: module `0x09`

Programs execute untrusted guest WASM under the invoking activity's authority. They do not gain direct balance-writing authority. Runtime, registry, SDKs, AI market, arbiter and porting kits live in `programs/`; see [Workspace and crate layout](../programs/workspace.md#workspace-and-crate-layout).

- **Program-owned accounts.** A program derives accounts from `(program_id, seed)`. Derivation is public and deterministic, while debit authority remains restricted to the owning program and validated call authority.
- **Downward-only grants.** A nested call can receive bounded spending authority over its caller's program accounts. It cannot widen the permitted source, destination, asset or amount; a refused call cannot leave partially applied monetary effects.
- **Occupancy accounting.** Namespace storage is metered and priced by committed parameters. Charges use ordinary ledger transfers and receipt evidence.
- **Verified balance views.** Balance proofs refer to canonical protocol account state. Registry metadata alone does not establish funds.

Guest ABI versions and module registration versions must agree at deployment, validation and execution. The default runtime ABI is v4; v5 adds `market_step_adjudicate`, used by the AI-market host route. ABI 1 through 5 have frozen vectors. See [Frozen ABI](../programs/workspace.md#frozen-abi) and [AI-market deployment qualification](../programs/workspace.md#ai-market-deployment-qualification).

---

## Kernel boundary

`lxp_module_iface` in `include/layerx/lxp_module.h` defines genesis, decode, validate, execute, epoch hooks, state-root and release callbacks. The module context supplies namespaced KV, staged accounts, transfer-set and event emission, batch time and gas accounting. It does not supply a general-purpose clock, HTTP client or balance setter.

`modules/lxphost/host.go` binds asset and escrow runtimes to restored canonical registries, supplies the service escrow reader, and binds Programs to accounts, transfer assets and the kernel's metering and occupancy parameter resolvers. `modules/lxphost/ctx.go` loads and publishes state through the node store with block time, height and sequence. This is the merged node's host binding.

`lxp_kernel_epoch_transition` remains the kernel API for epoch hooks. Its staged writes and journal are not a separate host deployment or a promise that a particular maintenance loop is scheduled. Establish active maintenance behavior from its node caller and the registered module hooks. State and receipts become final with the containing BFT block, as described in [Protocol](index.md).

## Start here

- [Home](../index.md)
- [Protocol](index.md): one network, activity envelopes and committed receipts
- [Programs workspace gates](../programs/workspace.md): ABI, host boundaries and qualification recipes
- [Programs](../programs/index.md): guest lifecycle and calls
- [Assets](../concepts/assets.md): asset and account encodings
