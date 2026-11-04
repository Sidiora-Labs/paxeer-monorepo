# Hosted platform services

Hosted services provide application ingress, identity, proof verification, indexing and operational support for Paxeer. They do not form an additional consensus network. Mainnet-beta is one BFT chain with EVM chain ID **125** (`0x7d`); its kernel is part of the network and proposes blocks. A receipt is final when its block is committed. Sub-chains are database tenant domains, launchable without another chain deployment.

Public node JSON-RPC uses the HTTPS root of `api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`, or `api-mainnet-beta.paxeer.network`. Node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`. Hosted `px_` methods are defined in [the gateway OpenRPC document](../services/platform/hosted/gateway/openrpc.json), not promised by every node. `node.hyperpaxeer.com` is the build library mirror, not RPC.

This page binds service families and historical registry anchors to their maintained source. The [gateway contract](platform-gateway.md) owns public ingress; [middleware](platform-middleware.md) owns application payment verification, [ramps](platform-ramps.md) own independent external-funds workflows, and [protobuf APIs](proto-api.md) own node gRPC/REST declarations. The [standalone relay/archive service is retired](platform-relay-archive.md).

## Env vars

The [platform Cargo workspace](../services/platform/Cargo.toml) declares the Rust service members. Deployment manifests and environment initialization scripts describe packaging; membership or a manifest does not prove a service is deployed or ready. The sections below link actual configuration readers. Historical `LAYERX_` prefixes and client type names remain in source; preserve exact names when configuring an active adapter, without inferring a separate chain from them.

Keep API credentials, TLS identities and signer handles at their declared boundaries. Do not translate a missing dependency into an empty success. Liveness, local storage health, dependency readiness, signed receipt verification and block commitment are separate observations. Forward tenant and principal bindings through authorization, persistence, cursor and idempotency paths rather than defaulting all traffic to one domain.

<a id="node-cli-commands"></a>
<a id="node-env-vars"></a>
<a id="node-error-codes"></a>

## Native node lifecycle

[bootstrap.sh](../services/platform/hosted/node/bootstrap.sh) and [supervisor.sh](../services/platform/hosted/node/supervisor.sh) enter [native_runtime.py](../services/platform/hosted/node/native_runtime.py). That code prepares and supervises native `paxd` generations. Its root-origin validation and `committed_health` check consensus status and native JSON-RPC, rather than accepting a process's liveness as committed readiness. [reset_state.py](../services/platform/hosted/node/reset_state.py) and [data_directory.py](../services/platform/hosted/node/data_directory.py) own durable generation/reset state and protected directory operations.

Native inputs include `LAYERX_NODE_CONSENSUS_RPC_URL`, `LAYERX_NODE_RPC_URL` and `LAYERX_NODE_CHAIN_ID`; the consensus chain identifier is distinct from numeric EVM chain ID 125. Preserve the generated manifest's identity and storage bindings. The legacy `layerxd` daemon is retired; remaining compatibility scripts referring to it are not native node startup instructions.

<a id="core-env-vars"></a>
<a id="core-error-codes"></a>
<a id="core-file-formats"></a>
<a id="core-http-routes"></a>
<a id="core-rust-pub-items"></a>

## Core application service

[core/src/main.rs](../services/platform/hosted/core/src/main.rs) owns hosted account, asset, funding and administrative adapters; [lib.rs](../services/platform/hosted/core/src/lib.rs) contains account derivation, registry and signer helpers. This hosted crate is not the embedded consensus kernel. Its internal HTTP and TLS boundaries must not be advertised as a second public execution endpoint.

[public_reads.rs](../services/platform/hosted/core/src/public_reads.rs) defines asset, fee and node-info reads. [funding_native.rs](../services/platform/hosted/core/src/funding_native.rs) and [chain_fund.rs](../services/platform/hosted/core/src/chain_fund.rs) implement native funding paths. Operator/admin credentials and treasury signing authority are distinct from customer identity. State files and journals are part of recovery; a fresh process or an API response must not erase an uncertain transfer. Configuration includes `LAYERX_CORE_NATIVE_ASSET_DENOMS` for native asset bindings and separate admin, wallet-capability and receipt-event token files in the main reader.

<a id="authority-cli-commands"></a>
<a id="authority-config-keys"></a>
<a id="authority-env-vars"></a>
<a id="authority-error-codes"></a>
<a id="authority-file-formats"></a>
<a id="authority-http-routes"></a>
<a id="authority-rust-pub-items"></a>

## Receipt authority

[authority/src/main.rs](../services/platform/hosted/authority/src/main.rs) exposes authenticated evidence reads including `/v1/authorized-batches/by-activity/…`; [lib.rs](../services/platform/hosted/authority/src/lib.rs) verifies canonical receipt and batch evidence. [chain_receipt.rs](../services/platform/hosted/authority/src/chain_receipt.rs), [committed.rs](../services/platform/hosted/authority/src/committed.rs) and [trust.rs](../services/platform/hosted/authority/src/trust.rs) bind native receipt, commitment and trust material. [human.rs](../services/platform/hosted/authority/src/human.rs) owns the human/agent adapter.

`LAYERX_AUTHORITY_CHAIN_RPC_URL` and `LAYERX_AUTHORITY_RECEIPT_KEYS_FILE` select chain evidence and trusted receipt keys. Evidence-read credentials, network identity and TLS inputs are separately validated by the main reader. The source still contains compatibility evidence adapters; their presence does not establish a separate finality authority. Preserve verification failures, missing evidence and unavailable backends as refusals. Persisted evidence must retain its receipt/activity and network binding; an authority response is useful only at the verification level it actually proves.

<a id="identity-env-vars"></a>
<a id="identity-error-codes"></a>
<a id="identity-file-formats"></a>
<a id="identity-http-routes"></a>

## Identity

[identity/src/main.rs](../services/platform/hosted/identity/src/main.rs) registers principal, session, signup and publication-key routes, including `POST /v1/sessions/introspect`. [store.rs](../services/platform/hosted/identity/src/store.rs), [seal.rs](../services/platform/hosted/identity/src/seal.rs) and [signup.rs](../services/platform/hosted/identity/src/signup.rs) own persistent and sealed identity material and signup transitions.

Configuration includes `LAYERX_IDENTITY_STATE_DIR`, `LAYERX_IDENTITY_SERVICE_TOKENS_DIR`, `LAYERX_IDENTITY_STORE_KEY_FILE` and TLS/client-CA inputs. Optional signup delivery uses its configured outbox and mail transport. Service tokens and session tokens have different purposes. A caller-provided principal or account is not a substitute for authenticated introspection. Do not expose private store formats as public session schemas or log credentials in error reports.

<a id="internal-cli-commands"></a>
<a id="internal-env-vars"></a>
<a id="internal-error-codes"></a>
<a id="internal-file-formats"></a>
<a id="internal-http-routes"></a>
<a id="internal-rust-pub-items"></a>

## Internal support

[internal/src](../services/platform/hosted/internal/src) contains shared protected-secret, TLS, HTTP, KMS, principal, journal and event-producer libraries, plus service binaries under [src/bin](../services/platform/hosted/internal/src/bin). It is not one public listener with every exported helper mounted.

[kms.rs](../services/platform/hosted/internal/src/kms.rs) and [evm_sender.rs](../services/platform/hosted/internal/src/evm_sender.rs) own signing-key and signature boundary clients. [events.rs](../services/platform/hosted/internal/src/events.rs) and [producer.rs](../services/platform/hosted/internal/src/producer.rs) implement authenticated enrollment, producer/reader readiness and event transport. [journal.rs](../services/platform/hosted/internal/src/journal.rs) owns durable record handling. Exported Rust types and private wire formats must retain secret-file checks, principal scoping, idempotency and error identity when reused.

<a id="registry-cli-commands"></a>
<a id="registry-env-vars"></a>
<a id="registry-error-codes"></a>
<a id="registry-file-formats"></a>
<a id="registry-http-routes"></a>
<a id="registry-metrics"></a>

## Program registry

[registry/src/routes.rs](../services/platform/hosted/registry/src/routes.rs) owns program registry HTTP admission; [main.rs](../services/platform/hosted/registry/src/main.rs) wires the build service and its health/metrics listeners. Registry reads use the `/v1/programs/registry` family. [programs_chain.rs](../services/platform/hosted/registry/src/programs_chain.rs), [program_state.rs](../services/platform/hosted/registry/src/program_state.rs) and [verified.rs](../services/platform/hosted/registry/src/verified.rs) bind registry results to chain state. [journal.rs](../services/platform/hosted/registry/src/journal.rs) records durable operations.

`LAYERX_REGISTRY_CHAIN_RPC_URL` supplies node access; request and publication tokens authorize distinct operations. Builds require the configured isolated environment and host cgroup boundary. Follow the [registry deployment contract](../services/platform/hosted/registry/README.md) for quota storage, privilege drop and cgroup prerequisites. A built artifact or accepted registration request does not establish committed on-chain registration. Keep guest/host ABI versions and payload limits aligned before submitting a program.

<a id="indexer-cli-commands"></a>
<a id="indexer-env-vars"></a>
<a id="indexer-error-codes"></a>
<a id="indexer-file-formats"></a>
<a id="indexer-metrics"></a>
<a id="indexer-rust-pub-items"></a>

## Indexer HTTP routes

[indexer/src/api.rs](../services/platform/hosted/indexer/src/api.rs) exposes health/readiness and chain, asset, history and holder reads, including `/v1/chains`, `/v1/assets`, `/v1/history/…` and `/v1/holders/…`. [config.rs](../services/platform/hosted/indexer/src/config.rs) reads database, listen, TLS, source, polling and backfill configuration. The default listener is `127.0.0.1:8095`; it is not a public node origin.

[store.rs](../services/platform/hosted/indexer/src/store.rs) owns positions, projection records and ingestion observations. [follow.rs](../services/platform/hosted/indexer/src/follow.rs), [paxeer.rs](../services/platform/hosted/indexer/src/paxeer.rs) and [backfill.rs](../services/platform/hosted/indexer/src/backfill.rs) own live following and historical ingestion. [explorer.rs](../services/platform/hosted/indexer/src/explorer.rs) joins explorer-backed history. Preserve ordered cursors and completeness information when serving the [gateway history methods](platform-gateway.md#gateway-jsonrpc-methods).

The executable supports the `backfill --cutover-height N` mode, with optional `--range-blocks N`, as parsed in `config.rs`. Backfill's database/TLS inputs are separate from live node configuration. [main.rs](../services/platform/hosted/indexer/src/main.rs) records errors such as `source_unavailable`, `source_decode_failed`, `store_unavailable`, `source_configuration_invalid`, `source_integrity_failed` and `reorg_beyond_finality`. Integrity and beyond-finality reorganization failures halt ingestion; a stalled projection must not be presented as a current chain answer. Metrics and health are observations of the indexer, not consensus evidence.

<a id="webhooks-config-keys"></a>
<a id="webhooks-env-vars"></a>
<a id="webhooks-error-codes"></a>
<a id="webhooks-file-formats"></a>
<a id="webhooks-rust-pub-items"></a>

## Webhooks HTTP routes

[webhooks/src/main.rs](../services/platform/hosted/webhooks/src/main.rs), [http.rs](../services/platform/hosted/webhooks/src/http.rs) and [endpoints.rs](../services/platform/hosted/webhooks/src/endpoints.rs) own endpoint registration, delivery operations and route admission. The service publishes its signature scheme through `/v1/webhooks/scheme`; endpoint routes use the `/v1/webhooks/` family, while internal observation routes require internal authority.

[scheme.rs](../services/platform/hosted/webhooks/src/scheme.rs) defines `LayerX/webhooks/v1`: an Ed25519 signature over `<event-id>.<unix-seconds>.` followed by exact body bytes. Required authenticated headers are `layerx-webhook-id`, `layerx-webhook-timestamp`, `layerx-webhook-key-id` and `layerx-webhook-signature`; signatures use `v1=` and standard padded base64. Redelivery preserves the event ID and body. Attempt and delivery metadata are not independent financial authority. See [middleware verification](platform-middleware.md#webhook-consumption).

[deliveries.rs](../services/platform/hosted/webhooks/src/deliveries.rs) owns durable delivery state; [events.rs](../services/platform/hosted/webhooks/src/events.rs), [chain.rs](../services/platform/hosted/webhooks/src/chain.rs) and [ai_market.rs](../services/platform/hosted/webhooks/src/ai_market.rs) own event production and evidence bindings. [hosted.rs](../services/platform/hosted/webhooks/src/hosted.rs) reads Redis, cursor-key, KMS and instance configuration; [trusted.rs](../services/platform/hosted/webhooks/src/trusted.rs) reads authenticated dependency configuration. `LAYERX_WEBHOOKS_ROLE` and `LAYERX_WEBHOOKS_LISTENER` select runtime roles/listener behavior. Preserve typed errors from [error.rs](../services/platform/hosted/webhooks/src/error.rs); acknowledge an event only after the corresponding durable transition.

<a id="faucet-env-vars"></a>
<a id="faucet-error-codes"></a>
<a id="faucet-http-routes"></a>
<a id="testnet-env-vars"></a>
<a id="testnet-error-codes"></a>
<a id="testnet-http-routes"></a>

## Faucet and testnet journeys

[faucet/src/main.rs](../services/platform/hosted/faucet/src/main.rs) registers `/v1/faucet/claims` and `/v1/faucet/service-claims`, with identity and funding dependencies configured separately. Quota, claim identity and a funding result must survive retries; a service credential cannot stand in for an arbitrary customer's principal.

[testnet/src/main.rs](../services/platform/hosted/testnet/src/main.rs) provides controlled funding, payment and receipt-inspection journeys, plus status, readiness and administrative surfaces. Its TLS and identity/faucet dependency inputs use the `LAYERX_TESTNET_` family. These are development/deployment facilities, not additional mainnet methods or permission to reset a production network. Journey states and error codes must reflect the verified underlying operation.

<a id="dashboard-env-vars"></a>
<a id="dashboard-error-codes"></a>
<a id="dashboard-http-routes"></a>

## Dashboard

[dashboard/src/service.rs](../services/platform/hosted/dashboard/src/service.rs) and [model.rs](../services/platform/hosted/dashboard/src/model.rs) project gateway keys, usage, requests and webhook/receipt observations. [gateway.rs](../services/platform/hosted/dashboard/src/gateway.rs) reads gateway storage, with its own Redis credentials; [main.rs](../services/platform/hosted/dashboard/src/main.rs) configures its listener. The dashboard is a read model. Preserve verification levels and principal visibility; do not upgrade an observed receipt to committed status or expose a key's secret through a summary. Errors are defined by [error.rs](../services/platform/hosted/dashboard/src/error.rs).

<a id="agent-boundary-env-vars"></a>
<a id="agent-boundary-error-codes"></a>
<a id="agent-boundary-http-routes"></a>
<a id="agentd-cli-commands"></a>
<a id="agentd-env-vars"></a>
<a id="human-cli-commands"></a>
<a id="human-env-vars"></a>
<a id="human-error-codes"></a>

## Agent and human boundaries

[agent-boundary/src/main.rs](../services/platform/hosted/agent-boundary/src/main.rs) implements authenticated node/program/receipt adapters. `LAYERX_AGENT_BOUNDARY_CHAIN_RPC_URL` selects the node, while gateway, registry and webhook tokens name distinct callers. Its deployment-proof operations live in [deployment.rs](../services/platform/hosted/agent-boundary/src/deployment.rs). Authenticate and bind preparation, approval and signed transaction bytes before accepting a submission or proof.

The signing agent implementation lives in [services/agent/crates/layerx-agentd](../services/agent/crates/layerx-agentd); [hosted/agentd](../services/platform/hosted/agentd) contains deployment probes and manifest checks. It is not a consensus replacement. Human onboarding and owner material are under [hosted/human](../services/platform/hosted/human), with [native_manifest.py](../services/platform/hosted/human/native_manifest.py) and [owner_native.py](../services/platform/hosted/human/owner_native.py) binding native operations. These scripts' arguments, environment and local refusal codes belong to their provisioning boundary, not to public JSON-RPC.

<a id="feeder-cli-commands"></a>
<a id="feeder-config-keys"></a>
<a id="feeder-env-vars"></a>
<a id="feeder-file-formats"></a>
<a id="feeder-metrics"></a>
<a id="feeder-rust-pub-items"></a>

## Oracle feeder

The `price-sidecar` binary in [feeder/src/bin/price-sidecar](../services/platform/hosted/feeder/src/bin/price-sidecar) and [feeder/src/lib.rs](../services/platform/hosted/feeder/src/lib.rs) implement signed oracle submission, sender binding, health and durable round material. [chainflow.rs](../services/platform/hosted/feeder/src/chainflow.rs) and [cosmos_tx.rs](../services/platform/hosted/feeder/src/cosmos_tx.rs) own chain reads and Cosmos transaction encoding.

The address lookup precompile constant is `0x0000000000000000000000000000000000001004`. `/healthz` is liveness; `/readyz` reflects freshness of the last complete accepted round, within three cadences. Keep feed configuration, persisted round state, signing authorization and submitted chain evidence together across restart. A metric increment or local round file does not prove a committed price update.

<a id="ai-evaluator-config-keys"></a>
<a id="ai-evaluator-env-vars"></a>
<a id="ai-evaluator-error-codes"></a>
<a id="ai-evaluator-http-routes"></a>
<a id="ai-evaluator-metrics"></a>

## AI evaluator

[ai-evaluator/src/main.rs](../services/platform/hosted/ai-evaluator/src/main.rs) reads committed market state and submits commit/reveal scores for a granted evaluator. Configuration comes from `LAYERX_AI_EVALUATOR_CONFIG`, with the key file named by `LAYERX_AI_EVALUATOR_KEY_FILE`. The programs precompile constant is `0x000000000000000000000000000000000000101d`; the current evaluator binds guest ABI 5 and entry point `layerx_call`.

The source defines commit and reveal windows within each 128-height epoch and journals the secret needed to reveal after restart. Its `/healthz`, `/readyz` and `/metrics` handlers describe evaluator health. A score submission is not yet a committed market update. Configuration and ABI compatibility must be checked at the actual registration/call path.

<a id="ai-artifacts-cli-commands"></a>
<a id="ai-artifacts-config-keys"></a>
<a id="ai-artifacts-file-formats"></a>
<a id="ai-artifacts-http-routes"></a>

## Private AI artifacts

[ai-artifacts/src/main.rs](../services/platform/hosted/ai-artifacts/src/main.rs) serves private artifact preparation, upload, finalize, resolve, import and access-control operations. [authority.rs](../services/platform/hosted/ai-artifacts/src/authority.rs), [store.rs](../services/platform/hosted/ai-artifacts/src/store.rs) and [retention.rs](../services/platform/hosted/ai-artifacts/src/retention.rs) own tenant/configuration admission, durable object state and retention/revocation.

Key releases are durable intents; tombstones, holds and retention high-water marks constrain recovery and deletion. Restoring below a live revocation or retention watermark must not become ready. Artifact service responses are off-chain storage evidence, not protocol receipts. Preserve tenant, publisher and access-envelope bindings when adapting its HTTP and file formats.

<a id="paxai-storage-keeper-file-formats"></a>
<a id="paxai-worker-cli-commands"></a>
<a id="paxai-worker-config-keys"></a>
<a id="paxai-worker-env-vars"></a>
<a id="paxai-worker-file-formats"></a>
<a id="paxai-worker-http-routes"></a>

## PaxAI worker and storage

[paxai-worker/src](../services/platform/hosted/paxai-worker/src) separates signed service transport, authenticated discovery, membership, jobs, runners, metadata and storage. [main.rs](../services/platform/hosted/paxai-worker/src/main.rs) wires the service; [src/bin](../services/platform/hosted/paxai-worker/src/bin) contains helper entry points. Worker configuration, persisted job/metadata formats and keeper material are governed by these readers, not generic node configuration.

Verify discovery and authorization before accepting work. Preserve restart state and the exact network/program/job identity in storage and submissions. Worker completion and stored artifacts do not establish an on-chain reward or finality; inspect the committed market state and proof at their owning boundary.

<a id="runtime-clock-cli-commands"></a>
<a id="runtime-clock-env-vars"></a>

## Runtime clock

[runtime-clock/src/main.rs](../services/platform/hosted/runtime-clock/src/main.rs) starts a bounded child command under a supervised clock server. Its command form is `layerx-runtime-clock [--runtime-dir PATH] -- COMMAND…`. It supplies `LAYERX_RUNTIME_CLOCK_SOCKET` and `LAYERX_RUNTIME_CLOCK_PID` to the child. These are runtime synchronization facilities, not consensus clocks or public service URLs. Preserve the supervisor lifecycle rather than substituting a fabricated time source.

<a id="paxeer-cli-commands"></a>
<a id="paxeer-env-vars"></a>
<a id="paxeer-error-codes"></a>
<a id="paxeer-http-routes"></a>
<a id="paxeer-jsonrpc-methods"></a>

## Retired Paxeer relay boundary

The separate hosted Paxeer relay role is retired by the merged-network retirement inventory. Remaining compatibility files under [hosted/paxeer](../services/platform/hosted/paxeer) do not justify deploying another relay or copying its old CLI, environment, HTTP, JSON-RPC or error contracts into a new integration. Send native node operations to the node root, hosted application operations to their configured gateway, and history queries to the node/archive or indexer boundary that actually owns the requested data.
