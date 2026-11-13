import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/network-operations')

export default function NetworkOperations() {
  return <DocsLayout>
    <h1>Network operations</h1>
    <p className="page-description">Operate Paxeer mainnet-beta nodes and the optional hosted services, monitor readiness, and preserve durable state.</p>
    <p>Paxeer mainnet-beta is one BFT network with EVM chain ID 125. The kernel runs inside <code>paxd</code> and participates in block proposal and execution. A receipt is final when its block is committed. Hosted identity, Human, gateway, registry, signing, and event-delivery services support applications around that network. The control routes below belong to <code>services/platform/hosted/testnet/</code>, the control service for an isolated hosted environment.</p>
    <h2>Service architecture</h2>
    <table><thead><tr><th>Service boundary</th><th>Responsibility</th><th>Operational evidence</th></tr></thead><tbody>
      <tr><td>Gateway</td><td>Application JSON-RPC admission, authentication, and upstream calls.</td><td>Readiness, JSON-RPC responses, and verified receipt lookup.</td></tr>
      <tr><td>Identity and Human</td><td>Identity sessions, human journeys, approvals, and account-facing workflows.</td><td>Identity readiness and durable journey or approval state.</td></tr>
      <tr><td>Node and core</td><td>The node commits execution; hosted core coordinates payment and funding operations.</td><td>Committed blocks, receipts, and core operation journals.</td></tr>
      <tr><td>Receipt authority</td><td>Verify signed receipt material for hosted consumers.</td><td>Configured receipt keys, transaction binding, and receipt digest.</td></tr>
      <tr><td>Registry and builder</td><td>Program artifacts, registration, and release material.</td><td>Artifact commitments, registry records, and compatible release versions.</td></tr>
      <tr><td>Chain consensus</td><td>Commit blocks through the validator set.</td><td>Block headers, commits, validator sets, and application state.</td></tr>
      <tr><td>Internal signing and event sources</td><td>Non-exporting signing APIs and verified journey, payment, approval, and program events.</td><td>Durable journals, principal binding, and event sequences.</td></tr>
      <tr><td>Control</td><td>Dependency status, package and wire gates, and journey admission.</td><td>Readiness document and explicit failing dependencies.</td></tr>
    </tbody></table>
    <p>Public node JSON-RPC serves the node namespaces at the root path. The hosted gateway additionally serves its declared <code>px_</code> methods. Human and administrative APIs have their own authorization boundaries. The testnet control service exposes protected funding and reset operations for its configured environment; application payments use the chain ledger.</p>
    <h2>Startup and release lifecycle</h2>
    <ol><li>Select the node release, genesis, chain identity, and node profile for the intended network.</li><li>Provision service trust material and persistent stores before starting their consumers.</li><li>Start the chain node with its configured kernel and verify synchronization.</li><li>Confirm the connected EVM chain identifier and enabled application interfaces.</li><li>Start configured hosted services after their dependencies are reachable.</li><li>Check package and wire versions, dependency readiness, and application journey admission.</li><li>Complete an application operation and inspect its transaction and receipt evidence before admitting traffic.</li></ol>
    <p>Use the same genesis and chain identity for nodes joining the same network, with a separate data directory and peer identity for each node. Hosted clients must use the matching EVM chain identifier and configured trust roots. Tenant domains share the network infrastructure and namespace their state in the database.</p>
    <h3>Package and protocol compatibility</h3>
    <p>The control release gate compares package semantic version and wire protocol version independently. A compatible package name does not override a wire-version mismatch. Global readiness includes the release gate; a journey can have healthy dependencies while the overall release remains degraded.</p>
    <h2>Health and journey admission</h2>
    <table><thead><tr><th>Control route</th><th>Meaning</th></tr></thead><tbody><tr><td><code>GET /livez</code></td><td>The control process answers requests.</td></tr><tr><td><code>GET /readyz</code></td><td>All declared dependencies, all journeys, and the release gate are ready; otherwise HTTP 503 includes the degraded document.</td></tr><tr><td><code>GET /v1/status</code></td><td>Summary state for control, gateway, core, and Paxeer components.</td></tr><tr><td><code>GET /v1/parameters</code></td><td>Testnet-specific parameters (network <code>layerx-testnet</code>, package and wire versions, monthly reset schedule).</td></tr><tr><td><code>GET /v1/journeys/&#123;journey&#125;</code></td><td>Admission for a named journey, including failing dependencies.</td></tr></tbody></table>
    <p>These routes describe the control service inside the operator&apos;s configured environment. Configure the control origin explicitly; the paths are not a promise of a public control hostname.</p>
    <p>The control service declares three journey routes: <code>funding</code>, <code>payment</code>, and <code>receipt-inspection</code>. The last route reports the journey name <code>receipt_inspection</code> in JSON.</p>
    <table><thead><tr><th>Journey</th><th>Required dependencies</th></tr></thead><tbody><tr><td><code>funding</code></td><td>Identity, faucet, Redis, core admin, core.</td></tr><tr><td><code>payment</code></td><td>Identity, gateway, core, receipt authority.</td></tr><tr><td><code>receipt-inspection</code></td><td>Gateway, receipt authority, core.</td></tr></tbody></table>
    <p>An admitted journey returns HTTP 200 with <code>admitted: true</code>, <code>ready: true</code>, and an empty <code>failing</code> array. A degraded journey returns HTTP 503 with named failures. The global probe also includes Paxeer; a Paxeer dependency failure can degrade global readiness while an execution journey remains admitted.</p>
    <h3>Probe depth</h3>
    <p>Identity, faucet, core, receipt authority, gateway, and Paxeer probes require HTTP 200 from their configured <code>/readyz</code> routes. The core-admin probe completes a TLS handshake. Redis is probed over TLS with PING; either PONG or an authentication-required response establishes reachability. These checks do not execute an application payment. Inspect a real transaction and its verified receipt for application evidence.</p>
    <h2>Durability and recovery</h2>
    <p>Core operation journals, Human state, registry records, internal event journals, signing material, and chain data have distinct owners. Preserve the complete material set needed by each service. Restore an existing store with its matching signing and sealing material.</p>
    <h3>Signing boundary</h3>
    <p>Internal signing uses authenticated APIs: callers create key handles and request signatures without receiving private material. Stored private material is sealed with AES-256-GCM using a key derived from the configured secret and domain label, with a fresh nonce for each sealed value.</p>
    <p>The signing API does not export seeds. KMS readiness requires the exclusive store lock and a successful durable-write probe. Access to signing routes requires a verified client certificate and bearer identity.</p>
    <h3>Verified event sources</h3>
    <p>Journey, payment, approval, and program event sources bind provisioned principals to upstream credentials. Payment facts come from verified receipt data. The event store advances a separate sequence for each principal and subject and validates the sequence while replaying its journal.</p>
    <p>Internal journals append and synchronize records before reporting success, hold an exclusive lock, and replay in append order. A failed append marks the journal unavailable. Event-source readiness requires upstream readiness, principal binding, and a writable journal; this prevents an apparently healthy process from silently losing new events.</p>
    <h2>Maintenance and draining</h2>
    <ol><li>Record the active release, network identity, latest committed block and verified receipt, and outstanding operation identifiers.</li><li>Remove the affected service from new ingress admission through the deployment&apos;s traffic controls.</li><li>Reconcile accepted operations from their durable receipts and journals, preserving idempotency keys for retries.</li><li>Preserve outstanding event delivery state, signing material, and persistent volumes before replacing a process.</li><li>Restore the service with matching identity and release material, then inspect replay and readiness results.</li><li>Readmit traffic after the relevant journey and its evidence checks pass.</li></ol>
    <p>Draining is a deployment and reconciliation workflow. The control API exposes readiness and journey admission; it does not define a universal drain endpoint. A pending operation remains pending until its canonical receipt or durable outcome resolves it.</p>
    <h2>Disposable environment tooling</h2>
    <p>The chain Makefile provides a disposable Docker cluster and a separate monitoring variant. Run these commands from the monorepo root. Startup stops the corresponding cluster and replaces generated cluster configuration; use a directory dedicated to development.</p>
    <pre><code>{`DOCKER_DETACH=true make -f chain.mk docker-cluster-start
# Or start the cluster with Prometheus and Grafana:
DOCKER_DETACH=true make -f chain.mk docker-cluster-start-monitoring
# Stop the matching cluster:
make -f chain.mk docker-cluster-stop
make -f chain.mk docker-cluster-stop-monitoring`}</code></pre>
    <p>The cluster targets in <code>chain.mk</code> use the Compose definitions under <code>docker/</code>. The monitoring variant adds the monitoring Compose file. Keep generated development configuration separate from a mainnet node home and retain the stores and signing state needed for any planned recovery.</p>
    <h2>Failure triage</h2>
    <ol><li>Read the global readiness document, then the specific journey&apos;s failing dependency list.</li><li>Check the named dependency at its own boundary and distinguish connection health from verified application results.</li><li>For release failures, compare package and wire versions before changing traffic admission.</li><li>For journal or seal failures, preserve the store and restore matching material before restarting consumers.</li><li>For payment failures, compare the intended chain, signed transaction, receipt digest, and committed-block evidence required by the application.</li></ol>
    <p>Continue with <Link href="/operators">Node operators</Link>, <Link href="/guarantors">Related protocol reference</Link>, <Link href="/security">Security and trust boundaries</Link>, and <Link href="/governance">Governance</Link>.</p>
  </DocsLayout>
}
