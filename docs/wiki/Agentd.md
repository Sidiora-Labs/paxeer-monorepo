# Agentd

`layerx-agentd` is the non-authoritative agent service and library in
`services/agent/crates/layerx-agentd/`. It connects to Paxeer node JSON-RPC,
verifies network evidence, and owns tenant sessions, policy, approvals,
capabilities, reservations and durable recovery. It does not mint balances
or protocol budget objects from local accounting.

Paxeer mainnet-beta is one BFT network. The kernel is part of the network and
proposes blocks; EVM chain id is **125 (`0x7d`)**. A receipt is final when its
block is committed. Sub-chains are tenant domains in the database, without a
new chain process for each tenant. Agentd is an application boundary, not an
additional consensus or settlement system.

The crate includes a library and binary. `src/main.rs` wires node reads,
the program-balance listener, Agent HTTP mutual TLS, and the human Unix owner.
Library constructors and compatibility types do not by themselves describe
what a running owner serves. The contract is [Agent API](AgentApi.md), and tool
routing is [MCP](Mcp.md).

---

## Authority and local state

Local balances, consumption and caches do not establish protocol authority.
A daemon-only limit is explicitly labelled `daemon-enforced`; bypassing
agentd bypasses that limit. Approvals restrict the daemon's behavior and do
not grant signing or spending authority.

A protocol budget is returned only from a signed network mutation with a
verified committed receipt and matching budget state. The owner retains the
budget object, transaction and recovery state through its durable paths and
reconciles them before admitting later writes.

---

## Protocol evidence

### Node readiness and receipt keys

`src/node_rpc.rs` connects through `NodeRpc::connect`, compares `eth_chainId`
with the configured chain id, and applies finite RPC deadlines. Use the public
root URL, for example `https://api-mainnet-beta.paxeer.network/` or
`https://api1.mainnet-beta.paxeer.network/`; numbered public nodes run through
`api14`. Hosted `px_` methods have their own contract in
`services/platform/hosted/gateway/openrpc.json`, so public namespace support
must not be inferred from an agent client method.

`src/boot.rs` implements `readiness_gate`: obtain a nonempty receipt-key set
from the chain-checked NodeRpc. `Gate::readiness_gate` first clears readiness,
then checks the returned key intervals against its protected evidence policy.
A successful refresh advances the generation. Disconnect or failed refresh
leaves the gate NotReady; `guard_write` then returns `WriteGateError::NotReady`.
The failure is not repaired by trusting a key supplied with an incoming proof.

There are two configuration representations for receipt keys: `NodeRpc` and
`PinnedFile`. The generic `Gate::new` currently requires PinnedFile:
`ProtocolEvidenceVerifier::load` returns `AuthoritySourceUnavailable` for
NodeRpc. Its compatibility file parser still recognizes the literal
`layerx-sequencer-authority-v1` header and interval records. That is a persisted
policy format, not a startup handshake with a separate network component.
Parser acceptance of `px_getReceiptKeys` alone does not open this gate.
The running binary has separate owner/node trust wiring; configure the
boundary actually used rather than copying a library fixture.

### Receipt, commit and storage verification

`src/finality.rs` verifies canonical execution receipts against the pinned
receipt-key id, public key and inclusive height interval. The receipt must
bind the transaction hash. Receipt keys are distinct from validator keys.
`VerifiedBlock` checks block height/hash and CometBFT canonical precommit
signatures against an independently trusted chain id and validator set;
strictly more than two thirds of voting power must verify. Duplicate
validators or signatures and malformed coordinates are refused.

Committed state reads in `src/read/committed.rs` bind a precompile call and
its storage witnesses to one authenticated committed block. State evidence
in `src/protocol_evidence.rs` retains the committed call, including the budget
id for `RawStateEvidence::budget_witness`. An ABI-decoded value without the
associated authenticated call and witness is not proof of state.

Compatibility receipt ingress also remains in `protocol_evidence.rs`:
`RawReceiptEvidence` carries canonical_receipt, proof, canonical_header and
header_signature. The verifier checks policy identity, inclusion, protocol,
sequence range, receipt/header execution identity and outcome before issuing
an opaque evidence token. This retained codec is not an extra finality rung.
Replay guards refuse repeated receipt digests and activity ids. Failed
verification must not create verified cache entries or spendable budgets.

Useful refusal types include `GateError::NodeRpc`, `NoReceiptKeys`,
`GateError::Evidence`, `VerifierPolicyError::HandshakeKey`,
`FinalityError::InvalidReceipt`, `InvalidCommit`, and
`StateEvidenceError::Committed`. The retained `HandshakeKey` error name means
a key-policy mismatch, not a requirement to contact a retired service.

---

## Budget lifecycle

The implementation is in `src/budget/{create,reconcile,hold,divergence,reserve}.rs`
and the owner budget methods in `src/human_runtime.rs`.

### Creation from a committed effect

`BudgetRequest` binds tenant, request id, kind, asset, ceiling, expiry in
milliseconds, authenticated core time, canonical signed bytes and a
`VerifiedSubmission`. Creation rejects zero limits, expired requests, empty
bytes, missing submissions and byte/idempotency mismatches before submission.

The signed call must target pay `0x000000000000000000000000000000000000101B`
and decode as the matching budget creation operation. Creation checks the
successful committed receipt, transaction identity and BudgetCreated log,
then reads and verifies the created budget at that receipt's height. Object
id, account, asset, limits, expiry, period, purpose, source, rollover and
revocation fields must match; initial consumption and carry must be zero,
and the record must be open and unrevoked. Only then is `ProtocolBudget`
returned. `CreatedBudgetUnconfirmed` refuses a mismatch.

Owner-facing `budget.create`, `budget.fund` and `budget.revoke` bind signed
authorization to the exact requested mutation. `budget.state` and
`budget.reconciliation` expose the owner state. `LocalLimit` and the durable
daemon limit registry are separate local controls, not protocol objects.

### Reserve and release

`BudgetLimiter` applies ceilings across tenant, agent, session, capability
and counterparty scopes. Reservation checks every applicable limit under one
lock: consumed + held + requested cannot exceed the ceiling. A refused
reservation leaves no partial hold.

A durable reservation identifies the limit, scope, amount, ceiling and expiry.
An Unknown release is a no-op. Executed consumes the held amount; Failed
releases it. Expired releases only after its sequence bound. Unknown outcomes
remain held until evidence resolves them, including across process loss.
Sequence expiry and protocol timestamps are separate units.

### Reconcile and divergence

Reconciliation verifies spend receipt identities and replay protection, then
uses a decoded canonical `ProtocolBudgetRecord` from verified state. The
committed alternative reads `getBudget(bytes32)` at an explicit block height
on `0x101B`. It rejects malformed ABI offsets, widths, ordering, flags and
invalid period/limit fields. Decoding alone does not authenticate the endpoint.

A successful reconciliation updates the local cache from protocol
`spent_this_period`, remaining funds and the observed height, and returns an
opaque `ReconciliationState`. Closed or revoked committed budgets have zero
remaining allowance. Missing or malformed evidence is refused; the schema is
not permanently unavailable.

`divergence_alert` can therefore report a mismatch. While it is open, enforce
`max(local, protocol)` consumed and the smaller remaining amount, and expose
`ready_for_writes: false`. The audit records both figures, the last verified
receipt and the observed window and height.

### Restart accounting

`hold_unknown` persists unresolved reservations. Rebuild verifies retained
receipts, decodes protocol budget state, and counts unresolved holds.
`RestartAccounting.protocol_consumed` is populated when the canonical record
can be decoded. Reconciliation succeeds only when protocol consumption lies
between verified receipt consumption and receipt consumption plus unresolved
holds. The holds remain unavailable even when accounting is consistent.

`require_write_ready` refuses absent schema as `ProtocolStateSchemaUnavailable`
and inconsistent accounting as `Unreconciled`. It can succeed with valid,
consistent evidence. Receipt sums alone do not establish write readiness.

| State | Authority |
| --- | --- |
| `LocalAccounting` | Rebuildable cache |
| `LimitState` / durable reservation | Daemon-local ceiling and held exposure |
| `UnknownReservation` | Unresolved amount, not available to spend |
| `ProtocolBudget` | Verified network-created budget and receipt |
| `ReconciliationState` | Verified protocol state compared with local accounting |
| `RestartAccounting` | Receipt/state/hold consistency used for recovery admission |

---

## Approval model

Approval holds are daemon-only restrictions. `ApprovalService` and
`src/policy/approval.rs` retain prepared bytes and their decoded disclosure,
not an editable caller request. The hold's digest, actor and validity window
must bind the preparation. A decision releases only that exact preparation;
changed disclosure is Defective, expiry never approves, and concurrent
decisions have one winner. Rejecting releases the reservation as Failed.

List/get are tenant-scoped and hide cross-tenant existence with NotFound.
Approve/reject use a bounded, nonempty `DecisionKey` for idempotency. Repeating
the winning decision key returns its outcome; a conflicting decision is
refused. This string is distinct from the 32-byte submission idempotency key.

Approval lifecycle events are audited and streamed as local events. Their
UNVERIFIED classification does not prove economic execution. The public
contract states are Held, Granted, Rejected, Expired and Defective; internal
policy states also use AwaitingApproval and Approved.

---

## Capability and attenuation

Capabilities are tenant-owned restrictions over activity types,
counterparties, assets, amounts, rate, purpose and expiry. Missing dimensions
are refused; explicit empty sets deny. Evaluation reports the first failed
dimension. `assert_narrowing` compares against resolved protocol authority;
the capability does not replace that authority.

Attenuation creates a child no wider than its parent on every dimension.
Amount and expiry cannot grow; a rate window cannot shrink to increase the
rate. Revoking a subtree cancels unsubmitted bound work. Controls that cannot
be established as protocol-enforced remain labelled DaemonOnly.

A `Ceiling` starts unreconciled. Receipt-only rebuild may restore consumption
but cannot authorize new reservations without protocol reconciliation.
Consumption requires a verified terminal receipt for the held activity and
amount; a failed receipt consumes no principal. Unknown reservations remain
held. See `src/capability/{consume,narrowing,attenuate,report}.rs` and
[Agent API](AgentApi.md#capabilities-and-budgets).

---

## Admission, quotas, rate, deadlines

`BoundaryAdmission` has seven strict priority lanes; submission and receipt
resolution outrank bulk reads. It does not wait for capacity and returns typed
Backpressure. Tenant quotas shed noncritical work while protecting submission
and receipt resolution.

Rate limits use authenticated logical time and a shared linearizable
`CounterLedger` across a tenant's instances. Requests have finite deadlines.
If a write may have reached the network, deadline handling transfers it to
UnknownResolving instead of treating cancellation as proof of non-execution.
These controls live in `src/limits/`; they do not create protocol budgets.

---

## Degraded mode and finality

`Controller` starts Unreachable at the submitted rung, with no verified
reference. Healthy observations enable preparation, submission acknowledgement
and live streams. A regressed head or maximum rung enters Behind and refuses
live writes. Unreachable also refuses unknown resolution; Halted, Emergency
and DataUnavailable refuse writes and live streams while allowing resolution
where the connection can serve it. Cached reads retain explicit staleness and
cannot claim a rung above their held evidence.

The transaction ladder is submitted → receipted → committed. Submitted is not
proof of execution. Receipted requires a pinned-key execution receipt;
status 0 is a revert. Committed binds that receipt to the landing block and
verified validator commit. Finality records in `src/finality.rs` sit beside
canonical receipt bytes; augmentation never changes the signed receipt.
`wait_for_rung` polls bounded progress and reports the actual rung at timeout.
There is no later settlement tier to wait for.

Legacy `VerificationLevel` values still occur in compatibility configuration
and state-proof code. They are not six current transaction rungs. The admin
retry path maps legacy checkpoint requests to Committed and refuses settlement
anchoring rather than inventing stronger evidence.

---

## Idempotency keys

Caller idempotency is durable per tenant (`src/idempotency.rs`). Keys are
nonzero, request bytes are nonempty and bounded to 1,048,576 bytes, and the
owner computes the canonical body digest under `LXP/agent/request/v1` followed
by a NUL byte. A repeated key with the same digest returns RepeatedOriginal;
a different digest conflicts. Retention is not shorter than the protocol
window. An ambiguous send resolves the retained signed transaction, not a
replacement at a fresh nonce.

---

## Config keys

The library's `StartupConfig` (`src/config.rs`) loads a protected UTF-8
`key=value` file. Exact environment names override file values. Missing,
blank, duplicate, unknown or incomplete settings are refused; security values
have no implicit defaults.

| File key | Environment key | Role |
| --- | --- | --- |
| `network_id` | `LAYERX_NETWORK_ID` | Nonzero evidence network id |
| `node_endpoint` | `LAYERX_NODE_ENDPOINT` | HTTP/HTTPS node JSON-RPC URL; use the public root URL |
| `expected_chain_id` | `LAYERX_EXPECTED_CHAIN_ID` | Nonzero expected EVM chain id; mainnet-beta is 125 |
| `tenants` | `LAYERX_TENANTS` | Nonempty unique tenant ids |
| `policy_sources` | `LAYERX_POLICY_SOURCES` | Exactly one absolute normalized path per tenant |
| `signer_configurations` | `LAYERX_SIGNER_CONFIGURATIONS` | Exactly one absolute normalized path per tenant |
| `verification_defaults` | `LAYERX_VERIFICATION_DEFAULTS` | Complete tenant map of legacy VerificationLevel labels |
| `receipt_key_source` | `LAYERX_RECEIPT_KEY_SOURCE` | `px_getReceiptKeys` or absolute pinned-file path; gate support differs as described above |

For compatibility, the verification-default parser accepts `sequencer-signed`,
`batch-included`, `state-proven`, `checkpoint-finalised` and
`settlement-anchored`. These are persisted parser labels, not claims that all
corresponding operations can be served. Retired `expected_protocol_version`
and `sequencer_authority_source` settings and their environment forms are
explicitly refused with `RejectionReason::Retired`.

URL parsing rejects non-HTTP schemes, embedded credentials, fragments and
whitespace. Protected source reads enforce bounds, ownership and file mode.
Other rejection reasons include Missing, Empty, Duplicate, Unknown,
InvalidInteger, InvalidTenant, IncompleteTenantMap, InvalidPath,
InvalidVerificationLevel, TooLarge, InvalidEncoding, Unavailable and Unprotected.

The binary has a distinct `LAYERX_AGENT_*` configuration surface. Its shared
node variables are `LAYERX_AGENT_NODE_RPC_URL` and
`LAYERX_AGENT_NODE_CHAIN_ID`; the latter accepts decimal or `0x` hexadecimal.
`src/main.rs` additionally configures listeners, distinct credentials, owner
trust, policy sources, deployment journal, human socket and limit records.
The library table is not a complete binary launch configuration.

---

## Makefile gates and sources

Focused targets in the repository Makefile include
`agent-test-agentd-budget-create`, `agent-test-agentd-budget-reconcile`,
`agent-test-agentd-budget-reserve`, `agent-test-agentd-budget-unknown`,
`agent-test-agentd-budget-divergence`, `agent-test-agentd-approval`,
`agent-test-agentd-capability`, `agent-test-agentd-cache`,
`agent-test-agentd-finality`, `agent-test-agentd-degraded`,
`agent-test-agentd-idempotency`, and `agent-test-agentd-config`.
They are development gates, not evidence that a deployed endpoint is ready.

Source paths above are relative to `services/agent/crates/layerx-agentd/`.
Contract schemas live in `services/agent/schema/agent-api/`.

- [Agent API](AgentApi.md)
- [MCP](Mcp.md)
- [Running an agent](RunningAnAgent.md)
- [Home](Home.md)
