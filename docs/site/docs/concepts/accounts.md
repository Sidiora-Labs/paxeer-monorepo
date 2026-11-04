# Accounts

Paxeer mainnet-beta is one BFT network, with EVM chain ID `125` (`0x7d`).
The kernel runs inside `paxd` and proposes blocks. Account state and transfers
are committed on that network; a receipt is final when its block is committed.

## Native accounts and balances

The Cosmos auth account carries its address, public key, account number and
sequence (`sdk/x/auth/types/account.go`). Balances are separate bank state,
indexed by address and denomination (`sdk/x/bank/keeper`). A new asset balance
does not create a new signer sequence. Module accounts carry permissions;
a name alone does not confer permission to debit funds.

The EVM keeper resolves EVM addresses to Cosmos addresses through its stored
address mappings. `GetPaxAddressOrDefault` uses the mapping when present and
otherwise casts the EVM address bytes to a Cosmos account address. The reverse
helper, `GetEVMAddressOrDefault`, likewise prefers the stored mapping
(`modules/evm/keeper/address.go`). Do not assume that two textual address forms
identify independent balances, or that casting replaces an existing mapping.

### Balance units

Bank queries return integer amounts in the requested denomination. For the
native `uhpx` denomination, one unit corresponds to `10^12` wei. EVM reads
expose 18-decimal native value. The conversion in
`modules/evm/keeper/balance.go` is:

```text
EVM balance = (bank base-denomination balance - locked base-denomination coins)
              * 10^12 + bank wei remainder
```

The bank stores the sub-`uhpx` remainder separately. `AddWei`, `SubWei` and
`SplitUhpxWeiAmount` in `sdk/x/bank/keeper/send.go` maintain the split. Keep
amounts as integers and preserve the denomination or unit when passing them
between APIs. An ERC-20 balance is read from its contract and is not this
native balance. See [Assets](assets.md) for asset-specific material.

## Identity binding and hosted selectors

The address precompile at `0x0000000000000000000000000000000000001004`
implements `getEvmAddrByLayerX` and `getUnifiedAccount`
(`precompiles/addr/addr.go`). A LayerX DID has the form
`did:layerx:<32-byte public key in hex>`. Use the binding returned by the
precompile to find its EVM address; the DID key is not an EVM address.

The hosted gateway's `px_getAccount` resolves identity through that precompile
and reads `eth_getBalance` and `eth_getTransactionCount` at a selected block
height (`services/platform/hosted/gateway/src/paxeer.rs`). Its response joins
`account` identity information with native `paxeer` balance, nonce and height.
A DID without an EVM binding has no native EVM balance in that join; a missing
binding must not be treated as a zero balance.

The [Hosted core](../platform/hosted-core.md) account routes accept either a
`0x`-prefixed 20-byte EVM address or an explicit `did:layerx:` selector with a
nonzero 32-byte hex key. They reject bare 32-byte account hashes with
`400 unsupported_account_selector`. A bare hash does not supply enough
information to reverse-resolve an EVM address.

## Namespace and identifier hash

Named identifiers remain useful for LayerX records and identity joins. For
example, the gateway validates the main-account commitment against
`agent:did:layerx:<lowercase public-key hex>:main`. Its DID account-list validator
also recognizes the `asset`, `budget`, `escrow`, `stream` and `margin` suffixes
under that DID (`services/platform/hosted/gateway/src/paxeer.rs`). These labels
describe records; they do not establish an independent bank balance or a
universal set of native bank accounts.

For the protocol-3 named-account commitment:

```text
account_id32 = SHA-256("LX:ACCOUNT:v1" || u32_be(name_length) || name_bytes)
```

The gateway checks canonical names and rejects a mismatched main-account
commitment or conflicting forward and reverse identity bindings. Preserve the
full name and asset context when using these identifiers. Do not substitute a
named-account hash for a Cosmos address or EVM address. Module-specific
accounts and authorization belong to the relevant [Modules](../protocol/modules.md).

## Sequences

| Operation or read | Counter source |
| --- | --- |
| Cosmos signed transaction | Auth account sequence, alongside its account number |
| EVM transaction | EVM nonce stored by address in `modules/evm/keeper/nonce.go` |
| `eth_getTransactionCount(address, height)` | EVM nonce at that block |
| `eth_getTransactionCount(address, "pending")` | Pending EVM nonce for transaction preparation |
| Hosted core account `next_sequence` | EVM nonce checked at the account snapshot height |
| Hosted core DID sequence `next_sequence` | Pending nonce of the DID's bound EVM address |

The hosted DID sequence response labels its source `sequencer_pending_nonce`;
that compatibility label does not mean a separate identity counter. Do not
infer sequence changes from an incoming credit, an asset identifier or a
named subaccount. Read the counter required by the transaction being signed.

## Consistent hosted reads

Hosted core compares the gateway join with balance and nonce reads at a fixed
height and checks that the chain ID and head did not change during the read.
It returns decimal strings for `balance`, `height` and `next_sequence`, with
`balance_unit: "wei"` and `balance_decimals: 18`. `account_snapshot_changed`
is a retryable `503`; conflicting snapshot values produce
`502 account_snapshot_mismatch`. See [Hosted core](../platform/hosted-core.md)
for routes and refusal handling, and [Public RPC endpoints](../reference/public-rpc.md)
for direct node access.
