# Hosted authority

`layerx-receipt-authority`, from crate `layerx-platform-authority`, serves
verified receipt and batch evidence over TLS. Its entry point is
[`src/main.rs`](../../services/platform/hosted/authority/src/main.rs), and its
verification core is [`src/lib.rs`](../../services/platform/hosted/authority/src/lib.rs).

Paxeer mainnet-beta runs `paxd` with EVM chain id **125** (`0x7d`). The LayerX
kernel inside `paxd` executes and orders each transaction and builds each block;
validators check and commit it. A receipt is final when its block is committed. Hosted authority supplies verified facts
for hosted consumers; it does not create another settlement or finality layer.
See [Public RPC](PublicRpc.md) for node access.

Receipt lookup uses node JSON-RPC. Batch evidence is read through a loopback
HTTP adapter when the optional batch evidence origin group is configured.
Without that group, activity reads verify signed node receipts and committed
transaction landing. With it, they verify signed batch inclusion as before;
consumers requiring finality also bind that evidence to a committed block.

## Deployment

The [Dockerfile](../../docker/platform-authority/Dockerfile) starts
`/usr/local/bin/layerx-runtime-clock --runtime-dir /tmp -- /usr/local/bin/layerx-receipt-authority`
as user `4020:4020`. The default listener is `0.0.0.0:9445`.
Provide the chain URL, protected receipt-key file, TLS material and service
credentials listed below. The batch-evidence adapter is optional as a group. The native node deployment
is described in [Hosted node](HostedNode.md).

The deployment's server certificate and private key come from the
`hosted-authority` row of the bring-up CA roster. Install the row's `cert.der`
and `key.der` files as `LAYERX_AUTHORITY_TLS_CERT_DER` and
`LAYERX_AUTHORITY_TLS_KEY_DER`; the roster defines the server name and its
`serverAuth` role.

[Hosted identity](HostedIdentity.md) authenticates hosted principals and
sessions. Authority's general bearer files authenticate calling services;
they do not replace receipt signatures or the governed receipt-key pins.

## Configuration

These keys are read by `config()` in `src/main.rs` and by
[`chain_receipt.rs`](../../services/platform/hosted/authority/src/chain_receipt.rs).
Unless marked optional or given a default, they are required.

| Key | Meaning |
| --- | --- |
| `LAYERX_AUTHORITY_LISTEN` | Socket address; default `0.0.0.0:9445`. |
| `LAYERX_AUTHORITY_TLS_CERT_DER` | Server certificate file in DER; bounded regular file, 1–65536 bytes. |
| `LAYERX_AUTHORITY_TLS_KEY_DER` | Server private key file in PKCS#8 DER, with the same file bound. |
| `LAYERX_AUTHORITY_CLIENT_CA_DER` | Optional client CA DER. Clients may omit a certificate; a presented certificate must validate against this CA. Bearer authentication still applies. |
| `LAYERX_AUTHORITY_TOKEN_FILES` | Colon-separated paths, one general bearer per file; at least one file. Each calling service receives its own credential. |
| `LAYERX_AUTHORITY_CHAIN_RPC_URL` | Loopback HTTP node JSON-RPC URL serving `pax_sequencerReceipt`. Use the node's root JSON-RPC path. |
| `LAYERX_AUTHORITY_RECEIPT_KEYS_FILE` | Protected JSON array of governed receipt-key entries, described below; loaded at startup. |
| `LAYERX_AUTHORITY_REPLICA_URL` | Optional origin group: loopback HTTP origin serving the batch-evidence route. Accepted hosts are `127.0.0.1`, `localhost`, and `[::1]`, with an explicit nonzero port. |
| `LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE` | Optional origin group: bearer credential presented to that evidence adapter. |
| `LAYERX_AUTHORITY_REPLICA_ID` | Optional origin group: nonzero 32-byte identity, encoded as 64 hex characters; must match evidence documents. |
| `LAYERX_AUTHORITY_EVIDENCE_READ_TOKEN_FILE` | Optional protected file: distinct 32–4096 printable bytes, without whitespace or a trailing newline. This credential can read only the batch-evidence relay. |
| `LAYERX_AUTHORITY_PROTOCOL_NETWORK_ID` | Nonzero `u32` checked against signed evidence; separate from the EVM chain id. |
| `LAYERX_AUTHORITY_NETWORK_ID` | Deployment identifier echoed in responses; 1–64 ASCII letters, digits, `-`, `_`, or `.`. |
| `LAYERX_AUTHORITY_WIRE_VERSION` | String matching the built state-commitment protocol version; defaults to that version, currently `3`. |
| `LAYERX_AUTHORITY_SEQUENCER_ID` | Required nonzero 64-hex compatibility authorization identity. Current receipt evidence is authorized using the governed key list. |
| `LAYERX_AUTHORITY_SEQUENCER_PUBLIC_KEY` | Required nonzero 64-hex compatibility public key. A single configured key does not replace the governed height ranges. |
| `LAYERX_AUTHORITY_FIRST_BATCH` | Optional origin group: compatibility authorization range start, greater than zero. |
| `LAYERX_AUTHORITY_LAST_BATCH` | Optional origin group: compatibility authorization range end, at least the start. |

The batch evidence origin group contains exactly
`LAYERX_AUTHORITY_REPLICA_URL`, `LAYERX_AUTHORITY_REPLICA_ID`,
`LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE`, `LAYERX_AUTHORITY_FIRST_BATCH`
and `LAYERX_AUTHORITY_LAST_BATCH`. All five enable the origin; omitting all five
starts the service from the node alone. A partial group refuses startup with
the group name and every missing setting. Supplied empty values must pass
normal validation. Other settings retain their requirements.

The deployment definition defaults to the node alone, with a commented
optional origin group of variables and operator inputs. Enabling it also
supplies `LAYERX_AUTHORITY_REPLICA_TOKEN_B64` for the container to stage the
bearer and export its file path. The container stages no origin bearer when
that encoded value is absent.

General and adapter bearer files trim trailing CR/LF and must contain
1–4096 printable non-whitespace ASCII bytes. Comparison is length-checked and
constant-time. Protected files use
[`protected.rs`](../../services/platform/hosted/authority/src/protected.rs):
an absolute canonical path, no symlink traversal, a regular file owned by
the process UID, no group/other permissions, and stable metadata while read.
The key-file bound is 256 KiB.

The optional human authority is configured by
[`human.rs`](../../services/platform/hosted/authority/src/human.rs). Enabling
`LAYERX_AUTHORITY_HUMAN_AGENT_TOKEN_FILE` also requires the
`LAYERX_AUTHORITY_` keys `HUMAN_AGENT_TENANT`, `HUMAN_AGENT_PRINCIPAL`,
`PRINCIPAL_POLICY_FILE`, `MODULE_REGISTRY_FILE`, `CORE_CLOCK_HORIZON`, and
`STATE_ROOT`. Partial configuration refuses startup. The state root must
already be a protected authority-owned directory. Optional identity binding
uses all three `LAYERX_AUTHORITY_` keys `IDENTITY_BINDING_SOCKET`,
`IDENTITY_BINDING_UID` and `IDENTITY_BINDING_GID`; partial binding configuration
refuses startup. Subject-bound requests require this binding client. See
[`human/dynamic.rs`](../../services/platform/hosted/authority/src/human/dynamic.rs).

## Receipt keys and committed blocks

`ChainReceipts::lookup` requests `pax_sequencerReceipt` with a `0x`-prefixed
transaction/activity hash. A null result or a `submitted` result without
receipt bytes is unknown to this lookup. `receipted` and `committed` results
carry canonical receipt bytes, `keyId`, `signingKey`, and a positive hex
`height`. The adapter checks the returned transaction and receipt activity
identities, signature, key identity, and applicable authorization range.
A `committed` result also carries `blockNumber` and a nonzero `blockHash`.

Obtain pins from the trusted, governed receipt-key source, retaining historical
keys and their inclusive height ranges. The node implements `px_getReceiptKeys`
in [`rpc/sequencer_receipt.go`](../../rpc/sequencer_receipt.go); its response is
`{"keys":[...]}`, with `keyId`, `publicKey`, `firstHeight`, and `lastHeight`.
A `signingKey` supplied beside a receipt is not a trust pin.

The authority's **file format differs from the JSON-RPC response**: it is a
bare array, with `keyId` and `publicKey` encoded as padded base64 of 32 bytes,
and numeric `firstHeight`/`lastHeight`. Snake-case aliases are accepted.
Do not copy the JSON-RPC envelope or its `0x` hex keys directly into this file.
The loader rejects empty lists, zero identities/keys, duplicate identities and
reversed ranges. `trust::current` also checks the key id is SHA-256 of the
public key. Restart the authority after installing an updated protected pin
file: the running process retains the startup list.

Committed human evidence uses
[`CommittedHeader`](../../services/platform/hosted/authority/src/committed.rs)
and `human::bind_committed`. It checks chain id 125, binds the EVM block hash
and app root to the loopback node's Comet commit, and authenticates state at
height **H−1** with the header at **H**. Receipt bytes must match the node's
committed lookup and the receipt block hash must match the corresponding
block. State membership checks verify the IAVL and multistore proofs under
that app root, then recheck that the pinned head has not changed. The configured
loopback node is the trust source for those committed headers; a signed batch
header alone is not a BFT commit proof.

The maintained ramp client uses native transaction receipt verification and
pinned keys through `resolve_transaction` and `authenticate_settlement` in
[`ramps/toolkit/src/clients.rs`](../../services/platform/ramps/toolkit/src/clients.rs).
It does not decode the authority's by-activity object as its payment receipt.

## Output fields

All routes below use `GET`. General service bearers authorize the by-activity
routes; `/livez` and `/readyz` are unauthenticated.

| Path | Response |
| --- | --- |
| `/v1/authorized-batches/by-activity/{activity_id}` | Verified facts and canonical receipt. |
| `/internal/v1/activities/{activity_id}/authority` | Same handler and response. |
| `/v1/authorized-batches/wait-by-activity/{activity_id}` | Same response; polls unknown receipts and node commitment or origin evidence at 25 ms intervals, with an eight-second deadline. |
| `/v1/batches/{batch_id}/receipt-authority?receipt_digest={digest}` | With an origin, relays HTTP 200/404 unchanged; without one, returns 410 `retired_http_contract`. Also accepts the restricted evidence-read bearer. |

With either configuration, the by-activity success object has these ten base
members. Without an origin, batch identity, asset and both roots are read from
the authenticated receipt; signed batch-header inclusion is available only
with an origin:

| Member | Representation |
| --- | --- |
| `activity_id` | Requested 64-hex activity id. |
| `receipt` | Canonical signed receipt bytes as unprefixed hex. |
| `batch_id` | Execution batch identity as unprefixed hex; signed receipt claim without an origin, re-derived with one. |
| `asset` | Receipt asset as unprefixed hex. |
| `previous_state_root` | Predecessor root as unprefixed hex; signed receipt in node-only form, verified header with an origin. |
| `resulting_state_root` | Successor root as unprefixed hex; signed receipt in node-only form, verified header with an origin. |
| `sequencer_public_key` | Receipt-authorized key from the pinned list, as unprefixed hex. |
| `network_id` | Configured deployment string. |
| `protocol_network_id` | Configured numeric protocol network id. |
| `wire_version` | Configured protocol-version string. |

Maintenance identities (`occupancy_maintenance_v2` or `batch_maintenance_v1`)
add `batch_evidence` from the verified adapter document when an origin is set.
Without an origin that member is absent. Consumers must handle
that form explicitly. Library `AuthorityFacts` additionally carries
`global_sequence` and `batch_number`; neither is a base HTTP response member.

Without an origin, activity verification requires a canonical signed receipt,
a pinned signing key within its inclusive receipt-height range and a committed
node result. It checks the transaction hash at the position in the landing
block, block number and hash, execution status, and the binding of that block
and its app root to the node's Comet commit. The committed block is rechecked
before answering. No origin batch proof, maintenance identity document or
retained replica record is created. Unknown immediate reads return 404;
uncommitted immediate reads return 503 `receipt_not_committed`. Wait reads
poll either condition until commitment or the eight-second deadline; an
exhausted response deadline returns 503 `receipt_source_unavailable`.

`GET /v1/protocol/account-state/head[?height={height}]` requires a general bearer.
With an origin, the existing pinned-identity committed-head document is relayed
unchanged. Without one, the node's committed state answers with `height` as a
decimal string, `block_hash`, `state_root` and
`commitment: "native_app_hash"`. Heights must be canonical positive decimals;
the node must supply consistent committed evidence at a height greater than
one, no later than its head. Malformed heights return 400 `invalid_height`;
unavailable or conflicting committed evidence returns 503
`committed_head_unavailable`. Native heads omit the origin document's
`authority_replica_id`, `receipt_hex`, `receipt_digest`, `observed_sequence`,
`observed_at`, `current`, `chain_evidence` and batch evidence. Readers requiring those members continue
to require an origin. Optional human operations that consume retained signed
batch evidence retain that requirement.

## Receipt digest

`receipt_locator` decodes the receipt, requires a protocol receipt, re-encodes
its unsigned form, and computes `receipt_digest` over those unsigned bytes.
That digest and the receipt's batch id select the adapter document through
`/v1/batches/{batch_id}/receipt-authority?receipt_digest={digest}`.

## Replica document

The retained JSON contract has `authority_replica_id`, `sequencer_public_key`,
and `batch_evidence`. The first two are 64-hex identities checked against
the configured adapter identity and receipt's authorized public key.
`batch_evidence` requires `header_hex`, `header_signature` (64 signature bytes
as hex), and `receipt_proof_hex` (a canonical index-aware Merkle proof).
Unknown fields are refused.

Optional `batch_identity` defaults to `{"kind":"historical"}`. The maintained
forms use `kind: "occupancy_maintenance_v2"` or `"batch_maintenance_v1"`,
`receipt_hex`, `receipt_proof_hex`, and optional `activity_receipts_hex`.
See `ReplicaBatchIdentity` in the verification core for the closed schema.

## Header signature, inclusion, and batch id

The verifier requires the requested activity id to match the receipt, verifies
the authorized header signature and receipt Merkle inclusion, and checks
protocol-version agreement and the global-sequence range. It re-derives the
execution batch id and checks it against the receipt. Maintained batches also
verify the maintenance receipt, its inclusion and position, activity count,
and occupancy-derived identity. Outcome verification uses those derived facts;
no partial facts are returned on refusal. Native owner/program receipts have
specialized checks in `verify_selected_authorized_receipt` and `native_state.rs`.

## Refusals

Refusals use `{"error":{"code":...,"retry":"never"|"after"}}`;
retryable responses include `retry_after_seconds` and `Retry-After`.

| HTTP | Code | Meaning |
| --- | --- | --- |
| 401 | `identity_required` | Missing or nonmatching service bearer. |
| 400 | `invalid_activity_id` | Activity id is not 64-hex or is zero. |
| 404 | `unknown_activity` | Node lookup has no usable receipt yet. |
| 502 | `sequencer_key_mismatch` | Receipt signing key/authorization fails the pinned-key checks. |
| 503 | `receipt_source_unavailable` | Node transport, JSON-RPC, receipt shape or other receipt-source verification failed; retry after 5 seconds. |
| 410 | `retired_http_contract` | Batch-evidence relay without a configured origin. |
| 503 | `receipt_not_committed` | Node receipt has no committed block yet; retry after 1 second. |
| 503 | `replica_evidence_unavailable` | Evidence adapter returned 404; retry after 1 second. |
| 503 | `replica_unavailable` | Evidence transport failed or answered a status other than 200/404; retry after 5 seconds. |
| 502 | `evidence_refused` | Receipt locator, evidence parse, signature, network authorization, inclusion, identity or outcome verification refused. |
| 503 | `receipt_network_mismatch` | By-activity header network check failed; retry after 5 seconds. Earlier trust checks can instead yield `evidence_refused`. |
| 503 | `state_persistence_unavailable` | Enabled human authority could not retain verified evidence; retry after 5 seconds. |
| 400 | `invalid_request` | Malformed HTTP or batch relay identifiers/query. |
| 405 | `method_not_allowed` | Method other than GET. |
| 404 | `not_found` | Unknown path or query on a non-relay activity route. |

The library distinguishes `ReceiptDecode`, `ReceiptShape`, `ActivityMismatch`,
`ReplicaDocument`, `ReplicaIdentity`, `SequencerKey`, `EvidenceEncoding`,
`Inclusion`, `ProtocolVersion`, `SequenceRange`, `BatchIdentity`, and `Receipt`.
These map to HTTP `evidence_refused`; authentication is a separate HTTP check.

## Readiness

`GET /livez` returns `200 {"live":true}` without dependency checks.
`GET /readyz` checks the node alone without an origin, or the adapter and node
concurrently with an origin, with a two-second readiness deadline. The adapter's zero batch/digest probe accepts HTTP 200 or
404; the node must answer the expected chain id 125. Readiness requires every configured dependency.
The body has `ready`, `network_id`, `protocol_network_id`, and `wire_version`.
Ready is HTTP 200; unavailable is HTTP 503 with `Retry-After: 5`.
Diagnostic headers are `X-LayerX-Authority-Replica` and
`X-LayerX-Authority-Chain`. The replica header is `not_configured` without an
origin and otherwise `ready`, `unavailable` or `unprobed`; the chain header uses
the latter three values. Readiness is not a proof that a particular receipt
is committed or that every authority route can produce evidence.

## Storage and restart

Node-only activity lookup re-reads node state and does not retain origin documents.
Ordinary receipt lookup re-reads node state; it needs no local receipt socket.
Enabled human authority retains verified receipt/evidence pairs as
`{activity_id}.json` beneath `LAYERX_AUTHORITY_STATE_ROOT`. Writes use a protected
temporary file, file sync, rename, and directory sync; conflicting content
for an existing activity refuses. Recovered evidence is verified again against
current startup pins and committed state when used. Preserve that directory,
the tenant/principal policy, and historical key ranges across restarts.
A changed principal policy produces `policy_changed_restart_required`.

## Tests

The [startup and node-only tests](../../services/platform/hosted/authority/tests/merged_start.rs)
cover configuration grouping, receipt members, commitment and landing refusals,
relay retirement, heads and the waiting deadline. Other source coverage lives in
[`tests/chain_receipts.rs`](../../services/platform/hosted/authority/tests/chain_receipts.rs)
(node receipt lookup, unpinned-key refusal, pin snapshots, startup without LNI),
[`tests/maintenance_identity.rs`](../../services/platform/hosted/authority/tests/maintenance_identity.rs)
(maintenance inclusion and identity refusals), and
[`tests/human_tls.rs`](../../services/platform/hosted/authority/tests/human_tls.rs)
(TLS, durable evidence and committed-state boundaries). Committed state-proof
coverage also lives in `tests/native_receipt_authority.rs`.

[Hosted identity](HostedIdentity.md) · [Hosted gateway](HostedGateway.md) · [Home](Home.md)
