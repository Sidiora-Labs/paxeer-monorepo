# Hosted authority

Paxeer mainnet-beta is one BFT network, with EVM chain id `125` (`0x7d`).
The kernel is part of the network and proposes blocks. A receipt is final when
its block is committed. A signed execution receipt alone does not establish
that commitment.

`layerx-receipt-authority`, from crate `layerx-platform-authority`, is a hosted
verification service. It reads node receipts, checks receipt keys and evidence,
and serves facts to hosted consumers. It does not establish a second consensus
or settlement domain. The pre-merge standalone replica and LNI deployment is
retired; do not recreate it to obtain network finality.

Public JSON-RPC uses the root path of the operator-provided node origin.
See [public RPC](../reference/public-rpc.md). The hosted authority's TLS HTTP
routes below are an internal service contract, distinct from those public node
endpoints. Hosted principal and session authentication is described in
[Hosted identity](identity.md).

---

## Committed-block authority

The maintained committed-state implementation is in
`services/platform/hosted/authority/src/committed.rs`, with callers in
`src/human.rs`. It uses the configured loopback node as the trust source for
committed BFT headers:

1. Require `eth_chainId` to equal `125`, then read the head with
   `eth_blockNumber` and `eth_getBlockByNumber`.
2. Obtain `px_getCommit` material for that height and bind the Comet commit's
   block hash and header app hash to the EVM block projection. The header at
   height `H` authenticates application state at `H - 1`; `H` must exceed one.
3. Re-read each selected receipt through `pax_sequencerReceipt`. Require the
   `committed` rung, identical receipt bytes, a nonzero block hash matching the
   block at the returned height, and a block height no later than the proved
   state height.
4. For native state, query `/store/lxphost/key` with `abci_query`, `prove: true`,
   at the pinned state height. Verify the exact key, value and height, then the
   `ics23:iavl` and `ics23:simple` proofs through the `lxphost` store root to the
   committed app hash. Recheck the pinned head hash and app root during use.

This implementation trusts its configured node's committed header; it does not
independently validate a validator quorum from an arbitrary HTTP response.
Node identity and access to the loopback endpoint are part of the trust boundary.

The human authority routes `/v1/agent/identity`, `/v1/agent/key-policy`, and
`/v1/agent/capability-scope` bind their retained receipt evidence to this committed
head before deriving policy. Native identity state uses module `7` and a
membership proof. Finality documents carry `height`, `block_hash`, `app_hash`,
and `state_height`. Failures include `committed_header_unavailable`,
`committed_receipt_evidence_unavailable`, and `identity_state_proof_unavailable`;
they refuse the request rather than inventing state.

Human requests require a dedicated bearer and tenant/principal binding.
Sub-chains are tenant domains in the database, so this binding selects a
namespace within the same network. It does not select a separate chain.

## Node receipts and keys at height

`src/chain_receipt.rs` reads canonical receipt bytes using
`pax_sequencerReceipt` with a `0x`-prefixed transaction hash. It checks the
response transaction identity, receipt context/key id, signing key and positive
receipt height. A null result, or a `submitted` result without a receipt, is
unknown. Both `receipted` and `committed` results can supply signed bytes;
only the latter supplies the commitment used above.

The service loads a protected JSON array from
`LAYERX_AUTHORITY_RECEIPT_KEYS_FILE`. Each entry contains:

| Field | Encoding |
| --- | --- |
| `keyId` | Base64 of a nonzero 32-byte key id; alias `key_id` accepted |
| `publicKey` | Base64 of a nonzero 32-byte public key; alias `public_key` accepted |
| `firstHeight` | Inclusive unsigned integer; alias `first_height` accepted |
| `lastHeight` | Inclusive unsigned integer, at least `firstHeight`; alias `last_height` accepted |

The list must be nonempty and key ids unique. `src/trust.rs` additionally
requires the key id to equal SHA-256 of the public key. For native receipts,
lookup selects a pinned context id and signing key whose range covers the
receipt height; signed batch evidence is also verified against its key's range.
Do not accept a response's `signingKey` as its own trust anchor. The node/client
receipt-key contract is `px_getReceiptKeys` (implemented by
`services/agent/crates/layerx-client/src/sequencer.rs`); its RPC representation
must be decoded before use as the authority's Base64 file format.

Keys are loaded at process startup. After updating reviewed key material,
restart the service to load it and retain historical ranges needed for replay.
Human evidence retained under `LAYERX_AUTHORITY_STATE_ROOT` is reverified when
read (`src/human.rs`); committed policy reads obtain fresh node commitment.

The ramp's current transaction verifier is
`services/platform/ramps/toolkit/src/clients.rs`: it authenticates native receipts,
raw transactions, receipt keys and signed logs. It does not consume the old
eight-field hosted `AuthorizedBatch` response as its settlement authority.

---

## Deployment

The binary entrypoint is `/usr/local/bin/layerx-receipt-authority`
(`docker/platform-authority/Dockerfile`). The image runs as user/group
`4020:4020` through `layerx-runtime-clock`. Deploy the authority separately
and provision its TLS listener, which defaults to port `9445`.

Supply the node receipt endpoint and protected receipt-key file required by
`config()` in `src/main.rs`. There is no LNI socket requirement. For the chain URL, use a
loopback HTTP node endpoint at its root path. This service's loader accepts
`127.0.0.1`, `localhost`, or `[::1]` with an explicit port.

## Configuration

| Key | Current service behavior |
| --- | --- |
| `LAYERX_AUTHORITY_LISTEN` | Default `0.0.0.0:9445` |
| `LAYERX_AUTHORITY_TLS_CERT_DER` | Required DER server certificate |
| `LAYERX_AUTHORITY_TLS_KEY_DER` | Required DER private key |
| `LAYERX_AUTHORITY_CLIENT_CA_DER` | Optional client CA; permits clients without a certificate, but checks a presented certificate |
| `LAYERX_AUTHORITY_TOKEN_FILES` | Required colon-separated bearer-token files for general routes |
| `LAYERX_AUTHORITY_CHAIN_RPC_URL` | Required loopback HTTP node endpoint for receipt and committed-state reads |
| `LAYERX_AUTHORITY_RECEIPT_KEYS_FILE` | Required protected receipt-key array, at most 256 KiB |
| `LAYERX_AUTHORITY_PROTOCOL_NETWORK_ID` | Required nonzero `u32` protocol network id; checked against signed evidence |
| `LAYERX_AUTHORITY_NETWORK_ID` | Required deployment label, echoed in responses; distinct from EVM chain id |
| `LAYERX_AUTHORITY_WIRE_VERSION` | Defaults to, and must equal, the built protocol version |
| `LAYERX_AUTHORITY_EVIDENCE_READ_TOKEN_FILE` | Optional protected file, 32–4096 printable bytes without whitespace; distinct from general and replica bearers; permits only the batch-evidence relay |

TLS files must be regular files of 1–65536 bytes. General token files contain
1–4096 printable non-whitespace bytes after trailing CR/LF removal. Protected
files must have canonical absolute paths, be regular files owned by the process
user, and deny all group/other permissions (`src/protected.rs`).

Human authority is enabled with `LAYERX_AUTHORITY_HUMAN_AGENT_TOKEN_FILE` and
requires `LAYERX_AUTHORITY_PRINCIPAL_POLICY_FILE`,
`LAYERX_AUTHORITY_MODULE_REGISTRY_FILE`, `LAYERX_AUTHORITY_CORE_CLOCK_HORIZON`,
`LAYERX_AUTHORITY_STATE_ROOT`, `LAYERX_AUTHORITY_HUMAN_AGENT_TENANT`, and
`LAYERX_AUTHORITY_HUMAN_AGENT_PRINCIPAL`. The optional identity binding client
is configured in `src/human/dynamic.rs`.

## Compatibility batch evidence

The current binary still requires compatibility configuration:
`LAYERX_AUTHORITY_REPLICA_URL`,
`LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE`,
`LAYERX_AUTHORITY_REPLICA_ID`, `LAYERX_AUTHORITY_SEQUENCER_ID`,
`LAYERX_AUTHORITY_SEQUENCER_PUBLIC_KEY`, `LAYERX_AUTHORITY_FIRST_BATCH`, and
`LAYERX_AUTHORITY_LAST_BATCH`. The URL must be loopback HTTP; identities and key
are nonzero 64-hex values; the batch range begins above zero and is ordered.
These names remain in `src/main.rs`. They are compatibility inputs, not a
second source of BFT finality or instructions to deploy the retired topology.

The general-bearer routes below still obtain signed receipt bytes from the node
and request batch evidence from the configured compatibility HTTP source:

| GET route | Result |
| --- | --- |
| `/v1/authorized-batches/by-activity/{activity_id}` | Verified batch facts |
| `/v1/authorized-batches/wait-by-activity/{activity_id}` | Same facts, with bounded polling for receipt/evidence publication |
| `/internal/v1/activities/{activity_id}/authority` | Same facts without publication polling |
| `/v1/batches/{batch_id}/receipt-authority?receipt_digest={digest}` | Evidence body relayed unchanged for upstream HTTP 200 or 404; also permits the evidence-read bearer |

Activity ids must be nonzero 64-hex strings. `receipt_locator` hashes the
canonical unsigned receipt, not its signed serialization. The evidence parser
checks the replica identity, selected receipt key, signed header, canonical
Merkle inclusion, protocol version, sequence range, execution batch identity,
and receipt outcome. Maintenance evidence has additional batch-identity checks
in `src/lib.rs`. No partial facts are returned on refusal.

The by-activity success object contains ten fields:

| Field | Meaning |
| --- | --- |
| `activity_id` | Requested activity id |
| `receipt` | Canonical signed receipt bytes, hex without `0x` |
| `batch_id` | Re-derived execution batch identity, hex |
| `asset` | Receipt asset, hex |
| `previous_state_root`, `resulting_state_root` | Signed batch header roots, hex |
| `sequencer_public_key` | Selected pinned receipt key, hex |
| `network_id` | Deployment label |
| `protocol_network_id` | Numeric protocol network id |
| `wire_version` | Configured wire version string |

Maintenance-backed results additionally carry `batch_evidence`. These responses
have no committed-block field: successful batch verification alone is not a
committed-block proof. `AuthorityFacts` in the library also carries
`global_sequence` and `batch_number`; those two fields are absent from this HTTP
object.

---

## Refusals

General HTTP refusals use `{"error":{"code":...,"retry":"never"}}`, or
`retry: "after"` with `retry_after_seconds` and `Retry-After`.

| HTTP | Code | Condition |
| --- | --- | --- |
| 401 | `identity_required` | Missing or unrecognized bearer |
| 400 | `invalid_activity_id` | Activity id is malformed or zero |
| 404 | `unknown_activity` | Node has no signed receipt for the activity |
| 502 | `sequencer_key_mismatch` | Receipt key/signature does not match the pinned keys |
| 503 | `receipt_source_unavailable` | Node lookup or receipt response validation failed; retry after 5 seconds |
| 503 | `replica_evidence_unavailable` | Compatibility evidence source returned 404; retry after 1 second |
| 503 | `replica_unavailable` | Evidence source transport failed or returned another status; retry after 5 seconds |
| 502 | `evidence_refused` | Receipt, header, batch identity or inclusion verification failed |
| 503 | `receipt_network_mismatch` | Signed header protocol network differs; retry after 5 seconds |
| 503 | `state_persistence_unavailable` | Enabled human evidence retention failed; retry after 5 seconds |
| 400 | `invalid_request` | Malformed HTTP framing or relay identifiers/query |
| 405 | `method_not_allowed` | Method other than GET |
| 404 | `not_found` | Unrecognized route or disallowed query |

Library failures retain their specific `EvidenceRefusal` variants, including
`ReceiptDecode`, `ReceiptShape`, `ActivityMismatch`, `ReplicaDocument`,
`ReplicaIdentity`, `SequencerKey`, `EvidenceEncoding`, `Inclusion`,
`ProtocolVersion`, `SequenceRange`, `BatchIdentity`, and `Receipt`. The general
HTTP surface maps them to `evidence_refused`. Human routes have their own policy
and committed-evidence refusals in `src/human.rs`.

## Readiness

`GET /livez` returns `200 {"live":true}` without dependency checks.
`GET /readyz` checks both the compatibility evidence source and the node chain
identity. Evidence-source 200 or 404 is reachable; node chain id must be `125`.
Both dependencies must be ready. The body contains `ready`, `network_id`,
`protocol_network_id`, and `wire_version`; a failed check returns 503 and
`Retry-After: 5`.

Readiness establishes dependency reachability and chain identity. It does not
verify a particular receipt, its committed block, or a native state proof.

## Tests

The maintained tests include `tests/chain_receipts.rs` for node receipt lookup,
unpinned-key refusal, pinned key snapshots, and startup without an LNI socket;
other authority tests exercise committed identity, capability and key-policy
evidence, receipt-key rotation with retention and replay, maintenance identity,
native outcomes, human TLS and proof exports.
All paths are under `services/platform/hosted/authority/`.

`make platform-test-authority` runs the crate through
`tools/runtime/run-with-clock.sh`; `platform-test-trusted-boundary` includes it.

[Hosted identity](identity.md) · [Home](../index.md)
