# Hosted program registry

`layerx-program-registry` is the hosted program catalog and source-rebuild service (crate `layerx-platform-registry`). It derives deployment, version, lifecycle, interface and source-verification information from protocol evidence; local files alone do not authorize a current read. Its implementation is in `services/platform/hosted/registry/src/`.

Paxeer mainnet-beta is one BFT network, with the kernel participating in block proposal. A receipt is final when its block commits. Registration and recovery must refer to that committed chain state. The separate receipt-authority topology is retired; it is not an additional finality requirement. Hosted reads authenticate chain evidence using configured trust and compare the declared evidence sources.

The public EVM JSON-RPC uses chain id **125** (`0x7d`) and the root HTTPS path of `api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`, or `api-mainnet-beta.paxeer.network`. See [Public RPC](../reference/public-rpc.md). Registry HTTP routes are hosted service routes, not extra public node namespaces. Sub-chains are tenant domains in the database; registering a program does not provision another chain.

The checked-in StatefulSet and Service are named `layerx-program-registry` in `layerx-testnet`. The Service exposes the mTLS registry listener on port **9420**. A separate pod port **9421** serves readiness. The StatefulSet selects `layerx.io/program-registry-boundary=v2` nodes and requires colocation with `app: layerx-node`. Ingress to 9420 admits the gateway, testnet-control and pods labelled `layerx-role: source-publication-operator` (`services/platform/hosted/registry/deployment.yaml`). The manifest declares service configuration; availability depends on the running deployment.

---

## Identity and credentials

The registry listener requires mTLS. `tls_config()` loads DER server certificate, private key and client CA files, and uses `WebPkiClientVerifier`; a connection without an accepted client certificate cannot complete the TLS handshake (`src/main.rs`).

Two different bearer credentials protect the HTTP routes:

| Plane | Configuration | HTTP use |
| --- | --- | --- |
| Request | `LAYERX_REGISTRY_REQUEST_TOKEN_FILE` | Registry reads, deployment/head admission, source verification and metrics |
| Publication | `LAYERX_REGISTRY_PUBLICATION_TOKEN_FILE` | Source archive publication at `/__registry/sources` |

`GET /healthz` needs no bearer. It still needs mTLS on 9420; the readiness listener on 9421 uses plain HTTP without credentials and serves only that path. Do not expose the readiness port as a registry API.

`RegistryAuthority` accepts the exact `Bearer ` scheme, hashes the secret with SHA-256 and compares in constant time. Secrets must contain 1–4096 bytes without ASCII controls; equal request and publication authorities are refused at startup. Secret files must be canonical absolute regular files with a single hard link and no world permission bits (`src/auth.rs`, `read_private_file` and `read_secret` in `src/main.rs`).

Publication and source verification also resolve the `LayerX-Key` header through the principal identity client configured by `LAYERX_REGISTRY_IDENTITY_*`. The listener bearer alone does not establish the publication principal (`publication_principal` in `src/routes.rs`). The identity transport uses `LAYERX_REGISTRY_IDENTITY_URL`, `LAYERX_REGISTRY_IDENTITY_CA_DER`, `LAYERX_REGISTRY_IDENTITY_TOKEN_FILE`, `LAYERX_REGISTRY_IDENTITY_CLIENT_IDENTITY_PKCS12` and `LAYERX_REGISTRY_IDENTITY_CLIENT_IDENTITY_PASSWORD_FILE` in the deployment.

The listener rejects invalid bearer credentials with `401 authentication_required` before starting a request worker. Direct calls to `Registrar::route` distinguish missing publication authority as `403 publication_authority_required`; clients of the binary see the listener's authentication response first.

---

## Routes

The listener parses bounded HTTP/1.1 requests, strips query strings, refuses duplicate headers and `Transfer-Encoding`, and requires `Content-Length` for POST/PUT/PATCH. Headers plus body are bounded to **32 MiB**. Responses use JSON, `Cache-Control: no-store` and `Connection: close` (`src/http.rs`).

| Method | Path | Authority | Result |
| --- | --- | --- | --- |
| GET | `/healthz` | No bearer | Serving readiness; success is `{"status":"ready","service":"program-registry"}` |
| GET | `/metrics` | Request | Listener-owned event producer counters |
| POST | `/__registry/deployments` | Request | Verify canonical deployment admission evidence and journal the deployment |
| POST | `/__registry/head` | Request | Refresh the observed head from native program state for a supplied program id |
| POST | `/__registry/sources` | Publication + principal | Mirror a source archive with its pinned build plan |
| GET | `/v1/programs/registry` | Request | Current receipt-verified catalog of program ids |
| GET | `/v1/programs/registry/{program_id}` | Request | Verified program record and current account evidence |
| GET | `/v1/programs/registry/{program_id}/interface` | Request | Published interface bound to the current deployment and head |
| POST | `/v1/programs/registry/{program_id}/source` | Request + principal | Rebuild mirrored source and compare it with the current deployment |

Program ids in route segments are 32 bytes encoded as hexadecimal. Wrong methods on known routes return `405 method_not_allowed`; unknown routes return `404 not_found`. Implementations are `route_request` in `src/main.rs` and `authorized_route`/`program_route` in `src/routes.rs`. See also the [developer CLI](cli.md).

### Deployment and head admission

`POST /__registry/deployments` accepts canonical `DeploymentProof` bytes, or
JSON with exactly one `admission` field containing an array of those bytes.
The running route decodes that proof and verifies it against the configured
chain trust before admitting a deployment or upgrade. A transaction hash alone
is insufficient input.

`NodeProgramStateSource::verify_stored_deployment` verifies chain-backed
execution evidence and its current-head binding. Admission checks the program,
code hash, ABI, version and predecessor. Repeating identical evidence for an
existing version is accepted; conflicting evidence is refused. The handler
appends the verified record and proof to one sealed journal envelope, replays
that journal, then returns HTTP 200 with `program_id`, `code_hash`,
`abi_version`, `version`, `transaction_hash` and `state: "deployed"`.
The transaction field is the verified evidence's activity identity. Preserve
the proof's native and EVM transaction bindings when consuming it.

Set `LAYERX_REGISTRY_CHAIN_RPC_URL` to the selected node's JSON-RPC root.
`ProgramsChain::new` accepts HTTPS, or HTTP on loopback, and refuses redirects.
Its reads have a 15-second bound within the request deadline
(`src/programs_chain.rs`). Select chain 125 when provisioning this URL.

`POST /__registry/head` accepts JSON containing only `program_id`, a 32-byte
hexadecimal identifier with an optional `0x` prefix. It obtains the head
height and timestamp and calls `getProgram(bytes32)` and
`getStateHead(bytes32)` at that height. It derives `state_root` as Keccak-256
of the returned state-head bytes and converts the timestamp to milliseconds.
This is the handler's program-state digest. It is distinct from the block's
application root. Caller-supplied sequence, time and root claims are refused
(`route_chain_head` and `refresh_head` in `src/routes.rs`).

---

## Record contracts

| Record | Location or response | Contract |
| --- | --- | --- |
| Admitted chain deployment | `{journal}/chain-deployments/{transaction_hash}.json` | `ChainDeployment`: transaction/block hashes, block number, program/code hashes, ABI/version, policy, authority and active state. A repeated transaction must match the stored record. |
| Deployment envelope | `{journal}/{receipt_digest}.envelope` | Domain `LayerX/deployment-envelope/v1\0`, framed canonical deployment record and proof, SHA-256 seal over both frames. |
| Legacy deployment unit | `{digest}.deployment` + `{digest}.admission` | Both halves are needed when no envelope exists. |
| Observed head | `{journal}/head` | Nonzero sequence and millisecond observation time, tab-separated. |
| Native program-state cache | `{journal}/program-state/{program}.{digest}.program-state` | Canonical native storage-head bytes from `getStateHead(bytes32)`; filename digest is SHA-256 of those bytes. Cached bytes are not read authority. |
| Scan cursor | `{journal}/program-state/canonical.cursor` | Tab-separated sequence/ordinal; advancing refuses nonzero ordinal and regression. |
| Mirrored source | `{mirror}/{digest}.archive`, `.plan`, `.uri` | Encoded source archive, `BuildPlan`, and URI; digest is SHA-256 of the encoded archive. |
| Completed rebuild | `{verified}/{program}-{version}.verified` | Program/version, source URI/digest, artifact digest, plan and deployment provenance. |
| Verification request | `{journal}/verification-requests/` | Versioned durable request identity, request/integrity digests, attempt, phase, timestamps and retained response/publication data. |

Sources: `src/programs_chain.rs`, `src/journal.rs`, `src/program_state.rs`, `src/mirror.rs` and `src/verified.rs`.

Catalog reads include `program_ids`, `state_root`, `observed_sequence`, `observed_at`, `valid_through` and `verification: registry-receipt-and-current-head-verified`. Program reads include upgrade policy, lifecycle, versions, lifecycle history, exit routes, value accounts and `receipt.verification: receipt-verified`. Version entries add deployment provenance when available. Interface responses include canonical interface bytes as hex, interface digest, code hash, ABI/version, deployment receipt digest, current head/freshness fields and `verification: deployment-interface-and-current-head-verified` (`src/routes.rs`). Consumers must use these producer field names; a response label by itself is not a cryptographic proof.

Source publication takes JSON `source_uri`, `plan` (encoded build-plan string) and `archive_hex`, returning `mirrored`, `source_uri` and `source_digest`. Verification takes only `source_uri` and `source_digest` (32-byte hex), plus an `Idempotency-Key` of 16–128 ASCII letters, digits, `-` or `_`. Success returns deployment identity/provenance, source and environment digests, reproduced artifact digest, source status and pipeline. A mismatch is `409 source_mismatch` with the verification outcome attached, never a verified verdict.

---

## Storage and durability

The default state root is `/var/lib/layerx-program-registry`, with `journal`, `sources`, `verified` and `builds` subdirectories unless overridden. The deployment overrides the journal to `/var/lib/layerx-registry-journal`, mounted from PVC `layerx-registry-journal`; the registry state claim requests 100Gi RWO, and builds use quota-backed host storage (`deployment.yaml`). Preserve these separate mounts across a restart.

`write_atomic` writes a temporary file, syncs it and renames it. Envelope publication additionally syncs the journal directory. The seven envelope write steps are create-temporary, write-record, write-proof, write-seal, sync-temporary, commit and sync-directory. Startup quarantines incomplete, corrupt or misfiled units with typed reports while retaining complete units. Journal proof and projection sets must agree (`src/lib.rs`, `src/journal.rs`).

`Registrar::rebuild` re-verifies stored deployment proofs, audits their projections, reconstructs version/interface history and replays completed source-verification records only when deployment provenance, code digest and pinned mirrored build plan agree. It audits the native state-cache filenames and clears in-memory head/balance authority. A restart must obtain fresh chain evidence before publishing current reads; retained files and a previous `ready` response do not suffice.

Synchronization scans program changes within 1024 pages (up to 4096 notices per page), resolves affected programs at the current verified head, stores native state via `eth_call`, restores account evidence, then advances the cursor and observed head. Head verification checks commitment/header evidence against retained chain trust, transaction inclusion and freshness; claimed receipt digest, state root, height and time must match the verified evidence (`src/node_state.rs`, `src/head_attestation.rs`, `src/routes.rs`). The authenticated evidence source is compared with the configured authority response before publishing a head.

Account synchronization skips ABI 1 when it has no value accounts, restores ABI 2 through the original account protocol and restores ABI 3–5 through account profile 2. Deployment replay and source-verification records use `VerifiedDeploymentEvidence::admit_abi_version`, which delegates to the runtime policy for guest ABIs 1–5. The deployed artifact, interface and account profile must agree (`src/routes.rs`, `src/verified.rs` and `programs/crates/layerx-programs-registry/src/protocol_evidence.rs`).

Known programs are synchronized before serving current reads. Stale or unavailable head/account evidence yields a typed refusal rather than stale cached success. An empty registry may open before the first protocol head only for the narrowly recognized pending answer (`503` with exactly `{"error":-903}`); reads still need a verified head. This exception does not hide authentication, malformed-proof or transport failures (`current_head_or_pending` in `src/node_state.rs`).

### Isolated rebuilds

Build workspaces use ext4 quota slots under `/var/lib/layerx-program-registry-builds`, mounted at `/run/layerx/quota` in the deployment. The node oneshot provisions/checks these slots; it does not delegate cgroups. The registry receives the host cgroup mount, starts with only CHOWN/SETUID/SETGID capabilities, discovers its container cgroup, enables cpu/memory/pids/io controllers, delegates worker control and irrevocably drops to UID/GID 4030 before opening secrets, state or listeners (`src/main.rs`, `node-provision-build-boundary.sh`).

Workers and their builds live beneath the delegated workers subtree. Each rebuild uses `layerx-cgroup-exec` and `bwrap --unshare-all --disable-userns --cap-drop ALL` with a read-only pinned environment, no network and `CARGO_NET_OFFLINE=true`. Artifact access uses `openat2` with `ResolveFlags::BENEATH`, and artifact size is bounded to 32 MiB. The builder refuses missing controllers, incorrect ownership, missing quota slots or changed pinned execution material (`src/builder.rs`). Enabling controllers prevents new direct processes in the container root, so `kubectl exec` and exec-based probes are unsuitable.

### Source verification across restart

Idempotency is durable, not an in-memory worker cache. The request journal scopes keys to the resolved principal and program and binds the request digest to the deployment and requested source. An advisory lease serializes workers. Identical completed requests replay the retained response; a changed request under the same identity returns `409 idempotency_conflict`.

The phases are `building`, `retryable`, `artifact`, `persisted` and `completed`. A crashed builder can be retried after its lease is released. Completed artifact work and pending event publication can resume without treating an incomplete publication as acknowledged success. Reconciliation preserves defective/interrupted records as evidence. `503` outcomes remain retryable. Retention is bounded to 4096 request identities; live identities are not evicted to make room (`src/verified.rs`, `verify`, `persist_artifact` and `publish` in `src/routes.rs`).

---

## Typed refusals

Most refusals use `{"error":{"code":"…","retry":"never","detail":"…"}}`. Source mismatch adds a `verification` object. Interpret the HTTP status and exact code; `503` verification outcomes can be retried with the same idempotency key despite the generic envelope's `retry` field.

| HTTP | Codes | Meaning |
| --- | --- | --- |
| 400 | `invalid_request`, `invalid_argument`, `idempotency_key_required` | Invalid HTTP framing, route payload/id, or missing source-verification key |
| 401 / 403 | `authentication_required`, `publication_authority_required` | Missing/incorrect bearer authority; 403 is the registrar-level publication refusal |
| 404 / 405 | `not_found`, `interface_absent`, `source_not_mirrored`, `method_not_allowed` | Missing route/program/interface/archive or unsupported method |
| 409 | `idempotency_conflict`, `source_mismatch` | Conflicting request identity or artifact code-hash mismatch |
| 422 | `deployment_proof_refused` | Receipt, deployment event or canonical Programs record fails admission |
| 422 | `deployment_inactive`, `deployment_mismatch` | Source verification does not match an active current supported deployment |
| 422 | `source_unverifiable`, `build_failed`, `build_not_reproducible` | Source/plan refusal, builder failure or nondeterministic rebuild |
| 502 | `unverified_read`, `balance_protocol_unsupported`, `balance_registry_mismatch`, `interface_registry_mismatch` | Unusable registry evidence or disagreement with current account/interface data |
| 503 | `deployment_proof_unavailable`, `journal_unavailable`, `protocol_state_unavailable` | RPC/configuration/state resolution or journal persistence unavailable |
| 503 | `read_unverifiable`, `stale_read`, `protocol_head_unavailable`, `balance_read_unavailable`, `stale_balance_read`, `interface_proof_unavailable` | Current proof/head/account/interface evidence absent, stale or refused |
| 503 | `request_deadline_exceeded`, `worker_unavailable`, `build_queue_full`, `registry_unavailable`, `builder_unavailable` | Deadline, worker/cgroup, queue, lock or build-environment boundary unavailable |
| 503 | `verification_pending`, `idempotency_store_unavailable`, `idempotency_store_corrupt`, `idempotency_quota_exhausted`, `persistence_unavailable` | Durable verification identity, artifact or publication cannot yet settle |
| 503 | `program_events_unavailable` | Readiness refuses because event delivery is unavailable |

These are common codes from `src/main.rs` and `src/routes.rs`, not an exhaustive schema for every dependency. Connections above `max_connections` are closed without an HTTP response. The HTTP writer knows the 413 reason phrase, but an oversized parsed request is not promised a `413` response.

---

## Readiness

The deployment uses HTTP `GET /healthz` on named port `health` (**9421**) for readiness: period 10 seconds, timeout 9 seconds, failure threshold 1 and success threshold 1. Liveness remains TCP on named port `registry` (**9420**): period 10 seconds, timeout 5 seconds, failure threshold 6. The Service exposes only the registry port (`deployment.yaml`).

`health_verdict` invokes the same serving path used for the mTLS health route, with an eight-second deadline capped by the configured request timeout. The plain health listener admits at most four concurrent connections. It refuses other paths/methods. A readiness request checks event delivery health, requires builder verification less than two seconds old, and opens an isolated worker to reconcile durable state and validate a current head or the explicit empty-network pending case. It does not wait behind the ordinary registrar request gate (`src/main.rs`).

Successful execution eventually reaches the registrar's `ready` JSON. That final constant response does not bypass the preceding checks. Worker startup, durable replay, node/proof access, builder or event-delivery failures prevent serving readiness; they can yield `503` or a failed/closed request. TCP liveness only establishes that a connection can be accepted. Readiness does not certify every program's balances or full ABI coverage: health-mode opening checks head availability without running the normal account synchronization pass (`open_with_builder` in `src/routes.rs`).

---

## Configuration keys

These keys are read by `config()`, `tls_config()` or the health listener in `src/main.rs`. Defaults describe code behavior; the checked-in deployment overrides several values.

| Key | Default or requirement |
| --- | --- |
| `LAYERX_REGISTRY_CHAIN_RPC_URL` | Required merged-node JSON-RPC URL; no default |
| `LAYERX_REGISTRY_LISTEN` | `127.0.0.1:9420`; deployment uses `0.0.0.0:9420` |
| `LAYERX_REGISTRY_HEALTH_LISTEN` | Absent disables the separate listener; deployment uses `0.0.0.0:9421` |
| `LAYERX_REGISTRY_STATE` | `/var/lib/layerx-program-registry` |
| `LAYERX_REGISTRY_JOURNAL` | `$STATE/journal`; deployment uses `/var/lib/layerx-registry-journal` |
| `LAYERX_REGISTRY_SOURCE_MIRROR`, `LAYERX_REGISTRY_VERIFIED`, `LAYERX_REGISTRY_BUILD_ROOT` | `$STATE/sources`, `$STATE/verified`, `$STATE/builds`; deployment build root is `/run/layerx/quota` |
| `LAYERX_REGISTRY_REQUEST_TOKEN_FILE`, `LAYERX_REGISTRY_PUBLICATION_TOKEN_FILE` | Required distinct secret files |
| `LAYERX_REGISTRY_TLS_CERT_DER`, `LAYERX_REGISTRY_TLS_KEY_DER`, `LAYERX_REGISTRY_CLIENT_CA_DER` | Required server TLS identity and client CA; files bounded to 64 KiB |
| `LAYERX_REGISTRY_BUILDER_IMAGE_DIGEST` | Required 32-byte hex digest of the environment tree |
| `LAYERX_REGISTRY_BUILDER_ENVIRONMENT_ROOT`, `LAYERX_REGISTRY_BUILDER_ENTRYPOINT` | Required pinned environment and absolute entrypoint; deployment uses `/opt/layerx-builder/rootfs`, `/bin/layerx-build` |
| `LAYERX_REGISTRY_BUILDER_ISOLATION_RUNTIME`, `LAYERX_REGISTRY_BUILDER_ISOLATION_RUNTIME_DIGEST` | Required runtime path and 32-byte hex digest; deployment uses `/usr/bin/bwrap` |
| `LAYERX_REGISTRY_BUILDER_JOB_SUPERVISOR`, `LAYERX_REGISTRY_BUILDER_JOB_SUPERVISOR_DIGEST` | Required supervisor path and 32-byte hex digest; deployment uses `/usr/bin/layerx-cgroup-exec` |
| `LAYERX_REGISTRY_HOST_CGROUP_MOUNT` | Required before privilege drop; deployment uses `/run/layerx/host-cgroup` |
| `LAYERX_REGISTRY_BUILD_TIMEOUT_SECONDS` | 1800; builder admits 1–3600 |
| `LAYERX_REGISTRY_BUILD_MEMORY_BYTES` | 2147483648; builder admits 67108864–8589934592 |
| `LAYERX_REGISTRY_BUILD_PROCESS_LIMIT` | 64; builder admits 1–256 |
| `LAYERX_REGISTRY_BUILD_FILE_SIZE_BYTES` | 67108864; builder admits 33554432–134217728 |
| `LAYERX_REGISTRY_ATTEMPTS` | 2 reproducibility attempts |
| `LAYERX_REGISTRY_MAX_STALENESS_SECONDS` | 300, converted to milliseconds; zero refused |
| `LAYERX_REGISTRY_REQUEST_TIMEOUT_SECONDS` | 1800; admits 1–3600 |
| `LAYERX_REGISTRY_MAX_CONNECTIONS`, `LAYERX_REGISTRY_MAX_BUILDS` | 128 / 4; admit 1–1024 / 1–64 |

The host quota provisioner separately reads `LAYERX_REGISTRY_NODE_QUOTA_ROOT`, `LAYERX_REGISTRY_MAX_BUILDS`, `LAYERX_REGISTRY_BUILD_QUOTA_BYTES`, `LAYERX_REGISTRY_BUILD_QUOTA_INODES` and `LAYERX_REGISTRY_NODE_LOCK` (`node-provision-build-boundary.sh`). Event delivery uses the `LAYERX_EVENTS_PROGRAM_UPSTREAM_*` and `LAYERX_EVENTS_WEBHOOKS_UPSTREAM_*` transport settings mounted by the deployment.

### Retained compatibility configuration

The following names still occur in `config()` and the hosted replay client; they describe remaining implementation dependencies, not instructions to deploy a separate authority for the merged network:

| Retained keys | What the current code does |
| --- | --- |
| `LAYERX_REGISTRY_NODE_ENDPOINT`, `LAYERX_REGISTRY_NODE_AUTHORIZATION` | Require an authenticated node-state transport |
| `LAYERX_REGISTRY_RECEIPT_AUTHORITY_ENDPOINT`, `LAYERX_REGISTRY_RECEIPT_AUTHORITY_AUTHORIZATION` | Still require a second authenticated transport and compare its chain evidence |
| `LAYERX_REGISTRY_OUTBOUND_CA_DER` | Required CA for those outbound transports |
| `LAYERX_REGISTRY_CLIENT_IDENTITY_PKCS12`, `LAYERX_REGISTRY_CLIENT_IDENTITY_PASSWORD_FILE` | Optional outbound client identity; both must be configured together |
| `LAYERX_REGISTRY_RECEIPT_AUTHORITY_REPLICA_ID` | Still parsed as a nonzero 32-byte hex identity by the client |
| `LAYERX_REGISTRY_SEQUENCER_TRUST_HISTORY` | Retained verifier input path; its name is compatibility vocabulary |

The checked-in deployment derives the replica id, `LAYERX_REGISTRY_KERNEL_GENERATION` and selected history path from one verified material generation. It does **not** read a replica-id ConfigMap entry directly into the registry process. The init container publishes `generation.json`, `replica-id` and `trust-history` into `/run/layerx/trust/material`; the launch wrapper verifies the retained tuple and exports its selected values. `material.py` checks schema, ownership, permissions, digests and identity/history agreement, and refuses a different retained generation pending owner reconciliation. Do not replace that tuple piecemeal or invent an identity on restart (`deployment.yaml`, `services/platform/hosted/human/material.py`).

---

## Make targets

`make platform-test-registry` runs the registry crate's offline, locked Cargo tests through `tools/runtime/run-with-clock.sh`. `make platform-hosted-topology-check` invokes the hosted manifest topology checker (`services/platform/Makefile.inc`).

---

## What the tests prove

The repository contains authentication and HTTP-framing tests in `src/auth.rs` and `src/http.rs`; journal interruption, quarantine, legacy-pair and observed-head tests in `tests/journal.rs`; and native admission/state-read coverage in `tests/programs_chain.rs` and `tests/program_state_eth_call.rs`. `tests/journal_chain_trust.rs` covers deployment-journal chain trust, and `tests/readiness.py` exercises serving readiness.

---

## Chain evidence and replay

Deployment admission and restart replay both verify the stored canonical proof.
Replayed envelopes rebuild program versions in authenticated sequence order.
RPC metadata under `chain-deployments` supplements those envelopes and requires
an already authenticated journal entry; metadata alone does not authorize a
catalog record.

Current-head verification checks the configured node and authority responses
for matching sequence, time, receipt digest, state root and chain evidence.
It validates the declared authority identity, then verifies the signed head,
transaction proof and freshness against retained chain trust. These are checks
on the same committed chain (`src/node_state.rs`).

The deployment supplies `LAYERX_REGISTRY_CHAIN_RPC_URL`. Keep it consistent
with the trust history, authenticated state endpoints and selected network.

[Programs](../programs/index.md) · [Public RPC](../reference/public-rpc.md) · [Home](../index.md)
