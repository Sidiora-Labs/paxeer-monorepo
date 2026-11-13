import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/identity')

export default function IdentityPage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Identity, Ownership and Credentials</h1><p className="page-description">Connect human ownership, agent identities, hosted sessions and scoped signing authority across Paxeer X.</p></div>
    <h2>One owner, explicit authority</h2>
    <p>Paxeer X connects an owner to an agent DID, a signing authority, an account and a recovery policy. The identity provider exports a principal DID, while the node binds an EVM address to a DID derived from an Ed25519 public key. Preserve each identity with its own registration evidence. A hosted login authenticates a principal; a protocol signature authorizes a particular action.</p>
    <table><thead><tr><th>Identity surface</th><th>What it establishes</th><th>What to retain</th></tr></thead><tbody>
      <tr><td>Human owner</td><td>The owner DID, protected Ed25519 authority and recovery commitment.</td><td>Public registration evidence and secure custody references.</td></tr>
      <tr><td>Native account</td><td>The account bound to the owner through the EVM/DID binding.</td><td>Account identity and the committed binding transaction.</td></tr>
      <tr><td>Hosted principal</td><td>Tenant, subject, allowed signer keys, optional account and audiences.</td><td>The principal record and tenant context.</td></tr>
      <tr><td>Hosted session</td><td>Time-limited access to hosted services.</td><td>Session ID, expiry and securely held token.</td></tr>
      <tr><td>Agent capability</td><td>The activities, counterparties, assets and limits an agent may use.</td><td>Capability scope, expiry and revocation state.</td></tr>
    </tbody></table>
    <h2>Register an owner</h2>
    <p>Registration, funding, and recovery are transactions on the network (EVM chain ID 125). The kernel runs inside the node, and a receipt is final when its block commits; see <Link href="/custody">custody</Link>.</p>
    <ol><li>Provision the owner principal and recovery policy, and separately generate and protect the signing key. Provisioning a principal alone does not register an on-chain identity.</li><li>For a key-derived node DID, read <code>layerXBindNonce(address)</code> from the address precompile at <code>0x0000000000000000000000000000000000001004</code>. Have the Ed25519 key sign <code>"LX:PAXEER-BIND:v1" || chain_id:uint256be || evm_address:20 bytes || nonce:u64be</code>, then call <code>bindLayerX(bytes32, bytes)</code> from that EVM address. The binding is one EVM address per DID and one DID per address.</li><li>Verify the committed transaction and <code>LayerXBound</code> event, then query <code>getUnifiedAccount(address)</code> for the main account ID. A successful query alone is insufficient, because an unbound address returns zero DID and account fields.</li><li>Register identity and recovery state through LayerXAuth at <code>0x000000000000000000000000000000000000101C</code>: <code>recordLifecycle(bytes payload, bytes32 idempotencyKey)</code> executes Governance module 7 for the bound caller. Identity registration is ordinal 1 (<code>71 01 00 02 || did_id32 || primary_key32</code>); the recovery commitment is ordinal 3, with an optional five-field form carrying delay bounds. Derive the DID identifier with the protocol codec and use big-endian integers.</li><li>Fund the intended account with the correct native asset, keeping EVM gas funding distinct from the kernel account balance. Retain the activity IDs and receipt evidence.</li></ol>
    <p>An account name such as <code>agent:&lt;actor-DID&gt;:main</code> identifies the intended account. Its account ID is derived by the protocol account codec, not by interpreting the DID as an address. Session authorization uses the dedicated <code>grantSession</code> and <code>revokeGrant</code> methods rather than the lifecycle payload.</p>
    <h3>Key and account representations</h3>
    <p>The node binding uses <code>did:layerx:</code> followed by the lowercase 64-character Ed25519 public-key hex. The Human sponsor configuration compares its authority reference to the resolved owner&apos;s lowercase public-key hex, and its actor to the exact exported DID. The recovery root is unpadded base64url of the registered 32-byte commitment. Preserve the representation required by each interface.</p>
    <h2>Hosted principals and sessions</h2>
    <p>The hosted identity API serves HTTPS with service Bearer authentication. The Provisioning service manages principals and sessions; the Registrar service can also create principals but cannot manage sessions. Consumer services introspect a user session using their own service credential, keeping user tokens separate from service authority.</p>
    <table><thead><tr><th>Method and path</th><th>Purpose</th></tr></thead><tbody>
      <tr><td><code>POST /v1/principals</code></td><td>Create or update a principal and its signer/account/audience bindings.</td></tr>
      <tr><td><code>POST /v1/sessions</code></td><td>Mint a session for a known subject.</td></tr>
      <tr><td><code>DELETE /v1/sessions/&#123;id&#125;</code></td><td>Revoke a session by its 16-byte hex identifier.</td></tr>
      <tr><td><code>POST /v1/sessions/introspect</code></td><td>Check whether a presented token is active for the calling service.</td></tr>
      <tr><td><code>POST /v1/introspect</code></td><td>Use the same introspection contract through its alternate path.</td></tr>
      <tr><td><code>POST /v1/publication-keys</code>, <code>/v1/signup</code>, <code>/v1/signup/verify</code></td><td>Publication-key registration and the signup flow.</td></tr>
      <tr><td><code>POST /v1/starter-credits</code>, <code>/v1/starter-credits/settle</code></td><td>Starter-credit issuance and settlement.</td></tr>
      <tr><td><code>GET /internal/v1/principal</code>, <code>GET /internal/readyz</code></td><td>Internal principal lookup and readiness.</td></tr>
    </tbody></table>
    <pre><code>{`POST /v1/principals
Content-Type: application/json
Authorization: Bearer <provisioning-service-token>

{
  "tenant": "treasury",
  "sub": "agent:treasury-operator",
  "allowed_signer_public_keys": ["<32-byte-lowercase-public-key-hex>"],
  "account": "<registered-account>",
  "audiences": ["ramp"]
}

POST /v1/sessions
{"tenant":"treasury","sub":"agent:treasury-operator","ttl_seconds":3600}`}</code></pre>
    <p>Replace bracketed values with established public bindings. The session response contains <code>session_id</code>, <code>tenant</code>, <code>sub</code>, <code>token</code>, <code>csrf_token</code> and <code>expires_at</code>. Tokens use <code>ses_&#123;32hex-id&#125;.&#123;64hex-secret&#125;</code>. Keep token and CSRF material out of URLs, logs and public configuration.</p>
    <h3>Session bounds and introspection</h3>
    <p>The default lifetime is 86,400 seconds. A session may request 1 through 2,592,000 seconds, with at most 4,096 live unrevoked sessions per principal. Principal subjects contain 1–128 lowercase identifier bytes. Signer lists contain at most 128 unique lowercase 32-byte hex keys; audience lists contain at most 32 unique identifiers.</p>
    <p>Gateway introspection returns the active subject and allowed signer keys. Developer consumers (Webhooks and Dashboard) receive the active subject and CSRF token. Ramp introspection additionally requires an authorized audience and a stored account. Unknown, expired, revoked or secret-mismatched sessions return an inactive result.</p>
    <h2>Delegate without surrendering ownership</h2>
    <p>Agent sessions carry tenant, agent DID, authority reference, permitted activity types, expiry, client and policy version. Capabilities restrict activity types, counterparties, assets, amount ceilings, rate ceilings, purpose constraints and expiry. Every dimension is required; an empty permission set denies the corresponding action.</p>
    <p>Use <code>capability.create</code> to establish authority, <code>capability.attenuate</code> to narrow a grant, <code>capability.list</code> to inspect grants and <code>capability.revoke</code> to withdraw authority. Combine capability scope with a <Link href="/agent-budgets">funded agent budget</Link> and approval policy for monetary operations.</p>
    <h2>Rotation, revocation and recovery</h2>
    <p>Session grants are authorized through <code>grantSession(bytes, uint64, bytes32)</code> and withdrawn through <code>revokeGrant(bytes32, uint8, uint64)</code>. The arguments carry the canonical grant, its expiry sequence, and a 32-byte action key. Rotation announcements use <code>recordLifecycle</code> with the 92-byte ordinal-2 form: <code>71 02 00 04 || did_id32 || next_key32 || begin_ms:u64 || end_ms:u64 || effective_sequence:u64</code>. Check committed identity, scope and revocation state before authorizing a spend.</p>
    <p>Recovery uses the registered guardian commitment and threshold from the ordinal-3 lifecycle payload. Keep recovery material exercisable and separate from the primary owner key, and maintain the registered delay policy alongside that commitment.</p>
    <h3>Respond to a compromised credential</h3>
    <ol><li>Revoke the affected hosted session and withdraw its agent capability.</li><li>Close or revoke budgets exposed to the compromised delegate.</li><li>Rotate the signing authority through the registered owner policy, or use the guardian recovery process.</li><li>Verify committed policy state and receipts before issuing replacement credentials.</li></ol>
    <h2>Errors and operational checks</h2>
    <p>The HTTP API returns typed errors with a retry policy. Invalid arguments, missing credentials and unauthorized service operations require corrected input or authority. Session capacity returns <code>429 session_bound_reached</code>; unavailable durable storage returns <code>503 store_unavailable</code>. Respect <code>Retry-After</code> where supplied.</p>
    <p><code>GET /livez</code> checks service liveness; <code>GET /readyz</code> verifies writable durable storage. Principal records, session digests and revocation state survive restart through a snapshot and journal. Token secrets are not stored in plaintext.</p>
    <div className="source-note"><strong>Technical references:</strong> <code>docs/wiki/HumanOwnerRegistration.md</code>; <code>services/platform/hosted/identity/src/main.rs</code>; <code>services/agent/schema/agent-api/identity.kvx</code>.</div>
    <p>Continue with <Link href="/agent-budgets">agent budgets and approvals</Link>.</p>
  </DocsLayout>
}
