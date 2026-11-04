<!--
order: 1
-->

# State

Paxeer's `x/bank` keeps committed coin balances, supply, denomination metadata,
fractional native balances, and denomination allowlists. It also uses a separate
memory store for deferred module credits. The key definitions are in
[types/key.go](../types/key.go); `|` below means byte concatenation, and `denom`
means the denomination's raw string bytes.

## Committed stores

These entries live under the `bank` store key:

| State | Key | Value |
| --- | --- | --- |
| Supply | `0x00 \| denom` | Marshaled `sdk.Int` amount |
| Denom metadata | `0x01 \| denom \| denom` | Protobuf `types.Metadata` |
| Account balance | `0x02 \| address-length \| address \| denom` | Protobuf `sdk.Coin` |
| Fractional wei balance | `0x04 \| address` | Marshaled `sdk.Int` amount |
| Denom allowlist | `0x11 \| denom \| denom` | Protobuf `types.AllowList` |

Account balance keys use `address.MustLengthPrefix`, with a one-byte address
length followed by raw address bytes. Wei keys use raw addresses without that
length prefix. The repeated denomination in metadata and allowlist keys reflects
the current implementation: it constructs a prefix containing `denom` and then
stores under another `denom` key within that prefix.

Zero coin balances, wei balances, and supply entries are deleted rather than
stored. A missing balance reads as zero. Bank parameters, including the default
and per-denomination send-enabled policy, are accessed through the keeper's
parameter subspace rather than the prefixes above.

## Fractional native balances

[keeper/send.go](../keeper/send.go) defines `OneUhpxInWei = 1_000_000_000_000`:
one `uhpx` is `10^12` wei. Ordinary coin balances represent whole base units;
`GetWeiBalance` reads the additional fractional component. For the native token,
the account's combined amount in wei is:

```text
GetBalance(address, "uhpx").Amount * 10^12 + GetWeiBalance(address)
```

`AddWei` carries whole `uhpx` into the coin balance; `SubWei` borrows from the coin
balance when necessary. These paths keep the remaining wei component below
`10^12`. Ordinary balance enumeration does not include that component; use
`GetWeiBalance` or `IterateAllWeiBalances` as well.

## Denomination allowlists and recovery

`SetDenomAllowList` persists `types.AllowList.Addresses`. The message handlers apply
this policy to denomination strings beginning with `factory`. A missing or empty
allowlist is unrestricted. For a non-empty list, both senders and recipients must
be listed for their respective coins. This policy is separate from send-enabled
parameters and the recipient blocklist; see [Messages](03_messages.md).

[keeper/genesis.go](../keeper/genesis.go) imports and exports ordinary balances,
wei balances, supply, metadata, and parameters. Initialization includes aggregate
wei in native supply and rejects a non-zero aggregate remainder modulo `10^12`.

Allowlists need the extended `keeper.GenesisState` in
[keeper/genesis_allowlists.go](../keeper/genesis_allowlists.go), with
`denom_allow_lists_version = 1` and `denom_allow_lists`. The
`ExportGenesisWithAllowLists` / `InitGenesisWithAllowLists` pair and
`MarshalGenesisJSON` / `UnmarshalGenesisJSON` helpers preserve those policies and
validate denominations, unique entries, and canonical addresses.

The bank module's current `InitGenesis` and `ExportGenesis` hooks still use
`types.GenesisState`. The legacy keeper export panics if any allowlist entries
exist, rather than silently dropping them. A restart from committed state retains
allowlists, but recovery through the normal module JSON genesis path is not a
complete allowlist round trip. Recovery tooling must explicitly use the extended
helpers; it must not treat an export without policy data as equivalent state.

## Deferred module balances

The node mounts `types.DeferredCacheStoreKey` (`deferredcache`) as a **memory**
store, separate from committed `bank` KV state. Its entries have the layout:

```text
0x03 | module-address-length | module-address | uint64-big-endian(tx-index) | denom
  -> Protobuf(sdk.Coin)
```

`DeferredSendCoinsFromAccountToModule` debits the sender's unlocked coins
immediately in the transaction context and records a pending module credit by
module, transaction index, and denomination. It requires a keeper constructed
with a deferred cache, an existing module account, and a non-negative transaction
index. The cache can contain several entries for the same module and denomination
under different transaction indices.

In [node/app.go](../../../../node/app.go), `CompleteBlock` flushes giga stores when
enabled, then calls `BankKeeper.WriteDeferredBalances` before the block's state is
committed. That method totals credits per module, sorts module addresses for a
deterministic write order, credits committed-bank balances in the block context,
returns the resulting events, and clears the deferred cache.

Deferred entries are pending work for the current block, not an independent
recovery ledger or a second supply. They are not exported in bank genesis or
restored as committed state after restart; their credits must be applied through
the block completion path before commit. See [Keepers](02_keepers.md) for the APIs
that read or modify each kind of balance.
