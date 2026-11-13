# Governance module

Module ID `7` (`LXP_MODULE_GOVERNANCE`). Activity types occupy `0x0007xxxx`.
Ordinals 1, 2, 3, 5, 6 and 8 are literal values accepted by
`lxp_governance_activity` (`src/modules/governance/lxp_governance.c`).
Ordinals 10 and 11 are named constants in `include/layerx/lxp_governance.h`.

Governance changes are ordinary activities: same envelope, authority,
ordering, fee, and receipt rules. Parameter tables and emergency pauses live in
`include/layerx/lxp_governance.h`.

Sources: `src/modules/governance/lxp_governance.c`,
`include/layerx/lxp_governance.h`, `src/modules/governance/lxp_gov_param.c`,
`src/modules/governance/lxp_emergency.c`.

---

## Registered activities

| Value | Ordinal | Role |
| --- | ---: | --- |
| `0x00070001` | 1 | Identity create (variant 0) or onboard (variant 2) |
| `0x00070002` | 2 | Primary-key rotation announce (variant 0), apply (variant 1), or lapse (variant 3) |
| `0x00070003` | 3 | Recovery-root update |
| `0x00070005` | 5 | Session-key grant |
| `0x00070006` | 6 | Grant revocation |
| `0x00070008` | 8 | Issue delegated capability or budget allowance |
| `0x0007000a` | 10 | Migration budget proposal (`LXP_GOVERNANCE_MIGRATION_BUDGET`) |
| `0x0007000b` | 11 | Programs fee proposal (`LXP_GOVERNANCE_PROGRAMS_FEE`) |

Ordinals `0x00070004`, `0x00070007` and `0x00070009` are not registered
and are not accepted by `lxp_governance_activity`.

---

## Wire envelope (ordinals 1–8)

```text
bytes[0] = 0x71
bytes[1] = ordinal
bytes[2] = variant
bytes[3] = field_count
body from offset 4
```

Length is 4–1024 bytes. Any other header is `LXP_ERR_NON_CANONICAL`.

| Ordinal | Variant | Fields | Length | Body (from offset 4 / 36) |
| ---: | ---: | ---: | ---: | --- |
| 1 | 0 | 2 | 68 | Identity create; public key at offset 36 must be the verified activity key |
| 1 | 2 | 1 | ≥ 8 | Onboard (`lxp_governance_onboard`) |
| 2 | 0 | 4 | 92 | Pending key at 36; `begin`, `end`, `effective` u64 at 68, 76, 84 |
| 2 | 1 | 1 | ≥ 8 | Apply rotation (`lxp_governance_rotation`) |
| 2 | 3 | 2 | 68 | Lapse; commitment at 36 must match hashed pending state |
| 3 | 0 | 3 or 5 | 70 or 86 | Recovery root at 36; optional delay / max-delay u64 at 70 and 78 |
| 5 | 1 or 2 | 3 or 5 | ≥ 52 | Session grant |
| 6 | 0 | 3 | 45 | `grant_id32` at 4; reason byte at 36; sequence u64 at 37 |
| 8 | 1 | 1 | ≥ 9 | Big-endian u32 byte length, then the grant body |

Activity 7:8 issues kind 3 (delegated capability) or kind 4 (budget allowance).
The payload is `0x71 0x08 0x01 0x01`
followed by the body length as a big-endian `u32`, then `lxp_grant_encode`
bytes (structure `0x2001`). The grant uses version 1 without a fee budget or
version 2 with a fee budget. No trailing bytes are accepted. The inner grantor
signature is all zero because the outer owner signature binds the body.
Protocol version 3 and owner authority matching the actor DID are required.
Identity must already exist via 7:1.

Ordinal 10 uses `lxp_programs_migration_proposal_decode`; ordinal 11 uses
`lxp_programs_fee_governance_proposal_validate`. These proposal payloads
have their own layouts rather than the four-byte envelope above.

---

## Refusals

Decode: `LXP_ERR_NON_CANONICAL`.

Validate: `LXP_ERR_AUTH_SCOPE` when the actor or DID does not match.

Execute: `LXP_ERR_SEQUENCE_REUSED` (identity already exists),
`LXP_ERR_BAD_SIGNATURE` (create key mismatch), `LXP_ERR_AUTH_SCOPE` (rotation,
recovery, session, and grant rules), `LXP_ERR_AUTH_REVOKED` (repeat revoke),
`LXP_ERR_UNKNOWN_AUTHORITY_KIND`, `LXP_FATAL_INVARIANT` (corrupt `LXGI1`
identity state).

Emergency halt, resume, and module enable are library operations on
`lxp_gov_emergency_state` (`lxp_gov_emergency_halt`,
`lxp_gov_emergency_resume`, `lxp_gov_module_enable`). They still require an
ordered governance activity; there is no operator path outside the log.

Parameter changes use `lxp_gov_param_propose` with a minimum activation
delay and rollout scope (all / module / market / account set). They do not
add a new activity ordinal beyond the registered set above.

---

## Start here

- [Modules](Modules.md)
- [Sequencing](Sequencing.md)
- [Protocol](Protocol.md)
- [Roadmap](Roadmap.md)
