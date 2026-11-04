# Assets and tokens

Paxeer is one BFT network with the kernel integrated into `paxd`, including
block proposal. EVM chain id is `125` (`0x7d`); a transaction is final when its
block is committed. Native coin balances live in the chain bank ledger.
EVM clients read bank facts at `0x0000000000000000000000000000000000001001`
and discover token pointers at `0x000000000000000000000000000000000000100A`.

This page covers native denominations, token pointers, account identifiers,
and funding. The merged ledger precompile also retains registered Asset
metadata and account operations; its ABI replaces the old public activity
submission instructions.

See also [Public JSON-RPC](PublicRpc.md), [Fees](Fees.md),
[Programs](Programs.md), and [Hosted agent boundary](HostedAgentBoundary.md)
for authenticated receipt reads.

## Amounts and asset facts

Amounts are unsigned integer counts of the asset's smallest unit. Keep them
as integers or decimal strings throughout payment processing; apply a decimal
scale only for display. A denomination, an ERC-20 contract address, and a
32-byte registered Asset id identify different things and must not be
interchanged.

The bank ABI in [`Bank.sol`](../../precompiles/bank/Bank.sol) exposes:

| Read | Result |
| --- | --- |
| `balance(address,string)` | Balance in raw units of the requested denomination |
| `all_balances(address)` | Array of `(uint256 amount, string denom)` coins |
| `name(string)` / `symbol(string)` | Bank denomination metadata; an unknown denomination is refused |
| `decimals(string)` | Currently returns `0`, reflecting integer bank units |
| `supply(string)` | Total bank supply for the denomination |

The `decimals` behavior is implemented in
[`bank.go`](../../precompiles/bank/bank.go). Do not use its zero result as an
ERC-20 display scale. Query the actual token or pointer contract's metadata.
For native EVM value, the launchpad chain profile uses PAX with 18 decimals
and deterministic committed finality
([`profile.go`](../../sdk/launchpad/shared/chain/profile/profile.go)). Bank
base denominations and EVM value units require their own conversions.

Use `eth_call` at a specified committed block when comparing metadata,
balances, and supply. An RPC read is a node observation; it does not by itself
provide an independent state proof.

## Token pointers and registration

[`Pointerview.sol`](../../precompiles/pointerview/Pointerview.sol) declares
these read methods at `0x100A`:

| Method | Token identity |
| --- | --- |
| `getNativePointer(string)` | Bank denomination |
| `getCW20Pointer(string)` | CW20 native contract address |
| `getCW721Pointer(string)` | CW721 native contract address |
| `getCW1155Pointer(string)` | CW1155 native contract address |

Each returns `(address addr, uint16 version, bool exists)`. Check `exists`
before using the address; a pointer is a contract interface to the underlying
asset, not an additional balance or issuance credit.

For a new tokenfactory denomination, submit `MsgCreateDenom` through the
native transaction path. It creates `factory/<creator>/<subdenom>` metadata
and assigns the creator as admin. `MsgMint` and `MsgBurn` enforce denomination
authority. The implementations are in
[`createdenom.go`](../../modules/tokenfactory/keeper/createdenom.go) and
[`msg_server.go`](../../modules/tokenfactory/keeper/msg_server.go).

For an existing native denomination, the pointer precompile at
`0x000000000000000000000000000000000000100b` exposes
`addNativePointer(string)`. It requires stored bank metadata; without it,
the call refuses and reports that the pointer can only be set through a
governance proposal. Creating a pointer does not mint tokens. The same ABI
includes `addCW20Pointer`, `addCW721Pointer`, and `addCW1155Pointer`; see
[`Pointer.sol`](../../precompiles/pointer/Pointer.sol) and
[`pointer.go`](../../precompiles/pointer/pointer.go).

## Account names and identifiers

Registered ledger accounts retain deterministic names. For agent accounts:

```text
account_id32 = SHA-256("LX:ACCOUNT:v1" || u32_be(name_length) || name_bytes)
```

The length is a byte count and the integer is big-endian. Hexadecimal text
inside canonical names is lowercase.

| Purpose | Canonical name or identifier |
| --- | --- |
| Main account for a DID | `agent:<DID>:main` |
| Account for one registered Asset | `agent:<DID>:asset:<asset_id_hex64>` |
| Native registered Asset id | `SHA-256("LX:ASSET:v1" || issuer_did_id32 || salt32)` |
| Bank denomination for a registered Asset | `layerx/<asset_id_hex64>` |

The DID rendering is `did:layerx:<64 lowercase hex public-key characters>`.
Use the derivation functions in
[`accountmap.go`](../../modules/lxphost/accountmap.go) and
[`layerx_binding.go`](../../modules/evm/types/layerx_binding.go).
`BankAddress(account_id32)` derives `address.Module("layerx", account_id32)`;
a 32-byte account id is not an EVM address and must not be truncated to one.
The denomination mapping is defined in
[`denom.go`](../../modules/layerxledger/types/denom.go).

Issuance uses the stored name
`module:asset:value:<issuance_account_id_hex64>`. Its id preserves the hash
of `asset:<asset_id_hex64>:issuance`; the module-value name is interpreted
by the account codec, rather than hashed as an ordinary agent name. See
[`lx_account_id.c`](../../src/ledger/lx_account_id.c).

## Registered Assets and issuance

For registered ledger Assets, use `layerxledger` at
`0x000000000000000000000000000000000000101A`. Its current
[`abi.json`](../../precompiles/layerxledger/abi.json) includes:

| Operation | ABI entry point |
| --- | --- |
| Register | `register(bytes32 assetId, bytes32 salt, string symbol, string name, uint8 decimals, uint128 supplyCap, uint8 issuerKind, bytes custodyReference)` |
| Open an Asset account | `openAccount(bytes32 assetId)` |
| Mint / burn | `mint(bytes32 assetId, bytes32 accountId, uint128 amount)` / `burn(bytes32 assetId, bytes32 accountId, uint128 amount)` |
| Pause / unpause | `pause(bytes32 assetId)` / `unpause(bytes32 assetId)` |
| Read | `getAsset(bytes32)`, `balanceOf(bytes32 accountId,bytes32 assetId)`, `totalUnits(bytes32)`, `accountId(bytes32 owner,bytes32 assetId)`, `accountOwner(bytes32)` |

State changes are nonpayable EVM calls. The precompile resolves the caller's
bound identity and invokes the keeper; it refuses state changes through
static calls and refuses delegate calls. Registration validates a symbol of
1–16 ASCII bytes, a UTF-8 name of 1–32 bytes, decimals at most 38, and a
`uint128` cap. Zero cap means uncapped. Native issuer kind `1` requires the
derived Asset id and an empty custody reference. The codec retains custody
issuer kind `2` and a bounded reference as metadata; this is not a deposit or
custody-credit funding API.

Registration creates the Asset record and issuance account. The keeper
creates its bank supply under `layerx/<asset_id_hex64>` and places it in the
issuance account; an uncapped record uses `2^128 - 1` issuance units. Mint
moves units from issuance to a destination account; burn returns them.
These operations update kernel and bank transfers together, enforcing issuer,
ownership, pause, balance, and supply rules. See
[`assets.go`](../../precompiles/layerxledger/assets.go),
[`mirror.go`](../../modules/layerxledger/keeper/mirror.go), and
[`supply.go`](../../modules/layerxledger/keeper/supply.go).

## Transfers and derived subaccount funding

Use a native bank `MsgSend` to fund a derived bank subaccount with the correct
denomination and raw amount. Derive its native address with the owning
account codec and verify the destination before signing. For EVM wallets,
transfer a native denomination through its registered ERC-20 pointer. The
bank `send(address,address,string,uint256)` entry point is restricted to
that denomination's pointer contract; calling it directly from a wallet
fails authorization. The five-argument overload additionally accepts a
`bytes32 contextHash` and emits `LayerXReceipt` for a nonzero transfer when
the merge plan is active.

For native value sent to a Bech32 recipient, bank `sendNative(string)` is
payable and requires nonzero transaction value and a mapped caller address.
It refuses static and delegate calls. After merge activation, a zero value
is `LXP_BANK_ZERO_VALUE`; an invalid address is
`LXP_PRECOMPILE_INVALID_ADDR`. The implementation is
[`bank.go`](../../precompiles/bank/bank.go).

Funding a DID's bound EVM wallet is a different destination from a ledger
module account. The hosted funding producer resolves the binding at address
precompile `0x1004`, prepares a value-bearing EIP-1559 transaction with empty
calldata, persists its identity, and submits the signed bytes. Its
confirmation checks the transaction hash, sender, recipient, status, and
nonzero block identity. See
[`chain_fund.rs`](../../services/platform/hosted/core/src/chain_fund.rs) and
[`funding_native.rs`](../../services/platform/hosted/core/src/funding_native.rs).
Do not treat a wallet top-up as registered Asset issuance.

## Fees, submission, and receipts

Public node JSON-RPC is at the root of
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`, plus
`https://api-mainnet-beta.paxeer.network`. Check `eth_chainId == "0x7d"` before
signing. Submit signed EVM transactions with `eth_sendRawTransaction`; query
the same hash with `eth_getTransactionReceipt`. Require a successful receipt
in a committed block before crediting a transfer, and check the expected
recipient, amount, denomination or token, and emitted events as appropriate.
On an uncertain response, retain the signed bytes and hash and reconcile
that transaction before creating another payment.

Estimate EVM gas with `eth_estimateGas` and obtain fee inputs from the node.
Programs expose their own metering and fee schedule views; the old Asset
activity ordinal prices and standalone genesis fee blob are retired as
public transaction instructions. Hosted `px_` methods have a separate
application contract in
[`openrpc.json`](../../services/platform/hosted/gateway/openrpc.json).
The [Hosted agent boundary](HostedAgentBoundary.md) serves authenticated
signed-receipt reads by transaction hash; it does not submit payments.

[Home](Home.md)
