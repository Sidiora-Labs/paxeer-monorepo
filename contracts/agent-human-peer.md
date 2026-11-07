# Agent Human peer contract

The Human peer is an authenticated local Unix-socket API between the Human
service and the agent owner runtime. It is not public node JSON-RPC. Its owning
implementation is [human.rs](../services/agent/crates/layerx-agentd/src/human.rs),
with production operations in
[human_runtime.rs](../services/agent/crates/layerx-agentd/src/human_runtime.rs).
See [configuration](agent-config.md#human-peers), [frames](agent-formats.md#human-peer-frame),
the [Agent API](agent-api.md) and [Rust modules](agent-rust-api.md#agentd).

The runtime operates on one Paxeer BFT network, chain ID 125 on mainnet-beta.
The kernel participates in that network and proposes blocks; committed blocks
establish receipt finality. Tenant domains are storage namespaces on the network.
There is no separate LayerX interface or sequencer service to connect to.

## Authentication

`HumanUnixServer` obtains the accepted socket's kernel peer credentials before
decoding requests. Its configured UID mapping establishes principal and transport
tenant; unmapped UIDs are refused. Socket and parent ownership/type/permissions
are checked, world permissions are refused, and an existing endpoint is not
silently replaced. Listener bounds and a nonzero deadline are mandatory.

`HumanPeer.subject` is initially absent. The `SUBJECT` operation establishes the
authorized subject binding, retaining transport tenant/principal as well as
owner/account/asset coordinates. A subject namespace is derived by
[`layerx-identity-binding`](../services/agent/crates/layerx-identity-binding/src/lib.rs),
not accepted as an arbitrary caller-selected storage tenant. Request text never
substitutes for the transport UID or current owner authority.

## Frames and refusals

Messages use a big-endian u32 length prefix. The ordinary body begins with
`LXHAGT01`, followed by a one-byte selector and its exact ordered payload.
Text is bounded to 255 bytes, binary fields to 1,048,576 bytes, and configured
maximum frames cannot exceed 1,048,576 bytes. The `LXHAGT02` trace wrapper
contains a fixed 36-byte `trc_` identifier and a length-delimited ordinary frame.
Invalid UTF-8, bounds, selectors and trailing bytes are rejected.

Mutation envelopes retain `request_id: u64`, key32, body-digest32 and typed
operation. The Human envelope digest is its own contract; it must not be replaced
by a raw JSON hash or the distinct Agent HTTP envelope digest. Preserve typed
transport, authentication, malformed-frame and owner refusals across the peer.
Do not interpret a disconnected submission as confirmed failure.

## Preparation and recovery

`PREPARE` retains the actor, authority, full packed native activity, account
sequence, validity bounds, idempotency key, fee limit, exact payload/hash and
optional capability ID. `HumanPrepare::validate_pending_nonce` refuses a sequence
that differs from the authoritative pending value. `SUBMIT`, `TRACK` and
`RECEIPT_LOOKUP` operate on the retained request and evidence; do not reconstruct
an economic action from display text after restart.

Native preview/envelope operations bind the authenticated credential, current
owner, exact canonical bytes, session generation, expiry, capability and signed
purpose. Local grant consent and signing the protocol activity are separate
authorizations. Send-specific selectors 66/67 use the distinct native Send purpose;
they do not relabel the existing selectors 63/65. Approval material and budget
facts must be fetched from their versioned owner operations before a decision.
Unknown writes keep their reservation until evidence allows settlement.

## Selectors

These selector values are declared constants in `human.rs`. The operation-specific
`HumanRequest`/`HumanResponse` variants and encoder/decoder arms in that file
define the payload layout; the names below are navigation anchors, not JSON
method names. Version suffixes are wire distinctions and must be preserved.

| Selector | Constant / registry anchor | Purpose |
| --- | --- | --- |
| 1 | <a id="prepare"></a>`PREPARE` | Prepare. |
| 2 | <a id="submit"></a>`SUBMIT` | Submit. |
| 3 | <a id="track"></a>`TRACK` | Track. |
| 4 | <a id="receipt-lookup"></a>`RECEIPT_LOOKUP` | Receipt lookup. |
| 5 | <a id="registry"></a>`REGISTRY` | Registry. |
| 6 | <a id="balance"></a>`BALANCE` | Balance. |
| 7 | <a id="head"></a>`HEAD` | Head. |
| 8 | <a id="evidence"></a>`EVIDENCE` | Evidence. |
| 9 | <a id="approval-list"></a>`APPROVAL_LIST` | Owner-bound approval lookup, facts or decision. |
| 10 | <a id="approval-get"></a>`APPROVAL_GET` | Owner-bound approval lookup, facts or decision. |
| 11 | <a id="approval-approve"></a>`APPROVAL_APPROVE` | Owner-bound approval lookup, facts or decision. |
| 12 | <a id="approval-reject"></a>`APPROVAL_REJECT` | Owner-bound approval lookup, facts or decision. |
| 13 | <a id="account-sequence"></a>`ACCOUNT_SEQUENCE` | Account sequence. |
| 20 | <a id="identity-resolve"></a>`IDENTITY_RESOLVE` | Identity resolve. |
| 21 | <a id="lease-map"></a>`LEASE_MAP` | Lease map. |
| 22 | <a id="owner-validate"></a>`OWNER_VALIDATE` | Owner validate. |
| 23 | <a id="owner-install"></a>`OWNER_INSTALL` | Owner install. |
| 24 | <a id="agent-list"></a>`AGENT_LIST` | Managed agent state, lifecycle, policy or budget operation. |
| 25 | <a id="agent-get"></a>`AGENT_GET` | Managed agent state, lifecycle, policy or budget operation. |
| 26 | <a id="agent-control"></a>`AGENT_CONTROL` | Managed agent state, lifecycle, policy or budget operation. |
| 27 | <a id="agent-limit"></a>`AGENT_LIMIT` | Managed agent state, lifecycle, policy or budget operation. |
| 28 | <a id="agent-journey"></a>`AGENT_JOURNEY` | Managed agent state, lifecycle, policy or budget operation. |
| 29 | <a id="agent-archive"></a>`AGENT_ARCHIVE` | Managed agent state, lifecycle, policy or budget operation. |
| 30 | <a id="capability-install"></a>`CAPABILITY_INSTALL` | Capability install. |
| 31 | <a id="agent-context"></a>`AGENT_CONTEXT` | Managed agent state, lifecycle, policy or budget operation. |
| 32 | <a id="agent-budget-state"></a>`AGENT_BUDGET_STATE` | Managed agent state, lifecycle, policy or budget operation. |
| 33 | <a id="agent-key-policy"></a>`AGENT_KEY_POLICY` | Managed agent state, lifecycle, policy or budget operation. |
| 34 | <a id="agent-session-snapshot"></a>`AGENT_SESSION_SNAPSHOT` | Managed session binding, lifecycle or restriction. |
| 35 | <a id="agent-session-suspend"></a>`AGENT_SESSION_SUSPEND` | Managed session binding, lifecycle or restriction. |
| 36 | <a id="agent-session-bind"></a>`AGENT_SESSION_BIND` | Managed session binding, lifecycle or restriction. |
| 37 | <a id="agent-lifecycle-publish"></a>`AGENT_LIFECYCLE_PUBLISH` | Managed agent state, lifecycle, policy or budget operation. |
| 38 | <a id="agent-session-restrict"></a>`AGENT_SESSION_RESTRICT` | Managed session binding, lifecycle or restriction. |
| 39 | <a id="native-fee-policy"></a>`NATIVE_FEE_POLICY` | Native fee policy. |
| 40 | <a id="session-fee-state"></a>`SESSION_FEE_STATE` | Session fee state. |
| 41 | <a id="session-seed-prepare"></a>`SESSION_SEED_PREPARE` | Session seed prepare. |
| 42 | <a id="account-state"></a>`ACCOUNT_STATE` | Account state. |
| 43 | <a id="agent-owner-rotated"></a>`AGENT_OWNER_ROTATED` | Managed agent state, lifecycle, policy or budget operation. |
| 44 | <a id="subject"></a>`SUBJECT` | Subject. |
| 45 | <a id="operator"></a>`OPERATOR` | Operator. |
| 46 | <a id="approval-list-facts-v2"></a>`APPROVAL_LIST_FACTS_V2` | Owner-bound approval lookup, facts or decision. |
| 47 | <a id="approval-get-facts-v2"></a>`APPROVAL_GET_FACTS_V2` | Owner-bound approval lookup, facts or decision. |
| 48 | <a id="approval-budget-after-v2"></a>`APPROVAL_BUDGET_AFTER_V2` | Owner-bound approval lookup, facts or decision. |
| 49 | <a id="managed-evidence-by-digest-v2"></a>`MANAGED_EVIDENCE_BY_DIGEST_V2` | Managed evidence by digest v2. |
| 50 | <a id="native-approval-list-facts-v2"></a>`NATIVE_APPROVAL_LIST_FACTS_V2` | Owner-bound approval lookup, facts or decision. |
| 51 | <a id="native-approval-get-facts-v2"></a>`NATIVE_APPROVAL_GET_FACTS_V2` | Owner-bound approval lookup, facts or decision. |
| 52 | <a id="native-effect-approval-list-v3"></a>`NATIVE_EFFECT_APPROVAL_LIST_V3` | Native effect approval, preserving the declared material/budget version. |
| 53 | <a id="native-effect-approval-get-v3"></a>`NATIVE_EFFECT_APPROVAL_GET_V3` | Native effect approval, preserving the declared material/budget version. |
| 54 | <a id="native-effect-approval-decide-v3"></a>`NATIVE_EFFECT_APPROVAL_DECIDE_V3` | Native effect approval, preserving the declared material/budget version. |
| 55 | <a id="native-effect-approval-material-v4"></a>`NATIVE_EFFECT_APPROVAL_MATERIAL_V4` | Native effect approval, preserving the declared material/budget version. |
| 56 | <a id="native-effect-approval-budget-v4"></a>`NATIVE_EFFECT_APPROVAL_BUDGET_V4` | Native effect approval, preserving the declared material/budget version. |
| 57 | <a id="agent-budget-proof-v4"></a>`AGENT_BUDGET_PROOF_V4` | Managed agent state, lifecycle, policy or budget operation. |
| 58 | <a id="native-program-approval-list-v5"></a>`NATIVE_PROGRAM_APPROVAL_LIST_V5` | Version 5 native program approval material, facts or decision. |
| 59 | <a id="native-program-approval-get-v5"></a>`NATIVE_PROGRAM_APPROVAL_GET_V5` | Version 5 native program approval material, facts or decision. |
| 60 | <a id="native-program-approval-material-v5"></a>`NATIVE_PROGRAM_APPROVAL_MATERIAL_V5` | Version 5 native program approval material, facts or decision. |
| 61 | <a id="native-program-approval-budget-v5"></a>`NATIVE_PROGRAM_APPROVAL_BUDGET_V5` | Version 5 native program approval material, facts or decision. |
| 62 | <a id="native-program-approval-decide-v5"></a>`NATIVE_PROGRAM_APPROVAL_DECIDE_V5` | Version 5 native program approval material, facts or decision. |
| 63 | <a id="native-send-preview-v1"></a>`NATIVE_SEND_PREVIEW_V1` | Versioned native preview, owner context or retained envelope. |
| 64 | <a id="native-owner-context-v1"></a>`NATIVE_OWNER_CONTEXT_V1` | Versioned native preview, owner context or retained envelope. |
| 65 | <a id="native-journey-envelope-v1"></a>`NATIVE_JOURNEY_ENVELOPE_V1` | Versioned native preview, owner context or retained envelope. |
| 66 | <a id="native-send-owner-preview-v1"></a>`NATIVE_SEND_OWNER_PREVIEW_V1` | Versioned native preview, owner context or retained envelope. |
| 67 | <a id="native-send-envelope-v1"></a>`NATIVE_SEND_ENVELOPE_V1` | Versioned native preview, owner context or retained envelope. |

Unassigned values are not extension slots for an independent client. Use the
matching versioned peer codec on both sides; selector allocation belongs to the
owner contract.

## Receipt lookup wire layout

`ReceiptLookupAnswer` begins with one u8 presence tag:

| Tag | Remaining bytes |
| --- | --- |
| 0 | Absent; no fields |
| 2 | Submitted; 32-byte transaction hash, then u64 nonce |
| 1 | Found; the complete `ReceiptLookupFrame` below |

Integers use the canonical wire encoder's big-endian encoding. Variable byte
arrays have a u32 byte length followed by exactly that many bytes. Fixed digests
and keys have exactly 32 bytes without a length prefix. A Found frame has this
exact order (a standalone `ReceiptLookupFrame::encode` omits the presence tag):

| Order | Field | Wire type |
| --- | --- | --- |
| 1 | `canonical_bytes` | Nonempty length-prefixed receipt bytes |
| 2 | `authorised_batch` | Five fixed digests in order: `batch_id`, `asset`, `previous_state_root`, `resulting_state_root`, `sequencer_public_key` |
| 3 | `verification_level` | One u8 Human verification code |
| 4 | `tx_hash` | Fixed digest |
| 5 | `receipt_hash` | Fixed digest |
| 6 | `block_height` | u64 |
| 7 | `rung` | One u8: 0 Submitted, 1 Receipted, 2 Committed |
| 8 | `signed_receipt` | Nonempty length-prefixed signed receipt bytes |
| 9 | `receipt_key_id` | Fixed digest |
| 10 | `public_key` | Fixed signing public key |
| 11 | `first_height` | u64 inclusive lower key bound |
| 12 | `last_height` | u64 inclusive upper key bound |
| 13 | `inclusion_height` | u64 |

Human verification codes are 1 SEQUENCER_SIGNED, 2 BATCH_INCLUDED,
3 STATE_PROVEN, 4 CHECKPOINT_FINALISED and 5 SETTLEMENT_ANCHORED. Codes 4 and 5
are retained historical migration metadata. They confer no extra chain authority;
the actual achieved rung is carried separately, and requests above Committed
are refused. UNVERIFIED, unknown tags/codes, empty receipt material, truncation,
trailing bytes and oversized frames are rejected by the codec. A decodable
frame still requires the consumer's cryptographic and binding checks.

## Producer, transport and consumer

The authenticated Human peer supplies the persisted submission selector and the
expected transaction hash. The owner reloads exact signed EVM bytes from Outbox,
checks the hash and obtains the canonical node receipt and current key intervals.
It verifies the receipt signature and coordinates, stores the receipt with its
indexes, and uses verified finality evidence to build the frame. A Unix peer is
identified by the configured UID/principal/tenant mapping; receipt fields do not
authenticate a peer or manufacture protocol authority.

The API dispatcher consumes the frame and verifies all of these bindings:

- `canonical_bytes` equals `signed_receipt`, and its SHA-256 equals `receipt_hash`.
- `tx_hash` equals the receipt's activity ID and the caller's expected transaction.
- `block_height` is nonzero, equals `inclusion_height`, and matches the receipt's
  batch/height coordinate.
- The receipt's batch, asset and state roots equal `authorised_batch`.
- `public_key` equals its authorised sequencer public key, the signature verifies,
  and `receipt_key_id` and the inclusive height interval admit the receipt.
- A found receipt's actual `rung` is at least Receipted. A stronger requested rung
  is reported below the request until verified evidence establishes it.

The [API contract](agent-api.md) specifies the complete `program.receipt` JSON
projection. The integration proof carries node-produced canonical receipt bytes
through durable storage, this codec, a loopback TLS HTTP peer, the production
API dispatcher and the real SDK, checking every projected field. It also checks
Absent and Submitted, and refuses altered hashes, keys, roots, heights and bytes.

## Restart behavior

On restart the owner reopens its Store and restores the outbox by transaction
hash. Receipt bytes, indexes and finality records remain durable; cache readiness
and process-local connection generations do not. The proof stops and reopens
these components and checks that the receipt answer is byte-for-byte unchanged.
Unknown submissions retain exact bytes and require authenticated resolution.

Health and authority credentials are separate: loopback `/healthz` uses
`LAYERX_AGENT_PROGRAM_BEARER_TOKEN`; outbound registry calls use
`LAYERX_AGENT_HUMAN_AUTHORITY_BEARER`. Equality is a startup refusal. See
[agent-config.md](agent-config.md) for the actual environment and readiness API.
