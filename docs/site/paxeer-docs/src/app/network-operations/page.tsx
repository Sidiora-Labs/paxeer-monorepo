import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/network-operations')

export default function NetworkOperations() {
  return <DocsLayout>
    <h1>Network operations</h1>
    <p className="page-description">Operate the hosted Paxeer X service graph, qualify journeys, preserve durable state, and recover across service boundaries.</p>
    <p>The hosted platform connects identity, Human workflows, gateway admission, protocol execution, verified receipt authority, program registration, settlement, and outbound events. Each service has a specific authority and persistence boundary. Operating the network means keeping those boundaries consistent through startup, release changes, maintenance, and recovery. The concrete control implementation in the monorepo is the hosted testnet control service (<code>services/platform/hosted/testnet/</code>); readiness, journeys, and parameters below describe that service, not a generic production control plane.</p>
    <h2>Service architecture</h2>
    <table><thead><tr><th>Service boundary</th><th>Responsibility</th><th>Operational evidence</th></tr></thead><tbody>
      <tr><td>Gateway</td><td>Public protocol API admission and routing.</td><td>Readiness, activity responses, and receipt lookup.</td></tr>
      <tr><td>Identity and Human</td><td>Identity sessions, human journeys, approvals, and account-facing workflows.</td><td>Identity readiness and durable journey or approval state.</td></tr>
      <tr><td>Node and core</td><td>Ordered execution, canonical receipts, and authorized administrative operations.</td><td>Node history, roots, receipts, and core journals.</td></tr>
      <tr><td>Receipt authority and replica</td><td>Verify receipt inclusion and derive authorized batch facts.</td><td>Pinned identities, signed headers, inclusion evidence, and derived roots.</td></tr>
      <tr><td>Registry and builder</td><td>Program artifacts, registration, and release material.</td><td>Artifact commitments, registry records, and compatible release versions.</td></tr>
      <tr><td>Paxeer and guarantors</td><td>Custody, independent replay, checkpoint registration, and finality evidence.</td><td>Settlement receipts, checkpoint events, and matching node evidence.</td></tr>
      <tr><td>Internal signing and event sources</td><td>Non-exporting signing APIs and verified journey, payment, approval, and program events.</td><td>Durable journals, principal binding, and event sequences.</td></tr>
      <tr><td>Control</td><td>Dependency status, package and wire gates, and journey admission.</td><td>Readiness document and explicit failing dependencies.</td></tr>
    </tbody></table>
    <p>The gateway provides the public protocol entry point. Human APIs and administrative services remain separate service boundaries. Private-network control tooling manages disposable environments; its funding and reset endpoints are not public production account APIs. Production account funding follows the custody-credit workflow.</p>
    <h2>Startup and release lifecycle</h2>
    <ol><li>Validate the selected network and custody profile, release images, genesis inputs, and retained-material inventory.</li><li>Provision trust material and persistent stores before services that consume them start.</li><li>Start the Paxeer and trusted execution boundaries, then bootstrap node genesis and settlement bindings.</li><li>Publish the resulting settlement contract addresses and canonical module registry.</li><li>Start control, gateway, registry, and outbound-delivery services after their dependencies exist.</li><li>Require compatible package and wire versions, dependency readiness, and admitted application journeys.</li><li>Complete a real journey through execution and evidence inspection before promoting the release.</li></ol>
    <p>Registry preparation compares published module identifiers and activity ordinals with the node&apos;s advertised preparation surface. An observer node uses the same genesis with a separate process and data store. Genesis bytes, Comet chain identity, EVM chain identifier, and CA digest identify the intended settlement environment.</p>
    <h3>Package and protocol compatibility</h3>
    <p>The control release gate compares package semantic version and wire protocol version independently. A compatible package name does not override a wire-version mismatch. Global readiness includes the release gate; a journey can have healthy dependencies while the overall release remains degraded.</p>
    <h2>Health and journey admission</h2>
    <table><thead><tr><th>Control route</th><th>Meaning</th></tr></thead><tbody><tr><td><code>GET /livez</code></td><td>The control process answers requests.</td></tr><tr><td><code>GET /readyz</code></td><td>All declared dependencies, all journeys, and the release gate are ready; otherwise HTTP 503 includes the degraded document.</td></tr><tr><td><code>GET /v1/status</code></td><td>Summary state for control, gateway, core, and Paxeer components.</td></tr><tr><td><code>GET /v1/parameters</code></td><td>Testnet-specific parameters (network <code>layerx-testnet</code>, package and wire versions, monthly reset schedule).</td></tr><tr><td><code>GET /v1/journeys/&#123;journey&#125;</code></td><td>Admission for a named journey, including failing dependencies.</td></tr></tbody></table>
    <p>These routes describe the control service inside the operator&apos;s configured environment. Configure the control origin explicitly; the paths are not a promise of a public control hostname.</p>
    <p>The control service declares exactly three journeys: <code>funding</code>, <code>payment</code>, and <code>receipt-inspection</code>. There is no <code>programs</code> journey.</p>
    <table><thead><tr><th>Journey</th><th>Required dependencies</th></tr></thead><tbody><tr><td><code>funding</code></td><td>Identity, faucet, Redis, core admin, core.</td></tr><tr><td><code>payment</code></td><td>Identity, gateway, core, receipt authority.</td></tr><tr><td><code>receipt-inspection</code></td><td>Gateway, receipt authority, core.</td></tr></tbody></table>
    <p>An admitted journey returns HTTP 200 with <code>admitted: true</code>, <code>ready: true</code>, and an empty <code>failing</code> array. A degraded journey returns HTTP 503 with named failures. The global probe also includes Paxeer; a Paxeer dependency failure can degrade global readiness while an execution journey remains admitted.</p>
    <h3>Probe depth</h3>
    <p>Readiness probes establish only their declared dependency condition (connection-level checks such as HTTP readiness, TLS handshake, TCP connection, or Redis connectivity). A real payment, registry operation, or settled checkpoint supplies deeper application evidence. The exact probe depth per dependency was not verified against the testnet probe bodies and is not specified here.</p>
    <h2>Durability and recovery</h2>
    <p>Core operation journals, Human state, registry records, internal event journals, signing material, and node history are durable state with distinct ownership. Preserve the complete material set needed by each service. Replacing a container does not authorize generating a new identity or seal secret for an existing store.</p>
    <h3>Signing boundary</h3>
    <p>Internal signing uses non-exporting signing APIs: callers create key handles and sign through authenticated routes without receiving seed material. Detailed sealing parameters were not verified in source and are not specified here.</p>
    <p>The signing API does not export seeds. KMS readiness requires the exclusive store lock and a successful durable-write probe. Access to signing routes requires a verified client certificate and bearer identity.</p>
    <h3>Verified event sources</h3>
    <p>Journey, payment, approval, and program event sources bind provisioned principals to their upstream credentials. Payment facts come from a decoded protocol receipt. Exact per-principal sequence and idempotency rules were not verified in source and are not specified here.</p>
    <p>Internal journals append and synchronize records before reporting success, hold an exclusive lock, and replay in append order. A failed append marks the journal unavailable. Event-source readiness requires upstream readiness, principal binding, and a writable journal; this prevents an apparently healthy process from silently losing new events.</p>
    <h2>Maintenance and draining</h2>
    <ol><li>Record the active release, network identity, latest verified batch and checkpoint, and outstanding operation identifiers.</li><li>Remove the affected service from new ingress admission through the deployment&apos;s traffic controls.</li><li>Reconcile accepted operations from their durable receipts and journals, preserving idempotency keys for retries.</li><li>Preserve outstanding event delivery state, signing material, and persistent volumes before replacing a process.</li><li>Restore the service with matching identity and release material, then inspect replay and readiness results.</li><li>Readmit traffic after the relevant journey and its evidence checks pass.</li></ol>
    <p>Draining is a deployment and reconciliation workflow. The control API exposes readiness and journey admission; it does not define a universal drain endpoint. A pending operation remains pending until its canonical receipt or durable outcome resolves it.</p>
    <h2>Disposable environment tooling</h2>
    <p>The repository provides these operator targets for an isolated hosted environment. Cluster creation and teardown change infrastructure; teardown deletes the disposable cluster, persistent volumes, and local generated material.</p>
    <pre><code>{`make platform-test-tooling
make platform-hosted-topology-check
make platform-beta-cluster-up
make platform-hosted-smoke
# After the disposable environment is no longer needed:
make platform-beta-cluster-down`}</code></pre>
    <p>Hosted smoke consumes the environment generated by bring-up and requires complete inputs. Retained-material mode validates inventory completeness, ownership, permissions, profile selection, registry consistency, and the live KMS seal digest before reuse. Retaining material supports another startup against the existing environment; it does not preserve data through teardown.</p>
    <h2>Failure triage</h2>
    <ol><li>Read the global readiness document, then the specific journey&apos;s failing dependency list.</li><li>Check the named dependency at its own boundary and distinguish connection health from verified application results.</li><li>For release failures, compare package and wire versions before changing traffic admission.</li><li>For journal or seal failures, preserve the store and restore matching material before restarting consumers.</li><li>For settlement failures, compare the intended domain, certificate, canonical transaction receipt, and registered checkpoint evidence.</li></ol>
    <p>Continue with <Link href="/operators">Node operators</Link>, <Link href="/guarantors">Guarantors</Link>, <Link href="/security">Security and trust boundaries</Link>, and <Link href="/governance">Governance</Link>.</p>
  </DocsLayout>
}
