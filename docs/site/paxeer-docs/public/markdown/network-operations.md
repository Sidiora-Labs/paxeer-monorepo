<!-- Source: https://docs.paxeer.app/network-operations/ -->

# Network operations

Operate the hosted Paxeer X service graph, qualify journeys, preserve durable state, and recover across service boundaries.

The hosted platform connects identity, Human workflows, gateway admission, protocol execution, verified receipt authority, program registration, settlement, and outbound events. Each service has a specific authority and persistence boundary. Operating the network means keeping those boundaries consistent through startup, release changes, maintenance, and recovery. The concrete control implementation in the monorepo is the hosted testnet control service (`services/platform/hosted/testnet/`); readiness, journeys, and parameters below describe that service, not a generic production control plane.

## Service architecture

| Service boundary | Responsibility | Operational evidence |
| --- | --- | --- |
| Gateway | Public protocol API admission and routing. | Readiness, activity responses, and receipt lookup. |
| Identity and Human | Identity sessions, human journeys, approvals, and account-facing workflows. | Identity readiness and durable journey or approval state. |
| Node and core | Ordered execution, canonical receipts, and authorized administrative operations. | Node history, roots, receipts, and core journals. |
| Receipt authority and replica | Verify receipt inclusion and derive authorized batch facts. | Pinned identities, signed headers, inclusion evidence, and derived roots. |
| Registry and builder | Program artifacts, registration, and release material. | Artifact commitments, registry records, and compatible release versions. |
| Paxeer and guarantors | Custody, independent replay, checkpoint registration, and finality evidence. | Settlement receipts, checkpoint events, and matching node evidence. |
| Internal signing and event sources | Non-exporting signing APIs and verified journey, payment, approval, and program events. | Durable journals, principal binding, and event sequences. |
| Control | Dependency status, package and wire gates, and journey admission. | Readiness document and explicit failing dependencies. |

The gateway provides the public protocol entry point. Human APIs and administrative services remain separate service boundaries. Private-network control tooling manages disposable environments; its funding and reset endpoints are not public production account APIs. Production account funding follows the custody-credit workflow.

## Startup and release lifecycle

1. Validate the selected network and custody profile, release images, genesis inputs, and retained-material inventory.
2. Provision trust material and persistent stores before services that consume them start.
3. Start the Paxeer and trusted execution boundaries, then bootstrap node genesis and settlement bindings.
4. Publish the resulting settlement contract addresses and canonical module registry.
5. Start control, gateway, registry, and outbound-delivery services after their dependencies exist.
6. Require compatible package and wire versions, dependency readiness, and admitted application journeys.
7. Complete a real journey through execution and evidence inspection before promoting the release.

Registry preparation compares published module identifiers and activity ordinals with the node's advertised preparation surface. An observer node uses the same genesis with a separate process and data store. Genesis bytes, Comet chain identity, EVM chain identifier, and CA digest identify the intended settlement environment.

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

The control service declares exactly three journeys: `funding`, `payment`, and `receipt-inspection`. There is no `programs` journey.

| Journey | Required dependencies |
| --- | --- |
| `funding` | Identity, faucet, Redis, core admin, core. |
| `payment` | Identity, gateway, core, receipt authority. |
| `receipt-inspection` | Gateway, receipt authority, core. |

An admitted journey returns HTTP 200 with `admitted: true`, `ready: true`, and an empty `failing` array. A degraded journey returns HTTP 503 with named failures. The global probe also includes Paxeer; a Paxeer dependency failure can degrade global readiness while an execution journey remains admitted.

### Probe depth

Readiness probes establish only their declared dependency condition (connection-level checks such as HTTP readiness, TLS handshake, TCP connection, or Redis connectivity). A real payment, registry operation, or settled checkpoint supplies deeper application evidence. The exact probe depth per dependency was not verified against the testnet probe bodies and is not specified here.

## Durability and recovery

Core operation journals, Human state, registry records, internal event journals, signing material, and node history are durable state with distinct ownership. Preserve the complete material set needed by each service. Replacing a container does not authorize generating a new identity or seal secret for an existing store.

### Signing boundary

Internal signing uses non-exporting signing APIs: callers create key handles and sign through authenticated routes without receiving seed material. Detailed sealing parameters were not verified in source and are not specified here.

The signing API does not export seeds. KMS readiness requires the exclusive store lock and a successful durable-write probe. Access to signing routes requires a verified client certificate and bearer identity.

### Verified event sources

Journey, payment, approval, and program event sources bind provisioned principals to their upstream credentials. Payment facts come from a decoded protocol receipt. Exact per-principal sequence and idempotency rules were not verified in source and are not specified here.

Internal journals append and synchronize records before reporting success, hold an exclusive lock, and replay in append order. A failed append marks the journal unavailable. Event-source readiness requires upstream readiness, principal binding, and a writable journal; this prevents an apparently healthy process from silently losing new events.

## Maintenance and draining

1. Record the active release, network identity, latest verified batch and checkpoint, and outstanding operation identifiers.
2. Remove the affected service from new ingress admission through the deployment's traffic controls.
3. Reconcile accepted operations from their durable receipts and journals, preserving idempotency keys for retries.
4. Preserve outstanding event delivery state, signing material, and persistent volumes before replacing a process.
5. Restore the service with matching identity and release material, then inspect replay and readiness results.
6. Readmit traffic after the relevant journey and its evidence checks pass.

Draining is a deployment and reconciliation workflow. The control API exposes readiness and journey admission; it does not define a universal drain endpoint. A pending operation remains pending until its canonical receipt or durable outcome resolves it.

## Disposable environment tooling

The repository provides these operator targets for an isolated hosted environment. Cluster creation and teardown change infrastructure; teardown deletes the disposable cluster, persistent volumes, and local generated material.

```
make platform-test-tooling
make platform-hosted-topology-check
make platform-beta-cluster-up
make platform-hosted-smoke
# After the disposable environment is no longer needed:
make platform-beta-cluster-down
```

Hosted smoke consumes the environment generated by bring-up and requires complete inputs. Retained-material mode validates inventory completeness, ownership, permissions, profile selection, registry consistency, and the live KMS seal digest before reuse. Retaining material supports another startup against the existing environment; it does not preserve data through teardown.

## Failure triage

1. Read the global readiness document, then the specific journey's failing dependency list.
2. Check the named dependency at its own boundary and distinguish connection health from verified application results.
3. For release failures, compare package and wire versions before changing traffic admission.
4. For journal or seal failures, preserve the store and restore matching material before restarting consumers.
5. For settlement failures, compare the intended domain, certificate, canonical transaction receipt, and registered checkpoint evidence.

Continue with [Node operators](https://docs.paxeer.app/operators), [Guarantors](https://docs.paxeer.app/guarantors), [Security and trust boundaries](https://docs.paxeer.app/security), and [Governance](https://docs.paxeer.app/governance).
