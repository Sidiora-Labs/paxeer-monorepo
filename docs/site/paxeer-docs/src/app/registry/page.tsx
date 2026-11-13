import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import Link from 'next/link'

export const metadata = metadataFor('/registry')

export default function RegistryPage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Program Registry</h1><p className="page-description">Resolve proof-verified program deployments, versioned interfaces and reproducible source in the hosted registry.</p></div>
    <DocArt name="model-registry" />
    <h2>A verified record of deployed programs</h2>
    <p>The Paxeer X registry connects a program identity to its deployment receipts, executable code hashes, ABI versions, upgrade policy, lifecycle and value accounts. Applications resolve the current verified version before preparing calls. Publishers attach source that the registry rebuilds in a pinned environment and compares with the registered artifact.</p>
    <p>Registry records derive from authenticated deployment proofs and committed program state. Deploy the program on chain, then submit its canonical deployment evidence. Registration verifies that evidence and records the deployment; it does not submit a deployment transaction.</p>
    <h2>Registry API</h2>
    <table><thead><tr><th>Method and path</th><th>Purpose</th></tr></thead><tbody>
      <tr><td><code>GET /v1/programs/registry</code></td><td>List registered program IDs with current verified head and freshness information.</td></tr>
      <tr><td><code>GET /v1/programs/registry/&#123;program_id&#125;</code></td><td>Resolve deployment history, current version, lifecycle and accounts.</td></tr>
      <tr><td><code>GET /v1/programs/registry/&#123;program_id&#125;/interface</code></td><td>Read the interface bound to the current verified version.</td></tr>
      <tr><td><code>POST /v1/programs/registry/&#123;program_id&#125;/source</code></td><td>Rebuild mirrored source and compare its artifact with the verified deployed program.</td></tr>
      <tr><td><code>POST /__registry/deployments</code></td><td>Verify canonical deployment admission evidence against chain trust and journal the accepted deployment.</td></tr>
      <tr><td><code>POST /__registry/sources</code></td><td>Publish an encoded source archive, build plan and source URI.</td></tr>
      <tr><td><code>POST /__registry/head</code></td><td>Refresh the journal head for the supplied program_id using the configured chain RPC.</td></tr>
    </tbody></table>
    <p>Program IDs contain 32 hexadecimal-encoded bytes. Direct registry access requires mutual TLS and the appropriate Bearer credential. Request and source-publication credentials are separate. Source publication also resolves the <code>LayerX-Key</code> header to an authorized principal.</p>
    <h2>Deploy and register</h2>
    <ol><li>Build the program and deploy it on chain through the ordinary Programs deployment path.</li><li>Send canonical <code>DeploymentProof</code> bytes to <code>POST /__registry/deployments</code>, either as the raw body or in a JSON object whose <code>admission</code> member is the canonical proof as an array of byte values. A malformed or refused proof returns <code>422 deployment_proof_refused</code> (<em>deployment evidence must authenticate the registered program</em>).</li><li>The registry authenticates a current chain head and verifies the submitted proof under its persisted chain trust. <code>LAYERX_REGISTRY_CHAIN_RPC_URL</code> is required at startup for chain RPC access; registration does not submit an activity.</li><li>Invalid deployment evidence or version history returns <code>422 deployment_proof_refused</code>. The accepted record is journaled and the registry projection is rebuilt; persistence or rebuild failure returns <code>503 journal_unavailable</code>. Success returns <code>200</code>.</li><li>Resolve the program and its current interface before enabling application calls.</li></ol>
    <p>The success response carries <code>transaction_hash</code>, <code>program_id</code>, <code>code_hash</code>, <code>abi_version</code>, <code>version</code> and <code>state: &quot;deployed&quot;</code>, derived from verified evidence. Register upgrades after their on-chain execution and retain the verified predecessor deployment.</p>
    <h3>Deployment proof validation</h3>
    <p>The registry verifies the deployment’s chain commitment, transaction execution and program-state evidence against a current authenticated head. It records the verified deployment in a sealed envelope. On rebuild, stored proofs are verified again against node state and their consumer exports are regenerated.</p>
    <h2>Read a program record</h2>
    <table><thead><tr><th>Field group</th><th>What it tells an application</th></tr></thead><tbody>
      <tr><td><code>program_id</code>, <code>latest_version</code>, <code>versions</code></td><td>Stable program identity and numbered deployments with code hash, ABI and receipt digest.</td></tr>
      <tr><td><code>upgrade_policy</code></td><td>Immutable, or upgradeable with a named authority.</td></tr>
      <tr><td><code>lifecycle</code>, <code>lifecycle_history</code>, <code>exit_routes</code></td><td>Active, deprecated or tombstoned status and relevant exit information.</td></tr>
      <tr><td><code>value_accounts</code></td><td>Program account bindings and proof-verified balance information for account-capable ABIs.</td></tr>
      <tr><td><code>state_root</code>, <code>observed_sequence</code>, <code>observed_at</code>, <code>valid_through</code></td><td>The authenticated state and permitted freshness window.</td></tr>
      <tr><td><code>receipt</code>, <code>source</code></td><td>Deployment evidence and source verification status.</td></tr>
    </tbody></table>
    <p>ABI 1 programs with no value accounts report <code>account-incapable-abi1</code>. Account-capable ABIs require current account-primary and state proofs matching the program, lifecycle, bindings and registry head. ABI 2 uses the account-primary profile; ABIs 3–5 use account profile 2, which additionally identifies the verified owner.</p>
    <h2>Publish and verify source</h2>
    <p>Publish the archive into the operator-controlled mirror using <code>source_uri</code>, encoded <code>plan</code> and <code>archive_hex</code>. The response returns the archive digest. The URI is an identifier for mirrored content; verification uses the retained archive and pinned build plan.</p>
    <pre><code>{`POST /v1/programs/registry/<program-id>/source
Content-Type: application/json
Idempotency-Key: verify-program-release-0001
Authorization: Bearer <request-token>
LayerX-Key: <publication-principal-key>

{
  "source_uri": "<mirrored-source-uri>",
  "source_digest": "<32-byte-source-digest-hex>"
}`}</code></pre>
    <p>The idempotency key contains 16–128 ASCII letters, digits, hyphens or underscores. Source verification binds the request to the resolved principal and verified deployment provenance. A changed deployment cannot inherit an earlier verification result.</p>
    <h3>Reproducible build verdicts</h3>
    <p>The builder runs offline in an isolated process tree using the pinned environment, runtime and supervisor digests. CPU, memory, process count, file size, workspace quota and deadline bounds constrain each build. The reproduced artifact digest must match the registered code hash.</p>
    <p>Source status is <code>unpublished</code>, <code>verified</code> or <code>mismatch</code>. Verified results include source and environment digests and pipeline information. A mismatch returns the expected deployed hash and reproduced artifact hash. Durable verification state and a publication outbox allow recovery to finish publication without relabeling unrelated evidence.</p>
    <h2>The deployment journal</h2>
    <p>The journal stores sealed <code>.envelope</code> units containing the canonical deployment record and proof under a domain-separated envelope with a SHA-256 seal. Publication writes a temporary file, synchronizes it, renames it and synchronizes the directory. The envelope is the atomic commit authority.</p>
    <p>Restart replay re-verifies complete records, rebuilds the registry projection and quarantines incomplete, corrupt or misfiled units. The verified source store is compared again with registered protocol state. Cached program state is hash-checked and refreshed against a verified current head before publication.</p>
    <h3>Export evidence to consumers</h3>
    <p>The journal&apos;s <code>pairs/</code> directory exports matching <code>&lt;unsigned-receipt-digest&gt;.admission</code> and <code>.deployment</code> files. These use canonical DeploymentProof and DeploymentRecord encodings. Startup replay regenerates interrupted exports from committed envelopes; a consumer refuses an incomplete pair.</p>
    <p>Configure evidence consumers to read the exported pair directory. Preserve owner-only file modes and materialize exports through an authorized process when consumers run under another identity. Keep the journal, source mirror, verification state and chain trust in protected durable storage.</p>
    <h2>Operate and troubleshoot</h2>
    <p><code>GET /healthz</code> reports registry readiness. Verify actual program reads and a current independently authenticated head before relying on registry data. The default freshness bound is 300 seconds and is configurable through <code>LAYERX_REGISTRY_MAX_STALENESS_SECONDS</code>.</p>
    <table><thead><tr><th>Refusal</th><th>Action</th></tr></thead><tbody>
      <tr><td><code>deployment_proof_refused</code>, <code>journal_unavailable</code></td><td>Supply valid deployment evidence and restore the required journal and node-state dependencies.</td></tr>
      <tr><td><code>stale_read</code>, <code>protocol_head_unavailable</code></td><td>Restore head synchronization before using the record for a new call.</td></tr>
      <tr><td><code>source_mismatch</code>, <code>build_not_reproducible</code></td><td>Correct source or build inputs and publish a new verification request.</td></tr>
      <tr><td><code>publication_principal_unresolved</code></td><td>Resolve the publication key to its authorized identity.</td></tr>
      <tr><td><code>protocol_state_unavailable</code></td><td>Restore authenticated program-state reads and retain the committed deployment evidence.</td></tr>
    </tbody></table>
    <p>Continue with <Link href="/discovery">program discovery and safe resolution</Link>, or <Link href="/identity">identity and ownership</Link>.</p>
    <div className="source-note"><strong>Technical references:</strong> <code>services/platform/hosted/registry/src/</code> contains <code>routes.rs</code>, <code>journal.rs</code>, <code>node_state.rs</code>, <code>head_attestation.rs</code> and <code>builder.rs</code>.</div>
  </DocsLayout>
}
