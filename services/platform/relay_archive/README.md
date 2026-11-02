# Relay/archive node

The relay/archive role lets an independent operator follow the LayerX kernel
domain of a running Paxeer X Network deployment, retain its complete canonical byte history, serve public reads, and
forward users' original signed activities. It does not order activities,
execute state transitions, hold a sequencer key, or participate in a consensus
or finality quorum.

This directory retains three Python modules:

- `__init__.py` identifies the package.
- `peers.py` implements bounded peer discovery with trust-pin checks and
  address validation.
- `protocol.py` defines configuration, canonical synchronization types,
  validation, and the native archive-codec interface.

The sections below describe the trust and protocol contracts represented by
these modules. The standalone runtime and its installation and deployment
assets have been retired.

## Trust and network model

An operator must obtain these values independently of every synchronization
server and put them in the configuration:

- protocol `network_id`;
- SHA-256 of the exact signed genesis manifest;
- sequencer identifier and Ed25519 public key;
- optionally, the first and last batch authorized for that sequencer.

An upstream's `/v1/sync/network` document is discovery metadata, not a trust
anchor. The daemon refuses a different network, genesis digest, sequencer
identity, signature, batch number, predecessor, committed section root, or
canonical byte encoding. It atomically stores the raw genesis, snapshot,
batches, activities, receipts, and maintenance records before advancing its
head. Restart resumes at the durable next-batch position without accepting a
gap or conflicting bytes.

Peer discovery extends read synchronization only. A compatible `/v1/peers`
document must carry the same four trust pins, a bounded expiration, and a
bounded list of credential-free HTTPS origins. Discovered peers are first
probed and pin-checked; they are never added to submission failover. DNS is
resolved and checked again immediately before use, and connections are made to
the checked address while TLS authenticates the original hostname. Redirects,
proxy environment variables, link-local, private, reserved, and loopback
advertisements are refused. Literal loopback HTTP is available only for an
operator-configured development seed when `allow_loopback_dev` is explicitly
true, and loopback peers are never publicly advertised.

## Configuration

`protocol.py` defines `RelayConfig` and validates configuration values,
including independently pinned network and sequencer identities.
`upstreams` are read-only synchronization origins.
`submission_upstreams` are the only forwarding destinations and may name an
origin, `/v1/activities`, or `/rpc`; URLs cannot carry credentials, query
strings, or fragments. Read discovery never mutates that list.

An origin colocated with the sequencer may additionally configure
`source_log`. To accept activity submissions over the real local LNI it must
configure both `source_lni_socket` and `source_submission_token_file`. Those
paths are private configuration and are never advertised.

## Canonical synchronization contract

All numeric batch cursors in paths are canonical unsigned decimal. IDs and
digests are 64 lowercase hexadecimal characters. Unknown routes are refused.
Public synchronization and archive reads do not accept credentials.

| Method and route | Contract |
| --- | --- |
| `GET /v1/sync/network` | JSON `{version, network_id, genesis_sha256, snapshot_sha256, sequencer_id, sequencer_public_key, first_batch, last_batch}`. Consumers compare the pins before downloading history. |
| `GET /v1/sync/genesis` | Exact signed genesis-manifest octets. |
| `GET /v1/sync/snapshot` | Exact canonical genesis-snapshot octets. |
| `GET /v1/sync/head` | JSON `{version, network_id, genesis_sha256, head_batch, head_batch_id, head_raw_sha256, next_batch}` from the durable head. |
| `GET /v1/sync/batches/N` | Exact canonical bytes of batch `N`, only after native verification and durable commit. |
| `GET /v1/peers` | A bounded, expiring compatible-peer advertisement described below. |
| `GET /v1/sync/readiness` | JSON durable head plus `ready`, `recovered_current_process`, `sequencer_public_key` and `freshness`; HTTP 503 until synchronization is current within the freshness budget. |
| `GET /healthz` | Process liveness only. |
| `GET /readyz` | Ready only after pinned bootstrap and durable synchronization state are available. |

Raw synchronization responses use their canonical media type and include
`Content-Length`, a quoted SHA-256 `ETag`, and `X-Content-SHA256`. Batch responses
also include `X-LayerX-Batch`. Servers do not redirect synchronization clients.

Complete public history is cursor-paginated under:

```text
GET /v1/history/activities?cursor=&limit=&actor=&account=&module=&batch=
GET /v1/history/activities/ACTIVITY_ID
GET /v1/history/receipts?cursor=&limit=&batch=
GET /v1/history/receipts/ACTIVITY_ID
GET /v1/history/batches?cursor=&limit=
GET /v1/history/batches/BATCH_NUMBER
GET /v1/history/maintenance?cursor=&limit=&batch=
GET /v1/history/maintenance/CURSOR
```

Pages are `{version:1, items:[...], next_cursor:string|null}`. Activity and
receipt details include `canonical_hex`; maintenance details include
`receipt_hex`; exact batch bytes remain available at the synchronization route.
The canonical bytes remain the authority. A cryptographic inclusion label means
only that native verification proved inclusion in the committed canonical
batch. It does not claim that this non-executing role replayed execution or
independently established settlement finality.

`POST /v1/activities` accepts only bounded original signed activity octets and
an optional `Idempotency-Key`. `POST /rpc` serves archive read methods and
forwards `lx_sendActivity`. The relay never signs, decodes and rebuilds, or
otherwise changes a user's activity. It durably binds idempotency to the exact
bytes, retries the same bytes only after transport or availability failure,
preserves a definitive upstream refusal, and reports an unresolved transport
outcome as unknown. Incoming `Authorization`, `LayerX-Key` or `X-LayerX-Key` is forwarded only
to configured submission endpoints; no credential is sent to a discovered
read peer.

The peer document has exactly this versioned shape:

```json
{
  "version": 1,
  "network_id": 402,
  "genesis_sha256": "64 lowercase hex",
  "sequencer_id": "64 lowercase hex",
  "sequencer_public_key": "64 lowercase hex",
  "generated_at": 0,
  "expires_at": 0,
  "peers": [
    {"url": "https://relay.example.net", "expires_at": 0}
  ]
}
```
