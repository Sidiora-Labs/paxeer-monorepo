# Escrow module

Module ID `2` (`LXP_MODULE_ESCROW`). Activity types are `0x0002xxxx`
(`include/layerx/lx_escrow.h`). Terms live in module state; money moves only
as `402LXP` legs into and out of `agent:<did>:escrow:<id>`.

Sources: `include/layerx/lx_escrow.h`, `src/modules/escrow/lx_escrow_module.c`,
`src/modules/escrow/lx_escrow_timeout.c`.

---

## Activities

| Constant | Value | Payload bytes |
| --- | --- | ---: |
| `LX_ESCROW_OPEN` | `0x00020001` | 288 |
| `LX_ESCROW_CAPTURE` | `0x00020002` | 80 |
| `LX_ESCROW_PARTIAL_CAPTURE` | `0x00020003` | 80 |
| `LX_ESCROW_RELEASE` | `0x00020004` | 64 |
| `LX_ESCROW_TIMEOUT` | `0x00020005` | 64 |
| `LX_ESCROW_DISPUTE_OPEN` | `0x00020006` | 32 |
| `LX_ESCROW_DISPUTE_RESOLVE` | `0x00020007` | 68 |

Record states (`lx_escrow_status`): OPEN, PARTIALLY_CAPTURED, CAPTURED,
RELEASED, DISPUTED, RESOLVED, TIMED_OUT.

Unknown ordinals are `LXP_ERR_UNKNOWN_ACTIVITY`.

---

## OPEN

```text
escrow_id32 || owner32 || escrow_account32 || beneficiary32 || arbiter32
|| asset_id32 || amount:u128 || expiry:u64 || dispute_window:u64
|| terms_hash32 || agreement_reference32
```

Validate refuses a zero amount (`LXP_ERR_ZERO_AMOUNT`). Execute refusals
include `LXP_ERR_UNAUTHORIZED_DEBIT`, `LXP_ERR_ESCROW_STATE` for an existing
escrow ID, and transfer-set codes.

---

## CAPTURE / PARTIAL_CAPTURE

```text
escrow_id32 || amount:u128 || idempotency_key32
```

Partial capture also refuses a zero amount. Capture refusals include
`LXP_ERR_ESCROW_STATE`, `LXP_ERR_HOLD_EXPIRED`, `LXP_ERR_HOLD_DISPUTED`,
`LXP_ERR_UNAUTHORIZED_CAPTURE`, `LXP_ERR_CAPTURE_EXCEEDS_HOLD`,
`LXP_ERR_UNAUTHORIZED_ESCROW_SPEND`, and `LXP_ERR_CONTEXT_MISMATCH`.
An exact replay with the same idempotency key and bound operation context
returns the stored economic result without moving funds again. Reusing that
key for a different escrow, operation or context is `LXP_ERR_CONTEXT_MISMATCH`.

---

## RELEASE / TIMEOUT

```text
escrow_id32 || idempotency_key32
```

TIMEOUT is the expiry sweep. It is `LXP_ERR_NOT_YET_VALID` before `expiry`
against the batch timestamp. Protocol-3 batch maintenance runs due escrow
deadlines using that timestamp (`src/modules/escrow/lx_escrow_timeout.c`).

---

## DISPUTE_OPEN

Payload is `escrow_id32`. Refusals include `LXP_ERR_ESCROW_STATE`,
`LXP_ERR_DISPUTE_WINDOW_CLOSED`, `LXP_ERR_HOLD_EXPIRED`.

---

## DISPUTE_RESOLVE

```text
escrow_id32 || beneficiary_basis_points:u32 || idempotency_key32
```

The decoder refuses basis points above 10,000 with `LXP_ERR_NON_CANONICAL`;
the idempotency key must be nonzero. The validator also checks the bound and
returns `LXP_ERR_PARAMETER_BOUNDS` for an out-of-range decoded value.
The split is `lx_escrow_split_bps`. Conservation failures are
`LXP_ERR_CONSERVATION`.

---

## Start here

- [Modules](Modules.md)
- [Fees](Fees.md)
- [Protocol](Protocol.md)
