<!-- Source: https://docs.paxeer.app/identity/ -->

# Identity, Ownership and Credentials

Connect human ownership, agent identities, hosted sessions and scoped signing authority across Paxeer X.

## One owner, explicit authority

Paxeer X connects an owner to an agent DID, a signing authority, an account and a recovery policy. The identity provider exports a principal DID, while the node binds an EVM address to a DID derived from an Ed25519 public key. Preserve each identity with its own registration evidence. A hosted login authenticates a principal; a protocol signature authorizes a particular action.

| Identity surface | What it establishes | What to retain |
| --- | --- | --- |
| Human owner | The owner DID, protected Ed25519 authority and recovery commitment. | Public registration evidence and secure custody references. |
| Native account | The account bound to the owner through the EVM/DID binding. | Account identity and the committed binding transaction. |
| Hosted principal | Tenant, subject, allowed signer keys, optional account and audiences. | The principal record and tenant context. |
| Hosted session | Time-limited access to hosted services. | Session ID, expiry and securely held token. |
| Agent capability | The activities, counterparties, assets and limits an agent may use. | Capability scope, expiry and revocation state. |

## Register an owner

Registration, funding, and recovery are transactions on the network (EVM chain ID 125). The kernel runs inside the node, and a receipt is final when its block commits; see [custody](https://docs.paxeer.app/custody).

1. Provision the owner principal and recovery policy, and separately generate and protect the signing key. Provisioning a principal alone does not register an on-chain identity.
2. For a key-derived node DID, read `layerXBindNonce(address)` from the address precompile at `0x0000000000000000000000000000000000001004`. Have the Ed25519 key sign `"LX:PAXEER-BIND:v1" || chain_id:uint256be || evm_address:20 bytes || nonce:u64be`, then call `bindLayerX(bytes32, bytes)` from that EVM address. The binding is one EVM address per DID and one DID per address.
3. Verify the committed transaction and `LayerXBound` event, then query `getUnifiedAccount(address)` for the main account ID. A successful query alone is insufficient, because an unbound address returns zero DID and account fields.
4. Register identity and recovery state through LayerXAuth at `0x000000000000000000000000000000000000101C`: `recordLifecycle(bytes payload, bytes32 idempotencyKey)` executes Governance module 7 for the bound caller. Identity registration is ordinal 1 (`71 01 00 02 || did_id32 || primary_key32`); the recovery commitment is ordinal 3, with an optional five-field form carrying delay bounds. Derive the DID identifier with the protocol codec and use big-endian integers.
5. Fund the intended account with the correct native asset, keeping EVM gas funding distinct from the kernel account balance. Retain the activity IDs and receipt evidence.

An account name such as `agent:<actor-DID>:main` identifies the intended account. Its account ID is derived by the protocol account codec, not by interpreting the DID as an address. Session authorization uses the dedicated `grantSession` and `revokeGrant` methods rather than the lifecycle payload.

### Key and account representations

The node binding uses `did:layerx:` followed by the lowercase 64-character Ed25519 public-key hex. The Human sponsor configuration compares its authority reference to the resolved owner's lowercase public-key hex, and its actor to the exact exported DID. The recovery root is unpadded base64url of the registered 32-byte commitment. Preserve the representation required by each interface.

## Hosted principals and sessions

The hosted identity API serves HTTPS with service Bearer authentication. The Provisioning service manages principals and sessions; the Registrar service can also create principals but cannot manage sessions. Consumer services introspect a user session using their own service credential, keeping user tokens separate from service authority.

| Method and path | Purpose |
| --- | --- |
| `POST /v1/principals` | Create or update a principal and its signer/account/audience bindings. |
| `POST /v1/sessions` | Mint a session for a known subject. |
| `DELETE /v1/sessions/{id}` | Revoke a session by its 16-byte hex identifier. |
| `POST /v1/sessions/introspect` | Check whether a presented token is active for the calling service. |
| `POST /v1/introspect` | Use the same introspection contract through its alternate path. |
| `POST /v1/publication-keys`, `/v1/signup`, `/v1/signup/verify` | Publication-key registration and the signup flow. |
| `POST /v1/starter-credits`, `/v1/starter-credits/settle` | Starter-credit issuance and settlement. |
| `GET /internal/v1/principal`, `GET /internal/readyz` | Internal principal lookup and readiness. |

```
POST /v1/principals
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
{"tenant":"treasury","sub":"agent:treasury-operator","ttl_seconds":3600}
```

Replace bracketed values with established public bindings. The session response contains `session_id`, `tenant`, `sub`, `token`, `csrf_token` and `expires_at`. Tokens use `ses_{32hex-id}.{64hex-secret}`. Keep token and CSRF material out of URLs, logs and public configuration.

### Session bounds and introspection

The default lifetime is 86,400 seconds. A session may request 1 through 2,592,000 seconds, with at most 4,096 live unrevoked sessions per principal. Principal subjects contain 1–128 lowercase identifier bytes. Signer lists contain at most 128 unique lowercase 32-byte hex keys; audience lists contain at most 32 unique identifiers.

Gateway introspection returns the active subject and allowed signer keys. Developer consumers (Webhooks and Dashboard) receive the active subject and CSRF token. Ramp introspection additionally requires an authorized audience and a stored account. Unknown, expired, revoked or secret-mismatched sessions return an inactive result.

## Delegate without surrendering ownership

Agent sessions carry tenant, agent DID, authority reference, permitted activity types, expiry, client and policy version. Capabilities restrict activity types, counterparties, assets, amount ceilings, rate ceilings, purpose constraints and expiry. Every dimension is required; an empty permission set denies the corresponding action.

Use `capability.create` to establish authority, `capability.attenuate` to narrow a grant, `capability.list` to inspect grants and `capability.revoke` to withdraw authority. Combine capability scope with a [funded agent budget](https://docs.paxeer.app/agent-budgets) and approval policy for monetary operations.

## Rotation, revocation and recovery

Session grants are authorized through `grantSession(bytes, uint64, bytes32)` and withdrawn through `revokeGrant(bytes32, uint8, uint64)`. The arguments carry the canonical grant, its expiry sequence, and a 32-byte action key. Rotation announcements use `recordLifecycle` with the 92-byte ordinal-2 form: `71 02 00 04 || did_id32 || next_key32 || begin_ms:u64 || end_ms:u64 || effective_sequence:u64`. Check committed identity, scope and revocation state before authorizing a spend.

Recovery uses the registered guardian commitment and threshold from the ordinal-3 lifecycle payload. Keep recovery material exercisable and separate from the primary owner key, and maintain the registered delay policy alongside that commitment.

### Respond to a compromised credential

1. Revoke the affected hosted session and withdraw its agent capability.
2. Close or revoke budgets exposed to the compromised delegate.
3. Rotate the signing authority through the registered owner policy, or use the guardian recovery process.
4. Verify committed policy state and receipts before issuing replacement credentials.

## Errors and operational checks

The HTTP API returns typed errors with a retry policy. Invalid arguments, missing credentials and unauthorized service operations require corrected input or authority. Session capacity returns `429 session_bound_reached`; unavailable durable storage returns `503 store_unavailable`. Respect `Retry-After` where supplied.

`GET /livez` checks service liveness; `GET /readyz` verifies writable durable storage. Principal records, session digests and revocation state survive restart through a snapshot and journal. Token secrets are not stored in plaintext.

**Technical references:** `docs/wiki/HumanOwnerRegistration.md`; `services/platform/hosted/identity/src/main.rs`; `services/agent/schema/agent-api/identity.kvx`.

Continue with [agent budgets and approvals](https://docs.paxeer.app/agent-budgets).
