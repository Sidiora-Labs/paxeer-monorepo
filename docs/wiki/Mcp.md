# MCP tools

`layerx-mcp` exposes tenant- and scope-bound tools through `layerx-agentd`.
Models receive tool access, not protocol authority. Calls pass the daemon's
policy, capability, budget, rate and audit controls; writes use the ordinary
owner preparation, external signing, submission and tracking path.

Paxeer is one network: the kernel participates in block proposal, and a
receipt is final when its block is committed. Tools report actual evidence
using `submitted`, `receipted` and `committed`, not a second settlement system.
See [Agent API](AgentApi.md) for the contract and [Agentd](Agentd.md) for trust
and recovery.

The core `TOOL_CATALOGUE` in
`services/agent/crates/layerx-mcp/src/server.rs` contains 22 definitions.
`WEB_TOOLS` and `SESSION_TOOLS` in `src/catalogue.rs` add paid web and session
operations. Listings are filtered by the bound scope, and read-only deployment
omits write tools. A listed definition is not a guarantee that the selected
daemon boundary implements its operation: unavailable paths return a typed
refusal.

---

## Catalogue

The table gives operation mappings from `server.rs::tool_operation` and
current read semantics, rather than repeating legacy description strings.

| Tool | Kind | Required scope | Daemon operation / meaning |
| --- | --- | --- | --- |
| `tenant.readiness` | read | `read` | TenantReadiness; local recovery/admission observation, not execution proof |
| `balance.get` | read | `read:balance` | ReadBalance; program-scoped balances with evidence and freshness |
| `history.list` | read | `read:history` | ReadHistory; bounded account history ordered by block position |
| `receipt.get` | read | `read:receipt` | ProgramReceipt; retained transaction execution receipt |
| `checkpoint.get` | read | `read:checkpoint` | ReadCheckpoint; block and verified CometBFT commit at a height |
| `proof.get` | read | `read:proof` | ReadProofBundle; transaction/receipt evidence with committed proof where available |
| `availability.get` | read | `read:availability` | AvailabilityFetch; block availability and roots at a height |
| `wallet.accounts` | read | `read:wallet:accounts` | ReadAccount; accounts observed for a program |
| `wallet.balance` | read | `read:wallet:balance` | ReadBalance; selected account/asset balance |
| `activity.prepare` | write | `write:prepare` | Prepare; bound bytes and disclosure, subject to approval |
| `activity.disclose` | write | `write:disclose` | Prepare; owner-decoded disclosure |
| `activity.sign` | write | `write:sign` | Sign; external-signature boundary |
| `activity.submit` | write | `write:submit` | Submit; prepared or raw signed transaction under the selected variant |
| `activity.track` | write | `write:track` | Track; actual submission state and rung |
| `activity.wait` | write | `write:activity:wait` | Wait; bounded wait for submission progress |
| `wallet.send` | write | `write:wallet:send` | Submit alias through ordinary Prepare/Sign/Submit/Track |
| `token.create` | write | `write:token:create` | Submit alias for asset registration |
| `token.mint` | write | `write:token:mint` | Submit alias for mint |
| `token.transfer` | write | `write:token:transfer` | Submit alias for transfer |
| `grant.issue` | write | `write:grant:issue` | Submit alias for issuing a grant |
| `grant.draw` | write | `write:grant:draw` | Submit alias for drawing a grant |
| `faucet.request` | write | `write:faucet:claim` | Catalogue definition retained; no ordinary daemon operation mapping |

`faucet.request` metadata describes testnet funding through `px_requestFunds`.
In this daemon-bound server `tool_operation` returns None, so do not treat the
listing as a served FaucetClaim operation. The CLI's hosted A2A toolset is a
different surface. Agent HTTP `faucet.claim` is retired and returns
`UnavailableCapability`; it is not an alternative MCP submission path.

Read mappings use the daemon's program-balance and `/v1/reads/` listeners.
`checkpoint.get`'s `sequence` and `availability.get`'s `batch` are retained
argument names for **block height**. Availability reports a complete block's
transaction count and roots, or a pruned/incomplete result; its native response
has `rung: null`. It does not prove execution by returning bytes.
Checkpoint reads bind the block hash and time to the verified signed header
and configured validator trust. Missing proof sources or trust are refused.

---

## Parameters

`src/catalogue.rs::input_schema` defines the advertised JSON Schema and
`validate` enforces it before routing. Argument objects are bounded to 65,536
encoded bytes; the route also has an outer 1,048,576-byte limit. Unknown fields,
wrong types and out-of-range values are refused rather than corrected.
Abbreviated scalar fields are strings, including numeric bounds.

| Tool | Catalogue arguments |
| --- | --- |
| `tenant.readiness` | Empty object |
| `balance.get` | Required program; optional account and asset (32-byte hex) |
| `wallet.accounts` | program (32-byte hex) |
| `wallet.balance` | program, account, asset (32-byte hex) |
| `history.list` | account (32-byte hex), limit (`1`–`256`); optional cursor |
| `receipt.get`, `proof.get` | activity_id (32-byte transaction hash) |
| `checkpoint.get` | sequence (decimal u64 height) |
| `availability.get` | batch (nonzero decimal u64 height) |
| `activity.disclose` | canonical_bytes (nonempty hex) |
| `activity.track` | submission_ref |
| `activity.wait` | submission_ref, timeout_ms (`1`–`600000`) |
| `faucet.request` | did, public_key (32-byte hex); definition only on this boundary |

Cursors are transport-specific. The Agent API history cursor is a three-value
decimal array `(blockHeight, txIndex, logIndex)`. The native daemon read adapter
also has a compact 16-byte position cursor, while the abbreviated MCP schema
still validates its optional cursor as 32-byte hex. These encodings cannot be
substituted; a mismatched cursor is refused, not silently restarted or
converted. Preserve the actual returned position and use a compatible owner
surface for pagination.

### Preparation, signing and submission

Network calls use EIP-1559 type-2 transactions on chain **125 (`0x7d`)**.
The owner binds the chain, nonce, destination, calldata, value, gas and fees.
Public node JSON-RPC uses a root URL such as
`https://api-mainnet-beta.paxeer.network/`. MCP tools do not authorize a direct
connection that bypasses daemon policy.

The preparation catalogue accepts the abbreviated fields activity_type,
payload, account_sequence, not_before_ms, expires_at_ms, fee_limit and
idempotency_key, plus explicit native variants `native_v1`, `native_effect_v1`
and `native_send_v1`. Native variants carry structured activity, actor,
authority, sequence, validity bounds, payload/hash, capability_id and a signed
purpose binding; an optional local_grant is a separate signed restriction.
Use the exact generated schema for the variant accepted by the owner.
Catalogue validation alone does not establish execution support.

`activity.disclose` sends the bytes to the owner's `native_disclosure_v1`
Prepare variant. It cannot invent or edit the disclosure. For external signing,
`activity.sign` supports:

```text
variant: external_signature_v1
preparation_ref: 32-byte lowercase hex
signature: 65-byte secp256k1 r || s || v, 130 lowercase hex characters
```

The boundary removes only the explicit carrier variant before invoking Sign.
No key material or implicit signing seed is supplied by the tool. The legacy
sign schema containing only preparation_ref remains a compatibility shape,
not permission to manufacture a signature.

`activity.submit` has a raw_transaction alternative: exact signed bytes as
lowercase `0x02...` hex. A prepared native submission has its own explicit
variant and fields; the abbreviated legacy signature/public-key shape is not
interchangeable with the type-2 signature. Approval must release the exact
preparation and disclosure digest before the ordinary owner can submit it.
Unknown or timed-out sends stay pending and are resolved through Track/Wait,
not automatically recreated at another nonce.

### Wallet, token and grant aliases

The `native_write_v1` carrier has exactly variant, intent, preparation,
canonical_bytes, signature and signer_address. The signature is 65 bytes and
signer_address is 20 bytes, both lowercase hex. The caller supplies the actual
unsigned preview bytes and external signature. The owner independently decodes
the preview, matches the signed purpose, intent and preparation, and requires
actual Prepare bytes to equal the preview before proceeding. Each
Prepare/Sign/Submit/Track stage reauthorizes the current full MCP credential.
A held Prepare returns its real approval hold instead of submitting.

Intent fields retain the useful tool-specific names:

| Alias | Intent fields |
| --- | --- |
| `wallet.send`, `token.transfer` | destination, asset, amount, idempotency_key |
| `token.mint` | asset, destination, amount, idempotency_key |
| `token.create` | symbol, decimals, supply_cap, idempotency_key |
| `grant.issue` | beneficiary, asset, amount, expires_at_ms, idempotency_key |
| `grant.draw` | grant_id, amount, idempotency_key |

The abbreviated legacy `token.create` field is `supply`; the native carrier
requires `supply_cap` and an approved `native-registration-v1` profile. Native
alias targets are ledger `0x000000000000000000000000000000000000101A`:
Asset ordinals 5 for send/transfer, 1 for create, 10 for mint, 7 for issue and
6 for draw. Registration must satisfy the owner policy and capability chain;
a model's request does not establish that authority.

---

## Session tools and untrusted output

Subscription tools are `subscription.create`, `list`, `pause`, `resume`,
`delete`, `health`, and `acknowledge`, with the `subscription.` prefix and
`subscribe` scope. Approval tools are `approval.list`, `get`, `approve`, and
`reject`, with the `approval.` prefix and `approve` scope. Decisions bind the
exact held request digest. Local lifecycle events and tenant readiness do not
establish a transaction rung.

Paid tools `web.search`, `web.fetch` and `web.content` require
`write:web:search`, `write:web:fetch` and `write:web:content` respectively.
They use the configured payer and approval boundary; returned external content
is marked untrusted and cannot change tenant, capability or counterparty.

Invocations are durably tracked so replay and recovery preserve Completed,
Refused, Unknown or Failed outcomes. A result is released only after the
current session boundary is revalidated. Scope changes and revocations are
not bypassed by a previously accepted tool call.

---

## Start here

- [Agent API](AgentApi.md)
- [Agentd](Agentd.md)
- [Running an agent](RunningAnAgent.md)
- [Payments developer path](PaymentsQuickstart.md)

Sources: `services/agent/crates/layerx-mcp/src/{server,catalogue,binding,boundary}.rs`,
`src/tools/write.rs`, and `services/agent/crates/layerx-agentd/src/read/{native,availability,history}.rs`.
