# Porting a CosmWasm contract to LayerX programs

LayerX programs execute inside `paxd` on the single Paxeer BFT network. The
kernel is part of the network and proposes blocks; a receipt is final when its
block commits. Public Ethereum JSON-RPC uses the root URL documented in the
[public RPC reference](../../../docs/site/docs/reference/public-rpc.md),
with chain ID 125 (`0x7d`). Program execution and monetary effects belong to the
same chain transaction. Sub-chains are tenant domains in the database.

For the same storage and payment model expressed in Solidity terms, see the
[EVM migration guide](../evm/MIGRATION.md).

## ABI v2 `Env`, `MessageInfo` and crypto

The guest adapter is [`layerx_porting_cosmwasm_guest`](guest/src/lib.rs), not
part of the host-side `messages` module. Its `current()` returns `(Env,
MessageInfo)`: `info.sender` is the authenticated invoking principal,
`env.contract` is the executing program ID, and `env.block_height` is
`Context::batch_height()`. The sender remains the invoking principal in nested
frames; this is a deliberate difference from a CosmWasm contract caller.

There is no attached-funds field in this adapter. Treat `MessageInfo.funds` as
unavailable, rather than an empty vector or an amount asserted by calldata.
Funding and payouts use explicit authorized 402LXP effects, described below.

Guest code uses [`layerx_program_sdk::crypto`](../../sdk/rust/src/crypto.rs).
It provides SHA-256, Keccak-256 and BLAKE3 via `hash`, Ed25519 verification,
secp256k1 verification and recovery, and integer arithmetic helpers. The guest
adapter also exports `blake3`. Inputs use bounded or fixed-width SDK types and
refusals propagate as `ProgramError`; these are guest host imports, not calls
to EVM precompile addresses.

[`reference-v2/src/lib.rs`](reference-v2/src/lib.rs) exercises authenticated
context and BLAKE3. The repository target `make programs-porting-v2-references`
builds the locked context references. The directory name describes those
mappings, not a promise that every SDK default is version 2: check
[`abi_policy.rs`](../../sdk/rust/src/abi_policy.rs), the guest imports and the
deployment's ABI declaration together. Building a guest or reproducing its
source alone does not qualify execution through the node's production host.

This guide is written for someone who knows `cosmwasm_std` and
`cw-storage-plus` and has never written a LayerX program. It maps the contract
vocabulary you already use onto the programs ABI, and it is
explicit about the constructs that do not carry over, because the kit refuses
those by name rather than emulating them.

The worked example is `src/contract.rs` in the published archive: the donation
contract from the CosmWasm book - an `Item` of configuration, a `Map` keyed by
`info.sender`, funds in and funds straight back out. Its complete port lives in
`src/reference.rs` and is deployed, source-verified and executed by
`src/qualify.rs`.

A CosmWasm contract is already WebAssembly, so the first question is always
whether the existing artifact can simply be relinked. It cannot. Its host
imports and entry-point conventions target `wasmd`, including its JSON message
boundary. The port replaces the host, the boundary and the encoding, and keeps
the names.

## What stays byte-identical

| Artifact | CosmWasm | After the port |
| --- | --- | --- |
| `Item` raw key | the namespace bytes verbatim | the same bytes, as the storage key |
| `Map` raw key | `u16` big-endian length, namespace, key | the same framing, `storage::map_key` |
| Message variant name | `{"donate":{ ... }}` | `donate`, unchanged |
| Field names | `times`, `count` | the same names, in the same declaration order |
| Event type | `wasm-donation` | the same bytes, as the program event topic |
| Attribute key | `count` | the same bytes, inside the payload |

The kit preserves these names while changing their encoding. CosmWasm has no
on-the-wire selector to preserve, because dispatch matches a JSON object key. The kit therefore gives each variant an
eight-byte tag, `sha256("<entry point>:<variant>")[..8]`, derived from the name
you already published. Renaming a variant is a breaking change after the port
exactly as it is before it.

## Storage

### The namespace replaces `info.sender`

A CosmWasm contract owns one flat key-value store that every caller shares, so
per-user state has to be keyed by the address the contract is storing state
*about*. A LayerX program owns a byte-keyed map inside a namespace that is
`(program, principal)`, and that namespace is fixed by the runtime *before your
code runs*. A different key cannot select another principal's namespace.
Shared state uses a separate program-scoped API, described below.

The direct consequence: **a key element that carries `info.sender` tells the
port nothing the runtime has not already fixed, and therefore drops out of the
key.**

```rust
pub const CONFIG: Item<Config> = Item::new("config");
pub const DONATIONS: Map<&Addr, DonationRecord> = Map::new("donations");
```

```text
CosmWasm  63 6f 6e 66 69 67                               CONFIG, one global cell
CosmWasm  00 09 64 6f 6e 61 74 69 6f 6e 73 <addr bytes>   one DONATIONS entry
LayerX    00 09 64 6f 6e 61 74 69 6f 6e 73                the same entry, collapsed
```

Every donor's record collapses onto that same eleven-byte key. The cells do not
collide, because each one is written in a different namespace.

### Raw keys keep their shape

`src/storage.rs` reproduces the `cw-storage-plus` composition rules exactly, so
an exported state dump can be located key for key before anything is written:

| Task | Function |
| --- | --- |
| The raw key of an `Item` | `item_key(namespace)` |
| The prefix a `Map` writes before every key | `map_prefix(namespace)` |
| The raw key of one `Map` entry | `map_key(namespace, key)` |
| The raw key of a composite-keyed entry | `composite_map_key(namespace, leading, last)` |
| The key the ported cell occupies | `StateBinding::layerx_key(namespace, leading)` |

The framing is the one `cw-storage-plus` uses: every element except the last is
written as a two-byte big-endian length followed by its bytes, and the last
element is written bare.

### What a piece of state becomes

| CosmWasm state | After the port |
| --- | --- |
| `Item<T>` holding deployment configuration | pinned into the module by the port descriptor |
| `Item<T>` holding per-caller state | `StateBinding::Item`, one cell per principal, at the namespace bytes |
| `Item<T>` holding a mutable global aggregate | `StateBinding::Shared`, one cell per program, preserving cross-sender state |
| `Map<&Addr, T>` keyed by `info.sender` | collapses onto the map's namespace prefix |
| `Map<(K, &Addr), T>` ending in `info.sender` | the trailing element collapses; the leading ones stay |
| `Map<K, T>` every account must be able to read | `StateBinding::Shared`, entries in `(program)` via `layerx_program_sdk::storage::shared` |
| `Map::range` / `Map::prefix(..).range` | bounded `storage::scan` or `storage::shared::scan`, with opaque continuation cursors |

`StateBinding::layerx_key` selects a cell or collapsed prefix; it is not a
complete arbitrary-map migration. `Shared` returns the bare namespace bytes,
so compose individual shared map entries with `map_key` or
`composite_map_key`. Preserve the original key encoding for each key type.
Only remove a sender component when its meaning is the authenticated invoking
principal; keep explicit owner keys for access to other users' shared records.

### Shared state is now portable

A name registry, an order book, a global leaderboard - anything one account
writes and another account reads - maps onto the program-shared namespace
`(program)`. Use `StateBinding::Shared` to classify it, and
`layerx_program_sdk::storage::shared` to read and write it. Every principal
invoking the program addresses the same cell, subject to `SharedStorageRead`
and `SharedStorageWrite` grants and the program's own authorization checks.
An `Item` used as a global aggregate belongs here too; it need not become one
cell per principal. [`src/shared_orderbook.rs`](src/shared_orderbook.rs)
shows a shared order count and order entries alongside principal-scoped
positions. Maintain secondary indexes explicitly as shared keys.

For example, a CosmWasm `Item<u64>::new("order_count")` that counts all
orders must remain global. Classify its raw key with
`StateBinding::Shared.layerx_key("order_count", &[])`, then use this guest
operation under `SharedStorageRead` and `SharedStorageWrite` grants:

```rust
use layerx_program_sdk::{
    storage::{shared, StorageValue}, Field, ProgramError, Reason,
};

fn increment_order_count() -> Result<(), ProgramError> {
    let key = shared::SharedStorageKey::new(b"order_count")?;
    let mut bytes = [0u8; 8];
    let count = match shared::read(key, &mut bytes)? {
        None => 0,
        Some(8) => u64::from_le_bytes(bytes),
        Some(_) => return Err(ProgramError::value(Field::StorageValue, Reason::Malformed)),
    };
    let next = count.checked_add(1)
        .ok_or_else(|| ProgramError::value(Field::StorageValue, Reason::TooLarge))?;
    shared::write(key, StorageValue::new(&next.to_le_bytes())?)
}
```

Sender A writes `1`; sender B reads that same cell and writes `2`. Using
`StateBinding::Item` and principal storage here would instead create two
independent counters and change the contract's meaning. A principal storage
grant does not authorize a shared read or write, and shared read authority
does not authorize a write. Shared visibility grants no application-level
right to change another user's order; enforce that policy in the handler.

State that belongs to the deploying configuration is still best pinned into the
module by the port descriptor, which is what makes the artifact reproducible.
State that must be readable and writable by every account now has a port.

### Values: JSON at the edge, framed bytes inside

The reference keeps JSON parsing in the host-side adapter and migration tool.
Its emitted guest uses framed bytes and fixed memory. This is the kit's chosen
encoding, rather than a claim that every possible deterministic guest must use
it. Clients must transcode their JSON before submitting program calldata.

| JSON type | `cosmwasm_std` | Canonical framing |
| --- | --- | --- |
| number | `u64` | 8 bytes, little-endian |
| decimal string | `Uint128` | 16 bytes, little-endian |
| string | `String`, `Addr` | `u16` little-endian length, then UTF-8 |
| boolean | `bool` | one byte |

`RecordSchema` owns both views of the same declaration: `encode`/`decode` for
the framed bytes the module reads and writes, `encode_json`/`decode_json` for
the document CosmWasm itself would produce or accept, and `transcode` to turn
one straight into the other.

```text
DonationRecord { count: u64 }

  JSON     {"count":3}
  framed   03 00 00 00 00 00 00 00
```

The JSON reader is strict on purpose: unknown fields, repeated fields, missing
fields, a `Uint128` written as a number, and trailing content after the object
are all refused. A migration that silently dropped a field would be a data-loss
bug wearing a success message.

### Migrating existing state

`sender_indexed_import` builds the plan. For each holder it emits a cell naming
the raw key to read out of a state dump and the collapsed key to write, in that
holder's own namespace. `qualify::import_state` performs the writes through the
real storage transaction, transcoding each exported document into the canonical
framing on the way in.

### Scoped iteration and remaining storage differences

`Map::range` and `Map::prefix(..).range` can be translated to bounded prefix
scans. [`storage::scan`](../../sdk/rust/src/storage.rs) reads the current
principal namespace; `storage::shared::scan` reads the program-shared namespace.
Pass a prefix, `ScanCursor::start()`, explicit `ScanLimits`, and a caller-owned
output buffer. Iterate `ScanPage::entries()` and pass its optional `cursor()`
to the next page. Pages contain at most 64 entries; cursors are opaque and at
most 591 bytes. Resume with the same namespace, prefix and limits; the runtime
refuses a cursor with a different scan contract. Scans retain canonical key
order. These operations do not permit cross-principal or cross-program
enumeration. `storage::scan` requires `StorageRead`; `storage::shared::scan`
requires `SharedStorageRead`. Both use ABI v2 `storage_scan_scoped`; callers
select a scope, never an arbitrary program or principal identity.

| Storage or scan quantity | Bound |
| --- | --- |
| Key | 1–256 bytes, including all namespace and composite-key framing |
| Value | 0–1,048,576 bytes |
| Scan prefix | 0–256 bytes; empty scans the selected namespace |
| `ScanLimits::new(max_entries, max_bytes)` | 1–64 entries; 5–67,126,228 complete encoded page bytes |
| Output buffer | at least the requested `max_bytes` in the SDK |
| Opaque continuation | at most 591 bytes; absent cursor means the scan is finished |

The page byte ceiling includes entry framing and the continuation, not just
values. Choose a ceiling that fits an entry plus its continuation; an entry
that cannot fit is refused rather than silently skipped. Handle that refusal
explicitly. Pass the returned cursor unchanged to resume after the last key;
changing scope, program, principal (for a principal scan), prefix or either
limit invalidates it. A shared cursor can continue under another sender of the
same program because the shared namespace has no principal component.
The cursor is a continuation of key order, not a snapshot: intervening writes
can change later pages. Preserve the source key encoding and ordering your
application needs. Reverse ranges, arbitrary end bounds and secondary indexes
need explicit application logic; a CosmWasm range API is not a drop-in call to
the SDK scan API.

### What has no equivalent

Shared aggregates and secondary indexes use explicit shared keys and grants.
Reading another contract's private namespace still requires
that program to expose a callable interface. A contract's retained funds map
to a registered program-derived account, described in **Money**.

## Messages and dispatch

In the donation reference, the three CosmWasm entry points are translated.
`instantiate` is replaced by deployment: the configuration a CosmWasm contract writes on
instantiation is pinned into the module by the port descriptor, because on
LayerX each deployment is its own program. `execute` and `query` variants each
become an export, and the whole set is also reachable through one dispatcher.

```text
{"donate":{"times":3}}   16 bytes   sha256("execute:donate")[..8]    . u64 le
{"donations":{}}          8 bytes   sha256("query:donations")[..8]
{"remaining":{}}          8 bytes   sha256("query:remaining")[..8]
```

A calling program first calls `layerx_reserve(len)` to obtain the region to
write the message into; `layerx_call` refuses any other pointer and any length
that does not match the selected variant. `MessageVariant::transcode` converts
the JSON a client already builds into that input, so the adapter lives at the
edge and not in the program.

Each variant is also exported under its own name, so an activity that invokes
the program directly names the export instead of encoding a tag.

The donation reference's query variants lose their address arguments. A
CosmWasm query has no sender, so
the contract has to be told whose state to read; a ported query is invoked by a
principal the runtime already authenticated, and reads that principal's
namespace. `{"donations":{"donor":"wasm1..."}}` becomes `{"donations":{}}`.
`reference::chain_query_message` still declares the chain's shape, which is what
an adapter drops the argument from.

The classic `layerx_call` operation returns an `i32` code; a negative code is
a refusal. The donation reference keeps counts within its configured cap and
uses this code path. The SDK also exposes the explicitly selected response
operation, `call::invoke_response`, with bounded caller-owned output and
`CallResponse::bytes()`. New response-based ports must use compatible entry
handlers and imports; the classic call does not automatically return bytes.

## Responses and events

A `Response` is not a return value after the port; its two halves separate.
Attributes become a program event, and messages become authenticated transfer
requests or narrowed calls.

```text
Response::new().add_attribute("action", "donate")
  topic   wasm

Event::new("donation").add_attribute("count", total)
  topic   wasm-donation
  data    05 63 6f 75 6e 74   then count, little-endian u64
```

The topic is the chain's own event type verbatim - `wasm` for a bare
`Response`, `wasm-<type>` for a custom `Event` - so an indexer that filters on
it keeps matching. Each attribute is framed as its one-byte key length, the key
verbatim, then the value in canonical framing.

The donation reference maps response attributes to typed event payloads;
indexers must decode this framing rather than expect string-valued CosmWasm
attributes. `set_data` is not represented by its classic code-only dispatcher.
A new port can explicitly use the SDK response operation and publish bounded
bytes instead. `ContractEvent` declares the attribute types used by this kit.

## Sub-messages

```rust
execute_submessage(callee, &variant, &values)?
```

returns the callee, the message in its ported encoding, and **one** narrowed
`Call` capability. The callee reaches only its own namespace, and it holds only
the authority the caller explicitly narrowed.

`execute_submessage` builds the callee, encoded input and `Call` grant; it
does not attach funds. Translate `WasmMsg::Execute { funds, .. }` separately
with `ValueFlow::SubMessageFunds` and the program-derived source account.
The payment leg and call remain part of the same atomic invocation; the
callee's `MessageInfo` does not gain a funds field. See **Money**.

The kit does not translate `ReplyOn` or the `reply` entry point into a reply-ID
callback. A negative callee result refuses the call graph; redesign workflows
that depend on catching a sub-message failure and continuing with partial
state. A successful call's result is consumed synchronously at the call site.

## Money

This is the part of the port that bends least, so read it before you plan one.

A LayerX program **writes no balance**. A contract balance maps to a real
account derived from the LayerX program and a public seed. Contract-funded
messages become bounded owner-frame 402LXP requests from that account, and the
kernel remains the only balance writer.

| CosmWasm | Port |
| --- | --- |
| `info.funds` forwarded on in the same invocation | `ValueFlow::SentFunds` becomes an explicit caller-authorized request; no attached-funds fact is exposed |
| `BankMsg::Send` paid from the contract's balance | `translate_with_program_account` and a rederived contract account |
| `BankMsg::Burn` | **refused**, `SupplyMutation`; a conserved transfer cannot burn supply |
| `WasmMsg::Execute { funds, .. }` | `translate_with_program_account` and a rederived contract account |
| `cw20` `TransferFrom` where the owner is the caller | `ValueFlow::AllowanceSpend` - carried over |
| `cw20` `TransferFrom` on a third party's allowance | `ValueFlow::AllowanceSpend` - **refused**, `DelegatedSpend` |
| `IbcMsg::Transfer` | `ValueFlow::IbcTransfer` - **refused**, `ChainQuery` |

`ValueFlow::translate_with_program_account(asset, principal, owner_program,
seed, source)` returns `TranslatedValueFlow::ProgramAccount` for `BankSend`
and `SubMessageFunds`, or `Principal` for caller-funded flows. The
`ProgramAccountTransferPlan` validates the exact derivation and produces the
owner-bound `ProgramSpend` capability. Invalid source context is
`InvalidProgramAccount`; burning through this path is `SupplyMutation`.

The context-free `translate`, `translate_all` and `portable()` cover only
principal-funded flows. `portable()` is a preliminary classification, not
proof that a particular account or grant is valid; its `false` result for a
contract payout does not mean the derived-account path is unsupported.
Context-free contract payouts return `ContractHeldBalance`.

Denominations do not carry over either: a program is paid in an authenticated
402LXP asset, and the port descriptor names which asset stands in for the
contract's denom. The price is otherwise unchanged - the reference port charges
`price * times` for `times` donations, exactly as the contract charges
`minimal_donation * times`.

The funds check inverts, and this is the single most important sentence in the
guide. On a chain the caller attaches coins and the handler *checks* that
`info.funds` equals what is due. After the port the program *requests* the exact
amount due, and the activity's capability caps what it may request. Nobody can
underpay, nobody can overpay, and the "sent funds do not equal the donation"
branch disappears because the state it guarded against cannot arise.

### Escrow, vaults and withdraw

A vault can accept deposits now and execute `Withdraw` later. Register the
program-derived account with the deployment authority before funding it.
[`PreparedProgramAccount`](../../sdk/rust/src/payments.rs) derives the account
and builds its registration payload, `ProgramDeposit`, funding grant and
`ProgramAccountPayment`. Guest transfer functions are in
[`transfer`](../../sdk/rust/src/transfer.rs).

Use the same program, public seed and asset in all three phases. For
example, this client-side preparation produces the registration payload and
separate bounded grants for a deposit of `30` and a later withdrawal of `5`:

```rust
use layerx_program_sdk::{
    payments::PreparedProgramAccount, AccountId, Amount, AssetId, Bytes,
    Capability, ProgramError, ProgramId,
};

fn prepare_vault(
    program: ProgramId, asset: AssetId, recipient: AccountId,
) -> Result<(Bytes<201>, Capability, Bytes<277>), ProgramError> {
    let vault = PreparedProgramAccount::new(program, b"vault", asset)?;
    Ok((
        vault.registration_payload()?,
        vault.funding_grant(Amount::from_u128(30))?,
        vault.spend_grant(recipient, Amount::from_u128(5))?,
    ))
}
```

Submit the registration payload as a signed `ProgramAccount` registration
with the deployment's registration authority and wait for committed success
before funding. Creating `PreparedProgramAccount` or its payload does not
register or fund anything. The funding grant must be authorized by the payer;
the spend grant must be explicitly authorized for the withdrawal invocation.

In the guest, obtain the owner from authenticated context, then make the
explicit request in the appropriate handler (these are separate invocations):

```rust
use layerx_program_sdk::{
    payments::PreparedProgramAccount, transfer, AccountId, Amount, AssetId,
    Context, ProgramError,
};

fn deposit(asset: AssetId) -> Result<(), ProgramError> {
    let vault = PreparedProgramAccount::new(Context::executing_program()?, b"vault", asset)?;
    transfer::fund_program_account(vault.deposit(Amount::from_u128(30))?)
}

fn withdraw(asset: AssetId, recipient: AccountId) -> Result<(), ProgramError> {
    let vault = PreparedProgramAccount::new(Context::executing_program()?, b"vault", asset)?;
    transfer::pay_from_program_account(vault.payment(recipient, Amount::from_u128(5))?)
}
```

`fund_program_account` calls ABI v2 `fund_program_402`; the kernel validates
registration and asset binding. `pay_from_program_account` calls
`transfer_program_402`. For a host-side `BankMsg::Send` adapter, use
`ValueFlow::BankSend { recipient, amount: 5 }` with
`translate_with_program_account(asset, principal, owner_program, b"vault", source)`
to produce the same exact `ProgramAccountTransferPlan` and its capability.
The runtime enforces cumulative `ProgramSpend` amounts: two requests of `5`
cannot fit a grant capped at `5`. A principal-funded `Transfer402` grant cannot
substitute for this owner-bound grant, nor can another program spend the vault.

Deposits debit the authorized caller. Later payouts run in the owning program's
frame under an explicit `ProgramSpend` grant binding owner program, public
seed, source, asset, recipient and maximum amount. The runtime and kernel
validate the source and limits; constructing a plan alone grants no authority.
The original depositor need not fund the later withdrawal again. Registration
and grants do not guarantee sufficient balance; only successful kernel
settlement moves money. `MessageInfo.funds` remains unavailable: never treat
calldata, a missing funds field, or a locally constructed deposit plan as proof
that coins were attached or paid. Keep claims
and withdrawal permissions in program state and update them atomically with
the payment. Storage accounting alone never moves native money.

The donation reference chooses immediate forwarding to its beneficiary; this
is an example policy, not a runtime restriction on custody. For a funded token
example, see the [EVM guide's LXT-20 section](../evm/MIGRATION.md#erc-20-lxt-20-and-native-assets).

## Failure and gas

| CosmWasm | Runtime behaviour |
| --- | --- |
| `Err(ContractError::Variant)` | `unreachable` |
| a `StdError` from storage, serde or an overflow check | `unreachable` |
| `panic!`, overflow, index out of bounds | `unreachable` |
| an uncaught sub-message failure | `unreachable` |
| out of gas | metered resource refusal |
| contract-call depth limit | declared stack bound exhausted |

Every one of those discards every staged write and every staged effect of the
whole invocation, which is exactly a CosmWasm transaction's all-or-nothing
behaviour. `FailureMapping::outcome` returns the mapping.

The ported module checks every host status and traps unless it is zero, so a
refused write or a refused transfer can never be silently skipped. Note the
difference from a status code: a host function returning a nonzero status is
*not* an error the program may ignore, and `Code::trap_unless_ok` is what the
emitter writes for every `?` in the original handler.

## Context mappings and unavailable constructs

| CosmWasm | Mapping or remaining difference |
| --- | --- |
| `env.block.height` | `layerx_porting_cosmwasm_guest::current()?.0.block_height`, backed by authenticated batch height |
| `env.block.time` | unavailable; use explicit protocol facts or a counted quantity |
| `env.contract.address` | `current()?.0.contract` supplies the executing program ID |
| `deps.api.addr_validate`, `addr_humanize` | explicitly map source addresses to 32-byte identities/accounts; the guest adapter supplies authenticated IDs |
| `deps.querier` bank, smart and staking queries | no general CosmWasm querier adapter; use explicit supported host reads or program calls |
| `migrate` | deployment lifecycle and `UpgradePolicy` govern code upgrades |
| `sudo`, `ibc_*` entry points | no automatic privileged or IBC callback translation |
| `Decimal`, `Decimal256` | implement checked fixed-point integer arithmetic; these types are not floating point |
| floating-point instructions | outside the admitted deterministic integer subset |
| ambient randomness, block-derived entropy | no equivalent context primitive; redesign around explicit authenticated protocol inputs |

## The reference port, line by line

`DonationPort::new(DonationTerms { asset, beneficiary, price, cap })`
takes the translated deployment configuration and pins it into
the module, because on LayerX each deployment is its own program and its
configuration is immutable for that program.

`DonationPort::code()` emits the module. `DonationPort::code_hash()` is the
digest the deployment authenticates.

| Contract line | Emitted program |
| --- | --- |
| `CONFIG.load(deps.storage)?` | nothing; the configuration is pinned into the module |
| `if times == 0 \|\| times > config.donation_cap` | `i64.lt_s 1` and `i64.gt_s` the cap, then `unreachable` |
| `minimal_donation.checked_mul(times)` | `i64.mul` on values already bounded by the cap |
| `if sent != due` | nothing; the program requests `price * times` and the capability caps it |
| `DONATIONS.may_load(deps.storage, &info.sender)?` | `storage_read` at the collapsed key, `0` for an absent cell |
| `.map(\|r\| r.count).unwrap_or_default()` | the absent case answers `0` |
| `held.checked_add(times)` | `i64.add` |
| `if total > config.donation_cap` | `i64.gt_s` the cap, then `unreachable` |
| `DONATIONS.save(..., &DonationRecord { count: total })` | `storage_write` of a little-endian `u64` |
| `BankMsg::Send { to_address: beneficiary, amount }` | `transfer_402(0, price * times, asset, beneficiary)` |
| the `?` on every fallible call | trap unless the status is `0` |
| `Event::new("donation").add_attribute("count", total)` | `event_emit` with `wasm-donation` as the topic |
| `Ok(Response::new()...)` | the export returns `i64` |

Four divergences are deliberate and are the honest translation rather than an
emulation:

- **An absent record reads as zero.** `may_load` returns `None` for a donor who
  has never donated; namespaced storage reports absence, and the query handlers
  answer `0` donations and the full cap remaining.
- **The configuration is not stored.** On a chain the beneficiary, the denom,
  the minimal donation and the cap live in the `config` cell and are read on
  every execution. Here they are pinned into the module by the descriptor,
  which is what makes the artifact reproducible from published source.
  `DonationPort::config_key()` still returns the raw key they occupied, so a
  migration can compare them.
- **The funds check becomes a request.** See **Money**.
- **The query argument disappears.** See **Messages and dispatch**.

The capabilities a donation needs are exactly four: `StorageRead`,
`StorageWrite`, `EmitEvent`, and one `Transfer402` capped at `price * times` to
the configured beneficiary. `DonationPort::donate_capabilities` builds that set,
and a query needs `StorageRead` alone.

## Deploying and verifying a port

[`src/qualify.rs`](src/qualify.rs) runs the library pipeline:

1. `source_archive` assembles the canonical archive: the CosmWasm source, the
   port descriptor, the pinned toolchain manifest and the pinned dependency
   lock.
2. `build_plan` declares the recipe - builder identity, toolchain and lock
   digests, the exact command and the artifact path.
3. `deploy_and_verify` deploys through the real lifecycle, journals the
   deployment as a canonical record, replays that record into the registry,
   then rebuilds the published source in independent hermetic attempts through
   the real `SourceVerifier` and refuses anything short of
   `SourceStatus::Verified`.

The descriptor-driven build is reproducible: `PortBuildRunner` reads the descriptor named by the plan's pinned
command out of the archive, checks the published CosmWasm source is the
contract this kit ports, parses the descriptor and re-emits the module.

The execution helpers `execute_donate`, `execute_donations` and
`execute_remaining` run the metered executor with explicit authorization.
`authorize_transfers` prepares the atomic transfer set; `settle` calls
`PreparedAuthorizedActivity::strict_settle` with a supplied
`KernelTransferPrimitive`. These library phases do not describe separate
network execution and settlement services. On the merged network the node
applies the effects within the chain transaction, and block commit determines
finality. Rebuilding source or running a local helper is not evidence of a live
registered deployment, a successful node transaction or committed receipts.

## Checklist before you port

1. List every `Item` and every `Map` and classify it with `StateBinding`.
   Principal-scoped state collapses; shared state uses the shared namespace.
2. For each `Map`, check whether a sender component denotes the invoking
   principal before collapsing it. Preserve keys for other owners and nested
   caller semantics.
3. Confirm every remaining raw key frames to a key inside the storage bound.
4. List every value-moving statement and run it through `ValueFlow`. Anything
   refused needs an explicit redesign. For retained funds, register the
   derived account and use the context-aware translation and owner grant.
5. Keep the variant names, the field names, the event types and the attribute
   keys. Renaming any of them breaks every existing client and indexer, exactly
   as it does on a chain.
6. Use authenticated context for height and identity. Redesign time and
   external query dependencies, and document their changed semantics.
7. Translate range queries to scoped bounded scans, persisting opaque
   continuations as needed; place global aggregates in shared storage.
8. Publish the source, the descriptor, the toolchain manifest and the lock, and
   verify the deployment reproduces before you announce it.
