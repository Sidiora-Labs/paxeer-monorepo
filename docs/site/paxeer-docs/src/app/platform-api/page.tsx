import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import Link from 'next/link'
export const metadata = metadataFor('/platform-api')
export default function PlatformApiPage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Platform API</h1><p className="page-description">Accounts, balances, history, assets, registration, funding and consensus evidence over the hosted gateway, plus the Agent API for policy-governed writes.</p></div>
    <h2>Choose an interface</h2>
    <p>Use the node JSON-RPC root for signed chain transactions and EVM reads. Configure the hosted gateway origin separately with coordinates supplied by its operator. Its <code>px_</code> methods provide account, balance, asset, history and consensus reads. EVM chain ID <code>125</code> identifies the network. The gateway serves <code>GET /openrpc.json</code> for its method contract and authenticated <code>GET /v1/routes</code> for product route discovery. Source: <code>services/platform/hosted/gateway/openrpc.json</code> and <code>services/platform/hosted/gateway/src/rpc.rs</code>.</p>
    <table><thead><tr><th>Surface</th><th>Transport</th><th>Purpose</th></tr></thead><tbody>
      <tr><td>Gateway RPC</td><td><code>POST /</code></td><td>Seventeen <code>px_</code> / <code>pax_sequencerReceipt</code> methods: accounts, balances, assets, history, registration, funding and consensus reads.</td></tr>
      <tr><td>Node JSON-RPC</td><td><code>POST /</code></td><td><code>eth_</code> and <code>net_</code> calls at the node root, including <code>eth_sendRawTransaction</code> for signed EVM transactions. See <Link href="/json-rpc">EVM JSON-RPC</Link>.</td></tr>
      <tr><td>Agent API</td><td>Contract major 2, minor 0</td><td>Scoped identities, disclosures, approvals, budgets and event streams.</td></tr>
    </tbody></table>
    <p>The gateway exposes hosted reads and route discovery; the Agent API provides governed preparation and tracking. Use signed native transactions for network execution. The node reports receipt progress through <code>pax_sequencerReceipt</code>, and a receipt is final when its containing block is committed.</p>
    <DocArt name="authentication" />
    <h2>Authentication and signing</h2>
    <p>Public account and consensus reads do not require a gateway key. Route discovery requires a gateway API key in the <code>Authorization</code> header, using the <code>LayerX-Key</code> scheme. Funding requests require an identity bearer session and the matching signer public key. Registration proves possession of its signing key. Authentication and scope requirements follow the individual route; transaction signatures authorize the exact chain action. Use <Link href="/identity">scoped identities and credentials</Link> and <Link href="/agent-budgets">budget controls</Link>.</p>
    <h2>Query gateway state</h2>
    <pre><code>{`POST /
Content-Type: application/json

{"jsonrpc":"2.0","id":1,"method":"px_getNetwork","params":[]}`}</code></pre>
    <p>Parameters are positional, in the order listed. The hosted dispatcher serves the following seventeen methods. JSON-RPC batches contain between 1 and 32 requests. Unsupported methods return <code>-32601</code>; unavailable configured backends return explicit errors.</p>
    <table><thead><tr><th>Method</th><th>Parameters</th><th>Purpose</th></tr></thead><tbody>
      <tr><td><code>px_getNetwork</code></td><td>none</td><td>Configured network identity, node head and consensus availability.</td></tr>
      <tr><td><code>px_getAccount</code></td><td><code>account</code>; or <code>did</code>, <code>asset</code></td><td>Account resolution and node state; the two-argument form reads a DID account for one Asset.</td></tr>
      <tr><td><code>px_resolveAccount</code></td><td><code>account</code></td><td>Resolve an account reference to its account record.</td></tr>
      <tr><td><code>px_getBalances</code></td><td><code>account</code></td><td>Balances held by one account, read through the bank precompile.</td></tr>
      <tr><td><code>px_listAssets</code></td><td>optional <code>cursor</code></td><td>Native asset list.</td></tr>
      <tr><td><code>px_register</code></td><td><code>signer_public_key</code>, <code>registration_signature</code>, optional <code>email_or_verification_token</code></td><td>Key-bound principal registration or signup verification.</td></tr>
      <tr><td><code>px_requestFunds</code></td><td><code>did</code>, <code>signer_public_key</code></td><td>One bounded testnet funding request, authenticated by identity session.</td></tr>
      <tr><td><code>pax_sequencerReceipt</code></td><td><code>txHash</code></td><td>The node&apos;s sequencer receipt and commitment rung, relayed unchanged.</td></tr>
      <tr><td><code>px_getCommit</code>, <code>px_getValidators</code>, <code>px_getSequenceBatch</code></td><td><code>height</code> as a positive decimal string</td><td>Signed commit, validator set and sequence batch at one block height.</td></tr>
      <tr><td><code>px_getReceiptKeys</code></td><td>none</td><td>Published receipt signing keys for local receipt verification.</td></tr>
      <tr><td><code>px_getHistory</code>, <code>px_getUnifiedHistory</code></td><td><code>account</code>, optional <code>cursor</code>, <code>limit</code>, <code>kind</code></td><td>Paged account history from the indexer read API; the unified variant tags the side each record answers for.</td></tr>
      <tr><td><code>px_listHolderRecords</code></td><td><code>address</code>, <code>kind</code>, optional <code>cursor</code>, <code>limit</code></td><td>Paged holder records of one kind for one address.</td></tr>
      <tr><td><code>px_getCapabilities</code></td><td>none</td><td>Which product surfaces are currently available on this endpoint.</td></tr>
      <tr><td><code>px_getRouteCatalogue</code></td><td>none</td><td>Authenticated product route catalogue and dependency readiness.</td></tr>
    </tbody></table>
    <p>Node-side signing methods (<code>eth_accounts</code>, <code>eth_coinbase</code>, <code>eth_sendTransaction</code>, <code>eth_sign</code>, <code>eth_signTransaction</code>, <code>eth_signTypedData</code>, <code>eth_signTypedData_v4</code>, <code>eth_mining</code>) and <code>eth_subscribe</code> / <code>eth_unsubscribe</code> are refused at the gateway rather than relayed. A raw transaction addressed to a precompile surface that <code>px_getCapabilities</code> reports false is refused with <code>-32003 surface_unavailable</code>; check capabilities before submitting.</p>
    <h2>Submit through the Agent API</h2>
    <p>Governed writes use the Agent API pipeline (prepare, sign, submit, track, wait). Native wallet writes submit signed EVM transactions to the node root. The steps are:</p>
    <ol><li>Verify the network with <code>px_getNetwork</code> and read the current account state.</li><li>Prepare canonical activity bytes with an actor, authority, time bounds, fee limit, payload and stable idempotency key.</li><li>Inspect the decoded disclosure and sign the exact signing preimage.</li><li>Submit and retain the submission reference, transaction hash and receipt.</li><li>Verify the receipt signature under independently trusted receipt keys, compare published keys through <code>px_getReceiptKeys</code>, and verify the commit and trusted validator set at the landing height before releasing goods, funds or dependent work.</li></ol>
    <h3>Idempotency and uncertain outcomes</h3>
    <p>A timeout does not establish rejection. Reconcile the existing submission and reuse its idempotency key rather than creating a second payment. Agent mutations return the same result for the same body and key; a changed body under the same key returns <code>IdempotencyConflict</code>.</p>
    <h2>Follow activity</h2>
    <p>Poll <code>px_getHistory</code> or <code>px_getUnifiedHistory</code> with a retained cursor for account activity, follow EVM heads and logs on the <Link href="/public-rpc">chain WebSocket endpoints</Link>, and use <Link href="/subscriptions">agent subscriptions</Link> for durable, acknowledged tenant-scoped streams.</p>
    <h3>Agent event streams</h3>
    <p>Agent subscriptions support create, list, health, acknowledge, pause, resume and delete. Their persisted cursor enables recovery. Scope evaluates tenant, capability, then filters for agents, accounts, activity types, modules, assets, counterparties and result classes. Delivery is at least once: deduplicate by <code>deduplication_id</code> and handle Event, Gap and Truncated variants.</p>
    <h2>Agent API pipeline</h2>
    <p>The wire envelope contains contract major and minor as big-endian u16, request ID as big-endian u64, operation as UTF-8 and a canonical-map payload. Amounts, sequences, limits and timestamps are decimal strings. Settlement domain is Paxeer.</p>
    <table><thead><tr><th>Operation</th><th>Inputs and outcome</th></tr></thead><tbody>
      <tr><td><code>prepare</code></td><td>Actor, authority, account sequence, timestamp bound, idempotency key, fee limit, payload and payload hash produce unsigned canonical bytes, signing preimage, disclosure, preparation reference and expiry.</td></tr>
      <tr><td><code>sign</code>, <code>submit</code></td><td>Preparation reference and signature bind the signed activity and tracked submission.</td></tr>
      <tr><td><code>track</code></td><td>Submission reference returns state, evidence, rung and transitions.</td></tr>
      <tr><td><code>wait</code></td><td>Submission reference, requested rung and deadline return the actual rung and whether the deadline elapsed.</td></tr>
      <tr><td>Reads</td><td><code>read.balance</code>, <code>read.account</code>, <code>read.module_state</code>, <code>read.history</code>, <code>read.batch</code>, <code>read.checkpoint</code>, <code>read.proof_bundle</code>.</td></tr>
      <tr><td>Availability and projection</td><td><code>availability.fetch</code>, <code>export.offline</code>, <code>project</code>. Projection is an estimate rather than a verified read.</td></tr>
    </tbody></table>
    <p>Submission progresses through Prepared, Signed, Queued, Submitted, Unknown, Executed, Committed, Failed and Expired. Unknown is a first-class pending state. On the wire, progress is reported as one of three rungs — <code>submitted</code>, <code>receipted</code> and <code>committed</code> — and <code>wait</code> reports the actual rung with <code>deadline_elapsed</code> when its deadline expires. A <code>receipted</code> result proves execution; <code>committed</code> establishes block finality.</p>
    <h2>Errors and retries</h2>
    <table><thead><tr><th>Code</th><th>Meaning</th><th>Action</th></tr></thead><tbody>
      <tr><td><code>-32600</code></td><td>Invalid request envelope.</td><td>Correct the JSON-RPC framing.</td></tr>
      <tr><td><code>-32601</code></td><td>Method not found in the hosted dispatcher.</td><td>Use the hosted method catalogue or send node methods to the node root.</td></tr>
      <tr><td><code>-32003</code></td><td><code>surface_unavailable</code>: a raw transaction targets a precompile surface reported false by <code>px_getCapabilities</code>.</td><td>Check capabilities; do not resubmit to a disabled surface.</td></tr>
    </tbody></table>
    <p>Agent errors distinguish TransportFailure, Deadline, ProtocolIncompatibility, UnavailableCapability, CoreRejection, VerificationFailure, PolicyRefusal, CapabilityRefusal, BudgetRefusal, RateLimit, IdempotencyConflict and InternalFault. Resolve authority and verification failures before retrying.</p>
    <p>Continue with <Link href="/cli">CLI workflows</Link>, <Link href="/sdk">SDK integration</Link> and <Link href="/json-rpc">EVM JSON-RPC</Link>.</p>
  </DocsLayout>
}
