# Program porting crates

The `programs/` workspace members `porting/solana`, `porting/evm`, and
`porting/cosmwasm` (plus each crate's `guest`) adapt source-chain account,
storage, message, and value-flow conventions to the Programs ABI in the merged
Paxeer network. The retained LayerX names identify Rust packages and ABI import
namespaces. Package names are `layerx-porting-solana`,
`layerx-porting-evm`, and `layerx-porting-cosmwasm`; lib names are
`programs_porting_solana`, `programs_porting_evm`, and
`programs_porting_cosmwasm`
(`programs/porting/solana/Cargo.toml`,
`programs/porting/evm/Cargo.toml`,
`programs/porting/cosmwasm/Cargo.toml`).

## Network and deployment boundary

Paxeer mainnet-beta is one BFT network, run by `paxd`, with EVM chain ID `125`
(`0x7d`). The kernel belongs to the network and proposes blocks; receipts are
final when their block commits. Program execution and its authorized monetary
effects commit through that same network. The `settle` functions below name
the runtime's checked application of effects through `KernelTransferPrimitive`,
not an asynchronous handoff to another chain.

Use the selected public node's HTTPS origin at the root path for JSON-RPC.
Node selection is described in
[Public RPC endpoints](../reference/public-rpc.md). Hosted gateway `px_` methods
are declared separately in `services/platform/hosted/gateway/openrpc.json`;
do not infer their presence on a node from the Programs ABI or crate names.
Tenant domains (sub-chains) are database namespaces, without a separate node
deployment for each tenant.

The merged node exposes the `layerxprograms` precompile at
`0x000000000000000000000000000000000000101D`. Its `deploy` and `call` handlers
submit activities through `modules/lxphost` in the EVM transaction context
(`precompiles/layerxprograms/events.go`, `method_deploy.go`, `method_call.go`,
`submit.go`, and `abi.json`). A local `deploy_and_verify` result exercises the
library lifecycle and registry; it is not a public deployment receipt.

Deployment packaging must satisfy the actual activity payload limit of
524,288 bytes (`LXP_MAX_PAYLOAD_BYTES` in `include/layerx/lxp_protocol.h`).
`EncodeDeployPayload` uses a 104-byte header without an interface, leaving at
most 524,184 bytes for WASM. With an interface it uses a 108-byte header,
followed by interface bytes and WASM; those bytes all count toward the same
limit. The encoder bounds the interface at 952 bytes
(`precompiles/layerxprograms/payload_lifecycle.go`). A module-size check alone
does not prove the deployment payload fits.

The context reference guests and LXT-20 reference use ABI v2. The library
qualification helpers instead pass the runtime's `ABI_VERSION` to the
lifecycle; it currently resolves to v4 in
`programs/crates/layerx-programs-runtime/src/lib.rs`. Match the emitted guest's
imports, deployment ABI, call ABI, and interface on every registration path;
the kit identifier's `ABI version 2` text is not a version negotiation.
Supported versions and monotonic upgrade rules live in
`programs/sdk/rust/src/abi_policy.rs`.

For the AI market guest, build release WASM with size optimization
(`CARGO_PROFILE_RELEASE_OPT_LEVEL=z`) and bind `PAXAI_CHAIN_DOMAIN` at build
time to the canonical chain-125 domain. `merged_chain_domain` derives that
domain as SHA-256 of `PAXAI/chain-domain/v1` followed by the big-endian u64
chain ID; `deployment_chain_domain_from_setting` checks the setting and
returns `WRONG_DOMAIN` for malformed or foreign values
(`programs/crates/layerx-programs-ai-market/src/lib.rs`). Apply the payload
budget to the finished artifact and envelope, not its unoptimized source.

## Build and qualification targets

`make programs-build` builds the workspace with `--locked --workspace` and
the `layerx-programs-sandbox/host-ffi` feature. `make programs-test` runs its
declared prerequisite suites, including `programs-porting-v2-references`,
before the workspace Cargo tests. The reference target builds the three
`reference-v2` guests for `wasm32-unknown-unknown` release, runs
`layerx-program-lint --abi-version 2` on each artifact, and passes all three
`.wasm` paths to `programs_call_activity` (root `Makefile`). These are developer
qualification targets, not evidence that the hosted beta is deployed or
authorization to move real value. See [Operators](index.md) for beta admission
and deployment caveats.

The EVM and Solana migration guides describe how their account models map to
registered native Asset accounts and program-derived accounts. See [Assets](../concepts/assets.md) and
[Payments developer path](../overview/payments.md).

Sources:

- `programs/porting/solana/` (`src/lib.rs`, `error.rs`, `qualify.rs`,
  `monetary.rs`, `account.rs`, `anchor.rs`, `pubkey.rs`, `reference.rs`,
  `shared_pool.rs`, `wasm.rs`, `MIGRATION.md`, `guest/`, `reference-v2/`)
- `programs/porting/evm/` (`src/lib.rs`, `error.rs`, `qualify.rs`,
  `monetary.rs`, `layout.rs`, `semantics.rs`, `value.rs`, `reference.rs`,
  `shared_supply.rs`, `MIGRATION.md`, `guest/`, `reference-v2/`)
- `programs/porting/cosmwasm/` (`src/lib.rs`, `error.rs`, `qualify.rs`,
  `monetary.rs`, `storage.rs`, `messages.rs`, `json.rs`, `reference.rs`,
  `shared_orderbook.rs`, `MIGRATION.md`, `guest/`, `reference-v2/`)
- `programs/Cargo.toml`
- `Makefile` (`programs-porting-v2-references`, `programs-test`,
  `programs-build`)

---

## `layerx-porting-solana`

### Purpose

The kit (`programs/porting/solana/src/lib.rs`, `monetary.rs`)
carries a Solana program onto the merged network's Programs ABI. Account data stays
byte-identical (Anchor discriminator first, `borsh` fields after). Instruction
and event discriminators stay byte-identical. Constructs the account model
assumes are translated under explicit storage and monetary authority. Direct
lamport mutation and amount-less rent sweeps are refused. Bounded PDA-funded
payouts are supported through a registered program-derived account and
`ValueFlow::translate_with_program_account`; knowing the seed does not grant
authority over somebody else's funds. The
[`reference`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/programs/porting/solana/src/reference.rs) port emits a
deterministic module, deploys through the real lifecycle, rebuilds from
published source, and executes under the real metered executor.

`programs_porting_solana()` returns
`programs/porting/solana targeting layerx_v2 ABI version 2`
(`programs/porting/solana/src/lib.rs`).

Guest crate `layerx-porting-solana-guest` maps Anchor context onto ABI v2:
`AnchorContext::current` reads executing program, invoking principal, and
batch height; `sha256` calls the SHA-256 syscall
(`programs/porting/solana/guest/src/lib.rs`). On `wasm32`,
`anchor::context` re-exports that guest
(`programs/porting/solana/src/anchor.rs`).

### Public entry points and types

Entries exported from `lib.rs` or their named public modules (`programs/porting/solana/src/lib.rs`):

| Entry | Request | Result |
| --- | --- | --- |
| `ported_account` | `&AccountSchema`, `&SeedPath`, envelope seed indices | `(Vec<u8>, usize)` key and space (`programs/porting/solana/src/account.rs`) |
| `account_discriminator` / `instruction_discriminator` / `event_discriminator` | account, handler, or event name | `[u8; 8]` (`programs/porting/solana/src/anchor.rs`) |
| `InstructionAbi::new` / `data` | handler name, `FieldType`s; `FieldValue`s | discriminator-plus-`borsh` instruction bytes (`programs/porting/solana/src/anchor.rs`) |
| `AnchorEvent::new` / `data` | event name, field types; values | `borsh` payload; discriminator is the topic (`programs/porting/solana/src/anchor.rs`) |
| `cross_program_invocation` | `ProgramId`, `&InstructionAbi`, `&[FieldValue]` | `CpiRequest { callee, input, authority: Capability::Call }` (`programs/porting/solana/src/anchor.rs`) |
| `Pubkey::new` | `[u8; 32]` | `Pubkey` (`programs/porting/solana/src/pubkey.rs`) |
| `SeedPath::new` / `verify` / `storage_key` / `collapse` | seeds; bump, program, published address; envelope indices | path, `()`, framed key, collapsed path (`programs/porting/solana/src/pubkey.rs`) |
| `per_signer_import` | base path, signer index, program, envelope, `&[AccountHolder]` | `Vec<MigrationCell>` (`programs/porting/solana/src/pubkey.rs`) |
| `AccountRole::translate` | role | `AccountMapping` (`programs/porting/solana/src/account.rs`) |
| `ValueFlow::translate` | asset, principal | `Transfer402Plan` (`programs/porting/solana/src/monetary.rs`) |
| `ValueFlow::translate_with_program_account` | asset, principal, owner, seed, source | `TranslatedValueFlow` (`programs/porting/solana/src/monetary.rs`) |
| `translate_all` | `&[ValueFlow]`, asset, principal | `Vec<Transfer402Plan>` (`programs/porting/solana/src/monetary.rs`) |
| `Transfer402Plan::new` | asset, to, amount | plan (`programs/porting/solana/src/monetary.rs`) |
| `ProgramAccountTransferPlan::new` | owner, seed, source, asset, to, amount | plan (`programs/porting/solana/src/monetary.rs`) |
| `source_archive` | `&MintLimitPort` | `SourceArchive` (`programs/porting/solana/src/qualify.rs`) |
| `published_source` | `&MintLimitPort`, `uri` | `PublishedSource` (`programs/porting/solana/src/qualify.rs`) |
| `build_plan` | none | `BuildPlan` (`programs/porting/solana/src/qualify.rs`) |
| `validated_module` | `&MintLimitPort` | `ValidatedModule` (`programs/porting/solana/src/qualify.rs`) |
| `deploy_and_verify` | port, `Publication`, `&PublishedSource`, `&mut Lifecycle`, `&mut Registry` | `DeployedGuard` (`programs/porting/solana/src/qualify.rs`) |
| `execute_mint` | port, `&Invocation`, `&mut Storage`, `amount` | `AuthorizedExecutionRecord` (`programs/porting/solana/src/qualify.rs`) |
| `execute_mint_count` / `execute_mint_remaining` | `&Invocation`, `&mut Storage` | `AuthorizedExecutionRecord` (`programs/porting/solana/src/qualify.rs`) |
| `settle` | `PreparedAuthorizedActivity`, `&mut Storage`, `&mut impl KernelTransferPrimitive` | `VerifiedStorageAssignment` (`programs/porting/solana/src/qualify.rs`) |
| `import_accounts` | `&mut Storage`, `ProgramId`, `&[(MigrationCell, Vec<u8>)]` | `usize` changed cells (`programs/porting/solana/src/qualify.rs`) |
| `MintLimitPort::new` | `GuardTerms` | port (`programs/porting/solana/src/reference.rs`) |

`Invocation` carries `module`, `program`, `principal`, `receipts`
(`programs/porting/solana/src/qualify.rs`). `Publication` carries
`program`, `sequence`, `observed_at`
(`programs/porting/solana/src/qualify.rs`). `AbsentReceipts` returns
`AbiError::ReceiptMismatch` for every digest
(`programs/porting/solana/src/qualify.rs`). `AuthorizedExecutionRequest`
uses `CALL_ENTRY_EXPORT`, isolated composition, `response_capacity: 0`
(`programs/porting/solana/src/qualify.rs`).

### Typed refusals

`PortRefusal` (`programs/porting/solana/src/error.rs`):

| Variant | Condition that raises it |
| --- | --- |
| `ZeroPubkey` | `Pubkey::new` of the all-zero key (`programs/porting/solana/src/pubkey.rs`); `MintLimitPort::new` with zero asset or destination (`programs/porting/solana/src/reference.rs`); `per_signer_import` with zero principal (`programs/porting/solana/src/pubkey.rs`) |
| `InvalidSeeds` | empty path, more than `MAX_SEEDS` 16, or a seed longer than `MAX_SEED_BYTES` 32 (`programs/porting/solana/src/pubkey.rs`); framed key longer than `MAX_STORAGE_KEY_BYTES` (`programs/porting/solana/src/pubkey.rs`); collapse/with_seed index out of range (`programs/porting/solana/src/pubkey.rs`); `MintLimitPort::code` when the storage key length does not fit `i32` (`programs/porting/solana/src/reference.rs`) |
| `DerivationMismatch` | `SeedPath::verify` when bump and program do not derive the published address (`programs/porting/solana/src/pubkey.rs`) |
| `DiscriminatorMismatch` | `AccountSchema::decode` when the first eight bytes are not the schema discriminator (`programs/porting/solana/src/account.rs`) |
| `AccountBounds` | schema `space()` above `MAX_STORAGE_VALUE_BYTES` (`programs/porting/solana/src/account.rs`); decode length not equal to declared space (`programs/porting/solana/src/account.rs`); bool byte other than 0 or 1, or a field slice shorter than its width (`programs/porting/solana/src/account.rs`) |
| `SchemaMismatch` | unnamed account, more than `MAX_FIELDS` 64, unnamed or repeated field (`programs/porting/solana/src/account.rs`); encode value count or type mismatch (`programs/porting/solana/src/account.rs`); unnamed instruction/event, more than `MAX_ARGUMENTS` 32, or argument list mismatch (`programs/porting/solana/src/anchor.rs`); `PoolReserve::decode` when the value list is not `(U64, U32, Pubkey)` (`programs/porting/solana/src/shared_pool.rs`) |
| `LamportMutation` | `AccountRole::TokenBalance` (`programs/porting/solana/src/account.rs`); `ValueFlow::translate` of `LamportWrite`, `ProgramAuthorityFunded`, or `RentSweep` (`programs/porting/solana/src/monetary.rs`); `ValueFlow::translate_with_program_account` of `LamportWrite` (`programs/porting/solana/src/monetary.rs`) |
| `InvalidProgramAccount` | `ProgramAccountTransferPlan::new` when asset/to/amount is reserved-zero or `derive_program_account` does not equal `source` (`programs/porting/solana/src/monetary.rs`); `ProgramAuthorityFunded` whose `authority` is not `source` (`programs/porting/solana/src/monetary.rs`) |
| `UnboundedRentSweep` | `RentSweep` through `translate_with_program_account` (`programs/porting/solana/src/monetary.rs`) |
| `DelegatedSpend` | `TokenTransfer` whose `authority` is not the invoking principal (`programs/porting/solana/src/monetary.rs`) |
| `OutOfRange` | `Transfer402Plan::new` with zero asset, recipient, or amount (`programs/porting/solana/src/monetary.rs`); `MintLimitPort::new` zero price/limit or limit above `MAX_LIMIT_BOUND` 65535, or `price * limit` outside `i64::MAX` (`programs/porting/solana/src/reference.rs`); mint amount not in `u16` (`programs/porting/solana/src/qualify.rs`); unknown query export (`programs/porting/solana/src/qualify.rs`) |
| `EventDataTooLarge` | `AnchorEvent::data` payload longer than `MAX_EVENT_DATA_BYTES` (`programs/porting/solana/src/anchor.rs`) |
| `InstructionDataTooLarge` | `InstructionAbi::data` longer than `MAX_CALL_INPUT_BYTES` (`programs/porting/solana/src/anchor.rs`) |
| `ModuleTooLarge` | emitted WASM longer than `DEFAULT_MAX_MODULE_BYTES` (`programs/porting/solana/src/reference.rs`) |
| `InvalidDescriptor` | `MintLimitPort::parse` malformed line, repeated key, unknown key, wrong version or program name, missing key, or non-integer number (`programs/porting/solana/src/reference.rs`) |
| `UnverifiedSource` | `deploy_and_verify` when `registry.verify_source` is not `SourceStatus::Verified` (`programs/porting/solana/src/qualify.rs`) |
| `Abi` / `Storage` / `Engine` / `Validation` / `Lifecycle` / `Execution` / `Registry` / `Archive` / `Build` / `TransferLaw` | `From` wrappers (`programs/porting/solana/src/error.rs`). `settle` maps `strict_settle` failure through `PortRefusal::from` (`programs/porting/solana/src/qualify.rs`) |

`FailureMapping` is not a `PortRefusal`. `Require`, `ConstraintViolation`,
`DiscriminatorMismatch`, and `Panic` map to `RuntimeOutcome::Trap`;
`ComputeBudget` to `ResourceRefusal`; `CpiDepth` to `StackExhausted`
(`programs/porting/solana/src/anchor.rs`).

### Qualification path

`qualify.rs` documentation: the engine, lifecycle, registry, reproducible-build
pipeline, metered executor, and monetary law are production types; the kernel
transfer primitive and receipt oracle stay caller-supplied
(`programs/porting/solana/src/qualify.rs`).

Checks before a ported program is admitted:

1. `source_archive` packs Anchor source, descriptor, toolchain manifest, and
   dependency lock (`programs/porting/solana/src/qualify.rs`).
2. `build_plan` pins builder identity `LayerX/porting/solana/builder/v1`,
   toolchain and lock digests, `SOURCE_EPOCH` 1, and `BUILD_COMMAND`
   (`programs/porting/solana/src/qualify.rs`).
3. `validated_module` runs `WasmEngine::declared()?.validate(&port.code()?)`
   (`programs/porting/solana/src/qualify.rs`).
4. `deploy_and_verify` deploys with `ABI_VERSION` and
   `UpgradePolicy::Immutable`, journals `DeploymentRecord`, replays into the
   registry, rebuilds with `SourceVerifier::new(PortBuildRunner, REBUILD_ATTEMPTS)`
   where `REBUILD_ATTEMPTS` is 2, and refuses unless status is `Verified`
   (`programs/porting/solana/src/qualify.rs`).
5. `PortBuildRunner::run` requires the plan command's last word as descriptor
   path, UTF-8 descriptor text, `SOURCE_PATH` equal to `ANCHOR_SOURCE`, then
   `MintLimitPort::parse` and `port.code()`
   (`programs/porting/solana/src/qualify.rs`).

### Tests that prove refusals

| Test | File | Asserted outcome |
| --- | --- | --- |
| `invoke_signed_uses_derived_account_authority` | `programs/porting/solana/src/monetary.rs` | `ProgramAuthorityFunded` with matching PDA source is `TranslatedValueFlow::ProgramAccount`; `RentSweep` through `translate_with_program_account` is `Err(PortRefusal::UnboundedRentSweep)` |

`shared_pool` tests assert role mappings and round-trips; they do not assert a
`PortRefusal` (`programs/porting/solana/src/shared_pool.rs`).

### SPL tokens, LXT-20, and native Assets

The LXT-20 reference is an ABI-v2 WASM program with eight exported methods:
`initialize`, `transfer`, `approve`, `transfer_from`, `balance_of`, `allowance`,
`total_supply`, and `metadata`. The seven ordinary request codecs are in
`programs/sdk/rust/src/lxt20.rs`; initialization has its own selector and
empty payload, described in [ERC-20, LXT-20, and native Assets](#erc-20-lxt-20-and-native-assets).
The interface, registry inputs, and signed native deploy and call receipts are in
`programs/fixtures/pay5`. Both paths are in this tree.
`programs/porting/solana/MIGRATION.md` carries the source-chain migration notes.
Replay the isolated native token and merchant settlements with:

```sh
python3 programs/fixtures/pay5/run_native_roundtrip.py
```

That command compares the committed token and merchant artifacts byte for
byte; it is not evidence of a live deployment. The runner builds
`programs/fixtures/pay5/native_roundtrip.c`, executes token and merchant cases
in isolation, and compares the resulting files with `native-token/` and
`native-merchant/`. The token sequence includes deployment, native account
registration, `05-initialize`, approvals, transfers, and queries. Use the
default replay mode to check the committed lifecycle; `--write-fixtures`
regenerates the expected artifacts and is not a replay check.

| SPL flow | LayerX mapping |
| --- | --- |
| Transfer / transfer checked | LXT-20 `Transfer`, or `TransferFrom` for an authenticated allowance spender; exact u128 units and an explicit backing Asset |
| Approve / revoke delegate | `Approve` with a spender DID; zero revokes the program allowance, not a kernel grant |
| Read token account balance | `BalanceOf`; native balance evidence is a separate authorized read |
| Read delegated amount | `Allowance` with owner and spender identities |
| Read mint supply and metadata | `TotalSupply` and `Metadata`; backing-Asset facts remain in the native Asset registry |
| PDA custody / CPI payout | Registered program-derived account plus `ProgramSpend`; a public seed is not spending authority |

The four-byte LXT-20 selector is followed by LayerX convention `01`, bounded
bytes tag `20`, a u32 big-endian payload length, and the declared fields. Borsh
little-endian integers, Anchor discriminators, Solana pubkeys, and associated
token-account derivations are not accepted unchanged
(`programs/sdk/rust/src/lxt20.rs`).

Register the program-derived account under deployment authority before
funding. `ProgramPaymentCapabilities` combines funding, spending, and ordinary
grants in canonical key order. The v2 interface's
`CallerAuthorizedSpend` descriptors bind the backing Asset, ceiling, and
recipient/amount calldata offsets; admission still requires the caller's
matching grant on every call. Descriptors and token allowances grant no kernel
debit authority (`programs/sdk/rust/src/payments.rs` and
`programs/crates/layerx-programs-registry/src/lxt20.rs`).

Native Asset register, account-open, mint, and burn remain separate signed
Asset activities. Per-Asset accounts use
`agent:<DID>:asset:<lowercase asset hex>` and `LX:ACCOUNT:v1`; program accounts
use the program-account derivation. The envelope's signer/identity sequence is
separate from the payment account and its sequence. Account-bound terminal
evidence retains signer authorization while committing the resolved account
endpoints (`src/ledger/lx_account_id.c` and
`programs/fixtures/pay5/native_roundtrip.c`). LXT-20 does not
provide SPL account closing/rent recovery, Token-2022 extensions, mint/freeze
authority migration, multisig signer lists, or an enumerable Anchor account
context (`programs/sdk/rust/src/lxt20.rs` and
`programs/sdk/rust/examples/token-lxt20/src/lib.rs`).

---

## `layerx-porting-evm`

### Purpose

The kit (`programs/porting/evm/src/lib.rs`, `monetary.rs`) carries
an EVM contract onto the merged network's Programs ABI. Storage slot addresses stay
byte-identical. Event topics and four-byte selectors stay byte-identical.
The guest maps authenticated context and explicit monetary capabilities onto
the runtime. There is no ambient debit authority: a bounded contract-funded
payout needs its registered program-derived source account, owner, seed, asset,
recipient, and amount. `ValueFlow::translate_with_program_account` supports
that path; context-free contract payouts and unbounded sweeps are refused.
The reference port in `reference` emits, deploys, rebuilds, and executes through
the library qualification path described below.

`programs_porting_evm()` returns
`programs/porting/evm targeting layerx_v2 ABI version 2`
(`programs/porting/evm/src/lib.rs`).

Guest crate `layerx-porting-evm-guest` maps `msg.sender`, `address(this)`,
`block.number`, `keccak256`, and `ecrecover` onto ABI v2
(`programs/porting/evm/guest/src/lib.rs`). On `wasm32`,
`semantics::context` re-exports that guest
(`programs/porting/evm/src/semantics.rs`).

### Public entry points and types

Entries exported from `lib.rs` or their named public modules (`programs/porting/evm/src/lib.rs`):

| Entry | Request | Result |
| --- | --- | --- |
| `value_slot` / `mapping_slot` / `nested_mapping_slot` / `array_slot` / `member_slot` | slot index and keys | `Word` EVM address (`programs/porting/evm/src/layout.rs`) |
| `storage_key` / `caller_indexed_key` / `shared_key` | slot | `[u8; 32]` (`programs/porting/evm/src/layout.rs`) |
| `caller_indexed_import` | slot, `&[(Address, [u8; 32])]` | `Vec<MigrationCell>` (`programs/porting/evm/src/layout.rs`) |
| `Address::new` / `Word::to_u64` / `Word::to_u128` | 20-byte address; 32-byte word | `Address` or narrowed integer (`programs/porting/evm/src/value.rs`) |
| `EventAbi::new` / `envelope_derived` / `data` | canonical signature; words | topic plus payload (`programs/porting/evm/src/semantics.rs`) |
| `MethodAbi::new` / `calldata` | canonical signature; words | selector-plus-words (`programs/porting/evm/src/semantics.rs`) |
| `external_call` | `ProgramId`, `&MethodAbi`, `&[Word]` | `CallRequest { callee, input, authority: Capability::Call }` (`programs/porting/evm/src/semantics.rs`) |
| `ValueFlow::translate` / `translate_with_program_account` / `translate_all` | asset, principal; plus owner/seed/source | `Transfer402Plan` or `TranslatedValueFlow` (`programs/porting/evm/src/monetary.rs`) |
| `source_archive` / `published_source` / `validated_module` | `&PublicLockPort` (and URI for published source) | `SourceArchive`, `PublishedSource`, `ValidatedModule` (`programs/porting/evm/src/qualify.rs`) |
| `build_plan` | none | `BuildPlan` (`programs/porting/evm/src/qualify.rs`) |
| `deploy_and_verify` | port, `Publication`, source, lifecycle, registry | `DeployedLock` (`programs/porting/evm/src/qualify.rs`) |
| `execute_purchase` | port, `&Invocation`, `&mut Storage`, `periods` | `AuthorizedExecutionRecord` (`programs/porting/evm/src/qualify.rs`) |
| `execute_has_valid_key` / `execute_remaining_periods` | `&Invocation`, `&mut Storage` | `AuthorizedExecutionRecord` (`programs/porting/evm/src/qualify.rs`) |
| `settle` | `PreparedAuthorizedActivity`, storage, kernel | `VerifiedStorageAssignment` (`programs/porting/evm/src/qualify.rs`) |
| `import_state` | storage, program, `&[(MigrationCell, [u8; 32])]` | `usize` (`programs/porting/evm/src/qualify.rs`) |
| `PublicLockPort::new` | `LockTerms` | port (`programs/porting/evm/src/reference.rs`) |

`execute_purchase` builds calldata with `MethodAbi::new(PURCHASE_METHOD)` and
`Word::from_u64(periods)` (`programs/porting/evm/src/qualify.rs`).

### Typed refusals

`PortRefusal` (`programs/porting/evm/src/error.rs`):

| Variant | Condition that raises it |
| --- | --- |
| `ZeroAddress` | `Address::new` of the 20-byte zero address (`programs/porting/evm/src/value.rs`); `PublicLockPort::new` with zero asset or beneficiary (`programs/porting/evm/src/reference.rs`) |
| `WordTooWide` | `Word::to_u64` when bytes `[..24]` are not zero; `Word::to_u128` when bytes `[..16]` are not zero (`programs/porting/evm/src/value.rs`) |
| `InvalidSignature` | canonical ABI signature missing `(`, not ending `)`, empty name, longer than 256 bytes, or empty parameter token (`programs/porting/evm/src/semantics.rs`); topic longer than `MAX_EVENT_TOPIC_BYTES` (`programs/porting/evm/src/semantics.rs`) |
| `ArgumentCountMismatch` | empty nested-mapping key path (`programs/porting/evm/src/layout.rs`); `envelope_derived` with `derived` larger than argument count (`programs/porting/evm/src/semantics.rs`); `EventAbi::data` / `MethodAbi::calldata` wrong word count (`programs/porting/evm/src/semantics.rs`); `SharedSupplyPort::new` with zero `price_per_token` (`programs/porting/evm/src/shared_supply.rs`) |
| `EventDataTooLarge` | encoded event data longer than `MAX_EVENT_DATA_BYTES` (`programs/porting/evm/src/semantics.rs`) |
| `CalldataTooLarge` | encoded calldata longer than `MAX_CALL_INPUT_BYTES` (`programs/porting/evm/src/semantics.rs`) |
| `ContractHeldBalance` | `ValueFlow::translate` of `ContractFunded` or `SelfDestructSweep` (`programs/porting/evm/src/monetary.rs`) |
| `InvalidProgramAccount` | `ProgramAccountTransferPlan::new` reserved fields or source mismatch (`programs/porting/evm/src/monetary.rs`) |
| `UnboundedBalanceSweep` | `SelfDestructSweep` through `translate_with_program_account` (`programs/porting/evm/src/monetary.rs`) |
| `DelegatedSpend` | `AllowanceSpend` whose `owner` is not the invoking principal (`programs/porting/evm/src/monetary.rs`) |
| `OutOfRange` | `Transfer402Plan::new` zero asset/to/amount (`programs/porting/evm/src/monetary.rs`); `caller_indexed_import` zero principal (`programs/porting/evm/src/layout.rs`); `PublicLockPort::new` zero price/token_id/per-purchase bound, per-purchase above `MAX_PERIODS_BOUND` 4096, key bound below per-purchase or above `MAX_TOTAL_PERIODS_BOUND`, or price product outside `i64::MAX` (`programs/porting/evm/src/reference.rs`); unknown query export (`programs/porting/evm/src/qualify.rs`) |
| `ModuleTooLarge` | `PublicLockPort::code` or `SharedSupplyPort::code` WASM above `DEFAULT_MAX_MODULE_BYTES` (`programs/porting/evm/src/reference.rs`, `programs/porting/evm/src/shared_supply.rs`) |
| `InvalidDescriptor` | `PublicLockPort::parse` malformed/repeated/unknown/missing keys, wrong version or contract name, non-integer (`programs/porting/evm/src/reference.rs`) |
| `UnverifiedSource` | `deploy_and_verify` when source status is not `Verified` (`programs/porting/evm/src/qualify.rs`) |
| `Abi` / `Storage` / `Engine` / `Validation` / `Lifecycle` / `Execution` / `Registry` / `Archive` / `Build` / `TransferLaw` | `From` wrappers (`programs/porting/evm/src/error.rs`). `settle` uses `PortRefusal::from(failure.error())` (`programs/porting/evm/src/qualify.rs`) |

`FailureMapping::Require` / `Revert` / `AssertPanic` map to `Trap`; `OutOfGas`
to `ResourceRefusal`; `CallDepth` to `StackExhausted`
(`programs/porting/evm/src/semantics.rs`).

### Qualification path

Same pipeline shape as Solana, with EVM names
(`programs/porting/evm/src/qualify.rs`):

1. Archive holds `SOLIDITY_SOURCE`, descriptor, toolchain, lock
   (`programs/porting/evm/src/qualify.rs`).
2. Builder domain `LayerX/porting/evm/builder/v1`, `REBUILD_ATTEMPTS` 2,
   `SOURCE_EPOCH` 1 (`programs/porting/evm/src/qualify.rs`).
3. `validated_module` is `WasmEngine::declared()?.validate(&port.code()?)`
   (`programs/porting/evm/src/qualify.rs`).
4. `deploy_and_verify` deploys immutable WASM using the runtime `ABI_VERSION`, journals, rebuilds
   via `PortBuildRunner`, refuses non-`Verified` status
   (`programs/porting/evm/src/qualify.rs`).
5. `PortBuildRunner` requires published Solidity equal to `SOLIDITY_SOURCE`,
   then `PublicLockPort::parse` and `port.code()`
   (`programs/porting/evm/src/qualify.rs`).

### Tests that prove refusals

| Test | File | Asserted outcome |
| --- | --- | --- |
| `accumulated_contract_value_uses_derived_account_authority` | `programs/porting/evm/src/monetary.rs` | `ContractFunded` through `translate_with_program_account` is `TranslatedValueFlow::ProgramAccount`; `SelfDestructSweep` is `Err(PortRefusal::UnboundedBalanceSweep)` |

`shared_supply` tests assert keys and capability sets; they do not assert a
`PortRefusal` (`programs/porting/evm/src/shared_supply.rs`).

### ERC-20, LXT-20, and native Assets

`programs/sdk/rust/examples/token-lxt20` implements eight methods over
registered program-derived backing accounts. `programs/sdk/rust/src/lxt20.rs`
defines `Request` encodings for the seven transfer, allowance, and query
methods; `initialize` is a separate export, not a `Request` enum variant.
`programs/porting/evm/MIGRATION.md` carries the source-chain migration notes.
Runtime tests cover the methods, allowances, and refusal rollback. The same
isolated native replay shown in the Solana section checks the committed token
and merchant artifacts; it does not certify a live deployment
(`programs/sdk/rust/src/lxt20.rs`,
`programs/sdk/rust/examples/token-lxt20/src/lib.rs`, and
`programs/crates/layerx-programs-registry/src/lxt20.rs`).

| ERC-20 flow | LayerX mapping |
| --- | --- |
| `transfer(to, amount)` | LXT-20 `Request::Transfer`; destination is a 32-byte account ID and amount is exact u128 units |
| `approve(spender, amount)` | `Request::Approve`; spender is a DID identity and zero revokes the program allowance |
| `transferFrom(owner, to, amount)` | `Request::TransferFrom`; allowance decrement and 402 payment stage atomically |
| `balanceOf`, `allowance`, `totalSupply` | Matching request variants; native balance evidence remains a separate read |
| `name`, `symbol`, `decimals` | One `Metadata` request; query the native Asset registry separately for backing-Asset metadata |
| Native value deposit and payout | Register a `PreparedProgramAccount`, then fund and spend it under explicit grants |

Selectors are `4c 58 14 method`, with method number 0 for initialization and
method numbers 1 through 7 for transfer,
approve, transfer-from, balance, allowance, supply, and metadata. After the
selector come LayerX convention `01`, bytes tag `20`, a u32 big-endian payload
length, and the declared fields. Identifiers are 32 bytes and amounts are
16-byte big-endian integers. This is not Solidity ABI encoding and does not use
ERC-20 Keccak selectors
(`programs/sdk/rust/src/lxt20.rs`,
`programs/sdk/rust/examples/token-lxt20/src/lib.rs`, and
`programs/crates/layerx-programs-registry/src/lxt20.rs`).

Before the token can transfer backing units:

1. Deploy the guest with the canonical interface from
   `programs/crates/layerx-programs-registry/src/lxt20.rs`
   (`reference_interface` binds all eight exports). Include the interface in
   the native deployment payload; generating a registry value alone does not
   publish a deployment receipt.
2. Under deployment authority, register the program-derived backing accounts
   with the native Programs account module. Each owner's seed is that DID's
   32-byte identity. `PreparedProgramAccount` computes the account; preparation
   alone does not register it on chain.
3. The configured issuer calls `initialize` once with canonical calldata
   `4c581400012000000000`: selector 0 and a bounded-bytes frame with zero
   payload length. Authorize the exact `Transfer402` funding grant for the
   fixed supply. The transaction funds the issuer's registered program account
   and writes supply and issuer balance together. The guest refuses the wrong
   issuer, repeated initialization, malformed input, and nested calls.
4. Recipients call `approve` at least once, including zero, to register their
   derived account in the token's shared storage. This token registration is
   separate from native account registration. Queries and approvals do not
   create supply. Transfers need the caller's matching `ProgramSpend` grant
   as well as token balance and allowance authorization.

Transfers neither mint nor burn native units, and
every `transfer_from` decrements the allowance, including `u128::MAX`
(`programs/sdk/rust/src/lxt20.rs`,
`programs/sdk/rust/examples/token-lxt20/src/lib.rs`, and
`programs/crates/layerx-programs-registry/src/lxt20.rs`).

Interface encoding v2 carries `CallerAuthorizedSpend` descriptors for the
backing Asset, ceiling, recipient offset, and amount offset. Native admission
requires an actual caller-authorized `ProgramSpend` grant; the descriptor and
an LXT-20 allowance do not confer kernel debit authority over another DID's
account. Infinite uint256 allowances, permits, rebasing, and transfer-tax
semantics are not in this interface. Values above u128 must be refused, and
Ethereum addresses require an explicit identity/account mapping
(`programs/sdk/rust/src/lxt20.rs`,
`programs/sdk/rust/examples/token-lxt20/src/lib.rs`, and
`programs/crates/layerx-programs-registry/src/lxt20.rs`).

Native-issued Asset IDs use SHA-256 of `LX:ASSET:v1 || issuer_id32 || salt32`.
Per-Asset, native-main, and program-derived accounts retain their distinct
derivations. Programs envelopes bind the signer DID and identity sequence;
funding consumes the canonical backing-account sequence, and fees may use a
different native account. Account-bound settlement evidence retains signer
authorization and commits the resolved account endpoints
(`src/modules/asset/lx_asset_registry.c`, `src/ledger/lx_account_id.c`,
`programs/sdk/rust/src/payments.rs`, and
`programs/fixtures/pay5/native_roundtrip.c`).

---

## `layerx-porting-cosmwasm`

### Purpose

The kit (`programs/porting/cosmwasm/src/lib.rs`, `storage.rs`, `monetary.rs`)
carries a CosmWasm contract onto the merged network's Programs ABI. Raw storage keys stay
byte-identical (`cw-storage-plus` `Item` and `Map` prefix framing). JSON
message and event names stay byte-identical. JSON is transcoded at the edge;
the running program moves canonically framed bytes. Bounded `BankSend` and
`SubMessageFunds` payouts use registered program-derived accounts. Context-free
bank payouts, third-party allowance spending without matching authority, and
inter-chain transfers are refused by the monetary translator. A `Deps::querier`
round trip is not supplied by these adapter helpers. Shared program state is
supported explicitly, as described below. The reference port emits, deploys,
rebuilds, and executes through the library qualification path.

`programs_porting_cosmwasm()` returns
`programs/porting/cosmwasm targeting layerx_v2 ABI version 2`
(`programs/porting/cosmwasm/src/lib.rs`).

Guest crate `layerx-porting-cosmwasm-guest` maps `Env` / `MessageInfo` onto
executing program, batch height, and invoking principal, and exposes BLAKE3
(`programs/porting/cosmwasm/guest/src/lib.rs`). On `wasm32`,
`messages::context` re-exports that guest
(`programs/porting/cosmwasm/src/messages.rs`).

### Shared and principal-scoped state

`StateBinding::Shared` is portable and `shared()` is true. Select the
program-shared namespace `(program)` for state that all senders must see;
principal-scoped state uses `(program, principal)`. A key alone does not select
its namespace (`programs/porting/cosmwasm/src/storage.rs`).

| Binding | Key mapping | Namespace |
| --- | --- | --- |
| `Item` | Namespace bytes via `item_key` | Principal-scoped by this binding |
| `SenderIndexed` | Length-prefixed namespace via `map_prefix`; sender moves into the namespace identity | Principal-scoped |
| `SenderSuffixed` | Leading elements remain; trailing sender collapses via `composite_map_key` | Principal-scoped |
| `Shared` | Namespace bytes via `item_key`; compose individual map-entry keys as needed | Program-shared |

`shared_orderbook::OrderBook` maps the order count and order map to shared
state and positions to `SenderIndexed`. `order_key` uses `map_key` with the
order ID bytes. `PortRefusal::SharedState` is an enum variant in
`error.rs`; shared state is supported by the explicit binding above. The
`import_state` helper
below imports sender-indexed cells into principal namespaces; do not use it
as an implicit shared-state importer.

### Public entry points and types

Entries exported from `lib.rs` or their named public modules (`programs/porting/cosmwasm/src/lib.rs`):

| Entry | Request | Result |
| --- | --- | --- |
| `item_key` / `map_prefix` / `map_key` / `composite_map_key` | namespace and key bytes | raw key (`programs/porting/cosmwasm/src/storage.rs`) |
| `StateBinding::layerx_key` | namespace, leading key elements | LayerX key (`programs/porting/cosmwasm/src/storage.rs`) |
| `sender_indexed_import` | namespace, `&[StateHolder]` | `Vec<MigrationCell>` (`programs/porting/cosmwasm/src/storage.rs`) |
| `RecordSchema::new` / `encode` / `decode` / `encode_json` / `decode_json` / `transcode` | name, `FieldSchema`s; values or JSON | framed bytes or JSON (`programs/porting/cosmwasm/src/json.rs`) |
| `variant_tag` | `EntryPoint`, variant name | `[u8; 8]` (`programs/porting/cosmwasm/src/messages.rs`) |
| `MessageVariant::new` / `json` / `data` / `transcode` | entry, variant, `RecordSchema`; values or JSON | JSON document or tag-plus-body (`programs/porting/cosmwasm/src/messages.rs`) |
| `ContractEvent::response` / `custom` / `data` | attributes; event type | topic plus framed attributes (`programs/porting/cosmwasm/src/messages.rs`) |
| `execute_submessage` | `ProgramId`, `&MessageVariant`, `&[FieldValue]` | `CallRequest` (`programs/porting/cosmwasm/src/messages.rs`) |
| `ValueFlow::translate` / `translate_with_program_account` / `translate_all` | asset, principal; plus owner/seed/source | `Transfer402Plan` or `TranslatedValueFlow` (`programs/porting/cosmwasm/src/monetary.rs`) |
| `source_archive` / `published_source` / `validated_module` | `&DonationPort` (and URI for published source) | `SourceArchive`, `PublishedSource`, `ValidatedModule` (`programs/porting/cosmwasm/src/qualify.rs`) |
| `build_plan` | none | `BuildPlan` (`programs/porting/cosmwasm/src/qualify.rs`) |
| `deploy_and_verify` | port, `Publication`, source, lifecycle, registry | `DeployedContract` (`programs/porting/cosmwasm/src/qualify.rs`) |
| `execute_donate` | port, `&Invocation`, storage, `times` | `AuthorizedExecutionRecord` (`programs/porting/cosmwasm/src/qualify.rs`) |
| `execute_donations` / `execute_remaining` | `&Invocation`, storage | `AuthorizedExecutionRecord` (`programs/porting/cosmwasm/src/qualify.rs`) |
| `settle` | activity, storage, kernel | `VerifiedStorageAssignment` (`programs/porting/cosmwasm/src/qualify.rs`) |
| `import_state` | storage, program, `&[(MigrationCell, String)]` | `usize`; JSON transcoded through `donation_record()` (`programs/porting/cosmwasm/src/qualify.rs`) |
| `DonationPort::new` | `DonationTerms` | port (`programs/porting/cosmwasm/src/reference.rs`) |

### Typed refusals

`PortRefusal` (`programs/porting/cosmwasm/src/error.rs`):

| Variant | Condition that raises it |
| --- | --- |
| `EmptyAddress` | `DonationPort::new` with zero asset or beneficiary (`programs/porting/cosmwasm/src/reference.rs`); `sender_indexed_import` with empty address bytes or zero principal (`programs/porting/cosmwasm/src/storage.rs`) |
| `InvalidNamespace` | empty namespace or length above `MAX_NAMESPACE_BYTES` 65535 (`programs/porting/cosmwasm/src/storage.rs`); length-prefix element longer than `u16` (`programs/porting/cosmwasm/src/storage.rs`) |
| `KeyTooLong` | composed key empty or longer than `MAX_STORAGE_KEY_BYTES` (`programs/porting/cosmwasm/src/storage.rs`); `DonationPort::code` when the storage key length does not fit `i32` (`programs/porting/cosmwasm/src/reference.rs`) |
| `SharedState` | defined (`programs/porting/cosmwasm/src/error.rs`). No raise site in this crate |
| `SchemaMismatch` | unnamed/oversized/duplicate record or field (`programs/porting/cosmwasm/src/json.rs`); encode/decode width or type mismatch; unnamed `MessageVariant`; unnamed/duplicate/oversized event attribute; `ContractEvent::data` count/type mismatch (`programs/porting/cosmwasm/src/messages.rs`) |
| `InvalidJson` | malformed document, unknown/repeated/missing field, or values that do not match the declared JSON types (`programs/porting/cosmwasm/src/error.rs`, `programs/porting/cosmwasm/src/json.rs`, `programs/porting/cosmwasm/src/messages.rs`) |
| `ContractHeldBalance` | `ValueFlow::translate` of `BankSend`, `BankBurn`, or `SubMessageFunds` (`programs/porting/cosmwasm/src/monetary.rs`) |
| `InvalidProgramAccount` | `ProgramAccountTransferPlan::new` reserved fields or source mismatch (`programs/porting/cosmwasm/src/monetary.rs`) |
| `SupplyMutation` | `BankBurn` through `translate_with_program_account` (`programs/porting/cosmwasm/src/monetary.rs`) |
| `DelegatedSpend` | `AllowanceSpend` whose `owner` is not the invoking principal (`programs/porting/cosmwasm/src/monetary.rs`) |
| `ChainQuery` | `IbcTransfer` through `ValueFlow::translate` (`programs/porting/cosmwasm/src/monetary.rs`). No other raise site |
| `OutOfRange` | `Transfer402Plan::new` zeros (`programs/porting/cosmwasm/src/monetary.rs`); `DonationPort::new` zero price/cap, cap above `MAX_CAP_BOUND`, or product outside `i64::MAX` (`programs/porting/cosmwasm/src/reference.rs`); unknown query export (`programs/porting/cosmwasm/src/qualify.rs`) |
| `EventDataTooLarge` | `ContractEvent::data` above `MAX_EVENT_DATA_BYTES` (`programs/porting/cosmwasm/src/messages.rs`); `DonationPort::code` event template length or count offset not fitting `i32`/`u32` (`programs/porting/cosmwasm/src/reference.rs`) |
| `TopicTooLarge` | event topic longer than `MAX_EVENT_TOPIC_BYTES` (`programs/porting/cosmwasm/src/messages.rs`); `DonationPort::code` topic length not fitting `i32` (`programs/porting/cosmwasm/src/reference.rs`) |
| `MessageTooLarge` | `MessageVariant::data` longer than `MAX_CALL_INPUT_BYTES` (`programs/porting/cosmwasm/src/messages.rs`) |
| `ModuleTooLarge` | emitted WASM above `DEFAULT_MAX_MODULE_BYTES` (`programs/porting/cosmwasm/src/reference.rs`) |
| `InvalidDescriptor` | `DonationPort::parse` malformed/repeated/unknown/missing keys, wrong version or contract name, non-integer (`programs/porting/cosmwasm/src/reference.rs`) |
| `UnverifiedSource` | `deploy_and_verify` when source status is not `Verified` (`programs/porting/cosmwasm/src/qualify.rs`) |
| `Abi` / `Storage` / `Engine` / `Validation` / `Lifecycle` / `Execution` / `Registry` / `Archive` / `Build` / `TransferLaw` | `From` wrappers (`programs/porting/cosmwasm/src/error.rs`). `settle` uses `PortRefusal::from(failure.error())` (`programs/porting/cosmwasm/src/qualify.rs`) |

`FailureMapping::ContractError` / `StdError` / `Panic` / `SubMessageFailure`
map to `Trap`; `OutOfGas` to `ResourceRefusal`; `CallDepth` to
`StackExhausted` (`programs/porting/cosmwasm/src/messages.rs`).

### Qualification path

Same pipeline shape (`programs/porting/cosmwasm/src/qualify.rs`):

1. Archive holds `COSMWASM_SOURCE`, descriptor, toolchain, lock
   (`programs/porting/cosmwasm/src/qualify.rs`).
2. Builder domain `LayerX/porting/cosmwasm/builder/v1`, `REBUILD_ATTEMPTS` 2,
   `SOURCE_EPOCH` 1 (`programs/porting/cosmwasm/src/qualify.rs`).
3. `validated_module` is `WasmEngine::declared()?.validate(&port.code()?)`
   (`programs/porting/cosmwasm/src/qualify.rs`).
4. `deploy_and_verify` deploys immutable WASM using the runtime `ABI_VERSION`, journals, rebuilds
   via `PortBuildRunner`, refuses non-`Verified` status
   (`programs/porting/cosmwasm/src/qualify.rs`).
5. `PortBuildRunner` requires published source equal to `COSMWASM_SOURCE`, then
   `DonationPort::parse` and `port.code()`
   (`programs/porting/cosmwasm/src/qualify.rs`). Relinking the original
   `wasmd` artifact is not a port (`programs/porting/cosmwasm/src/qualify.rs`).

### Tests that prove refusals

| Test | File | Asserted outcome |
| --- | --- | --- |
| `bank_send_uses_derived_account_authority` | `programs/porting/cosmwasm/src/monetary.rs` | `BankSend` through `translate_with_program_account` is `TranslatedValueFlow::ProgramAccount`; `BankBurn` is `Err(PortRefusal::SupplyMutation)` |

`shared_orderbook` tests assert `StateBinding` classification and keys; they
do not assert a `PortRefusal`
(`programs/porting/cosmwasm/src/shared_orderbook.rs`).

---

## Comparison

| Property | Solana | EVM | CosmWasm |
| --- | --- | --- | --- |
| Workspace member | `porting/solana` (`programs/Cargo.toml`) | `porting/evm` (`programs/Cargo.toml`) | `porting/cosmwasm` (`programs/Cargo.toml`) |
| Guest member | `porting/solana/guest` (`programs/Cargo.toml`) | `porting/evm/guest` (`programs/Cargo.toml`) | `porting/cosmwasm/guest` (`programs/Cargo.toml`) |
| Identifier | `programs/porting/solana targeting layerx_v2 ABI version 2` (`programs/porting/solana/src/lib.rs`) | `programs/porting/evm targeting layerx_v2 ABI version 2` (`programs/porting/evm/src/lib.rs`) | `programs/porting/cosmwasm targeting layerx_v2 ABI version 2` (`programs/porting/cosmwasm/src/lib.rs`) |
| Byte-identical carry | account / instruction / event discriminators and `borsh` account bytes (`programs/porting/solana/src/lib.rs`) | storage slots, event topics, four-byte selectors (`programs/porting/evm/src/lib.rs`) | `cw-storage-plus` raw keys; JSON message and event names (`programs/porting/cosmwasm/src/lib.rs`) |
| Reference port | `MintLimitPort` / `GuardTerms` (`programs/porting/solana/src/lib.rs`) | `PublicLockPort` / `LockTerms` (`programs/porting/evm/src/lib.rs`) | `DonationPort` / `DonationTerms` (`programs/porting/cosmwasm/src/lib.rs`) |
| Deployed type | `DeployedGuard` (`programs/porting/solana/src/qualify.rs`) | `DeployedLock` (`programs/porting/evm/src/qualify.rs`) | `DeployedContract` (`programs/porting/cosmwasm/src/qualify.rs`) |
| Execute entries | `execute_mint`, `execute_mint_count`, `execute_mint_remaining` (`programs/porting/solana/src/lib.rs`) | `execute_purchase`, `execute_has_valid_key`, `execute_remaining_periods` (`programs/porting/evm/src/lib.rs`) | `execute_donate`, `execute_donations`, `execute_remaining` (`programs/porting/cosmwasm/src/lib.rs`) |
| Import | `import_accounts` of `(MigrationCell, Vec<u8>)` (`programs/porting/solana/src/qualify.rs`) | `import_state` of `(MigrationCell, [u8; 32])` (`programs/porting/evm/src/qualify.rs`) | `import_state` of `(MigrationCell, String)` transcoded from JSON (`programs/porting/cosmwasm/src/qualify.rs`) |
| Rebuilds | `REBUILD_ATTEMPTS` 2 (`programs/porting/solana/src/qualify.rs`) | 2 (`programs/porting/evm/src/qualify.rs`) | 2 (`programs/porting/cosmwasm/src/qualify.rs`) |
| Provenance check | `ANCHOR_SOURCE` (`programs/porting/solana/src/qualify.rs`) | `SOLIDITY_SOURCE` (`programs/porting/evm/src/qualify.rs`) | `COSMWASM_SOURCE` (`programs/porting/cosmwasm/src/qualify.rs`) |
| Zero-identity refusal | `ZeroPubkey` | `ZeroAddress` | `EmptyAddress` |
| Direct program-balance write without derived-account context | `LamportMutation` | `ContractHeldBalance` | `ContractHeldBalance` |
| Unbounded close/sweep | `UnboundedRentSweep` | `UnboundedBalanceSweep` | no sweep variant; `BankBurn` is `SupplyMutation` |
| Third-party allowance | `DelegatedSpend` | `DelegatedSpend` | `DelegatedSpend` |
| Inter-chain / querier | no `ChainQuery` variant | no `ChainQuery` variant | `ChainQuery` on `IbcTransfer` only (`programs/porting/cosmwasm/src/monetary.rs`) |
| Emitter size refusal | `ModuleTooLarge` (`programs/porting/solana/src/reference.rs`) | `ModuleTooLarge` (`programs/porting/evm/src/reference.rs`) | `ModuleTooLarge` (`programs/porting/cosmwasm/src/reference.rs`) |
| Refusal test in crate | `Err(PortRefusal::UnboundedRentSweep)` (`programs/porting/solana/src/monetary.rs`) | `Err(PortRefusal::UnboundedBalanceSweep)` (`programs/porting/evm/src/monetary.rs`) | `Err(PortRefusal::SupplyMutation)` (`programs/porting/cosmwasm/src/monetary.rs`) |
| ABI v2 reference guest | `programs/porting/solana/reference-v2/src/lib.rs` | `programs/porting/evm/reference-v2/src/lib.rs` | `programs/porting/cosmwasm/reference-v2/src/lib.rs` |
| Make lint artifact | `layerx_anchor_context_reference.wasm` (`Makefile`) | `layerx_evm_context_reference.wasm` (`Makefile`) | `layerx_cosmwasm_context_reference.wasm` (`Makefile`) |

[Operators](index.md) · [Programs](../programs/index.md) · [Home](../index.md)
