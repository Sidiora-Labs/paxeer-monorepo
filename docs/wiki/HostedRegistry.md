# Hosted program registry

`layerx-platform-registry` builds the `layerx-program-registry` service and the
`layerx-cgroup-exec` build supervisor. The registry exposes program metadata,
published interfaces, source archives and reproducible-build verdicts. Program
execution belongs to the integrated kernel in `paxd`; the hosted registry is an
off-chain reader and builder. A receipt is final when its block is committed on
the BFT chain, whose EVM chain ID is `125` (`0x7d`). See
[Unified Network Architecture](UnifiedNetwork.md).

The current native read client is
[`programs_chain.rs`](../../services/platform/hosted/registry/src/programs_chain.rs).
It calls the Programs precompile at
`0x000000000000000000000000000000000000101d` using JSON-RPC at the node's root
URL. Public node names are `api1.mainnet-beta.paxeer.network` through
`api14.mainnet-beta.paxeer.network`, plus `api-mainnet-beta.paxeer.network`.
For example, `LAYERX_REGISTRY_CHAIN_RPC_URL=https://api1.mainnet-beta.paxeer.network`.
The hosted gateway's REST routes and `px_` methods are separate API transports;
their contract is
[`openrpc.json`](../../services/platform/hosted/gateway/openrpc.json).

The retained Kubernetes Service is `layerx-program-registry` in `layerx-testnet`,
on port `9420`. Its mTLS listener serves the registry routes below. The manifest
also declares a pod health port, `9421`. The checked-in manifest still contains
pre-merge transport settings; see [Current implementation limits](#current-implementation-limits)
before treating it as deployment instructions.

---

## Identity and credentials

The registry listener requires mTLS: `LAYERX_REGISTRY_TLS_CERT_DER`,
`LAYERX_REGISTRY_TLS_KEY_DER` and `LAYERX_REGISTRY_CLIENT_CA_DER` supply the server
certificate, private key and client CA. The dedicated HTTP health listener has
no client-credential requirement and serves only `GET /healthz`.

Request and source-publication bearer authorities remain distinct:

| Authority | Configuration | Routes |
| --- | --- | --- |
| Request | `LAYERX_REGISTRY_REQUEST_TOKEN_FILE` | Registry reads, deployment/head ingestion, source verification and metrics |
| Publication | `LAYERX_REGISTRY_PUBLICATION_TOKEN_FILE` | `/__registry/sources` |

`RegistryAuthority` requires the exact `Bearer ` prefix and compares token hashes
in constant time. The two configured authorities must differ. Token files must
be canonical absolute regular files, singly linked, with no world permission
bits; after trailing CR/LF removal a secret must contain 1–4096 bytes with no ASCII
control characters. See
[`auth.rs`](../../services/platform/hosted/registry/src/auth.rs) and
[`main.rs`](../../services/platform/hosted/registry/src/main.rs).

Both source publication and verification additionally require a `LayerX-Key`
header that the identity client resolves to a principal. Bearer authentication
alone does not establish that principal. Identity client configuration uses the
`LAYERX_REGISTRY_IDENTITY` prefix; the deployment supplies `_URL`, `_CA_DER`,
`_TOKEN_FILE`, `_CLIENT_IDENTITY_PKCS12` and `_CLIENT_IDENTITY_PASSWORD_FILE`.

The public node JSON-RPC URL does not use these hosted registry bearer roles.
The pre-merge independent receipt-authority provisioning procedure is retired;
remaining compatibility fields in the binary are described below as limits of
the current implementation.

---

## Routes

The HTTP parser accepts HTTP/1.1, strips query strings and bounds requests to
32 MiB. It rejects duplicate headers and `Transfer-Encoding`; POST, PUT and PATCH
require `Content-Length`. Responses use JSON, `Cache-Control: no-store` and
`Connection: close`. See
[`http.rs`](../../services/platform/hosted/registry/src/http.rs).

| Method | Path | Result |
| --- | --- | --- |
| GET | `/healthz` | Readiness verdict; success is `{"status":"ready","service":"program-registry"}` |
| GET | `/metrics` | Authenticated event-producer counters from the listener |
| POST | `/__registry/deployments` | Verify canonical deployment admission, journal it and rebuild the registry projection |
| POST | `/__registry/head` | Read a program's current native state head and refresh the journal's observed height/time |
| POST | `/__registry/sources` | Mirror an archive and pinned build plan under the publication authority |
| GET | `/v1/programs/registry` | List program IDs with the current verified head and freshness bound |
| GET | `/v1/programs/registry/{program_id}` | Read receipt-verified metadata, versions, lifecycle and value accounts |
| GET | `/v1/programs/registry/{program_id}/interface` | Read the published interface bound to the current verified version |
| POST | `/v1/programs/registry/{program_id}/source` | Rebuild mirrored source and compare it to the verified deployment |

Route program IDs encode 32 bytes as hex. Unsupported methods on known paths
return `405 method_not_allowed`; unknown paths return `404 not_found`.
[`routes.rs`](../../services/platform/hosted/registry/src/routes.rs) defines the
request and response shapes.

### Deployment and head reads

Deployment ingestion accepts raw canonical `DeploymentProof` bytes or JSON
containing only `admission`, an array of byte values. It verifies the historical
deployment against configured native chain trust and a current authenticated
protocol head, then checks the program's version history before appending the
sealed journal unit and rebuilding the projection. It records an existing
deployment without submitting a transaction. A transaction hash alone is not
an admission proof.

Success includes `program_id`, `code_hash`, `abi_version`, `version`,
`transaction_hash` and `state: deployed`. The transaction hash is the verified
activity ID. See [Registry deployment journal](RegistryDeploymentJournal.md)
for export and replay contracts. The separate `route_chain_deployment` helper
performs receipt/event/native-state lookups, but the service's request worker
routes deployment ingestion through `Registrar::route`.

Head ingestion accepts JSON containing only `program_id` (32-byte hex with an
optional `0x` prefix). It reads `eth_blockNumber`, `eth_getBlockByNumber`,
`getProgram(bytes32)` and `getStateHead(bytes32)` at the selected height. The
returned `state_root` is the Keccak-256 digest of the decoded native state-head
bytes. `observed_sequence` is the committed block height; `observed_at` is the
block timestamp in milliseconds. The response includes `observed`, `sequence`,
`observed_at` and `state_root`.

### Source publication and verification

Publication takes `source_uri`, `plan` (encoded `BuildPlan`) and `archive_hex`
(encoded `SourceArchive`). It returns `mirrored`, `source_uri` and `source_digest`.
The digest is SHA-256 of the encoded archive. Mirroring alone does not verify it.

Verification takes only `source_uri` and a 32-byte hex `source_digest`, plus an
`Idempotency-Key` of 16–128 ASCII letters, digits, dashes or underscores. Its
scope includes the resolved principal and program. Reusing a scoped key for a
different deployment/source request yields `409 idempotency_conflict`.

The verifier requires a current active deployment, reproduces the artifact in
the pinned environment, then rechecks that the deployment did not change during
the build. Success includes deployment provenance, source/environment digests,
the reproduced artifact digest, source status and pipeline information. A hash
mismatch yields `409 source_mismatch`, never a verified verdict. Source status
is `unpublished`, `verified` or `mismatch`.

---

## Record contracts

| Record | Durable representation / response |
| --- | --- |
| Chain deployment read | `{journal}/chain-deployments/{transaction_hash}.json`; helper metadata requires a matching authenticated deployment envelope; a conflicting retry is refused |
| Deployment proof unit | `{receipt_digest}.envelope`; sealed canonical deployment record and proof; complete legacy `.deployment`/`.admission` pairs can also load |
| Observed head | `{journal}/head`: `sequence\tobserved_at\n`, both nonzero |
| Native state cache | `{journal}/program-state/{program}.{digest}.program-state`; SHA-256 of the bytes must match the filename |
| Scan cursor | `{journal}/program-state/canonical.cursor`: `sequence\tordinal\n`; advancement refuses regression or a nonzero ordinal |
| Source mirror | `{mirror}/{digest}.archive`, `.plan` and `.uri` |
| Rebuild verdict | `{verified}/{program}-{version}.verified`; includes source/build plan and deployment provenance |
| Verification requests | `{journal}/verification-requests`; durable admission, artifact, publication and terminal-response state |

Registry reads include upgrade policy, lifecycle, versions, lifecycle history,
exit routes, value accounts, receipt verification and a current head with
`valid_through`. Interface reads include the canonical interface bytes and digest,
version, code hash, ABI, deployment receipt, source status and current head.
The current balance restoration path handles ABI 2 and profile-2 evidence for
ABIs 3–4; ABI 1 with no value accounts is account-incapable.

---

## Storage and durability

`LAYERX_REGISTRY_STATE` defaults to `/var/lib/layerx-program-registry`. Its default
children are `journal`, `sources`, `verified` and `builds`, each individually
overridable. The Kubernetes manifest overrides the journal to
`/var/lib/layerx-registry-journal` on a separate PVC and the build root to
`/run/layerx/quota`. Preserve the journal, source mirror, verified-source store,
verification requests and event outbox together during recovery.

Envelope writes create a temporary file, write the record/proof/seal, sync the
file, rename and sync the directory. Loading classifies incomplete, corrupt or
misfiled units as quarantined evidence while retaining complete units. Replay
re-verifies deployment evidence and stored source provenance; a cached verdict
cannot independently authorize a verified read. See
[`journal.rs`](../../services/platform/hosted/registry/src/journal.rs) and
[`verified.rs`](../../services/platform/hosted/registry/src/verified.rs).

Native program-state caching calls `getStateHead(bytes32)` through `eth_call` at
a specified committed height. The cache decoder currently requires the canonical
38-byte storage-head representation inside ABI `bytes`. A filename digest proves
cache integrity only. Restart must obtain fresh chain evidence and current state;
it must refuse stale, inconsistent or unavailable evidence. See
[`program_state.rs`](../../services/platform/hosted/registry/src/program_state.rs).

Proof-backed projection replay and the chain transaction lookup helper use
distinct journal formats. `Registrar::rebuild` replays deployment envelopes;
`chain-deployments/*.json` metadata cannot independently authorize the projection.

Source-verification idempotency is durable across workers and restart. Opening
the registrar reconciles request records and resumes pending publications.
Transient `503` outcomes remain retryable with the same key; completed outcomes
can replay. Publication is queued durably before its terminal response is
acknowledged. Preserve defective/interrupted records for diagnosis instead of
manually fabricating a completed verdict.

Build workspaces use quota-backed ext4 slots provisioned by
[`node-provision-build-boundary.sh`](../../services/platform/hosted/registry/node-provision-build-boundary.sh).
The registry establishes delegated cgroup-v2 cpu, memory, pids and io controllers,
then irrevocably drops to UID/GID `4030` before reading credentials or state.
Each build runs through `layerx-cgroup-exec` and `bwrap`, with an isolated network
namespace, offline Cargo, resource bounds and a pinned environment. Artifact
access uses `openat2` with `ResolveFlags::BENEATH`. See
[`builder.rs`](../../services/platform/hosted/registry/src/builder.rs).

---

## Typed refusals

Refusals normally carry `{"error":{"code":...,"retry":"never","detail":...}}`;
a source-mismatch response can also carry a `verification` object. Durable
verification retry behavior is described above despite the common envelope's
literal `retry` field.

| HTTP | Code | Meaning |
| --- | --- | --- |
| 400 | `invalid_request`, `invalid_argument`, `idempotency_key_required` | Invalid framing, fields, identifiers or missing source-verification key |
| 401 | `authentication_required` | Listener bearer authentication failed |
| 403 | `publication_principal_unresolved` | Source publication/verification key could not resolve to a principal |
| 404 | `not_found`, `interface_absent`, `source_not_mirrored` | Missing route, program, interface or archive |
| 405 | `method_not_allowed` | Unsupported method |
| 409 | `idempotency_conflict`, `source_mismatch`, `deployment_changed` | Conflicting request, differing artifact or deployment changed during rebuild |
| 422 | `deployment_proof_refused` | Canonical deployment proof or version history was refused |
| 422 | `source_unverifiable`, `build_failed`, `build_not_reproducible` | Source plan or rebuild refused |
| 502 | `unverified_read`, `interface_registry_mismatch`, `balance_registry_mismatch` | Evidence disagrees with the derived registry |
| 503 | `deployment_proof_unavailable`, `protocol_state_unavailable`, `journal_unavailable` | Chain read/configuration or deployment-journal failure |
| 503 | `read_unverifiable`, `stale_read`, `protocol_head_unavailable` | Read authority or current head unavailable/stale |
| 503 | `builder_unavailable`, `build_queue_full`, `worker_unavailable`, `request_deadline_exceeded` | Build isolation, queue, worker or deadline refusal |
| 503 | `verification_pending`, `idempotency_store_unavailable`, `idempotency_quota_exhausted` | Durable verification request cannot proceed yet |
| 503 | `program_events_unavailable`, `program_event_unavailable` | Event delivery unhealthy or publication could not be queued |

The listener and direct `Registrar::route` calls have different authentication
boundaries: the listener returns `401 authentication_required` for an invalid
publication bearer; the registrar's own publication check returns
`403 publication_authority_required`.

---

## Readiness

Readiness is an HTTP GET on the pod health port; liveness is TCP on the registry
port. A TCP accept does not prove serving readiness. The health route checks
builder freshness and event delivery, and an isolated health worker replays
durable evidence and checks node state without taking the build serialization
lock or advancing the protocol cursor.

The builder monitor checks metadata/inotify changes every 250 ms and accepts
only a successful check started less than two seconds ago. Changed environment
bytes clear readiness until full digest verification succeeds. Quota slots,
cgroup delegation, TLS and chain evidence must also be valid; `kubectl exec`
into the registry container is refused by the cgroup boundary design.

---

## Configuration keys

These keys are read by the retained binary; required builder and TLS material
must be supplied along with the native chain URL. Consult `config()` in
[`main.rs`](../../services/platform/hosted/registry/src/main.rs) for exact parsing.

| Key | Meaning / default |
| --- | --- |
| `LAYERX_REGISTRY_CHAIN_RPC_URL` | Required native JSON-RPC URL; HTTPS, or HTTP on loopback only; redirects disabled |
| `LAYERX_REGISTRY_LISTEN` | `127.0.0.1:9420` |
| `LAYERX_REGISTRY_HEALTH_LISTEN` | Optional dedicated health socket; manifest uses `0.0.0.0:9421` |
| `LAYERX_REGISTRY_STATE` | `/var/lib/layerx-program-registry` |
| `LAYERX_REGISTRY_JOURNAL` | `$STATE/journal` |
| `LAYERX_REGISTRY_SOURCE_MIRROR` | `$STATE/sources` |
| `LAYERX_REGISTRY_VERIFIED` | `$STATE/verified` |
| `LAYERX_REGISTRY_BUILD_ROOT` | `$STATE/builds`; deployment uses quota slots |
| `LAYERX_REGISTRY_BUILDER_IMAGE_DIGEST` | Required environment-tree digest, 32-byte hex |
| `LAYERX_REGISTRY_BUILDER_ENVIRONMENT_ROOT` | Required pinned root filesystem |
| `LAYERX_REGISTRY_BUILDER_ENTRYPOINT` | Required builder entrypoint |
| `LAYERX_REGISTRY_BUILDER_ISOLATION_RUNTIME`, `LAYERX_REGISTRY_BUILDER_ISOLATION_RUNTIME_DIGEST` | Required isolation binary and pin |
| `LAYERX_REGISTRY_BUILDER_JOB_SUPERVISOR`, `LAYERX_REGISTRY_BUILDER_JOB_SUPERVISOR_DIGEST` | Required supervisor and pin |
| `LAYERX_REGISTRY_HOST_CGROUP_MOUNT` | Required writable host cgroup mount used before privilege drop |
| `LAYERX_REGISTRY_BUILD_TIMEOUT_SECONDS` | `1800`; builder range 1–3600 |
| `LAYERX_REGISTRY_BUILD_MEMORY_BYTES` | `2147483648`; builder range 67108864–8589934592 |
| `LAYERX_REGISTRY_BUILD_PROCESS_LIMIT` | `64`; builder range 1–256 |
| `LAYERX_REGISTRY_BUILD_FILE_SIZE_BYTES` | `67108864`; builder range 33554432–134217728 |
| `LAYERX_REGISTRY_ATTEMPTS` | `2` rebuild attempts |
| `LAYERX_REGISTRY_MAX_STALENESS_SECONDS` | `300`, converted to milliseconds; must be nonzero |
| `LAYERX_REGISTRY_REQUEST_TIMEOUT_SECONDS` | `1800`; range 1–3600 |
| `LAYERX_REGISTRY_MAX_CONNECTIONS` | `128`; range 1–1024 |
| `LAYERX_REGISTRY_MAX_BUILDS` | `4`; range 1–64 |

TLS and bearer keys are listed under [Identity and credentials](#identity-and-credentials).

### Current implementation limits

The retained `config()` and `NodeProgramStateSource` still require compatibility
fields named `LAYERX_REGISTRY_NODE_ENDPOINT`, `LAYERX_REGISTRY_NODE_AUTHORIZATION`,
`LAYERX_REGISTRY_RECEIPT_AUTHORITY_ENDPOINT`,
`LAYERX_REGISTRY_RECEIPT_AUTHORITY_AUTHORIZATION` and
`LAYERX_REGISTRY_RECEIPT_AUTHORITY_REPLICA_ID`, plus
`LAYERX_REGISTRY_OUTBOUND_CA_DER`. The adapter validates two distinct HTTPS or
loopback URLs, nonempty authorizations and a nonzero 32-byte replica identifier.
It still fetches protocol projections through those transports while verifying
committed-chain evidence. This is a remaining compatibility dependency in the
code, not a second execution or settlement network. The old topology is retired;
this page does not prescribe provisioning its independent services.

`LAYERX_REGISTRY_SEQUENCER_TRUST_HISTORY` also remains a required configuration
name, but `persisted_chain_trust` decodes its contents as a native `TrustRoot`
(chain identity and validator trust). It requires a private, nonsymlink regular
file of at most 524288 bytes. The old sequencer-history provisioning procedure
is retired. Optional outbound mTLS uses
`LAYERX_REGISTRY_CLIENT_IDENTITY_PKCS12` together with
`LAYERX_REGISTRY_CLIENT_IDENTITY_PASSWORD_FILE`; either both are supplied or neither.

The checked-in Kubernetes manifest omits the required
`LAYERX_REGISTRY_CHAIN_RPC_URL` and still supplies pre-merge endpoint/trust
material. It cannot be treated as a complete merged-network deployment recipe.
Likewise, the native head worker bypasses opening the compatibility state adapter,
but normal registrar startup, projection reads and source verification still
open it. Supplying the chain URL alone does not remove that dependency.

---

## Make targets

`make platform-test-registry` runs the crate's offline locked Cargo tests through
`tools/runtime/run-with-clock.sh`, as defined in
[`services/platform/Makefile.inc`](../../services/platform/Makefile.inc).
Build-boundary and readiness checks need their declared quota, cgroup, TLS and
node configuration; see the
[registry README](../../services/platform/hosted/registry/README.md).

## What the tests prove

The source tree contains focused coverage for native deployment/head reads in
[`tests/programs_chain.rs`](../../services/platform/hosted/registry/tests/programs_chain.rs),
`eth_call` state caching in
[`tests/program_state_eth_call.rs`](../../services/platform/hosted/registry/tests/program_state_eth_call.rs),
and native chain-trust deployment verification in
[`tests/journal_chain_trust.rs`](../../services/platform/hosted/registry/tests/journal_chain_trust.rs).
[`tests/journal.rs`](../../services/platform/hosted/registry/tests/journal.rs)
covers interrupted writes, envelope replay and quarantine.

---

## Start here

- [Spot module](Spot.md) — kernel market execution on the same network
- [Unified Network Architecture](UnifiedNetwork.md)
- [Home](Home.md)
