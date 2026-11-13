import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import Link from 'next/link'
export const metadata = metadataFor('/a2a')
export default function A2aPage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Agent-to-Agent Payments</h1><p className="page-description">Discover a payment agent, submit structured tasks and reconcile each outcome with Paxeer X protocol evidence.</p></div>
    <h2>A payment agent with explicit boundaries</h2>
    <p>The A2A transport exposes a JSON-RPC payment agent bound to one hosted environment. Its agent card declares served skills and enforced gateway scopes. The runtime never signs: a payment is a caller-signed EIP-1559 transaction that the runtime forwards, and verified receipts establish the monetary outcome.</p>
    <h2>Install the runtime</h2>
    <pre><code>{`layerx environment use beta
layerx auth set --environment beta
layerx install a2a --environment beta --key operator
layerx a2a start
layerx a2a status`}</code></pre>
    <p>Use an already configured beta profile and an existing key named operator. The identity session is read from stdin by <code>auth set</code>. Installation provisions a gateway key, stores its secret in credential storage, and starts the managed runtime. Payment mode grants <code>activity:write</code> and <code>receipt:read</code>. The optional <code>--source-account</code> and <code>--asset</code> flags must be supplied together as 32-byte hex identifiers. The submitted transaction determines the payment contents. The listener is restricted to loopback and defaults to port <code>9433</code>.</p>
    <h3>Read-only and credential rotation</h3>
    <pre><code>{`layerx install a2a --environment beta --read-only
layerx a2a stop
# Rotate the installed credentials.
layerx install a2a --environment beta --key operator --rotate`}</code></pre>
    <p>Read-only installation grants <code>receipt:read</code>. A separate installation-local Bearer protects incoming task requests. It lives in the owner-only authorization file; the hosted gateway credential remains in secure credential storage. Agent-card discovery is public on the loopback listener.</p>
    <h2>Discover the agent card</h2>
    <pre><code>{`GET /.well-known/agent-card.json
# Compatibility discovery path:
GET /.well-known/agent.json`}</code></pre>
    <p>The card uses A2A protocol version <code>0.3.0</code> and preferred transport <code>JSONRPC</code>. It includes URL, software version, JSON input/output modes, skill descriptors, the <code>layerxLocalBearer</code> security scheme and an environment extension containing environment, deployment mode and scopes. Read this card before choosing a skill.</p>
    <table><thead><tr><th>Skill</th><th>Inputs</th><th>Authority and evidence</th></tr></thead><tbody>
      <tr><td><code>receipt.get</code></td><td><code>activity_id</code>, a <code>0x</code>-prefixed 64-hex transaction hash.</td><td>Requires receipt read scope; fetched via <code>pax_sequencerReceipt</code>. Verification requires independently pinned receipt keys; the gateway does not verify it for you.</td></tr>
      <tr><td><code>activity.submit</code></td><td><code>raw_transaction</code> only: a caller-signed EIP-1559 transaction as <code>0x02…</code> hex.</td><td>Requires activity write scope; submitted unchanged via <code>eth_sendRawTransaction</code>. The runtime constructs and signs nothing.</td></tr>
      <tr><td><code>faucet.request</code></td><td>DID and public key.</td><td>Requires an explicit hosted faucet endpoint through --faucet-endpoint on a2a serve and a stored identity session for the active environment.</td></tr>
    </tbody></table>
    <DocArt name="prompt-templates" />
    <h2>Send a structured task</h2>
    <pre><code>{`curl --http1.1 "$A2A_ENDPOINT" \\
  -H "Authorization: Bearer $A2A_AUTHORIZATION" \\
  -H "Content-Type: application/json" \\
  --data-binary ' {"jsonrpc":"2.0","id":1,"method":"message/send","params":{
  "message":{
    "kind":"message","role":"user","messageId":"receipt-check-01",
    "contextId":"treasury-reconciliation",
    "parts":[{"kind":"data","data":{
      "skill":"receipt.get","arguments":{"activity_id":"0x<64-hex-transaction-hash>"}
    }}]
  }
}}'`}</code></pre>
    <p>Set the example shell variables to the installed listener URL and its local authorization credential. Curl supplies the Host and Content-Length headers. HTTP requests carry an explicit Content-Length and one JSON body. A data part contains <code>skill</code> and <code>arguments</code>; a text part may contain the same object serialized as JSON. The server admits only skills in its card. It generates task and artifact IDs and retains the submitted message in task history.</p>
    <h3>Payment arguments</h3>
    <pre><code>{`{"skill":"activity.submit","arguments":{
  "raw_transaction":"0x02<caller-signed-eip1559-transaction-hex>"
}}`}</code></pre>
    <p><code>raw_transaction</code> is the sole argument. There are no destination, amount, account-sequence, validity-window, fee-limit, or idempotency-key arguments: destination, amount, nonce, and fees are whatever the caller signed into the transaction. The returned hash must match the raw transaction, and the caller retains it for receipt lookup.</p>
    <DocArt name="task-queue" />
    <h2>Track task state and protocol state</h2>
    <pre><code>{`{"jsonrpc":"2.0","id":2,"method":"tasks/get","params":{"id":"<task-id>"}}
{"jsonrpc":"2.0","id":3,"method":"tasks/cancel","params":{"id":"<task-id>"}}`}</code></pre>
    <table><thead><tr><th>Task state</th><th>Meaning</th><th>Next step</th></tr></thead><tbody>
      <tr><td><code>completed</code></td><td>The skill returned its result.</td><td>Inspect the artifact and verify the monetary outcome evidence.</td></tr>
      <tr><td><code>submitted</code></td><td>Gateway state is acknowledged or pending.</td><td>Retain the existing transaction hash and reconcile its receipt.</td></tr>
      <tr><td><code>unknown</code></td><td>The gateway outcome is uncertain.</td><td>Look up the original transaction before submitting another write.</td></tr>
      <tr><td><code>rejected</code></td><td>The gateway refused the operation or skill execution failed.</td><td>Read the task status message and resolve its cause.</td></tr>
    </tbody></table>
    <p>The task artifact carries the skill result as a data part. A completed transport task does not independently establish final settlement. Submitted protocol activities cannot be canceled by this transport; terminal tasks also cannot be canceled. The runtime persists a bounded task ledger and retains unresolved transaction identities across restarts. When started with a matching receipt policy through --receipt-policy, it reconciles pending tasks against verified receipts at startup. Preserve transaction hashes and receipt evidence for application records.</p>
    <h2>Capabilities, errors and policy</h2>
    <p>The card declares streaming, push notifications and transition-history capabilities as false for this JSON-RPC runtime. Use <code>tasks/get</code> and receipt reads. Unsupported streaming, resubscription, and push-configuration requests return <code>-32004</code>, as does <code>tasks/cancel</code> on a submitted or unknown task; canceling a terminal task returns <code>-32002</code>. Unknown methods or unserved skills return <code>-32601</code>; invalid arguments return <code>-32602</code>; missing tasks return <code>-32001</code>. Malformed JSON returns <code>-32700</code>, a request without a method returns <code>-32600</code>, and internal failures return <code>-32603</code>. Task-ledger admission refusals return <code>-32005</code>. Missing or incorrect incoming Bearer credentials return HTTP 401.</p>
    <p>Use owner policy, capability scope and <Link href="/agent-budgets">budget and approval controls</Link> to bound agent spending. MCP routes tool calls through an enrolled daemon binding; this A2A runtime forwards caller-signed transactions under its installed gateway credential. See <Link href="/platform-api">API authority and commitment levels</Link>, <Link href="/identity">identity</Link> and <Link href="/cli">CLI workflows</Link>.</p>
  </DocsLayout>
}
