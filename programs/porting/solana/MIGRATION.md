# Porting a Solana program to LayerX programs

This guide maps Anchor accounts, instructions and payments onto the programs
runtime in Paxeer. Paxeer mainnet-beta is one BFT network, with EVM chain ID
125 (`0x7d`). The kernel is part of the network and proposes blocks; a receipt
is final when its block is committed. Program execution, storage changes and
payments belong to that network's transaction processing.

For node access, use the root-path endpoints in the
[public RPC reference](../../../docs/site/docs/reference/public-rpc.md).
Sub-chains are tenant domains in the database. They do not require a separate
program execution service.

The account and instruction examples below come from
[`src/reference.rs`](src/reference.rs), which contains the Anchor mint-limit
and SOL-payment source as `ANCHOR_SOURCE` and emits its port as WebAssembly.
`programs/mint-limit/src/lib.rs` is the source's path **inside the published
archive**, not a source directory to build in this checkout. The reference
emitter uses the version-one host vocabulary. The SDK also provides newer
context, crypto, response and payment bindings; select the ABI explicitly
when building and registering a guest.

## ABI v2 account context and native crypto

On `wasm32`, `anchor::context::AnchorContext::current()` is re-exported by
[`src/anchor.rs`](src/anchor.rs) from the
[guest adapter](guest/src/lib.rs). It returns:

| Anchor vocabulary | Authenticated runtime fact |
| --- | --- |
| `program_id` | `Context::executing_program()` |
| `signer` | `Context::invoking_principal()` |
| `slot` | `Context::batch_height()` |

The signer is the invoking principal's identity, not an imported Solana
account handle. Batch height is a deterministic slot substitute, not Solana's
slot number or a wall-clock timestamp. ABI v2 supplies no enumerable
authenticated Anchor instruction-account list; do not manufacture one from
untrusted calldata. Declare authority through capabilities and validate any
application identifiers the caller supplies.

Use [`layerx_program_sdk::crypto`](../../sdk/rust/src/crypto.rs) for guest
hashing and signature checks. `hash` supports SHA-256, Keccak-256 and BLAKE3;
`ed25519_verify`, `secp256k1_verify` and `secp256k1_recover` expose typed
signature operations. `HashInput` bounds hash input to 1,048,576 bytes, and
`Ed25519Message` bounds messages to 64 bytes. Ed25519 keys/signatures are
32/64 bytes; secp256k1 verification takes a 32-byte digest, a 33- or 65-byte
public key and a 64-byte signature. Recovery also requires `RecoveryId` in
`0..=3`. These functions return `ProgramError` on refusal. Verifying a
signature does not grant spending authority.

The [ABI v2 reference guest](reference-v2/src/lib.rs) reads authenticated
context and hashes its input through these bindings. Its
[manifest](reference-v2/layerx-program.json) declares ABI 2. The repository's
`programs-porting-v2-references` Make target builds and lints this guest and
passes it to the native `programs_call_activity` harness. It is a context and
hashing example, not the mint-limit port. Consult the
[ABI policy](../../sdk/rust/src/abi_policy.rs) for admitted versions; the
runtime's current evidence version in
[`lib.rs`](../../crates/layerx-programs-runtime/src/lib.rs) is 4. Keep the guest
imports, declared ABI and host admission consistent.

## What stays byte-identical

The kit preserves Anchor's discriminator preimages and the supported field
encodings:

| Artifact | Anchor | Port |
| --- | --- | --- |
| Account discriminator | `sha256("account:MintCounter")[..8]` | first eight bytes of the stored cell |
| Instruction discriminator | `sha256("global:mint")[..8]` | first eight bytes of the reference call input |
| Event discriminator | `sha256("event:MintPerformed")[..8]` | event topic |
| Supported fields | Borsh, little-endian integers | same field bytes |

The encoding helpers live in [`src/account.rs`](src/account.rs) and
[`src/anchor.rs`](src/anchor.rs). This preserves the relevant data codecs;
clients still need the Paxeer transport, principal identities and capability
authorization. An event consumer must read the discriminator from the topic
and decode the fields from the payload.

## Accounts

### The namespace replaces the payer and program seeds

Principal-scoped state lives in the `(program, principal)` namespace fixed by
the runtime. A key cannot select another program's or principal's namespace.
For the mint-limit example, the Solana seeds are:

```rust
seeds = [
    COUNTER_SEED,               // b"mint_limit"
    &[GUARD_ID],                // 0x03
    payer.key().as_ref(),       // the signer
    candy_guard.key().as_ref()  // the guard instance
]
```

This port maps one guard instance to one program and each payer to an
invoking principal. Only under that mapping are the last two seeds redundant.
`SeedPath::collapse(&[2, 3])` drops them; `storage_key()` frames each remaining
seed as a one-byte length followed by its bytes:

```text
0a 6d 69 6e 74 5f 6c 69 6d 69 74 01 03
```

Do not drop an arbitrary user or instance seed unless the chosen namespace
already distinguishes it. Shared state uses the `(program)` namespace instead
and may need to retain seeds that distinguish users or instances.

### Every other derivation keeps its shape

[`src/pubkey.rs`](src/pubkey.rs) provides the migration helpers:

| Task | Function |
| --- | --- |
| Hash seeds, bump, program and `ProgramDerivedAddress` | `SeedPath::address(bump, program)` |
| Compare the result with a published address | `SeedPath::verify(bump, program, published)` |
| Drop namespace-supplied seeds by index | `SeedPath::collapse(envelope)` |
| Frame retained seeds into a storage key | `SeedPath::storage_key()` |
| Replace one seed for a particular holder | `SeedPath::with_seed(index, seed)` |

`SeedPath` admits 1 to 16 seeds, each at most 32 bytes, and bounds the framed
storage key. Collapsing away every seed is refused. The address helper
reproduces the hash preimage but does **not** perform Solana's off-curve check
or search for a bump. Use a trusted snapshot's published address and bump;
`verify` returns `DerivationMismatch` if the hash differs.

### What an account in an `Accounts` struct becomes

| Anchor account | Port |
| --- | --- |
| `#[account(init)]` / `init_if_needed` program state | principal-scoped cell, `AccountRole::ProgramState` |
| Program-owned shared state | shared cell, `AccountRole::ProgramOwnedShared` |
| `Signer<'info>` | authenticated invoking principal |
| `SystemAccount` / `UncheckedAccount` credited by a transfer | authenticated 402LXP transfer recipient |
| Program handle used for CPI | callee-specific `Call` capability |
| Token balance treated as directly mutable account bytes | `AccountRole::TokenBalance` refuses with `LamportMutation` |

`AccountRole::translate` returns these mappings. The last row does not prohibit
asset transfers: use the money and token paths below instead of a balance write.

Use `layerx_program_sdk::storage::shared` for totals, pool accounting and other
state every principal invoking the same program must reach. It requires shared
storage authority. The [shared-pool example](src/shared_pool.rs) preserves an
Anchor schema in this namespace. Shared storage does not itself authorize a
payment from a native account.

### Account data carries over unchanged

`AccountSchema` supports `u8`, `u16`, `u32`, `u64`, `i64`, `bool` and 32-byte
`Pubkey` fields. It admits at most 64 fields and checks names, types and the
storage value bound. It is not a general Borsh serializer for arbitrary Anchor
types. `decode` checks both the discriminator and the exact declared length;
booleans must be encoded as zero or one.

```text
MintCounter { count: u16 }   space = 8 + 2

00..08   sha256("account:MintCounter")[..8]
08..10   count, little-endian u16
```

### Migrating existing accounts

`per_signer_import` rebuilds each holder's seed path, checks its published bump
and address, and produces a `MigrationCell` containing that address, the
collapsed key and the destination principal. Each holder can use the same key
because its principal namespace differs. The migration must supply the correct
Solana-key-to-principal mapping; it is not inferred by the runtime.

`qualify::import_accounts` takes the plan and snapshot bytes and writes them
through a storage transaction. It preserves the bytes; validate them against
the intended `AccountSchema` before import. This helper does not fetch or
authenticate a Solana snapshot for you.

### What has no equivalent

| Solana construct | Required translation |
| --- | --- |
| Lamports held by a PDA | registered program-derived value account; see [Money](#money) |
| Rent and rent exemption | use declared storage bounds; there is no rent deposit |
| `#[account(mut, realloc = ...)]` | write the whole bounded value |
| `#[account(close = recipient)]` | delete state separately; an amount-less rent sweep is unsupported |
| Account owner checks | namespace isolation protects cells; validate application authority explicitly |
| `remaining_accounts` | redesign around explicit data and capabilities; no authenticated account-list enumeration |

## Instructions and dispatch

`InstructionAbi` encodes `sha256("global:name")[..8]` followed by supported
Borsh arguments. The mint-limit emitter implements these inputs:

```text
mint(amount: u16)      10 bytes   sha256("global:mint")[..8] . u16 le
mint_count()            8 bytes   sha256("global:mint_count")[..8]
mint_remaining()        8 bytes   sha256("global:mint_remaining")[..8]
```

Its `layerx_reserve(len)` returns the input region. Its `layerx_call(ptr, len)`
checks the pointer, discriminator and exact handler length, then dispatches
using the discriminator loaded as a little-endian `i64`. It also exports
`mint`, `mint_count` and `mint_remaining` directly.

That emitter's call entry returns an `i32` result and supplies no response
bytes; the counts fit in `u16`. This is a property of this reference, not a
restriction on newer ABIs. ABI v2 provides `response_write` and
`program_call_response`; use the [SDK call bindings](../../sdk/rust/src/call.rs)
when a port needs response data across a program call.

## Events

`AnchorEvent` separates the Anchor discriminator from the Borsh field payload:

```text
emit!(MintPerformed { count })

topic   sha256("event:MintPerformed")[..8]
data    count, little-endian u16
```

The mint reference emits exactly that schema. An indexer can retain its
discriminator filter with a transport adapter. If you remove or rearrange an
event field, its payload schema changes even if the event name, and therefore
the discriminator, stays the same. Publish the new schema for consumers.

## Cross-program invocation

`cross_program_invocation(callee, &instruction, &arguments)` returns a
`CpiRequest` with the callee, Anchor-encoded input and one
`Capability::Call { program: callee }`. The caller must already hold the
required authority. The helper constructs a request; it does not execute it.

A call does not lend Solana account handles or an `invoke_signed` PDA
signature. The callee accesses its own storage under the runtime's namespace
rules. Use explicit capabilities for additional effects. An owner program's
bounded account payout is supported through the payment API described below;
it is not a transferable PDA signing privilege.

## Money

Programs request transfers; only the kernel writes native balances. A
Solana PDA holding value maps to a real account derived from the Paxeer program
and a public seed, registered under deployment authority before use.

[`src/monetary.rs`](src/monetary.rs) distinguishes principal-funded legs from
program-account legs:

| Solana flow | Porting helper behaviour |
| --- | --- |
| Signer-funded `system_program::transfer` | `ValueFlow::SignerFunded` produces a `Transfer402Plan` |
| Direct mutable lamport balance write | `LamportWrite` refuses with `LamportMutation` |
| Bounded `invoke_signed` payout from program custody | `ProgramAuthorityFunded` with `translate_with_program_account` produces a `ProgramAccountTransferPlan` |
| Amount-less rent sweep | `translate_with_program_account` refuses with `UnboundedRentSweep` |
| SPL transfer authorized by the invoking principal | `TokenTransfer` produces a principal-funded plan |
| SPL transfer authorized by someone else | plain `translate` refuses with `DelegatedSpend`; redesign with explicit grants |

The principal-only `translate` and `translate_all` helpers refuse program-funded
payouts and rent sweeps with `LamportMutation`. `portable()` recognizes only
signer-funded and token-transfer shapes; it does not validate token authority
or cover program custody. Use the translation result, including the full
program-account context where needed, to decide whether a flow is supported.

`ProgramAccountTransferPlan::new` rederives the source from the owner program
and seed, checks the monetary fields and produces a bounded `ProgramSpend`
capability. `ProgramAuthorityFunded.authority` must be the replacement native
source account, not the original Solana PDA. A mismatched source is
`InvalidProgramAccount`. A derivation seed alone grants no spending authority.

The port descriptor selects the authenticated 402LXP asset that replaces SOL.
Amounts are exact asset units, not automatically converted lamports. The mint
reference charges `price * amount`; its constructor bounds the product of
price and limit to `i64::MAX`.

### Escrow, vaults and refunds

A vault can accumulate funds and pay them out in a later invocation through a
registered program-derived account. Record the migration mapping explicitly:

| Source identifier | Destination identifier |
| --- | --- |
| SOL or SPL mint | authenticated native asset ID selected by the port; amounts are exact asset units |
| Solana program ID | deployed Paxeer `ProgramId` owning the vault |
| Solana vault PDA and its seeds/bump | native account from `derive_program_account(owner_program, seed)`; the public seed is at most 128 bytes |
| Payer or refund beneficiary public key | authenticated principal and explicitly mapped native recipient account |

The native derivation replaces the PDA's value-holding role; the original
Solana PDA bytes and bump are not native transfer authority. Use
[`PreparedProgramAccount`](../../sdk/rust/src/payments.rs) to derive it and
produce its registration payload. Submit that payload through the signed
native registration path under deployment authority, binding the owning
program, seed and asset before funding. A bounded `Transfer402` grant and
`fund_program_account(prepared.deposit(amount)?)` fund the account from the
invoking principal. Registration alone neither funds it nor grants a payout.

For a later refund, check the stored liability and beneficiary in the owner
program, then construct the porting plan:

```rust
let source = derive_program_account(owner_program, seed)?.bytes();
let plan = ProgramAccountTransferPlan::new(
    owner_program, seed, source, asset, refund_account, refund_amount,
)?;
let grant = plan.capability(); // ProgramSpend, capped at refund_amount
```

`reference::program_account_payout` builds the same plan. Carry its grant in
the authorized call; the guest requests the corresponding transfer with
`pay_from_program_account(prepared.payment(recipient, amount)?)`. The plan
rederives the source and refuses mismatched owner/seed/source or reserved
monetary fields with `InvalidProgramAccount`. The host checks the executing
owner-program frame and the exact source, asset, recipient and amount ceiling;
a grant for another program cannot authorize this frame. The kernel also
checks the registered account and available funds before atomic settlement.
This bounded owner payout does not provide an `invoke_signed` signature to a
callee or authorize arbitrary writes to lamports.

Keep liabilities and refund eligibility in program storage and request an
exact bounded payout when the owner frame executes. Deleting a storage cell
does not release money, and writing an accounting balance does not move a
native asset. The [merchant guest](../../sdk/rust/examples/payments-merchant/src/lib.rs)
shows a deposit and fee split using these payment bindings.

## Failure and compute

`FailureMapping::outcome` describes the reference translation:

| Anchor failure | Runtime outcome |
| --- | --- |
| `require!`, `err!`, failed account constraint, discriminator mismatch | guest trap (`unreachable`) |
| Panic, checked failure mapped to panic, invalid indexing | guest trap |
| Exhausted compute budget | resource refusal |
| Exhausted declared call/stack bound | stack refusal |

The mint emitter traps on nonzero write, transfer and event host statuses.
Traps and refusals discard staged effects rather than committing a partial
invocation. Check every host result in a port. Preserve checked arithmetic;
do not assume every Rust integer overflow traps automatically. Newer SDK
entrypoints can also carry typed refusals; see the
[failure entrypoint reference](reference-v2/src/lib.rs).

## Context mappings and unavailable constructs

| Solana construct | Port |
| --- | --- |
| `Clock::get()?.slot` | authenticated batch height through `AnchorContext::current()?.slot` |
| `unix_timestamp` and other wall-clock fields | explicit protocol facts or counted quantities; absent from `AnchorContext` |
| `recent_blockhashes`, `SlotHashes` | no direct equivalent in this context adapter |
| `invoke_signed` | bounded owner-account payout where applicable; no borrowed PDA signature |
| Account reallocation and closure for rent | bounded writes/deletes, with payments authorized separately |
| `remaining_accounts` fan-out | explicit application inputs and capabilities |
| SHA-256 / Keccak-256 / BLAKE3 | guest `crypto::hash(HashAlgorithm, HashInput)` returns `[u8; 32]`; input is at most 1,048,576 bytes |
| Ed25519 verification | guest `ed25519_verify(Ed25519Message, &[u8; 32], &[u8; 64])`; message is at most 64 bytes |
| secp256k1 verification | guest `secp256k1_verify(&[u8; 32], &Secp256k1PublicKey, &[u8; 64])`; key is `Compressed([u8; 33])` or `Uncompressed([u8; 65])` |
| secp256k1 recovery | guest `secp256k1_recover(&[u8; 32], &[u8; 64], RecoveryId)` returns `[u8; 65]`; recovery ID is `0..=3` |
| Floating point and ambient randomness | deterministic-subset restrictions; do not use ambient entropy |

These deterministic SDK operations are available to `wasm32` guests at ABI v2
and later admitted versions. Signatures use fixed 64-byte encodings (`r || s`
for secp256k1), not DER. Bounded input constructors refuse oversized buffers
or an invalid recovery ID; host calls refuse malformed keys/signatures,
failed verification/recovery and resource exhaustion as `ProgramError`.
Propagate refusals rather than treating them as successful verification.
Transaction signatures, invoking-principal authentication and spending
capabilities remain host authority checks. A valid application signature
does not add a capability or authorize a native debit.

## The reference port, line by line

`MintLimitPort::new(GuardTerms)` accepts asset, destination, limit and price.
The descriptor pins them into the emitted module; this example deploys with
`UpgradePolicy::Immutable`. This is the reference design, not a requirement
that every program keep configuration immutable. `code()` emits its module
and `code_hash()` computes its SHA-256 digest.

| Anchor operation | Emitted reference behaviour |
| --- | --- |
| Require a positive amount within the limit | signed `i64` comparisons, trapping on refusal |
| Load `mint_counter` | read the collapsed principal-scoped key |
| Check its discriminator | compare the first eight bytes with the Anchor tag |
| Add amount and enforce the cumulative limit | bounded addition and comparison |
| Store `counter.count` | write the tag plus little-endian `u16` |
| Pay `price * amount` | `transfer_402` with high amount limb zero |
| Propagate transfer refusal | trap unless host status is zero |
| Emit `MintPerformed` | Anchor topic and two-byte count payload |
| Return the count | direct handler returns `i64`; call entry returns `i32` |

An absent counter yields zero taken and the full limit remaining; the mint
path initializes it. The configuration comes from the descriptor rather than
a mutable guard account. These are deliberate differences to describe to
clients during migration.

`mint_capabilities` builds `StorageRead`, `StorageWrite`, `EmitEvent` and one
`Transfer402` capped at the exact payment to the configured destination.
`query_capabilities` requires only `StorageRead`.

## Deploying and verifying a port

[`src/qualify.rs`](src/qualify.rs) contains library helpers for the reference
pipeline:

1. `source_archive` includes the embedded Anchor source, port descriptor,
   toolchain manifest and dependency lock.
2. `build_plan` pins builder identity, toolchain and lock digests, command and
   artifact path. `PortBuildRunner` checks the known Anchor provenance, parses
   the descriptor and invokes the emitter. It is a compiler for this reference
   descriptor, not a general Anchor-to-WASM compiler.
3. `deploy_and_verify` invokes the lifecycle, journals the deployment, replays
   it into the registry and asks `SourceVerifier` for two independent rebuilds.
   It returns only after the registry reports `SourceStatus::Verified`.

These functions describe a library pipeline, not evidence that this guide's
example is deployed or qualified on the current network. The version-one
emitter and the explicit ABI v2 context guest are distinct artifacts. Verify
ABI agreement and use the native deployment/call path for the selected guest.

The qualification helper `settle` accepts a `PreparedAuthorizedActivity` and
calls `strict_settle` with the kernel transfer primitive. This is an internal
atomic application boundary within the merged network. Source verification
establishes artifact reproducibility; it does not replace signed transaction
authority or a committed block receipt.

## Checklist before you port

1. Classify every account with `AccountRole`; distinguish storage bytes from
   native value and select principal-scoped or shared storage deliberately.
2. Record each PDA's seeds, bump and published address. Collapse only seeds
   already distinguished by the destination namespace.
3. Check framed keys, supported schema fields, discriminators and storage bounds.
4. Translate every payment. Register custody accounts and supply exact funding
   and spend authority; redesign refused direct balance writes or rent sweeps.
5. Preserve instruction and account codecs, and publish the event topic/payload
   mapping. Adapt transport and identity handling for clients.
6. Map time and crypto to authenticated context and bounded SDK operations.
7. Pin the artifact's ABI, source, descriptor, toolchain and lock. Verify
   reproducibility and exercise native admission, payments and rollback before
   publishing deployment claims.

## SPL tokens, LXT-20 and native assets

The current token interface has eight operations, including initialization
before funding and transfers. The
[LXT-20 request codec](../../sdk/rust/src/lxt20.rs) defines seven `Request`
variants (selectors 1–7); the
[token guest](../../sdk/rust/examples/token-lxt20/src/lib.rs) also exports the
separately encoded `initialize` (selector 0). The
[native interface fixture](../../fixtures/pay5/token-lxt20.interface) binds all
eight exports:

| Selector / export | SPL flow and reference mapping |
| --- | --- |
| 0 / `initialize` | Establish fixed supply; issuer-only, staging full-supply funding |
| 1 / `transfer` (`Transfer`) | Transfer / transfer checked; exact `u128` units and explicit backing asset |
| 2 / `approve` (`Approve`) | Approve / revoke delegate; zero revokes the stored allowance |
| 3 / `transfer_from` (`TransferFrom`) | Delegated transfer; requires stored allowance and native spend authority |
| 4 / `balance_of` (`BalanceOf`) | Read shared token accounting |
| 5 / `allowance` (`Allowance`) | Read delegated amount |
| 6 / `total_supply` (`TotalSupply`) | Read initialized supply |
| 7 / `metadata` (`Metadata`) | Read token metadata; native asset facts remain registry facts |

PDA custody uses a registered derived account and `ProgramSpend`. This token
guest requires direct calls and refuses an immediate program caller.

Each request starts with `[b'L', b'X', 20, method]`, where methods 1 through 7
follow the codec's enum order. It then carries convention byte `0x01`, bounded
bytes tag `0x20`, a four-byte big-endian payload length and the payload.
Amounts are 16-byte big-endian integers. Do not reuse Anchor's eight-byte
discriminators, Borsh integer encoding, Solana public keys or associated token
account derivations for this interface. The related
[EVM porting guide](../evm/MIGRATION.md) uses the same LXT-20 convention.

`initialize` is not a `Request` variant. Its separate input is exactly:

```text
4c 58 14 00 01 20 00 00 00 00
```

It requires the configured `REFERENCE_ISSUER`, zero existing supply, shared
storage read/write capabilities and a `Transfer402` grant for
`REFERENCE_ASSET` to the issuer's derived account capped at least at
`REFERENCE_SUPPLY` (1,000,000 units). On success it stages one program-account
funding leg, registers the issuer in token storage, records the full issuer
balance and supply, and returns canonical empty response bytes
`01 20 00 00 00 00`. A different caller, missing or insufficient funding grant,
wrong asset/recipient, malformed input or repeated initialization is refused;
no token storage or payment effects commit. A full grant does not override the
issuer check. Native account registration and sufficient payer funds are also
required for settlement.

The reference derives each owner's native account from
the program and the owner's 32-byte principal identity. Owners register in
token storage by calling `approve` (zero is sufficient); deployment authority
must separately register their native accounts. `PreparedProgramAccount`
produces the registration payload, and `ProgramPaymentCapabilities` builds
canonical payment grant sets while refusing duplicate keys.

The bound interface and native artifacts live in
[`programs/fixtures/pay5`](../../fixtures/pay5). Its dynamic-spend descriptors
bind asset and ceiling; native admission resolves recipient and amount from
calldata and validates actual caller grants. Descriptors and stored allowances
do not independently grant native spending authority. Token accounting changes
and backing-account payments must succeed atomically. A native balance read
uses receipt-bound `BalanceView` authority rather than reading token storage.

The fixture replay entrypoint is
[`run_native_roundtrip.py`](../../fixtures/pay5/run_native_roundtrip.py); it
replays isolated token and merchant activities and compares canonical artifacts.
The committed `native-token/05-initialize.*` artifacts record successful
initialization before approve and transfer calls; `08-transfer_from.*` records
the refused missing-spend-authority case, `09-transfer_from.*` the paused-asset
refusal and `10-transfer_from.*` the authorized delegated payment. The focused
[repair integration test](tests/repair_22_33_241.rs) executes the committed token
WASM in-process, including positive and refused initialization and later
owner-bound payouts; it needs no native harness or live endpoint.
Fixtures and runtime tests are not certification of a live deployment.

Native asset registration, account opening, mint and burn are separate signed
asset activities, not LXT-20 methods. The codecs do not provide SPL rent
recovery, Token-2022 extensions, mint/freeze authority migration, multisig
signer lists or an enumerable Anchor account context. Model or refuse those
requirements explicitly when planning a port.
