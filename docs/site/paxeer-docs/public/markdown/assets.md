<!-- Source: https://docs.paxeer.app/assets/ -->

# Assets and issuance

Register assets, control issuance, move value, and connect business instruments to payments and markets across Paxeer X.

## One network, several asset surfaces

Paxeer X combines native bank denominations, EVM token interfaces, and the LayerX Asset registry. Choose the authority and settlement path that match the instrument. A token name is display metadata: the registered asset identifier, bank denomination, or contract address identifies the actual balance authority.

| Surface | Identity | Primary use |
| --- | --- | --- |
| LayerX registered asset | 32-byte asset ID and issuer DID | Authorized issuance, agent payments, payer grants, supply-bound receipts. |
| Native bank asset | Bank denomination | Native transfers, tokenfactory issuance, module settlement. |
| ERC-20 interface | EVM address | Wallet and contract integration, including native-asset pointers. |
| Launchpad instrument | ERC-20 token and pool addresses; denomination `launchpad/<pool hex>` | Fixed-supply issuance and a virtual-reserve trading market. |

Agents create and operate businesses, and trade instruments those businesses issue. The business, its instrument, and the agent acting for it have separate identities. A token balance does not by itself define business ownership, revenue rights, or a legal claim: those terms belong to the instrument and its associated agreement. Read [Launchpad](https://docs.paxeer.app/launchpad) for issuance into a curve market and [Exchange](https://docs.paxeer.app/spot-exchange) for market execution.

## Register a LayerX asset

The Asset module is module `1`. Registration records the issuer, symbol, name, decimals, supply cap, custody reference, and salt. Amounts are unsigned integer counts of the smallest unit; decimals only affect presentation. For example, an asset with six decimals represents 12.5 display units as `12500000` base units.

| Field | Rule |
| --- | --- |
| Symbol | 1–16 ASCII bytes. |
| Name | 1–32 bytes of valid UTF-8. |
| Decimals | At most 38. |
| Supply cap | Unsigned 128-bit integer; zero means uncapped. |
| Issuer kind | 1 for native issuance; 2 for Paxeer custody. |
| Custody reference | At most 128 bytes; empty for native issuance. |

### Identifiers and accounts

```
native_asset_id = SHA-256("LX:ASSET:v1" || issuer_did_id32 || salt32)
account_id = SHA-256("LX:ACCOUNT:v1" || u32_be(name_length) || name_bytes)

agent:<DID>:main
agent:<DID>:asset:<asset_id_hex64>
```

Hexadecimal text inside canonical account names is lowercase. Custody-backed assets use the supplied custody asset ID rather than deriving a native ID. Registration creates the asset record and an issuance account. Duplicate asset IDs are rejected.

## Issue, transfer, and retire units

1. Register the asset under the issuing DID and record its asset ID.
2. Open a per-asset account for the actor receiving or holding units.
3. Mint a positive base-unit amount to a matching asset account, with issuer authorization and available issuance units.
4. Send units directly or authorize a recipient to draw against a scoped payer grant.
5. Burn units from an actor-owned account when retiring supply.

Mint moves units from the issuance account to the destination. Burn returns units to issuance. The module checks account ownership, asset identity, available balance, and supply invariants before updating balances. A nonzero supply cap limits issuance. Supply before and after are bound into asset transition receipts, with withdrawal handled through its separate settlement path.

| Ordinal | Operation | Authority or effect |
| --- | --- | --- |
| 1 | Register | Create the canonical asset and issuance account. |
| 2 / 3 | Pause / unpause | Recorded issuer controls the asset pause state. |
| 4 | Account open | Open the actor's account for the asset. |
| 5 / 6 | Send / receive | Owner-authorized send or grant-authorized recipient draw. |
| 7 / 8 | Grant issue / revoke | Create or revoke a narrowly scoped payer grant. |
| 9 | Withdraw | `LX_ASSET_WITHDRAW` (`0x00010009`): custody withdrawal through the settlement path. |
| 10 / 11 | Mint / burn | Issuer-controlled mint; actor-owned source for burn. |

## Scoped payments and recurring draws

A payer grant names one recipient, one payer account, one asset, per-draw maximum, total allowance, expiry, and purpose. Recurring grants also bind a window length. Wildcard recipients and accounts are excluded. Receive verifies the grant, receiver authorization, purpose, network, signed context, and revocation sequence together.

A business can authorize a service agent to collect metered fees without handing it unrestricted wallet authority. Keep the service purpose and maximum draw explicit; revoke the grant when the relationship ends. [Payments](https://docs.paxeer.app/payments) explains direct transfers and [x402](https://docs.paxeer.app/paid-apis) covers metered service collection.

## Pause and custody settlement

Only the recorded issuer pauses or unpauses an asset. A pause blocks send, mint, burn, account opening, grant issuance, and withdrawal. Revocation remains a distinct operation, allowing authorization to be withdrawn without creating a new payment.

Custody withdrawal debits the asset account into the withdrawal settlement account and binds a payout recipient, checkpoint ID, amount, and fee. Paxeer-side payout uses a claim against a finalized checkpoint. Use the [Bridge](https://docs.paxeer.app/bridge) workflow for that claim; an ordinary asset transfer does not release custody funds.

## Discover balances and verify context

`px_listAssets` lists registry records in ascending asset-ID order. Supply an optional exclusive `cursor` and a `limit` from 1 to 256; the default is 64. Follow `next_cursor` until it is null. There is no single-asset wire method: the SDK's `get_asset()` scans `px_listAssets` pages for the requested ID. `px_getBalances` reads DID accounts. Fee estimation (`estimate_fee()`) calls `eth_estimateGas` on an EIP-1559 transaction; it does not price canonical activities over a dedicated method.

Reads carry an observed sequence and committed state root. The label `authenticated_committed_snapshot` identifies an authenticated committed snapshot; independent inclusion proofs and finality are separate verification concerns. Match the snapshot, asset identifier, issuer, pause state, and base-unit amount before signing.

## Continue building

Use [Tokenfactory](https://docs.paxeer.app/modules/tokenfactory) for native denomination administration, [Pointers](https://docs.paxeer.app/contracts) for cross-runtime token interfaces, and [Launchpad](https://docs.paxeer.app/launchpad) for fixed-supply curve markets.
