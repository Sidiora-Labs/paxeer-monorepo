import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/developer-funding')

export default function DeveloperFunding() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Developer funding and allocations</h1>
        <p className="page-description">Request a bounded development allocation, track its signed treasury transfer, and verify the funded account before your first Paxeer X payment.</p>
      </div>
      <h2>Choose the right funding path</h2>
      <p>The hosted allocation service distributes operator-approved development funds in its configured environment. A developer or agent authenticates, names a recipient identity and public key, and receives a funded result or a typed pending or refusal outcome. The service controls the allocation amount, treasury, eligibility, and quota policy.</p>
      <table><thead><tr><th>Funding need</th><th>Mechanism</th><th>What the result establishes</th></tr></thead><tbody>
        <tr><td>Approved developer allocation</td><td>Authenticated claim backed by a signed treasury SEND</td><td>The configured environment allocated funds to the named identity</td></tr>
        <tr><td>User deposit</td><td>Custody deposit evidence and authenticated custody credit</td><td>A deposit-backed credit for its beneficiary</td></tr>
        <tr><td>Market-maker inventory</td><td>Inventory and liquidity arrangements for the selected venue and Asset</td><td>The balance and obligations of that arrangement</td></tr>
        <tr><td>Fiat purchase or conversion</td><td>The selected provider and payment rail</td><td>The provider-specific funded or converted outcome</td></tr>
      </tbody></table>
      <p>A development allocation is scoped to its environment and policy. It does not establish a general entitlement to free real-value Assets, fiat conversion, or unlimited liquidity. Public account deposit funding uses the custody credit path. Use the allocation endpoint supplied for your approved development environment rather than assuming a public faucet hostname.</p>
      <h2>Developer journey</h2>
      <ol>
        <li>Obtain access to the configured allocation environment and an active hosted identity session.</li>
        <li>Create or select a recipient signing identity. Record its public DID and 64-hex Ed25519 public key without exposing the seed.</li>
        <li>Choose and persist an idempotency key for this allocation request.</li>
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
        <tr><td>Treasury owner</td><td>Treasury signing authority</td><td>Sign the canonical transfer and its owner authorization</td></tr>
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
      <p>The configured gateway proxies this method to the service-claim flow. The <code>layerx faucet</code> CLI command and the <code>faucet.request</code> tool use the same allocation capability. Note: Rust <code>RpcClient::request_funds</code> currently still calls the retired <code>lx_requestFunds</code> name, which the gateway refuses with <code>-32601</code>; call <code>px_requestFunds</code> directly until the SDK is updated. Gateway and environment eligibility still apply.</p>
      <h2>Amounts and configurable ceilings</h2>
      <p>Amounts are integer base units of the configured treasury Asset. Interpret them using that Asset&apos;s decimals and metadata; do not assume an allocation amount is a fiat amount or a whole-token count. Responses encode amounts as decimal strings.</p>
      <table><thead><tr><th>Configuration</th><th>Default</th><th>Meaning</th></tr></thead><tbody>
        <tr><td><code>LAYERX_FAUCET_CLAIM_AMOUNT</code></td><td>1,000,000 base units</td><td>Positive allocation size per accepted claim</td></tr>
        <tr><td><code>LAYERX_FAUCET_IDENTITY_LIMIT</code></td><td>10,000,000 base units</td><td>Principal total per quota window</td></tr>
        <tr><td><code>LAYERX_FAUCET_ADDRESS_LIMIT</code></td><td>10,000,000 base units</td><td>Recipient public-key total per quota window</td></tr>
        <tr><td><code>LAYERX_FAUCET_NETWORK_LIMIT</code></td><td>50,000,000 base units</td><td>Network bucket total per quota window</td></tr>
        <tr><td><code>LAYERX_FAUCET_WINDOW_SECONDS</code></td><td>86,400 seconds</td><td>Allocation quota window</td></tr>
        <tr><td><code>LAYERX_FAUCET_NETWORK_REQUEST_LIMIT</code></td><td>60 requests</td><td>Network admission ceiling</td></tr>
        <tr><td><code>LAYERX_FAUCET_NETWORK_REQUEST_WINDOW_SECONDS</code></td><td>60 seconds</td><td>Admission-count window</td></tr>
        <tr><td><code>LAYERX_FAUCET_IDEMPOTENCY_SECONDS</code></td><td>604,800 seconds</td><td>Request retention; must cover the quota window</td></tr>
      </tbody></table>
      <p>These are service configuration defaults, not a promised allowance for every deployment. The gateway additionally bounds its claim request rate at 30 per minute. The direct service derives network admission from the TCP peer; service claims use a principal-scoped network bucket. Forwarded identity or IP headers are refused rather than trusted as quota authority.</p>
      <h2>Signed canonical funding</h2>
      <p>The allocation service reserves quota and an idempotency record atomically, then sends its controller a funding command containing <code>funding_id</code>, <code>did</code>, <code>public_key</code>, and <code>amount</code>. The controller constructs an owner-authorized native Asset SEND from the treasury main account to the recipient main account.</p>
      <p>Preparation reads the treasury identity sequence separately from its state-proven source-account sequence and verifies sufficient treasury Asset balance. The signing disclosure binds the canonical envelope, debit authorization, recipient, amount, validity interval, fee limit, and idempotency key. The transfer uses current sequences and the validity interval set at preparation.</p>
      <p>The core submission follows the ordinary signed activity and receipt verification path. Result code zero produces a funded result. A refused transfer produces a refusal, and a receipt timeout remains pending. Allocation does not mint arbitrary supply or bypass normal transfer authorization.</p>
      <h2>Response verification and recovery</h2>
      <pre><code>{`{
  "funded": true,
  "funding_id": "<funding-id>",
  "transaction_id": "<transaction-id>",
  "amount": "1000000",
  "network": "layerx-testnet"
}`}</code></pre>
      <p>This illustrates the default claim amount and the allocation response&apos;s environment label. Direct HTTP success is an unwrapped body rather than an <code>ok</code> envelope; its transaction identifier is optional at that layer. The gateway additionally requires a transaction identifier and a complete funded response before returning JSON-RPC success.</p>
      <p>Retain the returned identifiers and verify the transaction&apos;s canonical receipt, recipient, Asset, amount, result code, network authority, and requested commitment. Confirm the recipient account with <code>px_getAccount</code> or <code>px_getBalances</code>. The retired <code>lx_*</code> names return <code>-32601</code>. A funding identifier is a correlation reference, not an independent finality proof.</p>
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
      <p>Configure the allocation amount and caps, supply a funded treasury with fee balance, and provision separate identity, service-claim, and control credentials. Persist quota and idempotency state in the configured TLS Redis service. Keep the funding journal durable across controller restarts and retain its request digest bindings.</p>
      <p>The service exposes <code>GET /livez</code> for process liveness and <code>GET /readyz</code> for Redis readiness. A ready response establishes Redis availability; independently monitor identity introspection, controller connectivity, treasury balances, and receipt recovery. Its hash-chained allocation audit records event, result, and funding reference without recording authentication values.</p>
      <h2>Continue building</h2>
      <ul><li><Link href="/payments-quickstart">Send your first payment</Link></li><li><Link href="/custody">Deposit-backed custody funding</Link></li><li><Link href="/assets">Asset metadata and account balances</Link></li><li><Link href="/fees">Execution fees and fee limits</Link></li><li><Link href="/platform-api">Public payment API</Link></li></ul>
    </DocsLayout>
  )
}
