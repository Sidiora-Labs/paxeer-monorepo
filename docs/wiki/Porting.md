# Program porting crates

The Solana, EVM, and CosmWasm porting kits translate selected account, storage,
message, and value-flow patterns to the network's deterministic Programs ABI.
Their guest helpers and `reference-v2` programs target `layerx_v2`, ABI version
2. The separate reference qualification builders use the runtime's default
`ABI_VERSION`, currently 4. The kits translate selected patterns rather than
compile arbitrary source-chain programs or deploy source-chain bytecode unchanged.

Programs run in the merged Paxeer network: the kernel is part of `paxd`, and
BFT block commitment supplies transaction finality. A port does not create
another execution or settlement network. See [Sequencing](Sequencing.md) for
receipts and commitment, [Data availability](DataAvailability.md) for retained
execution data, and [Programs](Programs.md) for the program lifecycle.

The members and library names are declared in
[`programs/Cargo.toml`](../../programs/Cargo.toml) and each kit's manifest:

| Kit package | Workspace member | Rust library | Reference port |
| --- | --- | --- | --- |
| `layerx-porting-solana` | `porting/solana` | `programs_porting_solana` | `MintLimitPort`, configured by `GuardTerms` |
| `layerx-porting-evm` | `porting/evm` | `programs_porting_evm` | `PublicLockPort`, configured by `LockTerms` |
| `layerx-porting-cosmwasm` | `porting/cosmwasm` | `programs_porting_cosmwasm` | `DonationPort`, configured by `DonationTerms` |

Each has a `guest` workspace member for WASM context/syscall helpers and a
separate `reference-v2` guest. The `programs_porting_*()` identification functions
return their `programs/porting/<kit>` path and ABI version.

## State, authority, and value

Principal-scoped storage uses `(program, principal)`; shared storage uses
`(program)`. A port must choose the correct namespace and supply the matching
read/write capabilities. A shared cell is supported; it does not confer
unrestricted access to another program's state or a native balance.

The kits preserve source storage bytes or key derivations where described
below. They also collapse caller-indexed state into a principal namespace,
so an import plan must map source identities to authenticated principals.
Do not assume that every source key or source address can be used unchanged.

All three kits expose `ValueFlow::translate`,
`ValueFlow::translate_with_program_account`, `translate_all`, `Transfer402Plan`,
`ProgramAccountTransferPlan`, and `TranslatedValueFlow` from `monetary.rs`.
`translate_all` uses the principal-funded `translate` path; use the explicit
derived-account path for program-funded payouts.

A `Transfer402Plan` names an asset, recipient account, and exact u128 amount;
its capability bounds the principal-funded transfer. A
`ProgramAccountTransferPlan` additionally binds owner program, seed, and source
account. Construction rederives the source and refuses a mismatch or reserved
monetary fields with `InvalidProgramAccount`. Its `ProgramSpend` capability
bounds the exact payout. Public derivation seeds alone grant no spending
authority. Register the program account before funding it; see
[Assets](Assets.md) and [Payments developer path](PaymentsQuickstart.md).

These are monetary requests to the kernel. Writing a storage cell that resembles
a balance does not transfer native assets. Stored token allowances also do not
authorize debits from another principal's kernel account.

## `layerx-porting-solana`

### Purpose

The [Solana kit](../../programs/porting/solana/src/lib.rs) preserves Anchor account
bytes (eight-byte discriminator followed by `borsh` fields), instruction
discriminators, and event discriminators. Source account export and import must
still respect the destination namespace and schema bounds. The
[migration guide](../../programs/porting/solana/MIGRATION.md) describes the account
and payment mapping.

### Public entry points and types

| API | Use |
| --- | --- |
| `AccountSchema`, `Field`, `FieldType`, `FieldValue` | Declare account layout and encode/decode bounded account data |
| `account_discriminator`, `instruction_discriminator`, `event_discriminator` | Derive the eight-byte Anchor discriminators |
| `InstructionAbi::new` / `data` | Encode discriminator plus typed instruction fields |
| `AnchorEvent::new` / `data` | Encode event data; discriminator supplies the topic |
| `Pubkey`, `SeedPath` | Validate source identity and seed/bump derivation; form or collapse storage keys |
| `ported_account`, `per_signer_import` | Map account layout and per-holder migration cells |
| `AccountRole::translate` | Map account roles to storage or capabilities |
| `cross_program_invocation` | Form a `CpiRequest` with an explicit `Call` capability |

`AccountRole::ProgramState` maps to `NamespacedCell`, and
`ProgramOwnedShared` maps to `SharedCell`. A signer is the authenticated invoking
principal; a program handle becomes a call capability. Native token balances
remain kernel-owned rather than writable account data.

On `wasm32`, `anchor::context` re-exports the guest helpers.
`AnchorContext::current` obtains the executing program, invoking principal, and
batch height as `program_id`, `signer`, and `slot`. The guest's `sha256` helper
uses the ABI crypto syscall. These are authenticated network values, not an
independent Solana slot clock.

### Typed refusals

The [error enum](../../programs/porting/solana/src/error.rs) retains specific
refusals for zero keys, invalid seeds, derivation/discriminator mismatches,
account bounds, schema mismatches, and instruction/event/module size limits.
`AccountRole::TokenBalance` and direct `LamportWrite` yield `LamportMutation`.
Principal-only translation refuses program-funded value flows; the
`translate_with_program_account` path supports bounded
`ProgramAuthorityFunded` flows with the matching derived authority. An unbounded
`RentSweep` still yields `UnboundedRentSweep`. A third-party token spend without
the required authority yields `DelegatedSpend`.

Runtime failure mapping is separate: Anchor require/constraint/discriminator
failures and panic map to `Trap`, compute-budget failure to `ResourceRefusal`,
and CPI-depth failure to `StackExhausted`.

### Qualification path

[`qualify.rs`](../../programs/porting/solana/src/qualify.rs) exposes
`source_archive`, `published_source`, `build_plan`, `validated_module`,
`deploy_and_verify`, and `settle`. `execute_mint` uses a `MintLimitPort` and an
`Invocation`; `execute_mint_count` and `execute_mint_remaining` query its state.
`import_accounts` takes `(MigrationCell, Vec<u8>)` cells and writes them into
the owning principals' namespaces. The shared pool is a separate example in
[`shared_pool.rs`](../../programs/porting/solana/src/shared_pool.rs).

### Tests that prove refusals

The existing `invoke_signed_uses_derived_account_authority` test in
[`monetary.rs`](../../programs/porting/solana/src/monetary.rs) asserts that matching
PDA-shaped authority produces a `ProgramAccount` plan and that a rent sweep is
refused. This is a translation assertion; it does not certify live deployment.

### SPL tokens, LXT-20, and native Assets

An SPL transfer maps to LXT-20 `Transfer` or an authenticated `TransferFrom`;
approval/revocation maps to `Approve`, with zero revoking the program allowance.
`BalanceOf`, `Allowance`, `TotalSupply`, and `Metadata` provide token queries.
PDA custody and CPI payouts require registered program accounts and bounded
spend authority. The shared LXT-20 encoding below replaces SPL instruction
encoding; Solana pubkeys and associated-token-account derivations require an
explicit mapping. SPL account closing/rent recovery, Token-2022 extensions,
mint/freeze authority migration, and multisig signer lists are not supplied by
this request interface.

## `layerx-porting-evm`

### Purpose

The [EVM kit](../../programs/porting/evm/src/lib.rs) preserves EVM storage slot
addresses, event topics, and four-byte selectors in its translation helpers.
The [migration guide](../../programs/porting/evm/MIGRATION.md) explains where
caller-indexed state collapses and where monetary semantics require explicit
authority. This kit ports selected patterns to Programs WASM; it is distinct
from executing an EVM contract directly on the network's EVM.

### Public entry points and types

| API | Use |
| --- | --- |
| `value_slot`, `mapping_slot`, `nested_mapping_slot`, `array_slot`, `member_slot` | Compute source EVM storage addresses as `Word` values |
| `storage_key`, `caller_indexed_key`, `shared_key` | Preserve the slot bytes or select a collapsed caller key |
| `caller_indexed_import` | Map source holder addresses to destination principal cells |
| `Address`, `Word` | Validate addresses and perform checked integer narrowing |
| `MethodAbi::new` / `calldata` | Build selector-plus-word calls |
| `EventAbi::new` / `envelope_derived` / `data` | Build event topic and payload representations |
| `external_call` | Form a `CallRequest` with explicit callee authority |

`caller_indexed_key` collapses a mapping used only with `msg.sender` to its
base slot in principal storage. `shared_key` retains the slot bytes for
program-shared storage; the key bytes alone do not select a namespace.
[`shared_supply.rs`](../../programs/porting/evm/src/shared_supply.rs) demonstrates
a shared total supply alongside principal-scoped balances.

On `wasm32`, `semantics::context` re-exports guest helpers. `msg_sender` returns
the immediate calling program when present, otherwise the root invoking
principal; the helper takes the final 20 bytes of the authenticated identifier.
`address_this` likewise maps the executing program, and `block_number` reads
batch height. `keccak256` and `ecrecover` use the ABI crypto primitives. These
helpers do not supply ambient Solidity authority or a wall clock.

### Typed refusals

The [error enum](../../programs/porting/evm/src/error.rs) includes `ZeroAddress`,
`WordTooWide`, `InvalidSignature`, `ArgumentCountMismatch`, and size refusals.
`Word::to_u64` and `to_u128` refuse nonzero high bytes rather than truncating.
Principal-only translation returns `ContractHeldBalance` for contract-funded
payouts. Supplying valid derived-account context permits bounded
`ContractFunded` payouts; `SelfDestructSweep` still returns
`UnboundedBalanceSweep`. An allowance on another principal's funds is refused
with `DelegatedSpend` on this translator path.

`Require`, `Revert`, and `AssertPanic` map to runtime `Trap`; `OutOfGas` maps to
`ResourceRefusal`, and `CallDepth` to `StackExhausted`.

### Qualification path

[`qualify.rs`](../../programs/porting/evm/src/qualify.rs) exposes the common
archive/build/deploy pipeline. `execute_purchase` runs `PublicLockPort`;
`execute_has_valid_key` and `execute_remaining_periods` query it. `import_state`
accepts `(MigrationCell, [u8; 32])` pairs and writes principal-scoped cells.
It is not an import routine for arbitrary shared-state contracts.

### Tests that prove refusals

`accumulated_contract_value_uses_derived_account_authority` in
[`monetary.rs`](../../programs/porting/evm/src/monetary.rs) asserts a bounded
contract-funded `ProgramAccount` plan and refusal of `SelfDestructSweep`.

### ERC-20, LXT-20, and native Assets

LXT-20 provides the seven request methods listed below. Ethereum addresses need
an explicit identity/account mapping, and values wider than u128 must be refused.
Infinite uint256 allowances, permits, rebasing, and transfer-tax behavior are not
implemented by this interface. Preserving source selectors in `MethodAbi` does
not make LXT-20 use ERC-20 Keccak selectors.

## `layerx-porting-cosmwasm`

### Purpose

The [CosmWasm kit](../../programs/porting/cosmwasm/src/lib.rs) preserves
`cw-storage-plus` raw key framing and JSON message/event names. JSON is
transcoded at the edge; the deterministic guest consumes canonical framed bytes.
See its [migration guide](../../programs/porting/cosmwasm/MIGRATION.md).

Shared state is supported. In
[`storage.rs`](../../programs/porting/cosmwasm/src/storage.rs),
`StateBinding::Shared.layerx_key` uses `item_key`, `shared()` returns true, and
`portable()` returns true. `SharedState` remains in `PortRefusal`, but the
binding implementation does not raise it. The older claim that every cell
must be sender-scoped is retired.

### Public entry points and types

| API | Use |
| --- | --- |
| `item_key`, `map_prefix`, `map_key`, `composite_map_key` | Reproduce bounded raw storage-key framing |
| `StateBinding` | Choose `Item`, `SenderIndexed`, `SenderSuffixed`, or `Shared` |
| `sender_indexed_import` | Build principal-scoped migration cells for source holders |
| `RecordSchema`, `FieldSchema`, `FieldValue`, `ValueType` | Encode/decode canonical records and transcode JSON |
| `variant_tag`, `MessageVariant` | Encode entry-point variant and message body |
| `ContractEvent::response` / `custom` / `data` | Encode response or custom-event attributes |
| `execute_submessage` | Form a bounded cross-program `CallRequest` |

`Item` uses a principal-scoped item key. `SenderIndexed` collapses the sender
into the namespace; `SenderSuffixed` preserves the leading composite elements.
`Shared` selects the program namespace. The
[`shared_orderbook.rs`](../../programs/porting/cosmwasm/src/shared_orderbook.rs)
example maps order count and orders to shared bindings while keeping trader
positions sender-indexed.

On `wasm32`, `messages::context` exposes guest `current()`: `Env` contains the
executing program and batch height, and `MessageInfo` contains the invoking
principal. The guest also exposes BLAKE3. This subset is not a general
`Deps::querier` interface to other modules or chains.

### Typed refusals

The [error enum](../../programs/porting/cosmwasm/src/error.rs) includes
`EmptyAddress`, `InvalidNamespace`, `KeyTooLong`, `SchemaMismatch`, `InvalidJson`,
and message/event/topic/module bounds. Principal-only `BankSend`, `BankBurn`,
and `SubMessageFunds` translation returns `ContractHeldBalance`. With valid
program-account context, bounded bank sends and funded submessages become
`ProgramAccount` plans. `BankBurn` still returns `SupplyMutation`.
`AllowanceSpend` for another principal returns `DelegatedSpend`, and
`IbcTransfer` returns `ChainQuery`. The enum's existence does not imply a general
querier implementation.

`ContractError`, `StdError`, `Panic`, and `SubMessageFailure` map to runtime
`Trap`; `OutOfGas` maps to `ResourceRefusal`, and `CallDepth` to `StackExhausted`.

### Qualification path

[`qualify.rs`](../../programs/porting/cosmwasm/src/qualify.rs) exposes the common
pipeline. `execute_donate` runs `DonationPort`; `execute_donations` and
`execute_remaining` query it. `import_state` takes `(MigrationCell, String)`
pairs, transcodes through `donation_record()`, and writes principal-scoped state.
Use an explicit shared-state migration plan for shared bindings; the
sender-import helper does not select that namespace automatically.

### Tests that prove refusals

`bank_send_uses_derived_account_authority` in
[`monetary.rs`](../../programs/porting/cosmwasm/src/monetary.rs) asserts a bounded
bank-send `ProgramAccount` plan and refusal of `BankBurn`. The shared order-book
tests assert that shared bindings are portable and distinguish them from
principal-scoped positions.

## Qualification pipeline for the reference ports

The three `qualify.rs` modules use production engine, lifecycle, registry,
rebuild, execution, and monetary-law types. They still require caller-supplied
kernel transfer and receipt-oracle boundaries.

1. `source_archive` packs reference source, descriptor, toolchain manifest, and
   dependency lock. `published_source` binds the archive and source URI.
2. `build_plan` pins the build inputs and epoch. `REBUILD_ATTEMPTS` is 2 and
   `SOURCE_EPOCH` is 1 in each kit.
3. `validated_module` validates emitted WASM through `WasmEngine::declared`.
4. `deploy_and_verify` deploys with runtime `ABI_VERSION` (4) and `UpgradePolicy::Immutable`,
   journals a `DeploymentRecord`, replays it into the registry, and reproduces
   the artifact. A source status other than `Verified` yields `UnverifiedSource`.
5. Execution entry points return an `AuthorizedExecutionRecord` with explicit
   capabilities. `settle` accepts a `PreparedAuthorizedActivity`, storage, and
   a `KernelTransferPrimitive`, then calls `strict_settle` to obtain a
   `VerifiedStorageAssignment`.

The reference `PortBuildRunner`s parse their own descriptors and require the
published source to equal their pinned `ANCHOR_SOURCE`, `SOLIDITY_SOURCE`, or
`COSMWASM_SOURCE`. Their reproducibility pipeline demonstrates these reference
ports; it is not an arbitrary Anchor, Solidity, or CosmWasm compiler. Relinking
an original source-chain artifact does not satisfy it. `AbsentReceipts` refuses
every receipt digest with `AbiError::ReceiptMismatch`.

Here, `settle` names the program runtime's atomic storage/monetary operation.
It does not mean a second chain settlement step. Network finality remains the
block commit described in [Sequencing](Sequencing.md).

## LXT-20 request encoding and backing assets

The canonical codec is
[`programs/sdk/rust/src/lxt20.rs`](../../programs/sdk/rust/src/lxt20.rs).
The reference guest is
[`token-lxt20`](../../programs/sdk/rust/examples/token-lxt20).

| Method byte | Request | Source-chain counterpart |
| --- | --- | --- |
| 1 | `Transfer` | ERC-20 transfer / SPL transfer |
| 2 | `Approve` | Approval or delegate revocation; zero revokes |
| 3 | `TransferFrom` | Allowance-authorized transfer |
| 4 | `BalanceOf` | Token balance query |
| 5 | `Allowance` | Owner/spender allowance query |
| 6 | `TotalSupply` | Funded token supply query |
| 7 | `Metadata` | Name, symbol, and decimals |

The four-byte selector is `4c 58 14 <method>`, followed by convention `01`, bytes
tag `20`, a u32 big-endian payload length, and declared fields. Identifiers are
32 bytes; amounts are 16-byte big-endian u128 values. The decoder rejects wrong
selectors, lengths, tags, trailing bytes, reserved identifiers, and zero
transfers. Approval zero is valid. This encoding is distinct from both Solidity
ABI and SPL/Borsh instructions.

The reference requires registered program-derived backing accounts. Recipients
call `approve` at least once (zero suffices) to create their token storage; the
deployment authority registers their native program account. The configured
issuer calls `initialize` once with a funding grant for the complete supply.
Transfers do not mint or burn native units. `transfer_from` decrements every
allowance, including `u128::MAX`.

Interface encoding v2 declares dynamic spend through `CallerAuthorizedSpend`
descriptors binding backing asset, ceiling, and recipient/amount calldata
offsets. Actual caller-authorized `ProgramSpend` grants are still required.
`ProgramPaymentCapabilities` combines funding, spending, and ordinary grants in
canonical order. A descriptor or token allowance is not kernel debit authority.
Native Asset registration, account opening, minting, and burning remain separate
signed Asset activities. Signer identity sequence and backing-account sequence
must be handled separately.

The [Solana](../../programs/porting/solana/MIGRATION.md) and
[EVM](../../programs/porting/evm/MIGRATION.md) migration guides retain the complete
LXT-20/native-asset mapping. Committed token and merchant artifacts and signed
native receipts are under [`programs/fixtures/pay5`](../../programs/fixtures/pay5).
Its isolated replay command is:

```sh
python3 programs/fixtures/pay5/run_native_roundtrip.py
```

This compares canonical artifacts byte for byte; it does not certify a live
program deployment.

## Build and reference checks

The repository [Makefile](../../Makefile) defines `make programs-build` for the
programs workspace (including the sandbox `host-ffi` feature).
`make programs-porting-v2-references` builds all three `reference-v2` guests as
release `wasm32-unknown-unknown` modules, lints each with
`layerx-program-lint --abi-version 2`, and supplies their WASM artifacts to
`programs_call_activity`. `make programs-test` includes this target and ends in
workspace Rust tests after its prerequisite checks. These are developer targets;
this page does not claim they have run or that hosted deployment is qualified.

## Comparison

| Concern | Solana | EVM | CosmWasm |
| --- | --- | --- | --- |
| Preserved representation | Anchor discriminators and account bytes | Slot addresses, selectors, event topics | Raw storage framing and JSON names |
| Principal migration | `per_signer_import` / `import_accounts` | `caller_indexed_import` / `import_state` | `sender_indexed_import` / `import_state` |
| Shared-state example | `shared_pool` | `shared_supply` | `shared_orderbook` |
| Context | Program, signer, batch height | Caller, current program, batch height | Contract, sender, batch height |
| Bounded derived payout | `ProgramAuthorityFunded` | `ContractFunded` | `BankSend` / `SubMessageFunds` |
| Refused monetary construct | Unbounded rent sweep | Unbounded self-destruct sweep | Supply burn |

[Home](Home.md)
