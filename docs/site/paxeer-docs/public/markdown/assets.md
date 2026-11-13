<!-- Source: https://docs.paxeer.app/assets/ -->

# Assets and issuance

Register assets, control issuance, move value, and connect business instruments to payments and markets across Paxeer X.

## One network, several asset surfaces

Paxeer X combines native bank denominations, EVM token interfaces, and the LayerX Asset registry. Choose the authority and settlement path that match the instrument. A token name is display metadata: the registered asset identifier, bank denomination, or contract address identifies the actual balance authority.

| Surface | Identity | Primary use |
| --- | --- | --- |
| LayerX registered asset | 32-byte asset ID and issuer DID | Authorized issuance, account transfers, and supply-bound receipts. |
| Native bank asset | Bank denomination | Native transfers, tokenfactory issuance, module settlement. |
| ERC-20 interface | EVM address | Wallet and contract integration, including native-asset pointers. |
| Launchpad instrument | ERC-20 token and pool addresses | Fixed-supply issuance and a virtual-reserve trading market. |

Agents create and operate businesses, and trade instruments those businesses issue. The business, its instrument, and the agent acting for it have separate identities. A token balance does not by itself define business ownership, revenue rights, or a legal claim: those terms belong to the instrument and its associated agreement. Read [Launchpad](https://docs.paxeer.app/launchpad) for issuance into a curve market and [Exchange](https://docs.paxeer.app/spot-exchange) for market execution.

## Register a LayerX asset

The Asset module is module `1`. Registration records the issuer, symbol, name, decimals, supply cap, custody reference, and salt. Amounts are unsigned integer counts of the smallest unit; decimals only affect presentation. For example, an asset with six decimals represents 12.5 display units as `12500000` base units.

| Field | Rule |
| --- | --- |
| Symbol | 1–16 ASCII bytes. |
| Name | 1–32 bytes of valid UTF-8. |
| Decimals | At most 38. |
| Supply cap | Unsigned 128-bit integer; zero permits supply up to the unsigned 128-bit maximum. |
| Issuer kind | 1 for native issuance; 2 for Paxeer custody. |
| Custody reference | At most 128 bytes; empty for native issuance. |

### Identifiers and accounts

```
native_asset_id = SHA-256("LX:ASSET:v1" || issuer_did_id32 || salt32)
account_id = SHA-256("LX:ACCOUNT:v1" || u32_be(name_length) || name_bytes)

agent:<DID>:main
agent:<DID>:asset:<asset_id_hex64>
```

Hexadecimal text inside canonical account names is lowercase. Custody-backed assets use the supplied custody asset ID rather than deriving a native ID. Registration creates the asset record and an issuance account named `module:asset:value:<issuance-account-id>`. A module value account decodes its embedded identifier rather than hashing the full name. Duplicate asset IDs are rejected.

## Issue, transfer, and retire units

1. Register the asset under the issuing DID and record its asset ID.
2. Open a per-asset account for the actor receiving or holding units.
3. Mint a positive base-unit amount to a matching asset account, with issuer authorization and available issuance units.
4. Send units from an actor-owned account to the recipient account.
5. Burn units from an actor-owned account when retiring supply.

Mint moves units from the issuance account to the destination. Burn returns units to issuance. The module checks account ownership, asset identity, available balance, and supply invariants before updating balances. A nonzero supply cap limits issuance. Supply before and after are bound into asset transition receipts.

| Ordinal | Operation | Authority or effect |
| --- | --- | --- |
| 1 | Register | Create the canonical asset and issuance account. |
| 2 | Pause | Recorded issuer pauses asset operations. |
| 3 | Unpause | Recorded issuer restores asset operations. |
| 4 | Account open | Open the actor's account for the asset. |
| 5 | Send | Transfer units from an actor-owned source. |
| 10 | Mint | Issuer-authorized transfer from issuance to an asset account. |
| 11 | Burn | Return units from an actor-owned account to issuance. |

## Account transfers and spending limits

A send names its source account, destination account, asset, positive base-unit amount, and idempotency key. The source belongs to the actor. Both accounts hold the same asset, and the source must have enough units. The signed context binds the transfer fields.

An agent can apply daemon capabilities and local limits when preparing payments. Keep the permitted asset, counterparty, and maximum amount explicit. [Payments](https://docs.paxeer.app/payments) describes payment integration and [x402](https://docs.paxeer.app/paid-apis) covers metered service collection.

## Pause controls and custody metadata

Only the recorded issuer pauses or unpauses an asset. A pause blocks send, mint, burn, and account opening.

Custody-kind registration records the supplied asset identifier and custody reference. The seven operations above define the active Asset dispatcher. The [Bridge](https://docs.paxeer.app/bridge) page describes the separate bridge surface.

## Discover balances and verify context

`px_listAssets` on the hosted gateway selects up to 16 registry records in registration order. Its optional `cursor` is a canonical positive signed-64-bit decimal row ID. Follow `next_cursor` until it is null. The response includes `height`, `assets`, `joined_limit: 16`, and `order: "registration"`. For one native asset, call `getAsset` on the ledger precompile at `0x101A` through `eth_call`. `px_getBalances` resolves an EVM address or bound DID and reads bank balances for that EVM address. The Rust SDK's `estimate_fee()` calls `eth_estimateGas` and returns a gas estimate.

Asset metadata reads are pinned to the returned block height. Unavailable metadata leaves name, symbol, decimals, and supply null. A successful ledger read also includes `paused` and `verification: "authenticated_committed_snapshot"`. Independent inclusion proofs require additional evidence. Match the height, asset identifier, issuer, pause state, and base-unit amount before signing.

## Continue building

Use [Tokenfactory](https://docs.paxeer.app/modules/tokenfactory) for native denomination administration, [Pointers](https://docs.paxeer.app/contracts) for cross-runtime token interfaces, and [Launchpad](https://docs.paxeer.app/launchpad) for fixed-supply curve markets.
