<!-- Source: https://docs.paxeer.app/spot-exchange/ -->

# Spot Exchange

Trade asset pairs on Paxeer X with deterministic order books, price-time matching, escrowed liquidity, and immediate two-leg settlement.

## Markets and ownership

The LayerX spot module gives each market a base asset, quote asset, tick size, lot size, and administrator. A bid buys the base asset with quote units; an ask sells base units for quote units. Market identifiers, asset identifiers, account identifiers, and order identifiers are 32-byte values. Prices and quantities use unsigned 128-bit integer values: integrations apply the assets' unit conventions before constructing an activity.

Spot runs in the LayerX execution kernel as module `10`. It settles through the asset module and uses the same deterministic price-time book engine as perpetuals. Native spot activities and the EVM exchange intent interface are distinct integration surfaces within Paxeer X. A separate Cosmos Go spot module (`modules/spot`, with swap, vault, shares, stop-order, and TWAP keepers behind `precompiles/spot`) is a third surface; this page covers the native kernel book and the EVM intent interface.

## Choose an order

| Order | Price | Time in force | Remaining quantity |
| --- | --- | --- | --- |
| Limit GTC | Nonzero limit | Good till cancelled | Rests on the book with escrow after matching |
| Limit IOC | Nonzero limit | Immediate or cancel | Does not rest |
| Market | Zero in the payload | Immediate or cancel | Does not rest |

For native spot, `side` is bid or ask, `kind` is `1` for limit and `2` for market, and limit time in force is `1` for GTC or `2` for IOC. A market order executes against available opposing liquidity. Use a limit IOC when the strategy needs an explicit price boundary.

## Matching and fills

The matching engine selects the best opposing price first, the earliest global sequence second, and the order identifier as the final deterministic tie breaker. Every fill uses the resting maker's price. Matching reduces the maker's remaining quantity and escrow; a completely filled maker leaves the book.

### Worked example

An ask offers 5 base units at a price of 100 quote units per base unit. An incoming limit bid for 3 units at 105 crosses that ask. The trade executes at the maker price of 100: the buyer receives 3 base units, the seller receives 300 quote units, and the ask retains 2 units. This example uses whole units for readability; encoded values follow the market's asset-unit, tick, and lot conventions.

## Escrow and settlement

A resting ask escrows its remaining base quantity. A resting bid escrows the quote amount required by its price and remaining quantity. Each fill settles the base transfer and quote transfer immediately as `lxp_transfer_leg` entries with reason `LXP_REASON_ESCROW_CAPTURE` (`5`); locking escrow uses `LXP_REASON_ESCROW_LOCK` (`4`) and refunds use `LXP_REASON_ESCROW_RELEASE` (`6`). Spot keeps no separate shadow balance: asset-module ownership, pause rules, account namespaces, and debit authorization apply to settlement.

After cancellation or execution, reconcile the order's remaining quantity and the asset account state together. An accepted submission identifies an attempted activity; the resulting receipt and state establish the execution outcome. An agent should record its order identifier before submission and retain the corresponding activity and receipt evidence.

## Native activity reference

| Activity | Type | Canonical payload |
| --- | --- | --- |
| Create market | `0x000A0001` | Market, base asset, quote asset, tick size, lot size, administrator |
| Place order | `0x000A0002` | Market, order, base account, quote account, side, kind, time in force, price, quantity |
| Cancel order | `0x000A0003` | Market and order identifiers |
| Halt market | `0x000A0004` | Market identifier |
| Resume market | `0x000A0005` | Market identifier |

```
ORDER_PLACE: 163 bytes
market_id[32] || order_id[32] || base_account_id[32] || quote_account_id[32]
|| side:u8 || kind:u8 || time_in_force:u8 || price:u128be || quantity:u128be

ORDER_CANCEL: 64 bytes
market_id[32] || order_id[32]
```

Market creation uses a 160-byte payload; halt and resume use 32 bytes. Escrow identifiers are derived from the market and asset rather than supplied by the caller. Encode canonical big-endian integers and exact field widths.

## Agent trading workflow

1. Resolve the market's base and quote assets, trading increments, and halted state.
2. Check the strategy's authorized accounts and available asset balances.
3. Choose a price boundary, quantity, order kind, and time in force; allocate and persist an order identifier.
4. Submit the canonical native activity through the LayerX client and retain its activity identifier.
5. Verify the receipt, account changes, fills, and residual order state before updating inventory.
6. Cancel a residual quote using its market and order identifiers. Reconcile the cancellation result before treating its reserved inventory as available for another order.

Market-making agents can quote both sides, cancel stale quotes, and adjust inventory using the same accounts and matching rules as other principals. Strategy decisions remain outside consensus; order authorization, matching, and settlement follow deterministic protocol rules.

## EVM exchange intents

Contracts use `ILayerXExchange` at `0x0000000000000000000000000000000000001015` to deposit or withdraw margin, place or cancel orders, request settlement, and inspect intents. These calls record pending exchange intents for LayerX routing. Their EVM events are submission evidence; LayerX executes the corresponding trading operation.

The interface's state views require a batch number and native state witness. They verify the witness against the batch's finalized anchored state root. Its order interface uses its own time-in-force encoding: `0` GTC, `1` IOC, `2` fill or kill, and `3` post-only. Preserve each surface's encoding when building an adapter; do not copy native spot enum values into this ABI.

## Capacity and operational refusals

The native module supports 64 markets and 256 resting orders per market. One activity produces at most 8 fills and 18 transfer legs. These are compile-time execution bounds: strategies split large operations and reconcile each result rather than assuming unlimited book traversal.

A duplicate market returns `LXP_ERR_MARKET_ALREADY_EXISTS`; order placement on a halted market returns `LXP_ERR_MARKET_HALTED`. Asset mismatch, paused assets, unauthorized debits, unknown account namespaces, sequence reuse, and noncanonical payloads also refuse execution. Correct the underlying account, market, or encoding condition before submitting a new operation.

## Continue building

See [Market Makers](https://docs.paxeer.app/market-makers) for quoting and ramp inventory workflows, [Precompiles](https://docs.paxeer.app/precompiles) for contract integration, and [Modules](https://docs.paxeer.app/modules) for execution capabilities.

**Protocol references:** `include/layerx/lx_spot.h`, `src/modules/spot/lx_spot_market.c`, `src/modules/spot/lx_spot_book.c`, `src/modules/spot/lx_spot_dispatch.c`, and `precompiles/layerxexchange/LayerXExchange.sol`.
