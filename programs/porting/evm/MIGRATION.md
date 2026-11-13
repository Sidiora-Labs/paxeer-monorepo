# Porting a Solidity contract to LayerX programs

This guide is written for someone who knows Solidity and has never written a
LayerX program. It maps the EVM vocabulary you already use onto the programs
ABI, and it is explicit about the constructs that do not carry over,
because the kit refuses those by name rather than emulating them.

LayerX programs execute inside `paxd` on the single Paxeer BFT network. The
kernel is part of the network and proposes blocks; a receipt is final when its
block commits. Public Ethereum JSON-RPC uses the root URL documented in the
[public RPC reference](../../../docs/site/docs/reference/public-rpc.md),
with chain ID 125 (`0x7d`). Program execution and monetary effects belong to the
same chain transaction. Sub-chains are tenant domains in the database.

For the same storage and payment model expressed in CosmWasm terms, see the
[CosmWasm migration guide](../cosmwasm/MIGRATION.md).

## ABI v2 context and precompiles

The guest adapter is [`layerx_porting_evm_guest`](guest/src/lib.rs), separate
from the host-side `semantics` module. `msg_sender()` uses the immediate
calling program in a nested frame and the invoking principal at the root;
`address_this()` uses the executing program. Both return the low 20 bytes of
the respective 32-byte ID. They do not establish an Ethereum-address-to-DID
ownership binding. Use full SDK IDs for authorization and storage isolation.
`block_number()` reads the authenticated `Context::batch_height()`.

The adapter's `keccak256` and `ecrecover` call guest-reachable hash and
secp256k1-recovery imports. `ecrecover` takes a 32-byte digest, a 64-byte
signature and a typed `RecoveryId` (0 through 3), hashes the recovered
uncompressed key without its prefix, and returns its low 20 bytes. Translate
Ethereum `v` encodings explicitly. Invalid input returns `ProgramError`;
this helper does not emulate Solidity's zero-address failure result.
These are guest imports, not EVM calls to a precompile address.

The host-side Keccak implementation computes migration keys, selectors and
topics; use [`SDK crypto`](../../sdk/rust/src/crypto.rs) for dynamic guest
input. [`reference-v2/src/lib.rs`](reference-v2/src/lib.rs) exercises the five
context/crypto mappings. `make programs-porting-v2-references` builds the
locked context references. Check the guest imports, deployment ABI declaration
and [`ABI policy`](../../sdk/rust/src/abi_policy.rs) together; the directory
name does not pin every SDK default to version 2. A guest build alone does not
qualify execution through the node's production host or its committed receipt.

The worked example is `contracts/PublicLock.sol` in the published archive: a
paid-membership lock in the shape Unlock Protocol uses. Its complete port lives
in `src/reference.rs` and is deployed, source-verified and executed by
`src/qualify.rs`.

## What stays byte-identical

Three things survive the port unchanged, and the kit computes all of them with
a real `keccak256`:

| Artifact | Solidity | After the port |
| --- | --- | --- |
| Storage slot address | `keccak256(key . slot)` | the same 32 bytes, used directly as the storage key |
| Event `topic0` | `keccak256("Transfer(address,address,uint256)")` | the same 32 bytes, used as the program event topic |
| Function selector | `bytes4(keccak256("purchase(uint256)"))` | the same 4 bytes, dispatched by `layerx_call` |

These helpers preserve the supported slot, topic and selector bytes. The
caller-indexed collapse below changes the destination key and namespace.
Clients and indexers still need the program dispatch and event framing; this
is not a transparent replacement for an Ethereum contract address.

## Storage

### The namespace replaces `msg.sender` in your keys

An EVM contract owns one flat `uint256 => uint256` array and derives an address
inside it for every composite variable. A LayerX program owns a byte-keyed map
inside a namespace that is `(program, principal)`, and that namespace is fixed
by the runtime *before your code runs*. You cannot choose it, widen it, or read
another principal's cell by choosing a key.

A `mapping(address => V)` indexed only by the authenticated invoking principal
can collapse onto its declared slot. This is the PublicLock reference's
root-call mapping. In a nested call, the guest's `msg_sender()` is a program
while principal-scoped storage still belongs to the original principal; keep
an explicit key if the contract needs that immediate-caller distinction.

```solidity
mapping(address => uint256) public remainingPeriodsOf;   // slot 0
remainingPeriodsOf[msg.sender] = extended;
```

```text
EVM key    keccak256(bytes32(msg.sender) . bytes32(0))    derived at runtime
LayerX key bytes32(0)                                     the slot itself
```

`layout::caller_indexed_key(0)` returns that key. The `keccak256` disappears
from the hot path entirely, because the principal half of the address is
already carried by the namespace.

### Slot derivation for everything else

[`src/layout.rs`](src/layout.rs) provides these word-slot helpers:

| Declaration | Address | Function |
| --- | --- | --- |
| `uint256 x` at slot `n` | `n` | `value_slot(n)` |
| `mapping(K => V)` at slot `n` | `keccak256(k . n)` | `mapping_slot(n, k)` |
| `mapping(K1 => mapping(K2 => V))` | applied outermost first | `nested_mapping_slot(n, [k1, k2])` |
| one-word-per-element dynamic array at slot `n` | `keccak256(n) + i` | `array_slot(n, i)` |
| struct member at offset `m` | `base + m` | `member_slot(base, m)` |

These helpers do not implement every Solidity layout: packed fields and
arrays, dynamic `bytes`/`string`, signed key extension and non-word mapping keys
need their own encoding from the compiler's storage layout. Establish that
layout before importing a dump. The reference values are 32-byte big-endian
words. `storage_key(slot)` is the identity
function on the slot address: LayerX keys are arbitrary bounded byte strings, so
there is nothing to re-derive.

### Shared state

State that is not caller-indexed - a value slot, a constant, or any mapping
that does not collapse onto `msg.sender` - belongs in the program-shared
namespace `(program)` instead of the principal-scoped namespace
`(program, principal)`. Use `shared_key(slot)` to address it and the
`layerx_program_sdk::storage::shared` module to read and write it.

A total supply, pool reserve or order book can use shared keys under explicit
`SharedStorageRead` and `SharedStorageWrite` grants. The program must enforce
its own update permissions. [`src/shared_supply.rs`](src/shared_supply.rs)
demonstrates shared supply alongside principal-scoped holder state. Shared and
principal storage also support bounded prefix scans; see the
[CosmWasm scan mapping](../cosmwasm/MIGRATION.md#scoped-iteration-and-remaining-storage-differences).

### Migrating existing state

`layout::caller_indexed_import(slot, holders)` turns your live mapping into an
import plan: one `MigrationCell` per holder, naming the EVM slot to read from
your dump and the namespaced key to write. `qualify::import_state` applies the
plan, writing each cell into the namespace of the principal that owns it. Each
holder's cell lands in that holder's own namespace; there is no shared cell
anywhere, which is exactly why the mapping key was redundant.

### What has no equivalent

- **`address(this).balance` is not an ambient EVM balance read.** Retained
  value lives in a registered program-derived account. Use the explicit
  account model in **Money** and the SDK's authenticated balance interface
  when a balance observation is required.

## Events

The inherited event shape is one topic plus a data payload:

```text
event_emit(topic_pointer, topic_length, data_pointer, data_length)
```

Every emitted event already carries its program and its invoking principal, so
the port drops the arguments the envelope supplies and keeps the rest:

```solidity
event Transfer(address indexed from, address indexed to, uint256 indexed tokenId);
emit Transfer(address(0), msg.sender, TOKEN_ID);
```

```text
topic  keccak256("Transfer(address,address,uint256)")   unchanged
data   bytes32(TOKEN_ID)                                 32 bytes
```

`from` is the reference's constant mint sentinel and `to` is its root
invoking principal; the adapter reconstructs them from reference semantics and
the envelope, so `EventAbi::envelope_derived(TRANSFER_EVENT, 2)` declares them and
the payload carries only `tokenId`. For an event with nothing derivable, use
`EventAbi::new`, which carries every argument in declaration order.

Solidity's *additional* indexed topics have no programs equivalent: the ABI has
one topic. Indexed arguments become payload words in declaration order.
An indexer filtering on `topic0` is unaffected; an indexer filtering on an
indexed argument reads it out of the payload instead.

## Calling other contracts

`IOther(target).method(args)` becomes `program_call`, and `src/semantics.rs`
builds the request:

```rust
external_call(callee_program, &MethodAbi::new("purchase(uint256)")?, &[Word::from_u64(3)])?
```

The calldata is the EVM's own head-only encoding - the four-byte selector
followed by one 32-byte word per value-typed argument - so the callee's
dispatcher sees exactly the bytes an EVM `CALL` would have delivered.

Two differences you must design for:

1. **Authority is explicit.** The port must supply narrowed program grants
   rather than assume EVM call semantics carry over. A program call carries
   only what the caller explicitly narrows,
   and narrowing can never raise a transfer limit. `CallRequest::authority`
   names the single grant the call needs; the caller must already hold it.
2. **Choose the result operation.** The classic `layerx_call` returns an
   `i32` code; a negative code refuses the whole call graph. PublicLock uses
   that path. The SDK's explicit `call::invoke_response` operation accepts a
   bounded output buffer and returns `CallResponse` with a code and bytes.
   A new port can publish return bytes through compatible SDK entry handlers;
   classic calls do not acquire response bytes automatically.

The ported contract exports both shapes: `layerx_reserve`/`layerx_call` for
program-to-program calls with real calldata, and one named export per function
for direct activity invocation.

## Money

This is the part of the port that is not negotiable.

**No program writes a balance.** Value held for a contract lives in a real
account derived from the LayerX program identifier and a public seed. Guest
code can only request a bounded 402LXP transfer; the kernel rederives the
source, verifies owner-frame `ProgramSpend` authority, applies the whole set
atomically and issues the receipt.

| Solidity | Port |
| --- | --- |
| `payable` function funded by `msg.value` | `transfer_402` debiting the caller in the same invocation |
| `recipient.transfer(msg.value)` (forwarding what just arrived) | `ValueFlow::CallerFunded` - carried over |
| `recipient.transfer(x)` out of accumulated balance | `translate_with_program_account` and a rederived contract account |
| `selfdestruct(recipient)` | **refused**, `UnboundedBalanceSweep`; the source does not declare an exact bounded amount |
| `transferFrom(msg.sender, to, x)` | `ValueFlow::AllowanceSpend` with `owner == caller` - carried over |
| `transferFrom(other, to, x)` under an allowance | `ValueFlow::AllowanceSpend` - **refused**, `DelegatedSpend` |

`ValueFlow::translate_with_program_account` carries contract-funded payouts;
`ValueFlow::translate` remains the context-free principal-only mapping and
returns `ContractHeldBalance` for a contract payout without owner, seed and
source. The context-aware path returns `TranslatedValueFlow::ProgramAccount`
with a validated `ProgramAccountTransferPlan`; its `capability()` builds the
owner-bound `ProgramSpend` grant. Invalid source context returns
`InvalidProgramAccount`. `translate_all` and `portable()` remain
principal-only helpers; they do not classify the context-aware custody path.
Program storage may track claims and token accounting, but writing it alone
never moves native money. For principal-funded transfers, the payer grants a
capped `Transfer402` capability at invocation time. Stored allowances alone
cannot authorize a native debit; the backed LXT-20 model below combines
allowance checks with program-account spending authority.

The amount crosses the integer-only ABI as two `i64` limbs, high first;
`Transfer402Plan::amount_limbs` produces them.

### Escrow, splitting and refunds

Escrow, accumulated balances and later refunds use registered program-derived
accounts. [`PreparedProgramAccount`](../../sdk/rust/src/payments.rs) derives
the source and produces the native registration payload. The deployment's
registration authority submits it before the account is funded.
`ProgramDeposit` and `transfer::fund_program_account` stage a caller-funded
deposit under an explicit funding grant.

A later withdrawal runs in the owning program's frame using
`ProgramAccountPayment` and `transfer::pay_from_program_account`, with a
`ProgramSpend` grant binding the owner, seed, source, asset, recipient and
ceiling. The runtime and kernel validate those fields and apply payments
atomically with state changes. The original depositor need not pay again.
Use `ValueFlow::ContractFunded` with `translate_with_program_account` to plan
this case. A plan does not itself register an account or authorize a
payment.

For example, prepare an account with the contract's program ID, backing asset
and public seed. The following host-side preparation uses the SDK and porting
crate together (`owner` is a runtime `ProgramId`; `asset` and `recipient` are
nonzero 32-byte IDs):

```rust
use layerx_program_sdk::{payments::PreparedProgramAccount, AssetId, ProgramId as SdkProgramId};
use programs_porting_evm::ProgramAccountTransferPlan;

let prepared = PreparedProgramAccount::new(
    SdkProgramId::new(owner.bytes())?, seed, AssetId::new(asset)?,
)?;
let registration = prepared.registration_payload()?;
// Submit registration in a signed native activity under the deployment's
// registration authority, then fund this account in an earlier invocation.
let payout = ProgramAccountTransferPlan::new(
    owner, seed, prepared.account().bytes(), asset, recipient, 40,
)?;
let grant = payout.capability(); // ProgramSpend with maximum_amount = 40
```

In the later invocation, authorize that exact grant and have the owning guest
stage a `ProgramAccountPayment` of 40 with `pay_from_program_account`. The
original depositor need not be present: the source is the registered account,
not the later caller. The application must still authenticate the withdrawal
claim; possession of a plan is not withdrawal permission. A missing grant, a
ceiling of 39, another owner/seed/source, asset or recipient refuses the payout.
The integration test `evm_custody_guide_registered_later_payout` below follows
this path through the committed token guest: the issuer funds first, then an
authorized spender pays 40 from the issuer's program account using an allowance.
It also checks that refused payouts preserve the allowance and balances.

A `PaymentSplitter` can therefore pay accumulated funds in bounded legs from
its derived account, with matching grants, or forward caller-funded legs in
one invocation. Record claims and consumed withdrawals in program state, and
couple each update to its actual payment. An unbounded `selfdestruct` sweep
remains `UnboundedBalanceSweep` on the context-aware path.

## Failure and gas

| Solidity | Runtime behaviour |
| --- | --- |
| `require(cond)` | `unreachable` |
| `revert CustomError()` / `revert("reason")` | `unreachable` |
| `assert(cond)` and compiler panics | `unreachable` |
| out of gas | metered resource refusal |
| call-depth limit | declared stack bound exhausted |

`FailureMapping::outcome` states the mapping. Every one of those outcomes
discards every staged storage write and every effect of the whole invocation,
which is exactly Solidity's all-or-nothing revert. Note the
difference from a status code - a host function returning a nonzero status is
*not* a revert. The port checks every status and traps, which is what
`Code::trap_unless_ok` emits.

Program metering uses deterministic fuel. `Context::remaining_fuel()` exposes
remaining fuel; it is not an EVM gas stipend. A typed payment stages a transfer
without calling recipient code. Audit explicit program calls and authority
separately from payment effects.

## Context mappings and unavailable constructs

| Solidity | Mapping or remaining difference |
| --- | --- |
| `block.number` | `layerx_porting_evm_guest::block_number()`, backed by authenticated batch height |
| `block.timestamp`, `blockhash` | no wall clock or ambient chain view; use explicit protocol facts or counted periods |
| `tx.origin` | the authenticated invoking principal; unlike `msg.sender`, it does not change at nested program frames |
| `address(this).balance` | explicit program-derived account and authenticated balance read; no implicit EVM balance slot |
| `delegatecall` | there is no shared storage frame; every call runs in the callee's own namespace |
| `create`/`create2` | programs are deployed through the lifecycle, not from inside an execution |
| `ecrecover` | `layerx_porting_evm_guest::ecrecover`, backed by typed secp256k1 recovery and guest-reachable Keccak-256 |
| floating point | floating-point types and instructions are refused at validation |
| ambient randomness, `block.prevrandao` | no equivalent context primitive; use explicit authenticated protocol inputs |

`keccak256` inside an ABI-v2 contract calls the guest-reachable primitive. The
historical PublicLock port can still precompute its constant slot addresses,
topics and selectors at migration time; dynamic contract input must never be
hashed by host-side port tooling.

## The reference port, line by line

```solidity
function purchase(uint256 periods) external payable returns (uint256) {
    require(periods > 0);
    require(periods <= maxPeriodsPerPurchase);
    require(msg.value == keyPrice * periods);
    uint256 held = remainingPeriodsOf[msg.sender];
    uint256 extended = held + periods;
    require(extended <= maxPeriodsPerKey);
    remainingPeriodsOf[msg.sender] = extended;
    (bool paid, ) = beneficiary.call{value: msg.value}("");
    require(paid);
    if (held == 0) {
        emit Transfer(address(0), msg.sender, TOKEN_ID);
    }
    emit KeyExtended(TOKEN_ID, extended);
    return extended;
}
```

| Solidity line | Emitted program |
| --- | --- |
| `require(periods > 0)` | `i64.lt_s 1` then `unreachable` |
| `require(periods <= maxPeriodsPerPurchase)` | `i64.gt_s` the bound then `unreachable` |
| `require(msg.value == keyPrice * periods)` | there is no `msg.value`; the program *requests* `keyPrice * periods`, and the capability caps it at exactly that |
| `remainingPeriodsOf[msg.sender]` | `storage_read` at the collapsed key, returning `0` for an absent cell |
| `remainingPeriodsOf[msg.sender] = extended` | `storage_write` of a 32-byte big-endian word |
| `beneficiary.call{value: ...}("")` | `transfer_402(0, keyPrice * periods, asset, beneficiary)` |
| `require(paid)` | trap unless the status is `0` |
| `emit Transfer(...)` on first purchase | `event_emit` with the unchanged `topic0` |
| `emit KeyExtended(...)` | `event_emit` with the token id and new period count |
| `return extended` | the export returns `i64` |

The reference replaces timestamp expiry with a period count because the guest
has no ambient wall clock. This is a deliberate application-level change. `getHasValidKey(address)` became
`getHasValidKey()` in this reference: it reads the invoking principal's
namespace. A port that needs public owner lookups can instead retain owner keys
in shared state and expose an authorized query.

## Deploying and verifying a port

[`src/qualify.rs`](src/qualify.rs) supplies `source_archive`,
`published_source`, `build_plan` and `deploy_and_verify` for `PublicLockPort`.
The caller supplies the publication metadata, lifecycle and registry.

`deploy_and_verify` runs the library pipeline:

1. emit the module and hash it with `SHA-256` - that digest is the code hash the
   deployment activity authenticates;
2. deploy through the lifecycle, which validates the module against the
   deterministic subset before any code becomes callable;
3. journal the deployment as a canonical record and replay it into the registry;
4. rebuild the published archive twice in independent attempts through the
   reproducible-build pipeline, refusing any build whose own output is not
   stable;
5. compare the rebuilt artifact digest with the registered code hash and record
   `SourceStatus::Verified`, or refuse.

The archive carries the Solidity as provenance, the port descriptor as the
build input, and the pinned toolchain and dependency lock the plan commits to by
digest. The descriptor round-trips: `PublicLockPort::parse(&port.encode())` is
the original port, which is why the rebuild is reproducible.

The local `execute_purchase`, `execute_has_valid_key` and
`execute_remaining_periods` helpers run the metered executor. `settle` calls
`strict_settle` with a supplied `KernelTransferPrimitive`. These phases run in
the porting library. In production, the node applies program and monetary
effects in the chain transaction;
committed blocks determine finality. Source verification alone does not prove
account registration, inclusion or successful live execution.

## Checklist before you port

1. Classify every variable as principal-scoped or shared. Only mappings keyed
   by the invoking principal collapse; shared state uses the shared namespace and its dedicated
   capabilities.
2. For retained funds, use `PreparedProgramAccount::registration_payload` and
   the deployment's registration authority, then explicitly fund the account.
   Build each bounded later payout with `ProgramAccountTransferPlan`, authorize
   its exact owner/seed/source/asset/recipient/ceiling `ProgramSpend` grant, and
   enforce the withdrawal claim in the owning frame. The depositor need not
   participate again; an ambient contract balance grants no authority.
3. Does any function depend on time? Replace it with a counted quantity.
4. Does any external call need the callee to act with your authority? Narrow the
   capability explicitly; there is no ambient reach.
5. Choose classic result codes or the bounded response operation explicitly,
   and make the caller and callee use compatible bindings.

## ERC-20, LXT-20 and native assets

The current LXT-20 lifecycle has **eight exports**: `initialize`, `transfer`,
`approve`, `transfer_from`, `balance_of`, `allowance`, `total_supply` and
`metadata`. [`lxt20.rs`](../../sdk/rust/src/lxt20.rs) provides the seven
`Request` variants; initialization has its own exact input, described below.
The [`token-lxt20`](../../sdk/rust/examples/token-lxt20/src/lib.rs) guest and
[`reference_interface`](../../crates/layerx-programs-registry/src/lxt20.rs)
bind all eight exports. All require a direct principal call and refuse a nested
program caller.

The committed [`PAY5 fixtures`](../../fixtures/pay5/) include the real guest,
bound interface, registry state value and signed native activities, receipts
and terminal records. In `native-token`, `01-token-deploy` deploys the guest,
`02-account-register` through `04-account-register` register derived accounts,
and `05-initialize` funds the fixed supply. `06-approve` and `07-approve` prepare a recipient
and allowance; `08-transfer_from` refuses missing spend authority,
`09-transfer_from` refuses a paused backing asset, and `10-transfer_from`
succeeds after unpausing. `11-transfer` returns 15 units, `12` through `15`
query state and metadata, and `16`/`17` revoke and refuse further allowance
spending. These recorded fixtures do not certify a live deployment. The
integration tests in [`tests/repair_22_33_239.rs`](tests/repair_22_33_239.rs)
execute the committed guest in-process using the real SDK codec and the
runtime's authenticated ABI-v5 calls, and compare its
ABI-v2 registry interface to the fixture. They do not run the native C replay
tool or claim to re-execute kernel asset-pause checks.

| ERC-20 flow | LayerX mapping |
| --- | --- |
| Initial supply | `initialize` once, called by the configured issuer with explicit funding authority; no `Request::Initialize` variant |
| `transfer(to, amount)` | LXT-20 `Request::Transfer`; the destination is a 32-byte account ID and amounts are exact u128 units |
| `approve(spender, amount)` | `Request::Approve`; spender is a DID identity ID, and zero explicitly revokes the program allowance |
| `transferFrom(owner, to, amount)` | `Request::TransferFrom`; the reference checks the spender allowance and stages its decrement with the 402 payment |
| `balanceOf`, `allowance`, `totalSupply` | Corresponding request variants; the reference returns its token balances and fixed funded supply, while native balance evidence is a separate read |
| `name`, `symbol`, `decimals` | One `Metadata` request returns configured token metadata; query the native asset registry separately for backing-asset metadata |
| Native value deposit and payout | `PreparedProgramAccount`, `ProgramDeposit` and `ProgramAccountPayment`; register first, then fund and spend under explicit grants |

Each LXT-20 selector is `4c 58 14 method`: initialize=0, transfer=1, approve=2,
transfer_from=3, balance_of=4, allowance=5, total_supply=6 and metadata=7.
It is followed by LayerX convention `01`, bytes tag `20`, a u32 big-endian
payload length, and the method fields in declaration order. Identifiers are
32 bytes and amounts are 16-byte big-endian integers. This is not Solidity ABI
encoding and does not reuse ERC-20 Keccak selectors. See the committed
`programs/sdk/rust/vectors/lxt20-requests.txt` vectors.

Initialize before any funded transfer lifecycle:

1. Derive accounts with `PreparedProgramAccount`, using each owner's DID id32
   as the seed, this program's ID and `REFERENCE_ASSET`. The deployment's
   registration authority submits the native registration activities before
   funding or receiving backing units. An `approve` only records the account
   in token storage; it does not perform native registration.
2. The configured `REFERENCE_ISSUER` (`did:lxp:token-issuer`) calls `initialize`
   with exactly `4c 58 14 00 01 20 00 00 00 00` and shared-storage read/write
   grants plus `Transfer402 { asset: REFERENCE_ASSET, to: issuer_derived_account,
   maximum_amount: REFERENCE_SUPPLY }`. The reference supply is 1,000,000 u128
   units. Initialization stages a `ProgramDeposit` for that complete amount and
   writes issuer balance and shared supply together. It is not a `ProgramSpend`
   payout. Kernel settlement must fund the registered account and commit token
   state atomically.
3. Another issuer, malformed initialization input, a repeated initialization,
   missing funding authority, an insufficient ceiling, a wrong funding asset
   or destination, or missing storage authority refuses without publishing
   storage or payment effects. An interface descriptor alone grants nothing.
4. Before receiving transfers, the recipient calls `approve` at least once
   (zero is sufficient to register in token storage). For spending, supply the
   exact owner-bound `ProgramSpend` grant as well as any token allowance.

Transfers do not mint or burn units. Every `transfer_from` decrements allowance,
including u128::MAX; each payment also respects the configured per-call ceiling.
That ceiling is `REFERENCE_CEILING` (100,000), separate from the initialization
funding amount. Queries and approvals never initialize supply. Responses are
`01 20 || length:u32-be || payload`: mutations return an empty payload, balance,
allowance and supply queries return a 16-byte big-endian u128, and metadata
returns `REFERENCE_METADATA`.

Discovery uses interface encoding v2. Its dynamic-spend descriptor contains the
backing asset, ceiling and calldata offsets for the recipient and amount. Native
admission checks each call against actual caller-authorized ProgramSpend grants;
the descriptor itself grants no authority. `ProgramPaymentCapabilities` combines
funding, spending and ordinary grants in canonical runtime key order.

A program allowance cannot confer kernel debit authority over another DID's
account. A complete implementation needs authenticated ownership, registered
program-derived custody accounts, caller-approved spend capabilities and atomic
allowance/transfer settlement. Direct storage balance edits are not 402 payments.
Infinite uint256 allowances, permits, rebasing and transfer-tax semantics are not
implemented by this request interface. Values above u128 must be refused, not
truncated. Ethereum addresses must be explicitly mapped to identities/accounts.

Native issued asset IDs follow SHA-256 of `LX:ASSET:v1 || issuer_id32 || salt32`.
An actor's per-asset account uses the existing `LX:ACCOUNT:v1` derivation over
`agent:<DID>:asset:<lowercase asset hex>`; `agent:<DID>:main` remains the native
asset account. Program-derived accounts use a different domain and seed rule.
Programs envelopes identify the signer DID and consume its identity sequence.
Funding resolves the signer's canonical backing-asset account and consumes that
account's separate payment sequence; fees may use a separate native account.
Account-bound settlement evidence retains the signer authorization and commits
the actual account endpoints. The Go, Swift, .NET and JVM terminal verifiers
decode and bind the `LayerX/programs/402LXP/account-bound-set/v1` evidence; the
native PAY5 fixture exercises the same encoding end to end.

A transfer leg may name an identity as its destination and pays that identity's
one payment account for the leg's asset. If the identity has no payment account
for that asset, or more than one, the transfer is refused. When at least one
destination is an identity, the account-bound evidence uses
`LayerX/programs/402LXP/account-bound-set/v2\0`, followed by the u32 big-endian
length and bytes of the original authorization, then, for every leg, the u16
big-endian length and bytes of the source name followed by the u16 big-endian
length and bytes of the destination name. Source names follow the version 1
rules; destination names are empty for account-id and asset fee-account
destinations. The runtime derives each named destination account from the
identity, asset and canonical name before committing the account endpoints and
transfer root. Grant and capability checks retain the destination bytes supplied
by the program. A set without an identity destination keeps its version 1 bytes;
a version 2 wrapper with all destination names empty is refused.

Never sign an envelope as an account ID.
