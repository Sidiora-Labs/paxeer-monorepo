<!-- Source: https://docs.paxeer.app/network-operations/ -->

# Network operations

Operate Paxeer mainnet-beta nodes and the optional hosted services, monitor readiness, and preserve durable state.

Paxeer mainnet-beta is one BFT network with EVM chain ID 125. The kernel runs inside `paxd` and participates in block proposal and execution. A receipt is final when its block is committed. Hosted identity, Human, gateway, registry, signing, and event-delivery services support applications around that network. The control routes below belong to `services/platform/hosted/testnet/`, the control service for an isolated hosted environment.

## Service architecture

| Service boundary | Responsibility | Operational evidence |
| --- | --- | --- |
| Gateway | Application JSON-RPC admission, authentication, and upstream calls. | Readiness, JSON-RPC responses, and verified receipt lookup. |
| Identity and Human | Identity sessions, human journeys, approvals, and account-facing workflows. | Identity readiness and durable journey or approval state. |
| Node and core | The node commits execution; hosted core coordinates payment and funding operations. | Committed blocks, receipts, and core operation journals. |
| Receipt authority | Verify signed receipt material for hosted consumers. | Configured receipt keys, transaction binding, and receipt digest. |
| Registry and builder | Program artifacts, registration, and release material. | Artifact commitments, registry records, and compatible release versions. |
| Chain consensus | Commit blocks through the validator set. | Block headers, commits, validator sets, and application state. |
| Internal signing and event sources | Non-exporting signing APIs and verified journey, payment, approval, and program events. | Durable journals, principal binding, and event sequences. |
| Control | Dependency status, package and wire gates, and journey admission. | Readiness document and explicit failing dependencies. |

Public node JSON-RPC serves the node namespaces at the root path. The hosted gateway additionally serves its declared `px_` methods. Human and administrative APIs have their own authorization boundaries. The testnet control service exposes protected funding and reset operations for its configured environment; application payments use the chain ledger.

## Startup and release lifecycle

1. Select the node release, genesis, chain identity, and node profile for the intended network.
2. Provision service trust material and persistent stores before starting their consumers.
3. Start the chain node with its configured kernel and verify synchronization.
4. Confirm the connected EVM chain identifier and enabled application interfaces.
5. Start configured hosted services after their dependencies are reachable.
6. Check package and wire versions, dependency readiness, and application journey admission.
7. Complete an application operation and inspect its transaction and receipt evidence before admitting traffic.

Use the same genesis and chain identity for nodes joining the same network, with a separate data directory and peer identity for each node. Hosted clients must use the matching EVM chain identifier and configured trust roots. Tenant domains share the network infrastructure and namespace their state in the database.

### Package and protocol compatibility

The control release gate compares package semantic version and wire protocol version independently. A compatible package name does not override a wire-version mismatch. Global readiness includes the release gate; a journey can have healthy dependencies while the overall release remains degraded.

## Health and journey admission

| Control route | Meaning |
| --- | --- |
| `GET /livez` | The control process answers requests. |
| `GET /readyz` | All declared dependencies, all journeys, and the release gate are ready; otherwise HTTP 503 includes the degraded document. |
| `GET /v1/status` | Summary state for control, gateway, core, and Paxeer components. |
| `GET /v1/parameters` | Testnet-specific parameters (network `layerx-testnet`, package and wire versions, monthly reset schedule). |
| `GET /v1/journeys/{journey}` | Admission for a named journey, including failing dependencies. |

These routes describe the control service inside the operator's configured environment. Configure the control origin explicitly; the paths are not a promise of a public control hostname.

The control service declares three journey routes: `funding`, `payment`, and `receipt-inspection`. The last route reports the journey name `receipt_inspection` in JSON.

| Journey | Required dependencies |
| --- | --- |
| `funding` | Identity, faucet, Redis, core admin, core. |
| `payment` | Identity, gateway, core, receipt authority. |
| `receipt-inspection` | Gateway, receipt authority, core. |

An admitted journey returns HTTP 200 with `admitted: true`, `ready: true`, and an empty `failing` array. A degraded journey returns HTTP 503 with named failures. The global probe also includes Paxeer; a Paxeer dependency failure can degrade global readiness while an execution journey remains admitted.

### Probe depth

Identity, faucet, core, receipt authority, gateway, and Paxeer probes require HTTP 200 from their configured `/readyz` routes. The core-admin probe completes a TLS handshake. Redis is probed over TLS with PING; either PONG or an authentication-required response establishes reachability. These checks do not execute an application payment. Inspect a real transaction and its verified receipt for application evidence.

## Durability and recovery

Core operation journals, Human state, registry records, internal event journals, signing material, and chain data have distinct owners. Preserve the complete material set needed by each service. Restore an existing store with its matching signing and sealing material.

### Signing boundary

Internal signing uses authenticated APIs: callers create key handles and request signatures without receiving private material. Stored private material is sealed with AES-256-GCM using a key derived from the configured secret and domain label, with a fresh nonce for each sealed value.

The signing API does not export seeds. KMS readiness requires the exclusive store lock and a successful durable-write probe. Access to signing routes requires a verified client certificate and bearer identity.

### Verified event sources

Journey, payment, approval, and program event sources bind provisioned principals to upstream credentials. Payment facts come from verified receipt data. The event store advances a separate sequence for each principal and subject and validates the sequence while replaying its journal.

Internal journals append and synchronize records before reporting success, hold an exclusive lock, and replay in append order. A failed append marks the journal unavailable. Event-source readiness requires upstream readiness, principal binding, and a writable journal; this prevents an apparently healthy process from silently losing new events.

## Maintenance and draining

1. Record the active release, network identity, latest committed block and verified receipt, and outstanding operation identifiers.
2. Remove the affected service from new ingress admission through the deployment's traffic controls.
3. Reconcile accepted operations from their durable receipts and journals, preserving idempotency keys for retries.
4. Preserve outstanding event delivery state, signing material, and persistent volumes before replacing a process.
5. Restore the service with matching identity and release material, then inspect replay and readiness results.
6. Readmit traffic after the relevant journey and its evidence checks pass.

Draining is a deployment and reconciliation workflow. The control API exposes readiness and journey admission; it does not define a universal drain endpoint. A pending operation remains pending until its canonical receipt or durable outcome resolves it.

## Disposable environment tooling

The chain Makefile provides a disposable Docker cluster and a separate monitoring variant. Run these commands from the monorepo root. Startup stops the corresponding cluster and replaces generated cluster configuration; use a directory dedicated to development.

```
DOCKER_DETACH=true make -f chain.mk docker-cluster-start
# Or start the cluster with Prometheus and Grafana:
DOCKER_DETACH=true make -f chain.mk docker-cluster-start-monitoring
# Stop the matching cluster:
make -f chain.mk docker-cluster-stop
make -f chain.mk docker-cluster-stop-monitoring
```

The cluster targets in `chain.mk` use the Compose definitions under `docker/`. The monitoring variant adds the monitoring Compose file. Keep generated development configuration separate from a mainnet node home and retain the stores and signing state needed for any planned recovery.

## Failure triage

1. Read the global readiness document, then the specific journey's failing dependency list.
2. Check the named dependency at its own boundary and distinguish connection health from verified application results.
3. For release failures, compare package and wire versions before changing traffic admission.
4. For journal or seal failures, preserve the store and restore matching material before restarting consumers.
5. For payment failures, compare the intended chain, signed transaction, receipt digest, and committed-block evidence required by the application.

Continue with [Node operators](https://docs.paxeer.app/operators), [Related protocol reference](https://docs.paxeer.app/guarantors), [Security and trust boundaries](https://docs.paxeer.app/security), and [Governance](https://docs.paxeer.app/governance).
