# Perps module

Module ID `6` (`LXP_MODULE_PERPS`). Activity types are `0x0006xxxx`
(`include/layerx/lx_perps.h`). Positions, margin, funding, liquidation, and
insurance are module state. Losses and fees are transfer legs, not shadow
balances.

Oracle prices enter as signed `ORACLE_PUSH` activities. Execution does not
dial out (`src/modules/perps/lx_oracle_intake.c`).

Sources: `include/layerx/lx_perps.h`, `src/modules/perps/lx_perps_dispatch.c`,
`src/modules/perps/lx_perps_command.c`, `src/modules/perps/lx_oracle_intake.c`.

---

## Activities

| Constant | Value | Bytes |
| --- | --- | --- |
| `LX_PERPS_MARKET_CREATE` | `0x00060001` | 622 (`lx_perps_market`) |
| `LX_PERPS_MARKET_HALT` | `0x00060002` | 33 |
| `LX_PERPS_ORACLE_PUSH` | `0x00060003` | 137 on the node transport; 72 for the base command codec |
| `LX_PERPS_ORDER_PLACE` | `0x00060004` | 129 for GTC; 130 with a time-in-force suffix |
| `LX_PERPS_ORDER_CANCEL` | `0x00060005` | 64 |
| `LX_PERPS_POSITION_OPEN` | `0x00060006` | 145 |
| `LX_PERPS_POSITION_INCREASE` | `0x00060007` | 112 |
| `LX_PERPS_POSITION_CLOSE` | `0x00060008` | 64 |
| `LX_PERPS_FUNDING_TICK` | `0x00060009` | 32 |
| `LX_PERPS_LIQUIDATE` | `0x0006000a` | 96 |
| `LX_PERPS_ADL` | `0x0006000b` | 33–4129 |

Unknown ordinals are `LXP_ERR_UNKNOWN_ACTIVITY`. Malformed commands are
`LXP_ERR_NON_CANONICAL` or `LXP_ERR_LENGTH_LIMIT`.

Sides: `LX_PERPS_SIDE_BUY = 1`, `LX_PERPS_SIDE_SELL = 2`.

---

## MARKET_CREATE

The payload is the canonical 622-byte `lx_perps_market` blob
(`lx_perps_market_decode`): market id, quote asset, administrator, liquidity
/ long-funding / short-funding / insurance account ids, contract/tick/lot
sizes, price scale, margin and liquidation basis-point fields, funding
interval, oracle staleness and price bounds, up to eight permitted oracle
keys, parameter version, and halted flag.

A duplicate market is `LXP_ERR_MARKET_ALREADY_EXISTS`. Parameter bounds
failures are `LXP_ERR_PARAMETER_BOUNDS`.

---

## MARKET_HALT

```text
market_id32 || halted:u8
```

---

## ORACLE_PUSH

The base observation encoding (`lx_perps_oracle_command_decode`) is 72 bytes:

```text
market_id32 || observation_sequence:u64 || price:u128
|| observed_at:u64 || source_identifier:u64
```

The node registers `lx_perps_oracle_transport_module_iface`. Its protocol-3
transport uses `lx_perps_oracle_transport_decode` and requires 137 bytes:

```text
version:u8=1 || observation72 || signature64
```

The authenticated authority supplies the oracle public key; the transport does
not carry that key. The command struct holds the resolved key and signature.
Refusals include `LXP_ERR_UNAUTHORIZED_ORACLE`,
`LXP_ERR_ORACLE_SEQUENCE`, `LXP_ERR_ORACLE_BOUNDS`, `LXP_ERR_ORACLE_DEVIATION`,
`LXP_ERR_ORACLE_STALE`, `LXP_ERR_TIMESTAMP_REGRESSION`.

Programs ABI 3 and later expose `layerx_v3.oracle_read`, a read-only host
function returning the committed 64-byte oracle observation. See
[Roadmap: beta surface expansion](Roadmap.md).

---

## ORDER_PLACE

```text
market_id32 || order_id32 || owner_account_id32 || side:u8
|| price:u128 || quantity:u128
```

The 129-byte form means good till cancelled (GTC). A 130-byte form appends
`time_in_force:u8`: `1` immediate or cancel, `2` fill or kill, or `3` post only.
A zero suffix is noncanonical; GTC uses the shorter form.

A halted market is `LXP_ERR_MARKET_HALTED`. Insufficient margin is
`LXP_ERR_MARGIN_INSUFFICIENT`. Zero size is `LXP_ERR_ZERO_AMOUNT`.

---

## ORDER_CANCEL

```text
market_id32 || order_id32
```

---

## POSITION_OPEN

```text
market_id32 || position_id32 || margin_account_id32 || side:u8
|| size:u128 || entry_notional:u128 || margin_amount:u128
```

Opening posts margin from the owner main account into the margin account.

---

## POSITION_INCREASE

```text
market_id32 || position_id32 || size_delta:u128
|| notional_delta:u128 || margin_amount:u128
```

---

## POSITION_CLOSE

```text
market_id32 || position_id32
```

---

## FUNDING_TICK

```text
market_id32
```

Uses the deterministic execution timestamp supplied by the kernel context.

---

## LIQUIDATE

```text
market_id32 || position_id32 || liquidator_account_id32
```

---

## ADL

```text
market_id32 || position_count:u8 || position_id32 × count
```

Position ids must be sorted (`LXP_ERR_UNSORTED_SEQUENCE`). Count is bounded
by `LX_PERPS_ADL_CAPACITY` (128).

---

## Shared refusals

`LXP_ERR_MARKET_HALTED`, `LXP_ERR_UNAUTHORIZED_DEBIT`,
`LXP_ERR_SEQUENCE_REUSED`, `LXP_ERR_AGREEMENT_STATE`,
`LXP_ERR_CONSERVATION`, `LXP_ERR_TOO_MANY_LEGS`,
`LXP_ERR_ACCOUNT_NOT_EMPTY`, `LXP_ERR_NOT_YET_VALID`.

---

## Start here

- [Modules](Modules.md)
- [Fees](Fees.md)
- [Roadmap](Roadmap.md)
