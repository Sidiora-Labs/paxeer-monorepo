# Agentd

`layerx-agentd` is the non-authoritative agent process and library in
`services/agent/crates/layerx-agentd`. It owns tenant sessions, preparations,
local policy, durable reservations and recovery. It consumes node receipts
and committed state; it cannot mint balances or turn local accounting into
protocol authority.

Paxeer mainnet-beta is one BFT network, with EVM chain id **125** (`0x7d`).
The kernel participates in the network and proposes blocks. A transaction
receipt is final when its block is committed. Agentd is a client of this
network, not another consensus or settlement layer. Tenant domains are
namespaces in the database; they do not require separate chains or nodes.

For the node connection and hosted gateway boundary, see [Node access](lni.md).
For scope-bound tools over the same agent session, see [MCP](mcp.md).

## What it never does

Local figures are rebuildable caches. `LocalLimit::new` explicitly labels
its enforcement `daemon-enforced`: bypassing agentd bypasses that limit.
Approvals and locally narrowed capability dimensions have the same limitation.
Protocol budgets, in contrast, require a successful committed transaction and
a matching protocol budget record. Neither a local hold nor a successful
submission acknowledgement creates one.

Sources: `src/budget/create.rs`, `src/budget/reconcile.rs`,
`src/approval/mod.rs`, and `src/capability/report.rs`, relative to the agentd
crate throughout this page.

## Protocol evidence

### Startup and reconnection

`NodeRpc::connect` checks `eth_chainId` against `NodeRpcConfig.chain_id` before
returning a connection. An unavailable endpoint, malformed answer or chain
mismatch refuses startup. Calls have a finite deadline; the environment-based
configuration uses 30 seconds. HTTPS validates certificates using system roots
or the explicitly supplied CA in `connect_with_ca`.

`boot::readiness_gate` fetches `px_getReceiptKeys` on every check and refuses
an empty key set. The library `Gate` starts `NotReady`, clears readiness
before refreshing, and compares each published key's first and last heights
against its configured evidence policy. A failed refresh leaves writes closed;
a successful refresh advances its readiness generation. `disconnected` closes
the gate again. `guard_write` and `evidence_authority` refuse while not ready.

This is a chain-checked node connection and receipt-key refresh. The old LNI
handshake is retired. Identifiers such as `SequencerEndpoint`,
`pax_sequencerReceipt` and `HandshakeKey` remain in code; they do not identify
an additional network service.

The binary and the library have distinct startup wiring. Full binary mode
starts program reads, the shared human owner and the authenticated agent
boundary. `LAYERX_AGENT_MODE=human-owner` starts the human owner with its
loopback health listener and checks node RPC before serving; it refuses MCP
binding publication. An omitted mode selects `full`.

Sources: `src/node_rpc.rs`, `src/boot.rs`, `src/main.rs`,
`src/human_owner_mode.rs`.

### Receipt verifier and committed state

The node produces receipt signing keys with governed key ids and height
ranges. A signing key supplied beside a receipt is not a trust pin.
`finality::VerifiedReceipt::verify` verifies the canonical signature against
the published key interval and binds the transaction hash, EVM receipt module
and encoded height. Keep trusted key configuration separate from proof ingress.

`finality::VerifiedBlock::verify` checks the block hash and height against a
CometBFT commit under a separately trusted consensus chain id and validator
set. Duplicate signers, invalid signatures and insufficient voting power are
refused; signatures must represent more than two thirds of the voting power.
The consensus chain-id string and validator set are explicit trust inputs,
not values to copy unchecked from a receipt response.

The committed read path in `src/read/checkpoint.rs` authenticates the block
header and commit. `src/read/committed.rs` binds precompile reads and storage
witnesses to that committed state. Decoding an ABI result alone does not prove
its origin. Historical canonical receipt bytes remain unchanged when stronger
commit evidence is attached.

`protocol_evidence::EvidenceAuthority` also retains canonical activity,
receipt and state verification paths, including activity identity, inclusion,
outcome and replay checks. Their historical batch/header field names describe
the retained encoding, not a second finality mechanism.

### Evidence refusals

| Error | Meaning |
| --- | --- |
| `NodeRpcError::ChainIdMismatch` | Endpoint chain identity differs from configuration |
| `NodeRpcError::Unavailable` | RPC connection or deadline failed |
| `GateError::NoReceiptKeys` | Readiness cannot establish a usable receipt-key set |
| `VerifierPolicyError::HandshakeKey` | Refreshed keys do not match configured intervals |
| `AuthoritySourceUnavailable` / `AuthoritySourceUnprotected` / `AuthoritySourceMalformed` | The library evidence-policy source cannot be trusted or decoded |
| `FinalityError::InvalidReceipt` / `InvalidCommit` | Receipt or consensus evidence failed verification |
| `ReceiptReplayError::DuplicateReceipt` / `DuplicateActivity` | Receipt digest or activity was already admitted |

## Budget lifecycle

### Creation from a verified receipt

`BudgetRequest` carries the tenant, `request_id`, kind, asset, ceiling,
`expiry_ms`, authenticated `core_time_ms`, canonical transaction bytes and a
`VerifiedSubmission`. Expiry uses milliseconds of core time. The helper
`core_expiry_ms` converts a public expiry in seconds with overflow checks.

`create_protocol_budget` requires non-zero limits, expiry after core time,
and exact submission bytes and idempotency key. It decodes the signed EIP-1559
call and checks that it is the requested `budgetCreate` or `budgetCreateNative`
call to the pay precompile at `0x000000000000000000000000000000000000101b`.
Native creation also binds its source account and source sequence.

The pipeline submits via `eth_sendRawTransaction`, checks the returned hash,
then polls `pax_sequencerReceipt` for committed evidence. It confirms the
creation log and reads `getBudget(bytes32)` at the receipt height. The record
must match the created budget id, account, asset, limits, period, expiry,
revocation, rollover, purpose and optional source account, with no initial
spend, carry, closure or revocation. Only then does it return a
`ProtocolBudget` labelled `protocol-enforced`, with the receipt bytes and
confirming height. This function's store argument is unused; returning that
object does not itself persist a local budget record.

Sources: `src/budget/create.rs`, `src/budget/reconcile.rs`.

### Reserve

`BudgetLimiter` enforces configured ceilings across tenant, agent, session,
capability and counterparty scopes. Reservation checks and insertion run under
one lock; `consumed + held + requested` cannot exceed an applicable ceiling.
A refused reservation creates no hold.

`ReleaseKind::Unknown` keeps the reservation. `Executed` moves its amount to
consumed; `Failed` drops the hold. `Expired` releases an ordinary reservation
only after its expiry condition. Work that may already have reached the node
must remain in unknown-outcome recovery rather than being freed on a timeout.

Program reservations retain their charge kind, source, asset and applicable
limits. Source resolution checks the prepared call and committed inputs;
principal, program spend and fee exposure are not interchangeable amounts.
Durable records restore held exposure and consumed totals after restart.

Sources: `src/budget/reserve.rs`, `src/budget/mod.rs`,
`src/budget/program_sources.rs`, `src/budget/program_settlement.rs`.

### Reconcile

Reconciliation has real success paths. `budget::reconcile` verifies receipt
identity, rejects replay, verifies state inclusion and decodes a canonical
`ProtocolBudgetRecord`. It replaces local consumed accounting with
`spent_this_period` and returns an opaque `ReconciliationState` carrying the
window, remaining amount, divergence and observed head.

For native pay reads, `ProtocolBudgetState::read_at` obtains
`getBudget(bytes32)` at an explicit committed height. Its `reconcile` method
checks receipt identity, duplicate spends and that no spend is newer than that
state. Closed or revoked budgets have zero remaining allowance. The public
`decode_get_budget` decoder validates the ABI layout but does not authenticate
an endpoint or establish state inclusion by itself.

Source: `src/budget/reconcile.rs`.

### Divergence

`divergence_alert` reports a verified local/protocol mismatch. While it is
open, enforcement uses the greater consumed figure and the smaller remaining
allowance; `ready_for_writes` is false. The alert preserves both figures,
window bounds, last verified receipt and observed head for audit.

Source: `src/budget/divergence.rs`.

### Unknown-budget refusal and restart

`hold_unknown` persists unresolved reservations under `ObjectKind::Budget`.
`rebuild` re-verifies persisted receipts, checks their expected activity ids,
rejects replay and sums successful receipt amounts. It verifies and decodes the
protocol budget state, then totals unresolved durable holds.

`RestartAccounting.protocol_consumed` can now be present. Reconciliation
succeeds when protocol consumption falls between the verified receipt sum and
that sum plus unresolved holds. `require_write_ready` refuses missing schema
or unreconciled accounting. A process restart or an elapsed expiry is not
evidence that an unknown transaction failed.

Source: `src/budget/hold.rs`. Native call recovery also retains exact
preparation, disclosure, decision, signed bytes and queue/receipt state in
`src/approval/call_bound.rs` and `src/outbox/`.

### Budget states

| Record | Role |
| --- | --- |
| `LocalAccounting` | Rebuildable consumed/window/last-receipt cache |
| `LimitState` / `DurableBudgetReservation` | Local ceiling, consumed amount and held exposure |
| `UnknownReservation` | Durable unresolved amount, unavailable for reuse |
| `RestartAccounting` | Verified protocol and receipt consumption plus unresolved holds |
| `ProtocolBudget` | Confirmed protocol object, canonical record and creation receipt |
| `ReconciliationState` | Opaque verified result that can reopen reconciled controls |

### Budget refusals

| Error | When |
| --- | --- |
| `BudgetCreationError::InvalidLimit` / `Expired` | Invalid ceiling or expiry, or expiry not after core time |
| `EmptyActivity` / `ActivityBindingUnavailable` / `ActivityBindingMismatch` | Missing or substituted canonical submission |
| `NotBudgetCreation` / `IdentityMismatch` | Wrong call or creation identity |
| `Submission` / `ReceiptActivityMismatch` | Submission failed or the returned transaction identity differs |
| `UnverifiedReceipt` / `CoreRejected` | Missing committed evidence or failed execution |
| `CreatedBudgetUnconfirmed` | Creation effect or matching state could not be confirmed |
| `LimitRefusal::UnknownLimit` / `Exceeded` | Missing configured limit or projected exposure above it |
| `ReconcileError::UnverifiedReceipt` / `UnverifiedProtocolState` | Receipt or protocol-state evidence failed |
| `ProtocolStateSchemaUnavailable` | Budget record could not be decoded |
| `RestartError::Unreconciled` | Restart accounting does not yet permit writes |

## Approval model

Approvals restrict the local agent; they confer no protocol authority.
The registry binds a decision to the canonical preparation and disclosure
digest the approver saw. Changed bytes or actor identity are refused. Hold
expiry never grants approval, and concurrent decisions have one winner.
Approve releases the exact held preparation into the submission queue;
reject releases its local budget reservation as failed. Lists and lookups
are tenant-scoped; cross-tenant identifiers return `NotFound`.

The native call path retains the EVM preparation, sender, disclosure,
session generation, budget reservation and receipt-key intervals. Its states
include `Awaiting`, `Granted`, `Rejected`, `Expired`, `Signed`, `Queued` and
`Settled`. Recovery restores and validates those bytes rather than preparing
a replacement call. A local approval or queue transition does not claim chain
execution; receipt and debit evidence are checked before settling exposure.

Approval lifecycle events are local audit/stream facts with `UNVERIFIED`
protocol verification, even when authenticated. They must not be reported as
transaction receipts. The registry's approve/reject `DecisionKey` is a
separate bounded string: non-empty, at most 255 bytes, without NUL. Repeating
the winning key returns its original outcome; another key conflicts.

Sources: `src/policy/approval.rs`, `src/approval/{mod,events,expiry,call_bound}.rs`.

## Capability and attenuation

Capabilities are tenant-owned and explicitly bound on activity type,
counterparty, asset, amount, rate, purpose and expiry. Evaluation refuses the
first failing dimension. `assert_narrowing` checks the protocol authority's
activity types, counterparties, assets, amount and expiry; rate and purpose
are local dimensions unless explicitly enforceable. The binding never
replaces the original protocol submission authority.

Attenuation produces a child no wider than its parent. Amount and expiry may
shrink; the rate window cannot shorten in a way that widens permission.
Revoking a subtree cancels unsubmitted work, while already submitted work
still needs receipt resolution.

`Ceiling::new` and a receipt-only rebuild start unreconciled. A verified
`ReconciliationState` supplied to `Ceiling::reconcile` enables reservations.
Receipt application must match the held activity and amount. Unknown outcomes
remain held across expiry; a failed result consumes no transfer amount.

Sources: `src/capability/{mod,narrowing,attenuate,consume,report}.rs`.

## Local limit versus protocol-equivalent

Keep enforcement labels with the result: a protocol budget cites its confirmed
object and state, while local ceilings, approvals and unsupported capability
dimensions say `daemon-enforced` or `DaemonOnly` and disclose the bypass.
Local bookkeeping cannot upgrade those restrictions into protocol guarantees.

The `[u8; 16]` budget `LimitId` and the string-based rate-limiter `LimitId`
are different types. Their scopes and accounting must not be interchanged.

## Admission, quotas, rate, deadlines

`BoundaryAdmission` has seven priority lanes and returns typed `Backpressure`
without waiting. Submission and receipt resolution outrank bulk reads.
`Quota` accounts per tenant for subscriptions, prepared activities, outbox
entries, audit retention and stored receipts; pathological clients shed
non-critical work while submission and receipt resolution remain admitted.

`RateLimiter` uses authenticated `logical_time_ms` and requires a shared,
linearizable `CounterLedger` across instances of a tenant. Each request has
a finite `RequestDeadline`. A write that may have reached the node transfers
to `UnknownResolving` rather than being cancelled as if it never happened.

Sources: `src/limits/{admission,quota,rate,deadline}.rs`.

## Degraded mode and finality

`Controller` starts `Unreachable` at maximum rung `Submitted`, without a
verified reference. Only `Healthy` serves preparation, submission
acknowledgements and live streams. `Behind`, `Halted`, `Emergency` and
`DataUnavailable` refuse those operations but can still resolve unknown
submissions; `Unreachable` refuses resolution too.

Cached reads return already verified bytes and explicit staleness. Their
reported rung is capped by both the held evidence and the controller's
maximum rung. Requests above either cap, or before any verified reference,
are refused.

Transaction progress uses these rungs:

| Rung | Evidence held by the agent |
| --- | --- |
| `Submitted` | Transaction hash accepted by `eth_sendRawTransaction` |
| `Receipted` | Canonical receipt verified against a trusted receipt-key interval |
| `Committed` | Receipt plus a verified CometBFT commit at its height |

The network's finality event is block commit. These rungs describe how much
evidence the client has verified. `augment` / `augment_verified` store the
extra evidence beside the unchanged canonical receipt bytes.
`SubmissionProgress::refresh` fetches receipts, keys and blocks, but its
consensus commit and validator trust inputs are supplied separately; a block
response alone cannot raise the rung to `Committed`. `wait_for_rung` reports
the reached rung and whether the explicit deadline elapsed.

Sources: `src/degraded.rs`, `src/cache.rs`, `src/finality.rs`,
`services/agent/crates/layerx-types/src/rung.rs`.

## Idempotency keys

Caller idempotency is durable and tenant-scoped. Keys are non-zero 32-byte
values; canonical request bytes are non-empty and at most 1,048,576 bytes.
The request digest uses the `LXP/agent/request/v1` domain with a trailing NUL.
A completed repeat with identical bytes returns `RepeatedOriginal`; changed
bytes conflict. Retention cannot be shorter than the protocol window.
Pending durable work is recovered rather than submitted as a new request.

Source: `src/idempotency.rs`. Approval decision keys remain distinct from
transaction request keys.

## Config keys

The library `StartupConfig` loader reads one explicitly named protected UTF-8
`key=value` file. Exact environment keys override it. Duplicate file keys,
blank overrides and unknown `LAYERX_*` settings are refused. Per-tenant maps
must contain exactly one value per configured tenant; no security-relevant
value has a default.

| File key | Environment key | Value |
| --- | --- | --- |
| `network_id` | `LAYERX_NETWORK_ID` | Non-zero network id |
| `node_endpoint` | `LAYERX_NODE_ENDPOINT` | HTTP or HTTPS JSON-RPC URL |
| `expected_chain_id` | `LAYERX_EXPECTED_CHAIN_ID` | Non-zero chain id; mainnet-beta uses 125 |
| `tenants` | `LAYERX_TENANTS` | Non-empty unique tenant ids |
| `policy_sources` | `LAYERX_POLICY_SOURCES` | One absolute normalized policy path per tenant |
| `signer_configurations` | `LAYERX_SIGNER_CONFIGURATIONS` | One absolute normalized signer path per tenant |
| `verification_defaults` | `LAYERX_VERIFICATION_DEFAULTS` | Per-tenant compatibility verification labels, described below |
| `receipt_key_source` | `LAYERX_RECEIPT_KEY_SOURCE` | `px_getReceiptKeys` or an absolute normalized pin-file path |

The parser still accepts `sequencer-signed`, `batch-included`, `state-proven`,
`checkpoint-finalised` and `settlement-anchored` as compatibility labels.
They are not the live transaction rung vocabulary and do not establish a
second settlement or finality stage.

There is a narrower limitation in the library `Gate::new` path:
`ProtocolEvidenceVerifier::load` currently requires `ReceiptKeySource::PinnedFile`;
selecting `NodeRpc` there returns `AuthoritySourceUnavailable`. That retained
file decoder still recognizes the `layerx-sequencer-authority-v1` format.
This is a compatibility policy loader, not a requirement to start a retired
LNI service. The gate then checks current node keys against those pins.

The binary does not load this library file as its environment contract.
Its `NodeRpcConfig` uses `LAYERX_AGENT_NODE_RPC_URL` and
`LAYERX_AGENT_NODE_CHAIN_ID` (decimal or `0x` hexadecimal). Full-mode program
reads separately use `LAYERX_AGENT_NODE_ENDPOINT` and the remaining explicit
`LAYERX_AGENT_*` configuration in `src/main.rs`, including authority trust,
policy sources, listener credentials and journals. These similarly named
settings are not interchangeable. The human owner also requires its protocol
and network configuration. See [MCP serving](mcp.md#serving) for binding inputs.

Configuration errors name the setting and reason: missing, empty, duplicate,
unknown or retired keys; invalid integers, URLs, paths, tenants, verification
labels or maps; and unavailable, unprotected, oversized or invalidly encoded
files. Source: `src/config.rs`.

## Makefile gates

The repository `Makefile` retains focused targets such as
`agent-test-agentd-budget-create`, `agent-test-agentd-budget-reconcile`,
`agent-test-agentd-budget-reserve`, `agent-test-agentd-budget-unknown`,
`agent-test-agentd-budget-divergence`, `agent-test-agentd-approval`,
`agent-test-agentd-capability`, `agent-test-agentd-ceiling`,
`agent-test-agentd-finality`, `agent-test-agentd-degraded`,
`agent-test-agentd-idempotency` and `agent-test-agentd-config`.
The retained name `agent-test-agentd-handshake-gate` exercises node readiness;
it does not reintroduce LNI. Select the suite for the implementation being
changed; this reference is not a claim that those suites ran for this page.

[Node access](lni.md) · [MCP](mcp.md) · [Home](../index.md)
