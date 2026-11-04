# Assets and tokens

Paxeer is one network with EVM chain id `125` (`0x7d`). Its kernel executes
inside the node; native ledger operations and bank movements settle in committed
BFT blocks. Amounts are integer counts of base units. `decimals` is display
metadata, never permission to round a payment.

The merged asset path consists of `modules/layerxledger/`, the shared kernel
host in `modules/lxphost/`, and the SDK bank. It does not require a second
custody ledger or a later checkpoint to make an ordinary transfer final.
Sub-chains are tenant domains in the database, not additional settlement chains.

See [Paxeer settlement](paxeer-settlement.md),
[Settlement evidence](../operators/settlement-evidence.md), and
[Agent boundary](../platform/agent-boundary.md).

## Ledger, bank, and pointer views

| Surface | Address | Purpose |
| --- | --- | --- |
| Bank | `0x0000000000000000000000000000000000001001` | Coin balances, metadata, supply and native sends |
| Address association | `0x0000000000000000000000000000000000001004` | Resolve EVM and native account bindings |
| Pointer view | `0x000000000000000000000000000000000000100A` | Look up an existing token pointer |
| Ledger | `0x000000000000000000000000000000000000101A` | Asset records, account ownership, ledger balances and ledger operations |

The served ABIs are `precompiles/bank/Bank.sol`,
`precompiles/pointerview/Pointerview.sol`, and
`precompiles/layerxledger/abi.json`. Pointer lookup
`getNativePointer(string token)` returns `(address addr, uint16 version,
bool exists)`. Check `exists` before using the address. A pointer is a token
interface, not another balance authority or a deposit proof.

A kernel asset's bank mirror denom is `layerx/<asset_id_hex64>` with lowercase
hexadecimal digits (`modules/layerxledger/types/denom.go`). A kernel account's
bank address is derived by `lxphost.BankAddress(accountID)` using the `layerx`
module address domain. A 32-byte account id is not an EVM address; do not truncate
it to obtain a wallet balance.

`Keeper.GetAsset`, `BalanceOf`, `TotalUnits` and `AccountOwner` read the shared
host state. Successful registration and transfer receipts are mirrored into
bank by `modules/layerxledger/keeper/mirror.go`. Registration funds the issuance
account with the cap, or `2^128 - 1` for an uncapped asset. Mint and burn move
units between issuance and holder accounts. Thus bank supply includes issuance
inventory; the ledger's `totalUnits` measures issued units and is not the same
quantity as bank supply.

## Transaction entry points

Use the served ABI and a signed chain transaction. The ledger ABI includes
`register`, `openAccount`, `send`, `payExact`, `pause`, `unpause`, `mint` and
`burn`. The keeper validates actor authority, account ownership, asset identity,
sequence, pause state and supply invariants. The ABI also retains grant methods,
but `IssueGrant` and `RevokeGrant` currently refuse with `ErrNoPayerGrant`
(`modules/layerxledger/keeper/grants.go`); their presence is not availability.

Bank exposes `send(address,address,string,uint256)` and its overload with a
`bytes32 contextHash`. The latter emits `LayerXReceipt(from,to,denom,amount,
contextHash)` after a successful send. Bind that event to the successful
transaction receipt and the committed block when reconciling a payment.

Public chain JSON-RPC is at the root of the numbered node hosts, for example
`https://api1.mainnet-beta.paxeer.network/`. The public node set is
`api1` through `api14.mainnet-beta.paxeer.network` and
`api-mainnet-beta.paxeer.network`. Confirm `eth_chainId == "0x7d"`.
`node.hyperpaxeer.com` is the library mirror, not an RPC endpoint.

Broadcast with `eth_sendRawTransaction`; estimate EVM transaction gas with
`eth_estimateGas`. On the hosted gateway, consult `px_getCapabilities` first:
raw submissions addressed to an unavailable ledger surface are refused with
`-32003` `surface_unavailable`. An empty `eth_getCode` result does not establish
whether a native precompile is active. A broadcast acknowledgement is not
settlement; see [Paxeer settlement](paxeer-settlement.md#receipt-and-commit).

## Hosted asset reads

The `px_` methods below belong to the hosted gateway, not a promise about the
namespaces served by every public node. Their declared contract is
`services/platform/hosted/gateway/openrpc.json`; implementation is in
`src/paxeer.rs` in that crate.

| Method | Positional parameters | Result |
| --- | --- | --- |
| `px_resolveAccount` | EVM address or explicit `did:layerx:<hex64>` | EVM, native and kernel account bindings |
| `px_getBalances` | EVM address or explicit DID | `{account,height,balances:[{denom,amount}]}` |
| `px_listAssets` | None | `{height,assets,joined_limit:16}` |

`px_getBalances` resolves the account and calls bank `all_balances(address)`
at one height obtained from `eth_blockNumber`. Amounts are exact decimal strings
of on-chain 256-bit values. An unbound DID cannot supply the required EVM address
and returns an unavailable error; it is not an empty funded account. This read
lists coins held by the resolved EVM address, not every kernel sub-account.

`px_listAssets` sorts the indexer's asset inventory by `asset_id`, returns at
most 16 entries, and reads their metadata at the returned chain height. It has
no cursor/limit parameters. Bank denoms use bank metadata and supply; ledger
assets use `getAsset` and expose `totalUnits` as `supply`. Successful ledger
reads also include `paused` and
`verification: "authenticated_committed_snapshot"`. An unsupported or failed
metadata read leaves name, symbol, decimals and supply null and omits those two
optional fields. Null metadata must not be interpreted as zero supply.

Invalid parameters return `-32602`; unavailable chain or read dependencies use
`-32001` or `-32005` with the typed error data. These height-pinned answers are
not independent Merkle proofs. For proof verification, use
[Settlement evidence](../operators/settlement-evidence.md).

The old `lx_listAssets`, `lx_getAsset`, `lx_getBalances`, `lx_estimateFee` and
`lx_sendActivity` methods are retired. The gateway rejects all `lx_` methods
with `-32601` `Method not found` before dispatching to a backend.

## Account names and identifiers

The canonical account-name hash is:

```text
account_id32 = SHA-256("LX:ACCOUNT:v1" || u32_be(name_length) || name_bytes)
```

| Purpose | Canonical name or identifier |
| --- | --- |
| DID main account | `agent:<DID>:main` |
| Per-asset account | `agent:<DID>:asset:<asset_id_hex64>` |
| Native asset id | `SHA-256("LX:ASSET:v1" || issuer_did_id32 || salt32)` |

Hexadecimal text in canonical names is lowercase. The issuance record name is
`module:asset:value:<seed_id_hex64>`, where `seed_id` is the account-name hash of
`asset:<asset_id_hex64>:issuance`. Use the owning layer's helper:
`lx_asset_issuance_name` produces that name and seed id;
`lxphost.IssuanceAccountID` derives the host account id from the returned name.
Do not infer one identifier from the other by slicing bytes. Implementations
are `src/ledger/lx_account_id.c` and `modules/lxphost/accountmap.go`.

## Canonical payload and record encodings

These are kernel codec details, not a public activity submission interface.
The merged keeper builds payloads internally in
`modules/layerxledger/types/payload.go`. All integers below are big-endian.

```text
register = version:u16=1 || asset_id32 || salt32
  || symbol_len:u8 || symbol || name_len:u8 || name || decimals:u8
  || supply_cap:u128 || issuer_kind:u8 || custody_ref_len:u8 || custody_ref
open = version:u16=1 || asset_id32
mint_or_burn = version:u16=1 || asset_id32 || account_id32 || amount:u128
```

Registration bounds are symbol 1–16 ASCII bytes, name 1–32 UTF-8 bytes,
`decimals <= 38`, and custody reference at most 128 bytes. Supply cap zero means
uncapped. Native issuer kind `1` requires an empty custody reference and the
derived id; historical custody issuer kind `2` retains its supplied id. That
encoding does not establish that a custody-credit route is enabled.

The version-3 asset record retained by the kernel codec is:

```text
version:u16=3 || asset_id32 || symbol_len:u8 || symbol || decimals:u8
|| custody_kind:u8 || custody_ref_len:u16 || custody_ref || paused:u8
|| name_len:u8 || name || supply_cap:u128 || issuer_did32 || issuer_kind:u8
|| total_units:u128 || salt32
```

Decode using `src/modules/asset/lx_asset_registry.c`; do not treat a record's
metadata or custody reference as evidence of an external deposit.

## Historical payment payloads

The following pre-merge layouts are retained only for decoding archived
artifacts. They do not authorize current grant issuance, receive or submission.

Asset module `1` historically used ordinals `1` register, `2` pause, `3` unpause,
`4` account open, `5` send, `6` receive, `7` grant issue, `8` grant revoke,
`10` mint and `11` burn; ordinal `9` was reserved. The old ten-field receive
payload was 733 bytes:

```text
0x5201 || field_count:u16=10 || from32 || to32 || asset32 || amount:u128
|| grant_id32 || receiver_sequence:u64 || idempotency_key32 || context_hash32
|| receiver_authorization || payer_grant
```

Its receiver authorization was
`kind:u8 || controller32 || public_key32 || signature64 || signed_context_hash32
|| network_id:u32 || protocol_version:u16`. Its 346-byte payer grant was:

```text
grant_id32 || from32 || recipient32 || asset32 || per_draw_maximum:u128
|| allowance:u128 || recurring:u8 || window_length:u64 || expiration:u64
|| purpose_hash32 || has_reference:u8 || reference_hash32
|| revocation_sequence:u64 || public_key32 || signature64
```

The historical authorization domains were `LXP:GRANT:v1` and `LXP:RECEIVE:v1`.
Archive readers must retain the network, recipient, asset, purpose and signed
context bindings. Historical LNI read contracts and LXGB asset metadata are not
current public balance APIs.

### Historical fee schedule v2

The version-2 schedule remains decodable in `src/protocol/lxp_fee.c`. Its
247-byte layout is:

```text
version:u16=2 || base_fee:u128 || per_activity_type_unit:u128
|| per_encoded_byte:u128 || per_execution_unit:u128 || per_storage_unit:u128
|| multiplier_basis_points:u32 || asset_price_count:u8=10
|| ten asset prices:u128
```

The ten price names are `fee.asset.register`, `fee.asset.pause`,
`fee.asset.unpause`, `fee.asset.account_open`, `fee.asset.send`,
`fee.asset.receive`, `fee.asset.grant_issue`, `fee.asset.grant_revoke`,
`fee.asset.mint` and `fee.asset.burn`, in that order. Retained price slots do not
enable retired operations. The codec also supports later schedule versions;
do not decode them with a fixed version-2 length or use this archived schedule
as an EVM transaction fee quote.

[Home](../index.md)
