# Spot module

Kernel module ID `10` (`LXP_MODULE_SPOT`) owns activity types `0x000Axxxx`.
It handles two-asset limit and market orders using the shared perps price-time
matching engine and settles both legs of each fill through the asset module.
Its implementation is
[`include/layerx/lx_spot.h`](../../include/layerx/lx_spot.h) and
[`src/modules/spot/`](../../src/modules/spot/).

Spot executes in the integrated kernel inside `paxd`. Paxeer is one BFT network
with EVM chain ID `125` (`0x7d`); its kernel participates in block proposal, and a
receipt is final when its block is committed. See
[Unified Network Architecture](UnifiedNetwork.md).

---

## Activities

| Constant | Value | Role |
| --- | --- | --- |
| `LX_SPOT_MARKET_CREATE` | `0x000A0001` | Create a base/quote market |
| `LX_SPOT_ORDER_PLACE` | `0x000A0002` | Place a limit or market order |
| `LX_SPOT_ORDER_CANCEL` | `0x000A0003` | Cancel a resting order |
| `LX_SPOT_MARKET_HALT` | `0x000A0004` | Halt trading |
| `LX_SPOT_MARKET_RESUME` | `0x000A0005` | Resume trading |

`LX_SPOT_MARKET_CAPACITY` is 64 markets; `LX_SPOT_BOOK_CAPACITY` is 256 resting
orders per market. These compile-time kernel limits are declared in `lx_spot.h`.

---

## Markets

`MARKET_CREATE` has the following 160-byte payload
(`LX_SPOT_MARKET_PAYLOAD_BYTES`):

```text
market_id32 || base_asset32 || quote_asset32 ||
tick_size:u128be || lot_size:u128be || administrator32
```

[`lx_spot_market.c`](../../src/modules/spot/lx_spot_market.c) requires nonzero
identifiers, distinct base/quote assets and nonzero tick/lot sizes. Escrow account
IDs are derived by `lx_spot_escrow_id(market_id, asset_id)` and are not transmitted
in the payload. An existing market refuses creation with
`LXP_ERR_MARKET_ALREADY_EXISTS`.

`MARKET_HALT` and `MARKET_RESUME` take only a 32-byte market ID
(`LX_SPOT_MARKET_ID_PAYLOAD_BYTES`). Creation and halt/resume require the
administrator to match `lxp_authority_resolved.actor`. A new order against a halted
market returns `LXP_ERR_MARKET_HALTED`.

## Orders

`ORDER_PLACE` has the following 163-byte payload
(`LX_SPOT_ORDER_PAYLOAD_BYTES`):

```text
market_id32 || order_id32 || base_account_id32 || quote_account_id32 ||
side:u8 || kind:u8 || time_in_force:u8 || price:u128be || quantity:u128be
```

`side` aliases the perps buy/sell enum as `LX_SPOT_SIDE_BID` and
`LX_SPOT_SIDE_ASK`. `kind` is `1` for limit or `2` for market. Limit orders require
nonzero price and time-in-force `1` GTC or `2` IOC. Market orders require zero
price and IOC. Quantity must be nonzero; order/account IDs must be nonzero, and
the base and quote accounts must differ. Dispatch enforces quantity multiples
of the lot size and limit-price multiples of the tick size.

`ORDER_CANCEL` takes the 64-byte payload `market_id32 || order_id32`.
The debit accounts must belong to the actor and correspond to the market assets;
cancellation checks ownership before releasing the remaining escrow.

`lx_spot_book_match` uses best price, then earliest global sequence, then order
ID. Fills execute at the maker's price. They reduce the maker's remaining
quantity and escrow; fully filled makers leave the book. A resting ask escrows
base units; a resting bid escrows price times remaining quantity in quote units.
One activity is bounded to `LX_SPOT_DISPATCH_MAX_FILLS` (8) fills and
`LX_SPOT_DISPATCH_MAX_LEGS` (18) transfer legs, fitting one transfer set.

## Settlement

[`lx_spot_dispatch.c`](../../src/modules/spot/lx_spot_dispatch.c) emits canonical
transfer sets for both legs of each fill and escrow movements. Spot maintains
order and escrow records while the canonical ledger applies the transfers in
the same activity; it does not maintain an independent balance ledger.

Refusals include `LXP_ERR_ASSET_MISMATCH`, `LXP_ERR_ASSET_PAUSED`,
`LXP_ERR_UNAUTHORIZED_DEBIT`, `LXP_ERR_UNKNOWN_ACCOUNT_NAMESPACE`,
`LXP_ERR_SEQUENCE_REUSED`, `LXP_ERR_NON_CANONICAL`, `LXP_ERR_LENGTH_LIMIT`,
`LXP_ERR_UNKNOWN_FIELD` and `LXP_ERR_UNKNOWN_ACTIVITY`.
[`tests/modules/test_spot_book.c`](../../tests/modules/test_spot_book.c)
(the `test-spot-book` Make target) covers matching and conservation.

### Integrated runtime

[`modules/lxphost/host.go`](../../modules/lxphost/host.go) registers
`lx_spot_module_iface()` with `lxp_kernel_register_module` during host reset.
`Host.HostExecute` enters the registered module's validation/execution hooks
through cgo, uses the SDK context's block time/height, and loads and commits
module state through the host store. The separate daemon and its runtime-binding
procedure are retired; current operation uses the integrated host.

### Native EVM spot routes

The kernel activity payloads above are distinct from the ABI calls served by
[`modules/spot/`](../../modules/spot/) and
[`precompiles/spot/`](../../precompiles/spot/) on the same chain. The native spot
keeper's addresses are declared in
[`modules/spot/types/keys.go`](../../modules/spot/types/keys.go):

| Address | Surface |
| --- | --- |
| `0x0000000000000000000000000000000000001021` | PECOR market and swap rail |
| `0x0000000000000000000000000000000000001022` | Limit orders |
| `0x0000000000000000000000000000000000001023` | Stop-limit and stop-loss orders |
| `0x0000000000000000000000000000000000001024` | Read-only quoter |
| `0x0000000000000000000000000000000000001025` | Liquidity vault |

The served ABI files live in
[`precompiles/spot/abi/`](../../precompiles/spot/abi/), and registration is gated
by the `v6.12.0` activation plan. The kernel's activity capacities and binary
payloads do not define those ABI contracts. Both surfaces execute on Paxeer;
there is no extra settlement network to configure.

---

## Start here

- [Modules](Modules.md)
- [Perps](Perps.md)
- [Asset](Asset.md)
- [Hosted program registry](HostedRegistry.md) — program metadata and source verification
- [Unified Network Architecture](UnifiedNetwork.md)
- [Home](Home.md)
