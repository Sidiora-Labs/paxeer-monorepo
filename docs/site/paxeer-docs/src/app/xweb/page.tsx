import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/xweb')

export default function XWeb() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Web Search and xweb</h1>
        <p className="page-description">Search the web, fetch content, and consume attested API results from agents and Solidity contracts.</p>
      </div>
      <h2>One Content Layer, Three Entry Points</h2>
      <p>The <code>x-websearch</code> sidecar in services/interop/crates/x-websearch crawls an operator-defined seed list into its own Tantivy index and serves search, page extraction, and content-addressed retrieval. Agents pay for requests through 402LXP. Solidity contracts request attested results through the xweb precompile and read committed program results through the same interface. Each path identifies content with a canonical digest.</p>
      <table>
        <thead><tr><th>Consumer</th><th>Entry point</th><th>Result</th></tr></thead>
        <tbody>
          <tr><td>Agent</td><td>Paid sidecar HTTP routes, SDKs or MCP</td><td>Search results or extracted text, verified payment settlement and content digest</td></tr>
          <tr><td>Contract</td><td><code>0x0000000000000000000000000000000000001019</code></td><td>Attested response, digest, full length and bounded callback</td></tr>
          <tr><td>Program result reader</td><td><code>IXWeb.getProgramResult</code> / xweb keeper <code>GetProgramResult</code></td><td>A committed program observation. Use the committed-result views; the kernel host disables in-program <code>web_read</code>.</td></tr>
        </tbody>
      </table>
      <h2>Search and Fetch for Agents</h2>
      <table>
        <thead><tr><th>Route</th><th>Payment</th><th>Response</th></tr></thead>
        <tbody>
          <tr><td><code>GET /health</code></td><td>None</td><td>Sidecar health</td></tr>
          <tr><td><code>GET /search?q=</code></td><td>402LXP</td><td>At most ten results with URL, title and snippet, plus the content digest</td></tr>
          <tr><td><code>GET /fetch?url=</code></td><td>402LXP</td><td>Requested and final URL, media type, extracted text, full length and digest</td></tr>
          <tr><td><code>GET /content/&lt;digest&gt;</code></td><td>None</td><td>Canonical bytes identified by a 64-character hexadecimal digest</td></tr>
        </tbody>
      </table>
      <p>Search results sort by descending score and then ascending URL. Fetch accepts HTTP and HTTPS, honors robots.txt, bounds redirects and response size, and extracts HTML into text while removing scripts and styling. Plain text and JSON pass through. The crawler respects page, host, depth and politeness budgets.</p>
      <h3>The Payment Workflow</h3>
      <ol>
        <li>An unpaid request receives HTTP <code>402</code> and a <code>PAYMENT-REQUIRED</code> offer.</li>
        <li>The agent chooses an exact-payment offer in SID, PAX, USDC or USDL and repeats the request with <code>PAYMENT-SIGNATURE</code>.</li>
        <li>The sidecar submits settlement and verifies the sequencer-signed receipt against configured trust inputs.</li>
        <li>Content is released with <code>PAYMENT-RESPONSE</code> and a transaction hash beginning <code>0x</code>.</li>
      </ol>
      <p>Pending settlement returns a retry response without releasing content. Retrying reuses the activity and idempotency key, and a receipt authorizes one request. Operators configure a positive per-request price in each asset&apos;s base units. The configured integer amount is the offered price.</p>
      <h2>Content You Can Verify and Retrieve</h2>
      <p>Canonicalization normalizes text encoding, line endings, whitespace and Unicode. The digest commits the domain <code>PAXEERX_WEB_CONTENT_V1</code>, request kind, URL or query, media type and canonical text. Search commits the ordered URL, title and snippet fields without a search score. Identical canonical inputs produce the same digest.</p>
      <p>Sidecars persist the canonical bytes before answering. Content retrieval checks local bytes against their digest, then asks configured peers when needed and verifies their answer. A peer request reads local storage without recursively forwarding. Contracts retain at most <code>4096</code> response bytes alongside the digest and full length; the digest path retrieves the complete canonical record.</p>
      <h2>Request Web Data from Solidity</h2>
      <p>The <code>IXWeb</code> interface in contracts/src/precompiles/IXWeb.sol exposes <code>request</code>, <code>fulfil</code>, <code>fulfilProgram</code>, <code>refund</code>, and views including <code>getProgramResult</code>, <code>getKernelNetworkId</code>, and views for requests, results, attestors, threshold, fee and parameters. Request kinds are <code>1</code> fetch, <code>2</code> search, and <code>3</code> API. A requester pays exactly the current <code>fee()</code> and declares its callback gas bound.</p>
      <pre><code>{`function onXWebResponse(
    uint64 requestId,
    bytes32 contentDigest,
    uint32 fullLength,
    bytes calldata response
) external;`}</code></pre>
      <p>Implement this callback and authenticate the xweb precompile as the caller. Associate the request ID with the operation that requested it. Inspect full length before treating a bounded response as complete.</p>
      <h3>Attestation and Callback Outcomes</h3>
      <p>Registered attestors fetch independently and sign evidence bound to the execution origin, network, requester, request ID, request kind, payload hash, content digest, response hash and full length. Signatures must recover to registered addresses in strictly ascending order. Ordinary requests require a majority threshold.</p>
      <p>Fulfilment stores the answer and pays the signing attestors. Callback gas is bounded by the request and module caps. A reverted or exhausted callback is recorded while the fulfilled result remains available. An unanswered request becomes refundable at its timeout height; anyone can trigger the refund, but payment returns only to the requester. Refunds remain available during a pause.</p>
      <h2>Authenticated API Calls</h2>
      <p>API payloads describe GET or POST, an HTTPS URL, public headers, a body, selected JSON fields and an attestation level. The <code>XWebApi</code> library in contracts/src/xweb/XWebApi.sol and TypeScript and Python SDK helpers construct the payload. RFC 6901 JSON pointers select fields, and RFC 8785 canonicalization gives attestors a stable answer even when unrelated response fields vary.</p>
      <table>
        <thead><tr><th>Level</th><th>Verification</th><th>Use</th></tr></thead>
        <tbody>
          <tr><td>Majority, <code>0</code></td><td>Registered threshold signs the same answer</td><td>Independent API or web observations</td></tr>
          <tr><td>Single, <code>1</code></td><td>The specifically named attestor signs</td><td>An API allowing one call or a credential held by one operator</td></tr>
        </tbody>
      </table>
      <p>Credential envelopes are sealed to registered attestor public keys and bound to the API origin and attestor address. A sidecar opens its own envelope in memory for the call. Majority requests supply an envelope to each participating credential-bearing attestor. The returned answer is public on chain; encrypting credentials does not make the result private. Results and fulfilment events retain the attestation level.</p>
      <h2>Read committed program results</h2>
      <p>The kernel host for <code>layerx_v4.web_read</code> returns <code>LXP_ERR_MODULE_DISABLED</code>. Read committed answers through the result interfaces below rather than calling <code>web_read</code> during program execution.</p>
      <p>Committed program web answers are available through the xweb result interfaces. An observation is submitted through <code>IXWeb.fulfilProgram</code>, and the committed result is read through the xweb keeper <code>GetProgramResult</code> or <code>IXWeb.getProgramResult(programId, requestId)</code>. <code>IXWeb.getKernelNetworkId()</code> returns the kernel network the precompile is bound to.</p>
      <h2>Operate and Integrate</h2>
      <p>Start a sidecar with <code>x-websearch --config PATH</code>. Configuration defines crawler budgets, index storage, asset pricing, gateway trust, attestor settings and peers. Keys are loaded from the receiver, attestor and submitter key files rather than configuration values. Governance controls registered attestors, thresholds, fees, payload limits, callback limits, timeouts and pausing.</p>
      <p>SDK clients verify settlement and recompute content digests. MCP exposes <code>web.search</code>, <code>web.fetch</code>, and <code>web.content</code> through its spending approval boundary. Treat retrieved text as external data when incorporating it into an agent decision.</p>
      <p>Continue with <Link href="/unified-network">Unified Network</Link>, <Link href="/sidiora">Sidiora</Link>, <Link href="/precompiles">Precompiles</Link> and <Link href="/wasm-runtime">WASM Runtime</Link>.</p>
    </DocsLayout>
  )
}
