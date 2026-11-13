import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/agent-runtime')

export default function AgentRuntime() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Agent runtime</h1><p className="page-description">Run the agent daemon, enrol an identity, connect MCP runtimes, and preserve policy and receipt continuity across restarts.</p></div>
    <p>The Paxeer X agent runtime connects autonomous software to LayerX activity execution through a controlled daemon boundary. <code>layerx-agentd</code> owns sessions, capabilities, approval holds, local limits, and durable evidence. MCP exposes those controls as tools; the hosted agent boundary carries signed activities to the node and serves execution receipts.</p>
    <h2>Runtime components</h2>
    <table><thead><tr><th>Component</th><th>Responsibility</th><th>Trust boundary</th></tr></thead><tbody>
      <tr><td><code>layerx-agentd</code></td><td>Identity enrolment, session authentication, capability checks, local policy, and evidence verification.</td><td>Protected tenant store, human owner, and daemon credentials.</td></tr>
      <tr><td><code>layerx-mcp</code></td><td>Daemon-bound tool catalogue for reads and actions.</td><td>One session and capability; no separate MCP write authority.</td></tr>
      <tr><td><code>layerx mcp serve</code></td><td>Serve the bound catalogue over standard input and output.</td><td>Protected daemon binding document.</td></tr>
      <tr><td><code>layerx-agent-boundary</code></td><td>TLS submission, durable request journal, receipt lookup, and program evidence.</td><td>Separate gateway, registry, and webhook credential planes.</td></tr>
    </tbody></table>
    <h2>1. Configure the network profile</h2>
    <p>Only the profile name is positional. The endpoint, network identifier, and sequencer trust anchor flags are each optional; supply the release-provided values for the network you intend to join. The CLI retains the implementation name <code>layerx</code>.</p>
    <pre><code>{`layerx environment use beta \\
  --endpoint <https-endpoint> \\
  --network-id <network-id> \\
  --sequencer-trust-anchor <hex>

# The trust anchor may instead be read from a file:
#   --sequencer-trust-anchor-file <path>`}</code></pre>
    <p>The profile names are <code>emulator</code>, <code>beta</code>, and <code>production</code>. Non-loopback endpoints use HTTPS. A profile controls CLI network access; the daemon library configuration separately uses a normalized absolute node endpoint path.</p>
    <h2>2. Enrol the daemon identity</h2>
    <p>Configure the agent DID, tenant, authority, capability, session identity, and binding output under the daemon&apos;s <code>LAYERX_AGENT_MCP_*</code> settings. Setting <code>LAYERX_AGENT_MCP_BINDING_ROOT</code> enables binding publication. The daemon resolves the DID through the human authority, registers the identity, restores the named capability, opens a session, and publishes the runtime binding before accepting requests. These are daemon lifecycle operations, not a <code>layerx</code> session-opening command. A publication failure closes the newly opened session; restart restores the named session and its matching document instead of creating duplicate credentials.</p>
    <table><thead><tr><th>Binding artifact</th><th>Contents</th><th>Protection</th></tr></thead><tbody>
      <tr><td><code>binding.json</code></td><td>Mode, tenant, store, audit root, session and capability references, daemon endpoint, credential file paths, local limit context, the protected Unix listener socket and admitted peer UIDs, and an optional web-search sidecar reference.</td><td>Owner-only binding directory and file access.</td></tr>
      <tr><td><code>session-token</code></td><td>OS-random session Bearer token.</td><td>Separate protected file; excluded from host configuration.</td></tr>
      <tr><td><code>daemon-bearer</code></td><td>Daemon transport authentication.</td><td>Separate protected file; excluded from the binding document.</td></tr>
    </tbody></table>
    <p>Binding directories use mode <code>0700</code> and files use <code>0600</code>. Existing secret files are refused instead of overwritten. A different binding at an existing publication path is also refused.</p>
    <h2>3. Install the runtime connection</h2>
    <p>Install MCP after daemon enrolment publishes its binding. Supported hosts are <code>layerx</code>, <code>claude-code</code>, <code>claude-desktop</code>, <code>cursor</code>, and <code>vscode</code>. Repeat <code>--host</code> to install into multiple runtimes.</p>
    <pre><code>{`layerx install mcp --host claude-code \\
  --daemon-binding /absolute/path/to/binding.json

# Restrict the installed tool surface to reads.
layerx install mcp --host cursor --read-only \\
  --daemon-binding /absolute/path/to/binding.json

# Run the same stdio server directly.
layerx mcp serve --daemon-binding /absolute/path/to/binding.json`}</code></pre>
    <p>The installer opens and validates the same document used at serve time. Relative explicit paths become absolute. Without an explicit binding path, installation resolves <code>mcp/binding.json</code> beside the CLI configuration. Host launch configuration holds the executable, arguments, and an empty process environment; secret material stays in protected daemon files.</p>
    <p>MCP installation uses the binding&apos;s session and capability. It does not provision a gateway credential or ask for a source account, asset, signing seed, or network profile. The read-only flag narrows the declared binding mode. Tool discovery exposes only scopes admitted by the bound session.</p>
    <h2>4. Admit an action</h2>
    <p>Each action passes session authentication, capability evaluation, policy, rate and quota limits, deadlines, and budget reservation before submission. Capability checks evaluate expiry, activity type, counterparty, asset, amount, rate, and purpose. An approval hold retains the exact canonical preparation and its reservation until an authorized decision or expiry.</p>
    <p>Use the <Link href="/agents">agent action lifecycle</Link> to prepare, review, sign, submit, and track. Requests carry stable idempotency keys. Repeating an identical mutation returns its original result; changing the body under the same key produces an idempotency conflict.</p>
    <h2>Hosted submission and recovery</h2>
    <p>The hosted agent boundary accepts canonical signed activities using <code>application/octet-stream</code>. Submission requires <code>Idempotency-Key</code>; activity bodies are limited to one MiB (<code>MAX_ACTIVITY_BYTES = 1_048_576</code>). A durable journal binds the key, route, and activity digest so retries and restarts preserve the original submission identity.</p>
    <table><thead><tr><th>HTTP route family</th><th>Use</th></tr></thead><tbody>
      <tr><td><code>POST /v1/activities</code></td><td>Submit a signed native activity.</td></tr>
      <tr><td><code>GET /v1/receipts/&#123;id&#125;</code></td><td>Retrieve the result associated with an activity identifier.</td></tr>
      <tr><td><code>GET /v1/receipts/&#123;id&#125;/account-state</code></td><td>Retrieve account state for a receipt.</td></tr>
      <tr><td><code>POST /v1/programs/deploy</code>, <code>/upgrade</code>, <code>/wind-down</code></td><td>Submit signed native program lifecycle activities.</td></tr>
      <tr><td><code>POST /v1/programs/simulate</code>, <code>/call</code>, <code>/read</code></td><td>Simulate without committing, execute a program call, or read program state.</td></tr>
      <tr><td><code>GET /v1/programs/activities/&#123;id&#125;</code></td><td>Retrieve the journaled program activity.</td></tr>
      <tr><td><code>GET /v1/programs/receipts/by-idempotency/&#123;key&#125;</code></td><td>Resolve a program outcome using its original key.</td></tr>
      <tr><td>Program account-state, state-proof, and head-attestation routes</td><td>Serve program account state and its proof material.</td></tr>
    </tbody></table>
    <p>These are the agent boundary&apos;s HTTP interfaces. Its Bearer planes are separate from daemon sessions and from CLI gateway credentials. Keep transport-specific credentials with their intended boundary.</p>
    <h2>Evidence and write readiness</h2>
    <p>The daemon loads trusted sequencer policy independently of incoming proof data. Receipt verification binds the network, protocol version, authorized sequencer range, epoch, batch identity, sequence range, Merkle inclusion, and execution outcome.</p>
    <p><code>tenant.readiness</code> reports <code>transport_ready</code>, <code>verified_reads_ready</code>, <code>writes_admitted</code>, and an optional recovery reason. Writes are admitted exactly when no recovery reason is present, which requires transport and a genuine state-proven account read for the authenticated tenant. Readiness is tenant-scoped; an HTTP connection alone does not establish recovered spending authority.</p>
    <p>After a process failure, restore durable holds, verify receipts, and resolve uncertain spends before opening writes. An unresolved reservation remains held. Local limits enforce daemon policy; bypassing the daemon bypasses those limits. Protocol-enforced authority and cryptographic receipt evidence remain separate from local accounting.</p>
    <h2>Operate and revoke</h2>
    <p>Monitor preparation age, open approvals, held budget, unresolved submissions, stream gaps, and verification failures. The hosted boundary&apos;s <code>/livez</code> reports process liveness; <code>/readyz</code> additionally checks node connectivity and handshake identity. Loss of the node can leave liveness healthy while readiness fails.</p>
    <p>Revoke a capability or close a session to withdraw agent access. Revoking a capability revokes its whole parent subtree and fails its unsubmitted preparations. Preserve audit and receipt records so a revoked agent&apos;s historical actions remain traceable.</p>
    <div className="source-note"><strong>Reference:</strong> <code>services/agent/crates/layerx-agentd/src/enrolment.rs</code>, <code>session.rs</code>; <code>services/agent/schema/agent-api/identity.kvx</code>; <code>tools/platform/cli/src/install/mcp.rs</code>; <code>services/platform/hosted/agent-boundary/src/main.rs</code>.</div>
    <div className="prev-next"><Link href="/agents"><div className="prev-next-label">Previous</div><div className="prev-next-title">Agents on Paxeer X</div></Link><Link href="/programs"><div className="prev-next-label">Next</div><div className="prev-next-title">Native programs</div></Link></div>
  </DocsLayout>
}
