import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/discovery')

export default function DiscoveryPage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Program Discovery</h1><p className="page-description">Discover registered programs and resolve current interfaces, lifecycle, accounts and signed deployment provenance.</p></div>
    <h2>Resolve evidence before execution</h2>
    <p>Paxeer X discovery lets wallets, agents and applications find registered programs and inspect the version they are about to call. Discovery combines the program registry, a verified current protocol head, published interfaces and optional sequencer-signed discovery attestations. Each resolved program carries identity, version and freshness information that the client can bind to its preparation.</p>
    <p>A program ID is a stable 32-byte identifier. The latest code hash, ABI, lifecycle and account bindings describe its current verified execution context. An application preserves both the stable identity and the version-specific evidence when presenting an action to its user.</p>
    <h2>Start with the registry index</h2>
    <pre><code>{`GET /v1/programs/registry
Authorization: Bearer <registry-request-token>

Response fields:
program_ids
state_root
observed_sequence
observed_at
valid_through
verification = "registry-receipt-and-current-head-verified"`}</code></pre>
    <p>Direct calls use the registry&apos;s mutual TLS connection and request credential. The index synchronizes protocol state and checks journal read authority before returning registered IDs. Its authenticated head and freshness window describe the observation used for that response.</p>
    <h3>Resolve one program</h3>
    <pre><code>{`GET /v1/programs/registry/<64-hex-program-id>
GET /v1/programs/registry/<64-hex-program-id>/interface`}</code></pre>
    <p>The first response includes versions, current lifecycle, upgrade policy, account bindings, source status and deployment receipts. The interface response binds canonical interface bytes and their digest to a specific program, version, code hash and ABI. Its verification marker is <code>deployment-interface-and-current-head-verified</code>.</p>
    <h2>What a resolver checks</h2>
    <table><thead><tr><th>Check</th><th>Required relationship</th></tr></thead><tbody>
      <tr><td>Program identity</td><td>The returned program ID equals the requested 32-byte identity.</td></tr>
      <tr><td>Deployment</td><td>The selected version has a verified deployment receipt and matching code hash.</td></tr>
      <tr><td>Interface</td><td>Interface version, code hash and ABI match the selected current deployment.</td></tr>
      <tr><td>Lifecycle</td><td>The application handles active, deprecated and tombstoned status deliberately.</td></tr>
      <tr><td>Freshness</td><td>The current time remains inside <code>valid_through</code> for the verified observation.</td></tr>
      <tr><td>Value accounts</td><td>Account bindings and balance proofs match program state and the observed head.</td></tr>
      <tr><td>Source</td><td>A verified source claim reproduces the deployed artifact under its named environment.</td></tr>
    </tbody></table>
    <p>Resolve related records against compatible current evidence. If an upgrade or lifecycle change occurs between reads, fetch the program and interface again before preparing a call. Keep the canonical interface digest and code hash attached to the user-facing disclosure or internal execution record.</p>
    <h2>Signed discovery attestations — current status</h2>
    <p>The signed-discovery format is defined on the client side: an attested tuple of program ID, version, code hash, ABI version, observed sequence, observation time, validity bound and state root, with the expected head receipt digest and an independently authenticated sequencer authority.</p>
    <table><thead><tr><th>Field consumed by clients</th><th>Meaning</th></tr></thead><tbody>
      <tr><td><code>receipt_digest</code></td><td>Digest of the program discovery head attestation.</td></tr>
      <tr><td><code>discovery_public_key</code></td><td>The verified 32-byte sequencer public key.</td></tr>
      <tr><td><code>discovery_signature</code></td><td>The 64-byte signature over the discovery proof digest.</td></tr>
    </tbody></table>
    <p>The registry&apos;s program read response does not currently emit <code>discovery_public_key</code> or <code>discovery_signature</code>; its receipt object carries <code>deployment_receipt_digest</code> only. The CLI&apos;s program-call head verification requires all three discovery fields and fails with <em>program discovery omitted public key</em> (or <em>signature</em>) when they are absent. Signed discovery as a registry-published feature is therefore not yet servable: a client whose policy requires it will refuse to proceed against the current registry, while the receipt-and-current-head verification above remains available.</p>
    <h3>Missing proof fields</h3>
    <p>A registry record carries verified deployment information without the discovery signature fields. A client that requires signed discovery must withhold execution until a registry build publishes them. Display the verification level supported by the returned evidence.</p>
    <h2>Application journey</h2>
    <ol><li>Fetch the registry index and retain its observed head and validity bound.</li><li>Resolve the chosen program and inspect its current version, lifecycle and upgrade authority.</li><li>Fetch the matching interface and validate its program, ABI, code hash and interface digest.</li><li>Verify discovery signatures when required by the application&apos;s trust policy.</li><li>Resolve program accounts and source provenance relevant to the intended action.</li><li>Prepare the activity and disclose the target program, action, amounts, counterparties and fees.</li><li>Track execution and retain the resulting receipt alongside the discovery evidence.</li></ol>
    <p>For example, a treasury agent discovers a payment program, confirms its current ABI and active lifecycle, checks its permitted assets and accounts, then prepares a bounded payment under its capability and budget. A changed version or stale discovery window triggers a fresh resolution before signing.</p>
    <h2>Lifecycle and account-aware discovery</h2>
    <p>Immutable programs advertise an immutable upgrade policy. Upgradeable programs identify their authority and preserve a numbered deployment history. Lifecycle history and exit routes let applications explain changes and route users through appropriate recovery or withdrawal journeys.</p>
    <p>ABI 1 programs without value accounts declare <code>account-incapable-abi1</code>. Account-capable records supply proof-verified bindings and balances. Account profile 2 includes the owner established by its verified state. Resolve these bindings directly instead of inferring account ownership from names or UI labels.</p>
    <h2>Source and publisher identity</h2>
    <p>Source verification links a mirrored archive, reproducible build environment and artifact digest to a verified deployment. Publication keys resolve through hosted identity, associating publication with a principal. Source status and publisher identity complement execution evidence: application trust policy decides whether a source verdict or publisher binding is required for a particular interaction.</p>
    <p>Use the <Link href="/registry">registry source workflow</Link> to publish archives and request verification. Preserve source digest, environment digest and deployment provenance when reviewing releases or generating a program catalog.</p>
    <h2>Operate a discovery service</h2>
    <p>Back discovery with the canonical deployment journal, authenticated node-state connection and independent receipt authority. Keep sequencer trust history current. Synchronization persists verified program state before advancing its cursor, and refuses regressions or incompatible state. Restart replay reconstructs the projection from committed evidence and refreshes its current head.</p>
    <p>The default registry staleness bound is 300 seconds. Cache records with their <code>valid_through</code>, state root and version; revalidate before an economic action. A healthy HTTP listener does not itself establish fresh program evidence.</p>
    <table><thead><tr><th>Result</th><th>Resolver behavior</th></tr></thead><tbody>
      <tr><td><code>404 not_found</code></td><td>Treat the program as absent from this verified registry view.</td></tr>
      <tr><td><code>404 interface_absent</code></td><td>Do not invent an interface; require published current-version metadata.</td></tr>
      <tr><td><code>503 stale_read</code>, <code>protocol_head_unavailable</code></td><td>Refresh evidence before preparing a new action.</td></tr>
      <tr><td><code>interface_registry_mismatch</code>, <code>balance_registry_mismatch</code></td><td>Reject inconsistent records and resolve matching evidence.</td></tr>
      <tr><td>Missing discovery signature</td><td>Apply the client&apos;s required verification policy explicitly.</td></tr>
    </tbody></table>
    <p>Continue with <Link href="/registry">registry deployment and journals</Link>, <Link href="/identity">owner identity</Link> or <Link href="/agent-budgets">agent spending controls</Link>.</p>
    <div className="source-note"><strong>Technical references:</strong> <code>services/platform/hosted/registry/src/routes.rs</code>, <code>head_attestation.rs</code>, <code>node_state.rs</code> and the deployment journal contract in the monorepo.</div>
  </DocsLayout>
}
