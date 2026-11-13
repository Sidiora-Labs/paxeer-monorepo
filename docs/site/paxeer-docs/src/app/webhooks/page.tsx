import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/webhooks')

export default function Webhooks() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Webhooks</h1><p className="page-description">Receive signed Paxeer X events with durable delivery, per-subject ordering, bounded retries, and cursor-based replay.</p></div>
    <p>Hosted webhooks deliver journey, payment, approval, and program events to your HTTPS receiver. Events and pending deliveries are persisted before transmission. Each endpoint has its own Ed25519 signing key, and retries preserve the event identifier and exact body bytes so your application can safely recognize repeats.</p>
    <h2>Register an endpoint</h2>
    <p>Use an active developer session with <code>Authorization: Bearer</code>, or the <code>__Host-layerx-session</code> cookie. Cookie-authenticated POST and DELETE requests also carry <code>x-layerx-csrf</code> matching the session CSRF token. Registration requires an <code>Idempotency-Key</code>.</p>
    <pre><code>{`POST /v1/webhooks/endpoints
Authorization: Bearer <developer-session>
Idempotency-Key: <stable-registration-key>
Content-Type: application/json

{
  "url": "<public-https-receiver-url>",
  "kinds": ["payment", "approval"],
  "minimum_verification": "unverified"
}`}</code></pre>
    <p>The URL uses a canonical DNS name and resolves exclusively to public addresses. Literal IP destinations are refused. Each developer principal can register up to 32 endpoints. Registration returns the endpoint identifier, signing key identifier, public key, and receiver obligation. Preserve the returned key in your receiver&apos;s trusted key configuration.</p>
    <h2>Event families and evidence</h2>
    <table><thead><tr><th>Kind</th><th>Resource and facts</th><th>Evidence</th></tr></thead><tbody>
      <tr><td><code>journey</code></td><td>Journey identity, kind, state, updated time.</td><td>Source facts with optional activity receipt binding.</td></tr>
      <tr><td><code>payment</code></td><td>Activity identity, amount, asset, settled state.</td><td>Successful verified receipt for settlement; source amount and asset retain their own evidence labels.</td></tr>
      <tr><td><code>approval</code></td><td>Approval identity, agent, state, creation time.</td><td>Source facts with optional activity receipt binding.</td></tr>
      <tr><td><code>program</code></td><td>Program identity, lifecycle, version, code hash, receipt digest.</td><td>Source facts with optional activity receipt binding.</td></tr>
    </tbody></table>
    <p>Webhook verification levels are <code>unverified</code>, <code>receipt-verified</code> and <code>block-committed</code>. Registering an endpoint with any other <code>minimum_verification</code> value is rejected as an invalid request. Each fact retains its own level; the event&apos;s aggregate verification uses the weakest fact. A payment event can therefore be unverified in aggregate while its settled-state fact carries receipt evidence. A minimum-verification filter applies to that aggregate level. Inspect and verify the facts your decision uses.</p>
    <h2>Verify the raw delivery</h2>
    <pre><code>{`layerx-webhook-id: <event-id>
layerx-webhook-timestamp: <unix-seconds>
layerx-webhook-key-id: <endpoint-key-id>
layerx-webhook-signature: v1=<padded-base64-ed25519-signature>

signed_message = UTF8(event_id + "." + timestamp + ".") || raw_body`}</code></pre>
    <ol><li>Validate bounded identifiers and canonical integer timestamp syntax.</li><li>Reject signatures older than the accepted age or too far in the future. Defaults are 300 seconds of age and 30 seconds of future skew.</li><li>Verify Ed25519 against the trusted public key named by the key identifier, over the exact incoming body bytes.</li><li>Claim the event identifier and payload digest in a durable replay store before applying business effects.</li><li>For economic effects, resolve and verify the referenced receipt and compare its digest to the event evidence.</li></ol>
    <p>Read the body as raw bytes before JSON parsing. Re-serializing parsed JSON changes the signed message. Operational headers for delivery, attempt, kind, subject, sequence, and endpoint are outside the signature; use authenticated body fields for business decisions. <code>GET /v1/webhooks/scheme</code> publishes the receiver contract.</p>
    <h2>Commit once, accept repeats</h2>
    <p>Delivery is at least once. Store the event identifier with its payload digest and business outcome in the same transactional database as your application effects. A completed repeat returns success without repeating fulfilment; an in-progress claim waits or retries; the same identifier with a different digest is a conflict. Release an unsuccessful processing lease so a legitimate retry can proceed.</p>
    <p>The framework integrations provide verified webhook consumers and replay-store interfaces. An in-memory single-process store is useful for local execution; a durable shared store preserves deduplication across workers and restarts.</p>
    <h2>Ordering, retries, and dead letters</h2>
    <p>Events are ordered per subject, rather than globally across every resource. Publish rejects a non-advancing subject sequence. Delivery progresses through pending, in-flight, retrying, delivered, and dead-lettered. Only an accepting 2xx status from your endpoint marks a delivery delivered.</p>
    <table><thead><tr><th>Default control</th><th>Value</th></tr></thead><tbody>
      <tr><td>Maximum attempts</td><td>8</td></tr><tr><td>Initial retry delay</td><td>10 seconds</td></tr><tr><td>Maximum retry delay</td><td>3,600 seconds</td></tr><tr><td>Deterministic backoff spread</td><td>20%</td></tr><tr><td>In-flight lease</td><td>120 seconds</td></tr><tr><td>Automatic suspension</td><td>20 consecutive dead letters</td></tr>
    </tbody></table>
    <p>Backoff doubles up to its bound. Redirects are refused; HTTP 410 is a permanent failure. A permanent failure or exhausted attempts creates a dead letter. A suspended endpoint receives no dispatch until resumed. A lost worker&apos;s in-flight lease permits recovery without losing the persisted event.</p>
    <h2>Manage delivery and replay</h2>
    <table><thead><tr><th>Method and path</th><th>Purpose</th></tr></thead><tbody>
      <tr><td><code>GET /v1/webhooks/endpoints</code></td><td>List endpoint registrations.</td></tr>
      <tr><td><code>GET /v1/webhooks/endpoints/&#123;id&#125;/events</code></td><td>Read endpoint event history and continuation cursor.</td></tr>
      <tr><td><code>GET /v1/webhooks/endpoints/&#123;id&#125;/keys</code></td><td>Read current and overlapping verification keys.</td></tr>
      <tr><td><code>POST /v1/webhooks/endpoints/&#123;id&#125;/keys</code></td><td>Rotate the signing key with an idempotency key.</td></tr>
      <tr><td><code>POST /v1/webhooks/endpoints/&#123;id&#125;/redeliveries</code></td><td>Enqueue replay from a signed cursor with an idempotency key.</td></tr>
      <tr><td><code>POST /v1/webhooks/endpoints/&#123;id&#125;/suspensions</code></td><td>Suspend an endpoint with a reason.</td></tr>
      <tr><td><code>POST /v1/webhooks/endpoints/&#123;id&#125;/resumptions</code></td><td>Resume delivery and clear consecutive dead-letter accounting.</td></tr>
      <tr><td><code>GET /v1/webhooks/events</code>, <code>/deliveries</code>, <code>/dead-letters</code></td><td>Inspect events and operational outcomes.</td></tr>
      <tr><td><code>POST /v1/webhooks/dead-letters/&#123;id&#125;/replay</code></td><td>Retry a dead letter with an idempotency key.</td></tr>
    </tbody></table>
    <p>Pagination defaults to 50 items with a maximum of 200. Preserve cursors as opaque signed values. Expired retention returns <code>410 cursor_expired</code>; repair continuity using retained event and receipt history rather than silently assuming all events were delivered. Replay keeps the original event identifier and body digest.</p>
    <h2>Rotate keys and monitor receivers</h2>
    <p>Key rotation announces a pending public key before activation; the default overlap is 86,400 seconds. Configure the new public key during the overlap while retaining the old key for eligible deliveries. Signing private keys stay with the signing service.</p>
    <p>Monitor delivery latency, retry age, dead letters, suspension, and receiver signature failures. Return 2xx after durable acceptance. For long processing jobs, commit an inbox record and process it asynchronously. Use <Link href="/subscriptions">agent subscriptions</Link> for tenant-scoped streams with explicit acknowledgement and gap recovery.</p>
    <div className="source-note"><strong>Reference:</strong> <code>services/platform/hosted/webhooks/src/main.rs</code> and its sibling files <code>hosted.rs</code>, <code>scheme.rs</code>, <code>events.rs</code>, <code>endpoints.rs</code>; endpoint management routes are in routes.rs in the same directory.</div>
  </DocsLayout>
}
