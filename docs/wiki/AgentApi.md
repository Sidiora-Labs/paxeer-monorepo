# Agent API

The Agent API contract is **major 2, minor 0**, defined by
`services/agent/schema/agent-api/v1.kvx` and its included schemas. It covers
identity, preparation, submission, reads, approvals, budgets, subscriptions,
and programs. `layerx-agentd` consumes network evidence and enforces local
restrictions; it does not create balances or protocol authority from a cache.
See [Agentd](Agentd.md) for trust and recovery, and [MCP](Mcp.md) for tool routing.

Paxeer mainnet-beta is one BFT network, with EVM chain id **125 (`0x7d`)**.
The kernel is part of the network and proposes blocks. Public node JSON-RPC
uses the root URL, for example `https://api-mainnet-beta.paxeer.network/` or
`https://api1.mainnet-beta.paxeer.network/` (numbered nodes run through `api14`).
The hosted gateway's `px_` contract is declared separately in
`services/platform/hosted/gateway/openrpc.json`; do not assume every node
serves those methods. Sub-chains are tenant domains in the database.

---

## Wire and authentication

Contract version negotiation and HTTP envelope versioning are different.
The binary version exchange described by `v1.kvx` carries contract major,
minor, and node-interface major. Its request framing is:

```text
contract_major:u16be || contract_minor:u16be || request_id:u64be
|| operation:utf8 || payload:canonical-map
```

The Agent HTTP boundary (`layerx-agentd/src/agent_rpc.rs`) instead accepts a
bounded UTF-8 JSON object. The hosted gateway registers `POST /v1/agent/rpc`
for this envelope; this application route is distinct from public node
JSON-RPC at the root. The envelope has these exact top-level fields:

| Field | Encoding and rule |
| --- | --- |
| `version` | JSON integer `1`, the HTTP envelope version |
| `request_id` | Canonical decimal u64 string |
| `operation` | Exact catalogue operation name |
| `request` | Operation-specific JSON object |
| `credential` | Required tenant/session/token/generation coordinates |
| `idempotency_key` | Nonzero, 64 lowercase hex characters for mutations; absent or null for reads |

The credential is `{tenant, session_id, token_id, generation}`. Session and
token ids are 32-byte lowercase hex; generation is a canonical decimal u64
string. The owner derives the agent from the authenticated token and checks
the current session and generation. Request coordinates cannot override that
identity. Forwarded principal headers confer no authority. Gateway admission
and daemon session authentication are distinct checks.

Unknown or duplicate JSON keys and trailing bytes are refused. The body bound
is 1,048,576 bytes. Consensus-critical amounts, sequences, limits and timestamps
use decimal strings at the dynamic-language boundary; each operation defines
its units. A sequence is never a timestamp.

Successful HTTP responses carry `request_id`, `value`, and
`verification_status`. The status is `achieved` with a rung, `unverified` with
requested/achieved/reason, `pending` for a submitted lookup awaiting evidence,
or `absent` when no submission is known. A pending or unverified achieved rung
must be strictly below the requested rung; inconsistent status is refused.

---

## Write pipeline

Network writes are signed **EIP-1559 type-2 transactions**. The EVM preparation
code in `layerx-agentd/src/prepare/mod.rs` binds chain id, pending nonce,
precompile destination, ABI calldata, value, gas and fees to the disclosure
and signing preimage. The supported precompile targets are ledger `0x101A`,
pay `0x101B`, auth `0x101C`, and programs `0x101D` (20-byte, zero-padded
addresses). A native payload is encoded inside that call; it is not a second
network submission interface.

The typed preparation request retains `protocol_activity_type`, actor,
authority, account_sequence, timestamp_bound, idempotency_key, fee_limit,
payload and payload_hash. Optional `capability_id` binds an owner-issued
capability. Use the exact request variant served by the owner; the MCP
abbreviated argument schema is not interchangeable with this typed request.

| Operation | Input | Result |
| --- | --- | --- |
| `prepare` | Complete typed request and optional capability binding | `Prepared`: preparation_ref, unsigned_canonical_bytes, signing_preimage, disclosure, expiry |
| `sign` | preparation_ref and external signature | `Signed`, bound to the preparation |
| `submit` | preparation_ref and signature, under the selected owner variant | `TrackedSubmission` |
| `track` | submission_ref | state, evidence, rung, transitions |
| `wait` | submission_ref, requested_rung, deadline | submission, actual_rung, deadline_elapsed |

Contract-2 `SignatureBytes` is a 65-byte secp256k1 signature, encoded as 130
lowercase hex characters. Signature carriers are variant-specific; native
owner consent and legacy tool signatures must not be substituted for the
transaction signature. Signing is external: a tool name does not authorize an
implicit signer.

Disclosure is derived from the prepared bytes, including canonical_digest,
activity_type, actor, authority, counterparties, amounts, asset, fee_limit,
expiry and idempotency_key. Approval releases those exact bytes. Changing the
call requires new preparation and approval.

Submission states are Prepared, Signed, Queued, Submitted, Unknown, Executed,
Committed, Failed and Expired. Unknown is unresolved pending work, never an
instruction to create a new transaction. Executed requires a verified receipt;
Committed requires committed-block evidence. A deadline returns the actual
reached rung without promoting it.

---

## Verification and reads

There are three transaction rungs (`errors.kvx`, `layerx-types/src/rung.rs`):

| Wire rung | Evidence |
| --- | --- |
| `submitted` | Exact signed bytes and transaction hash; no verified receipt yet |
| `receipted` | Canonical node execution receipt verified against independently pinned receipt keys, height interval and transaction fields |
| `committed` | Matching landing receipt and block, with CometBFT commit verified by more than two thirds of validator voting power |

A status-0 receipt proves a revert, never successful execution. The receipt is
final when its block is committed. There is no later settlement tier.
Receipt-key ids outside the pinned list or height interval cause
`VerificationFailure` with reason `receipt_key_unknown`.

Read operations remain named `read.balance`, `read.account`,
`read.module_state`, `read.history`, `read.batch`, `read.checkpoint`,
`read.proof_bundle`, `availability.fetch`, and `export.offline`. Their meaning
is defined in `read.kvx` and the implementations in `layerx-agentd/src/read/`:

- `read.batch` selects a block height through `eth_getBlockByNumber`.
- `read.checkpoint` returns the CometBFT commit and validator set at the height.
- Availability selects a block with full transactions, reports bounds and
  provider attribution, and does not invent missing data or merge providers.
- History uses `(blockHeight, txIndex, logIndex)`, encoded as exactly three
  canonical decimal u64 strings in a JSON array. `page_limit` above 65,535 is
  refused rather than clamped.
- `VerifiedRead` reports `value`, `achieved_verification_level` and freshness
  with `chain_head`, `value_height`, and `relative_to`. Pending and committed
  snapshots must not be presented as the same observation. Ordinary state
  reads have a null transaction rung; transaction evidence uses the three rungs.

Portable proof bundles use `layerx-receipt-proof-v2`: rawTransaction,
signedReceipt, receiptDigest and receiptKey; committed evidence additionally
includes blockHeader, receiptInclusion, commit and validatorSet. Optional
accountProof binds the same account and committed height. Ownership, receipt
signatures, transaction hash, domain, logs, gas and block inclusion are
checked; a returned key or rung label is not a trust anchor. Bundles are
bounded to 262,144 bytes before hex encoding. Offline export requires committed
evidence for every requested fact and independent consumer verification.

`project` returns a fee estimate (`ProjectionResult`), never a VerifiedRead or
an executed receipt. Policy evaluation uses `policy.dry_run`.

---

## Capabilities and budgets

Sessions carry tenant, agent DID, authority reference, permitted activity
types, expiry, client and policy version. Capabilities require every dimension:
activity_types, counterparties, assets, amount_ceilings, rate_ceilings,
purpose_constraints and expiry. An explicit empty set denies; omission does
not mean unrestricted. Operations are `capability.create`,
`capability.attenuate`, `capability.list`, and `capability.revoke`.
Attenuation narrows every dimension; revocation includes descendants and
unsubmitted preparations bound to them.

Budgets distinguish `ProtocolBudget` from `DaemonLimit`. Bypassing the daemon
bypasses a DaemonLimit. Protocol creation uses the signed pay-precompile call,
a successful committed receipt and matching committed budget state; it can
return a verified `ProtocolBudget`. The owner retains durable budget and
transaction state and reconciles it on restart.

Operations are `budget.create`, `budget.fund`, `budget.list`,
`budget.reconciliation`, `budget.state`, and `budget.revoke`. Protocol mutations
bind their signed authorization to the requested object and effect. Local
accounting and receipt sums alone cannot establish protocol remaining funds.
[Agentd](Agentd.md#budget-lifecycle) describes reconciliation and unknown holds.

The Agent HTTP operation `faucet.claim` is retired and returns terminal
`UnavailableCapability`, reason `unavailable_capability.faucet.claim`.
The separately listed MCP `faucet.request` is not an Agent API operation.

---

## Approval holds

Approvals are daemon-enforced restrictions and confer no protocol authority.
`ApprovalRecord` contains approval_id, tenant, held_activity,
canonical_bytes_digest, hold_reason, created_at, expires_at and state. States
are Held, Granted, Rejected, Expired and Defective; all except Held are terminal.

`approval.list`, `approval.get`, `approval.approve` and `approval.reject` are
tenant-scoped. Decisions use an idempotency key. Approving releases the exact
held preparation; expiry does not approve, and changed bytes are defective.
Local approval events do not establish a transaction rung.

---

## Subscriptions

Scope is tenant, agent and capability. Filters restrict agents, accounts,
activity_types, modules, assets, counterparties and result_classes, in that
order of authority: tenant → capability → filter. Operations are
`subscription.create`, `list`, `health`, `acknowledge`, `pause`, `resume`, and
`delete` under the `subscription.` prefix.

Definitions and acknowledged cursors are durable. Delivery is at-least-once;
consumers deduplicate on `deduplication_id`. Delivery variants are Event, Gap
and Truncated. A gap blocks later delivery until backfill or explicit
truncation. Optional verified receipt references are distinct from local
lifecycle events.

---

## Errors and related surfaces

Error classes are TransportFailure, Deadline, ProtocolIncompatibility,
UnavailableCapability, CoreRejection, VerificationFailure, PolicyRefusal,
CapabilityRefusal, BudgetRefusal, RateLimit, IdempotencyConflict and
InternalFault. An error carries class, optional protocol_result_code,
retriability, request_id and machine-readable reason.

Mutations bind a durable idempotency key to the owner-derived canonical request
digest. The same body replays the original result; another body conflicts.
Callers do not supply the internal body digest. Transport ambiguity remains
Unknown and must be resolved against the retained transaction.

Programs have a distinct HTTP family including `/v1/programs/deploy`,
`/v1/programs/upgrade`, `/v1/programs/wind-down`, `/v1/programs/simulate` and
`/v1/programs/call`; see [Programs](Programs.md).

- [Agentd](Agentd.md)
- [MCP](Mcp.md)
- [Running an agent](RunningAnAgent.md)
- [Public RPC](PublicRpc.md)
