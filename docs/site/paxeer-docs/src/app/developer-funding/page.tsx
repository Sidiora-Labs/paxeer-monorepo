import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/developer-funding')

export default function DeveloperFunding() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Developer funding and allocations</h1>
        <p className="page-description">Request a bounded development allocation, track its signed treasury transfer, and verify the funded account before your first Paxeer payment.</p>
      </div>
      <h2>Choose the right funding path</h2>
      <p>The hosted allocation service distributes operator-approved development funds in its configured environment. A developer or agent authenticates, names a recipient identity and public key, and receives a funded result or a typed pending or refusal outcome. The service controls the allocation amount, treasury, eligibility, and quota policy.</p>
      <table><thead><tr><th>Funding need</th><th>Mechanism</th><th>What the result establishes</th></tr></thead><tbody>
        <tr><td>Approved developer allocation</td><td>Authenticated claim backed by a signed native EVM transfer</td><td>The configured environment allocated funds to the named identity</td></tr>
        <tr><td>Native account funding</td><td>Native transfer to the recipient EVM address</td><td>The transaction receipt and recipient balance</td></tr>
        <tr><td>External-chain transfer</td><td>Governed bridge with configured chains, attestors and caps</td><td>The bridge transfer evidence for the selected chain and asset</td></tr>
        <tr><td>Local account setup</td><td>Emulator prefunding</td><td>A local development balance for inspection scenarios</td></tr>
      </tbody></table>
      <p>A development allocation is scoped to its environment and policy. Funding uses the recipient&apos;s DID-to-EVM address binding and requires that binding to resolve before transfer. Use the configured hosted gateway for allocation requests; its availability is separate from the public node JSON-RPC.</p>
      <h2>Developer journey</h2>
      <ol>
        <li>Obtain access to the configured allocation environment and an active hosted identity session.</li>
        <li>Create or select a recipient signing identity. Record its public DID and 64-hex Ed25519 public key, and bind that key to the recipient EVM address through the address precompile.</li>
        <li>For a direct HTTP claim, choose and persist an idempotency key. Gateway claims bind idempotency to the principal and starter-credit grant.</li>
        <li>Submit the claim and retain the funding identifier and transaction identifier.</li>
        <li>Resolve pending outcomes under the same request key.</li>
        <li>Verify the transfer receipt and recipient balance before sending a payment.</li>
      </ol>
      <h2>Authentication and roles</h2>
      <table><thead><tr><th>Actor</th><th>Credential</th><th>Authority</th></tr></thead><tbody>
        <tr><td>Developer or agent</td><td>Hosted identity session Bearer</td><td>Request an allocation subject to the session principal and quota policy</td></tr>
        <tr><td>Gateway service</td><td>Dedicated service-claim Bearer</td><td>Submit a claim naming the authenticated principal</td></tr>
        <tr><td>Allocation service</td><td>Identity introspection service credential</td><td>Validate the session and resolve its active principal</td></tr>
        <tr><td>Funding controller</td><td>Control-admin credential</td><td>Authorize the funding operation against the configured treasury</td></tr>
        <tr><td>Funding signers</td><td>EVM sender and treasury attestation authority</td><td>Sign the native EVM transaction and attest the durable funding intent</td></tr>
      </tbody></table>
      <p>The caller&apos;s session token is introspected as request data; it is never reused as an upstream service authorization credential. The direct claim resolves an active session subject. The service claim accepts its separate configured credential and supplies a principal explicitly. Keep service and treasury credentials on trusted servers.</p>
      <h2>Submit a direct claim</h2>
      <pre><code>{`POST /v1/faucet/claims
Authorization: Bearer <hosted-session-token>
Idempotency-Key: developer-allocation-order-01
Content-Type: application/json

{
  "did": "did:layerx:<lowercase-public-key-hex>",
  "public_key": "<64-hex-public-key>"
}`}</code></pre>
      <p>The JSON body accepts only <code>did</code> and <code>public_key</code>. The allocation amount is configured by the service rather than chosen in the claim. Use the key-derived DID that the treasury funding path validates. <code>Idempotency-Key</code> is 1–128 characters from letters, digits, hyphen, underscore, dot, and colon.</p>
      <h3>Service and JSON-RPC access</h3>
      <p>A configured service caller uses <code>POST /v1/faucet/service-claims</code> with the same headers and a body containing exactly <code>principal</code>, <code>did</code>, and <code>public_key</code>. Direct and service requests for the same principal share its identity quota. The service route must be configured with its dedicated credential.</p>
      <pre><code>{`{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "px_requestFunds",
  "params": ["did:layerx:<lowercase-public-key-hex>", "<64-hex-public-key>"]
}`}</code></pre>
      <p>The configured hosted gateway checks that the session authorizes the signer, obtains the principal&apos;s starter-credit grant, and submits a service claim. It requires the funded amount to match the grant before settling it. The <code>layerx faucet</code> CLI command, <code>faucet.request</code> tool and Rust <code>RpcClient::request_funds</code> call <code>px_requestFunds</code>. This is a hosted method rather than a public node method.</p>
      <h2>Amounts and configurable ceilings</h2>
      <p>The native funding path places the integer amount directly in the EVM transaction value, in wei. The response encodes the amount as a decimal string; the default claim is not a whole-PAX amount.</p>
      <table><thead><tr><th>Configuration</th><th>Default</th><th>Meaning</th></tr></thead><tbody>
        <tr><td><code>LAYERX_FAUCET_CLAIM_AMOUNT</code></td><td>1,000,000 wei</td><td>Positive allocation size per accepted claim</td></tr>
        <tr><td><code>LAYERX_FAUCET_IDENTITY_LIMIT</code></td><td>10,000,000 wei</td><td>Principal total per quota window</td></tr>
        <tr><td><code>LAYERX_FAUCET_ADDRESS_LIMIT</code></td><td>10,000,000 wei</td><td>Recipient public-key total per quota window</td></tr>
        <tr><td><code>LAYERX_FAUCET_NETWORK_LIMIT</code></td><td>50,000,000 wei</td><td>Network bucket total per quota window</td></tr>
        <tr><td><code>LAYERX_FAUCET_WINDOW_SECONDS</code></td><td>86,400 seconds</td><td>Allocation quota window</td></tr>
        <tr><td><code>LAYERX_FAUCET_NETWORK_REQUEST_LIMIT</code></td><td>60 requests</td><td>Network admission ceiling</td></tr>
        <tr><td><code>LAYERX_FAUCET_NETWORK_REQUEST_WINDOW_SECONDS</code></td><td>60 seconds</td><td>Admission-count window</td></tr>
        <tr><td><code>LAYERX_FAUCET_IDEMPOTENCY_SECONDS</code></td><td>604,800 seconds</td><td>Request retention; must cover the quota window</td></tr>
      </tbody></table>
      <p>These are service configuration defaults, not a promised allowance for every deployment. The gateway additionally bounds its claim request rate at 30 per minute. The direct service derives network admission from the TCP peer; service claims use a principal-scoped network bucket. Forwarded identity or IP headers are refused rather than trusted as quota authority.</p>
      <h2>Signed canonical funding</h2>
      <p>The allocation service reserves quota and an idempotency record atomically, then sends its controller a funding command containing <code>funding_id</code>, <code>did</code>, <code>public_key</code>, and <code>amount</code>. The core constructs an EIP-1559 native transfer from its configured EVM sender to the address bound to the recipient public key on chain ID 125.</p>
      <p>Preparation reads the sender&apos;s pending nonce, gas price and priority-fee estimate, then estimates transfer gas. The signed funding record binds the raw transaction, hash, sender, recipient, amount and chain ID. The core persists a treasury-authorized intent before broadcasting, so recovery can reuse the signed transaction.</p>
      <p>The core broadcasts with <code>eth_sendRawTransaction</code> and reads <code>eth_getTransactionReceipt</code>. It checks the confirmation against the saved transfer before returning a funded result. A failed receipt produces a refusal; an unavailable receipt remains pending. The source account must hold the transferred value and gas funds.</p>
      <h2>Response verification and recovery</h2>
      <pre><code>{`{
  "funded": true,
  "funding_id": "<funding-id>",
  "transaction_id": "<transaction-id>",
  "amount": "1000000",
  "network": "layerx-testnet"
}`}</code></pre>
      <p>This illustrates the default claim amount and the service&apos;s fixed <code>layerx-testnet</code> response label. Verify the transfer&apos;s chain ID separately; the label does not select a chain. Direct HTTP success is an unwrapped body rather than an <code>ok</code> envelope; its transaction identifier is optional at that layer. The gateway additionally requires a transaction identifier and a complete funded response before returning JSON-RPC success.</p>
      <p>Retain the returned identifiers and verify the transaction receipt, recipient, value, status and chain ID. Confirm the recipient&apos;s native balance with <code>eth_getBalance</code> at the node; hosted <code>px_getAccount</code> and <code>px_getBalances</code> provide account views. A receipt is final once its block is committed. A funding identifier is a correlation reference.</p>
      <pre><code>{`HTTP/1.1 202 Accepted
Retry-After: 10

{
  "state": "still_checking",
  "retry": "after",
  "retry_after_seconds": 10
}`}</code></pre>
      <p>Repeat the original claim with the same idempotency key and unchanged body after the carried delay. The service recovers the same funding identifier. A stored funded response is replayed; a different request digest under the same key returns <code>409 idempotency_conflict</code>. The gateway maps pending allocation to JSON-RPC <code>-32001</code> with <code>faucet_claim_pending</code>, never a funded result.</p>
      <h2>Typed refusals</h2>
      <table><thead><tr><th>HTTP</th><th>Code</th><th>Resolution</th></tr></thead><tbody>
        <tr><td>400</td><td><code>idempotency_key_required</code>, <code>invalid_idempotency_key</code>, <code>invalid_argument</code></td><td>Correct the request before retrying</td></tr>
        <tr><td>400</td><td><code>untrusted_identity_header</code></td><td>Remove caller-supplied forwarded identity headers</td></tr>
        <tr><td>401</td><td><code>identity_required</code>, <code>service_token_required</code></td><td>Authenticate with the correct credential for the route</td></tr>
        <tr><td>409</td><td><code>idempotency_conflict</code></td><td>Recover the original request rather than modifying its body</td></tr>
        <tr><td>429</td><td><code>network_request_rate</code>, <code>identity_quota</code>, <code>address_quota</code>, <code>network_quota</code></td><td>Respect the supplied retry delay and configured allowance</td></tr>
        <tr><td>503</td><td><code>identity_unavailable</code>, <code>persistence_unavailable</code></td><td>Retain the request key while dependencies recover</td></tr>
        <tr><td>503</td><td><code>funding_rejected</code></td><td>Inspect the destination and treasury funding refusal</td></tr>
      </tbody></table>
      <h2>Owner operations</h2>
      <p>Configure the allocation amount and caps, supply a funded EVM sender with gas balance, and provision separate identity, service-claim, and control credentials. Persist quota and idempotency state in the configured TLS Redis service. Keep the funding journal durable across controller restarts and retain its request digest bindings.</p>
      <p>The service exposes <code>GET /livez</code> for process liveness and <code>GET /readyz</code> for Redis readiness. A ready response establishes Redis availability; independently monitor identity introspection, controller connectivity, treasury balances, and receipt recovery. Its hash-chained allocation audit records event, result, and funding reference without recording authentication values.</p>
      <h2>Continue building</h2>
      <ul><li><Link href="/payments-quickstart">Send your first payment</Link></li><li><Link href="/custody">Account funding and key custody</Link></li><li><Link href="/assets">Asset metadata and account balances</Link></li><li><Link href="/fees">Execution fees and fee limits</Link></li><li><Link href="/platform-api">Public payment API</Link></li></ul>
    </DocsLayout>
  )
}
