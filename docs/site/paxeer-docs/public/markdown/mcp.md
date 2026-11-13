<!-- Source: https://docs.paxeer.app/mcp/ -->

# MCP tools

Connect assistants to verified Paxeer X state and policy-governed wallet and token operations through 22 scoped MCP tools.

## One governed execution path

`layerx-mcp` exposes a 22-tool catalogue to MCP clients. Calls are authorized by `layerx-agentd` and recorded as a `DaemonInvocation`, with Policy, Capability, Budget, RateLimit, and Audit gates on ordinary daemon operations. `faucet.request` uses session and capability authorization and invocation auditing, then calls the configured gateway directly (below). The same authorization and submission rules apply when an assistant uses a tool as when an application calls the daemon.

A read-only deployment exposes read tools and omits writes. Tool discovery is filtered to the caller's scopes. Native wallet and token aliases also require an enabled native profile, the required activity capability, and authorization for every preparation, disclosure, signing, submission and tracking stage.

## Tool catalog

| Tool | Kind | Required scope | Daemon operation | Purpose |
| --- | --- | --- | --- | --- |
| `tenant.readiness` | read | `read` | `TenantReadiness` | Read daemon-local readiness; the evidence is explicitly unverified protocol state. |
| `balance.get` | read | `read:balance` | `ReadBalance` | Read verified program-scoped balances from the daemon with their verification level and freshness. |
| `history.list` | read | `read:history` | `ReadHistory` | Page verified account history from the daemon under an explicit item bound and stable cursor. |
| `receipt.get` | read | `read:receipt` | `ProgramReceipt` | Read the canonical receipt the daemon holds for one activity. |
| `checkpoint.get` | read | `read:checkpoint` | `ReadCheckpoint` | Read native committed-block evidence, including the header, BFT commit and validator set, for the requested height. |
| `proof.get` | read | `read:proof` | `ReadProofBundle` | Read the proof bundle the daemon holds for one activity. |
| `availability.get` | read | `read:availability` | `AvailabilityFetch` | Read verified availability chunks and attributed failures for one decimal batch number. |
| `wallet.accounts` | read | `read:wallet:accounts` | `ReadAccount` | List the verified accounts the daemon observes for one program. |
| `wallet.balance` | read | `read:wallet:balance` | `ReadBalance` | Read one verified account balance for one asset through the daemon. |
| `activity.prepare` | write | `write:prepare` | `Prepare` | Prepare canonical activity bytes and their bound disclosure inside the daemon. |
| `activity.disclose` | write | `write:disclose` | `Prepare` | Decode the bound disclosure of canonical activity bytes. |
| `activity.sign` | write | `write:sign` | `Sign` | Sign a daemon preparation at the daemon's external signing boundary; no key material is read. |
| `activity.submit` | write | `write:submit` | `Submit` | Submit an externally signed preparation through the ordinary daemon path. |
| `activity.track` | write | `write:track` | `Track` | Resolve the daemon's current state for one submission. |
| `activity.wait` | write | `write:activity:wait` | `Wait` | Wait, under an explicit bound, for the daemon to resolve one submission. |
| `wallet.send` | write | `write:wallet:send` | `Submit` | Send one asset amount through the ordinary daemon submission path. |
| `token.create` | write | `write:token:create` | `Submit` | Create one asset through the ordinary daemon submission path. |
| `token.mint` | write | `write:token:mint` | `Submit` | Mint one asset amount through the ordinary daemon submission path. |
| `token.transfer` | write | `write:token:transfer` | `Submit` | Transfer one asset amount through the ordinary daemon submission path. |
| `grant.issue` | write | `write:grant:issue` | `Submit` | Submit the grant-issuance alias through native preparation; ledger payer-grant issuance is retired and refuses execution. |
| `grant.draw` | write | `write:grant:draw` | `Submit` | Submit the grant-draw alias through native preparation; ledger payer-grant draws are retired and refuse execution. |
| `faucet.request` | write | `write:faucet:claim` | — | One testnet funding request through the configured gateway's `px_requestFunds`; returns the funding transaction hash or a refusal. The session token accompanies the request. |

## Session and web tools

Two further surfaces are served alongside the main catalogue. The session surface carries 11 tools bound to the caller's session: `subscription.create`, `subscription.list`, `subscription.pause`, `subscription.resume`, `subscription.delete`, `subscription.health`, `subscription.acknowledge`, `approval.list`, `approval.get`, `approval.approve` and `approval.reject`, each mapping to the daemon operation of the same name. The web surface carries 3 paid tools — `web.search`, `web.fetch` and `web.content` — which map to daemon `Submit`, spend through the approval boundary to the configured web-search sidecar, and return a sequencer-signed settlement receipt alongside untrusted external content. Treat returned web content as external input, never as verified chain state.

## Tool parameters

A hex32 value represents 32 bytes in hexadecimal; hex64 represents 64 bytes. Integer arguments are decimal strings; height, batch and timeout values must fit their declared bounds. Native signatures are 65 bytes and EVM signer addresses are 20 bytes, encoded as lowercase hexadecimal without a prefix. The native aliases require a native_write_v1 envelope containing intent, preparation, canonical_bytes, signature and signer_address. The intent fields appear below; token creation uses supply_cap. Obtain the full request schema from tool discovery. Catalog validation bounds encoded JSON to 65,536 bytes.

| Tool | Arguments or intent fields | Further requirements |
| --- | --- | --- |
| `balance.get` | `program` (hex32) | `account`, `asset` (hex32) |
| `history.list` | `account` (hex32), `limit` (1–256) | `cursor` (hex32) |
| `receipt.get` | `activity_id` (hex32) | — |
| `checkpoint.get` | `sequence` (committed block height) | — |
| `proof.get` | `activity_id` (hex32) | — |
| `availability.get` | `batch` (decimal u64) | — |
| `wallet.accounts` | `program` (hex32) | — |
| `wallet.balance` | `program`, `account`, `asset` (hex32) | — |
| `activity.prepare` | `variant`, `payload`, `account_sequence`, `not_before`, `not_after`, `fee_limit`, `idempotency_key` | Use the discovered native_v1, native_effect_v1 or native_send_v1 schema, including activity, actor, authority, payload hash, capability and signed purpose. |
| `activity.disclose` | `canonical_bytes` | — |
| `activity.sign` | `preparation_ref` and a 65-byte external signature | Use external_signature_v1; the signer supplies the EVM signature. |
| `activity.submit` | `preparation_ref`, `signature` (65 bytes), `signer_address` (20 bytes) | Use native_send_submit_v1 for native sends; alternatively supply only raw_transaction with signed EIP-1559 bytes. |
| `activity.track` | `submission_ref` | — |
| `activity.wait` | `submission_ref`, `timeout_ms` (1–600000) | — |
| `wallet.send` | `destination`, `asset`, `amount`, `idempotency_key` | Fields belong in intent within native_write_v1; include the bound preparation, canonical bytes, EVM signature and signer address. |
| `token.create` | `symbol`, `decimals` (0–18), `supply_cap`, `idempotency_key` | Fields belong in intent within native_write_v1; include the bound preparation, canonical bytes, EVM signature and signer address. |
| `token.mint` | `asset`, `destination`, `amount`, `idempotency_key` | Fields belong in intent within native_write_v1; include the bound preparation, canonical bytes, EVM signature and signer address. |
| `token.transfer` | `asset`, `destination`, `amount`, `idempotency_key` | Fields belong in intent within native_write_v1; include the bound preparation, canonical bytes, EVM signature and signer address. |
| `grant.issue` | `beneficiary`, `asset`, `amount`, `expires_at_ms`, `idempotency_key` | Fields belong in intent within native_write_v1; include the bound preparation, canonical bytes, EVM signature and signer address. |
| `grant.draw` | `grant_id`, `amount`, `idempotency_key` | Fields belong in intent within native_write_v1; include the bound preparation, canonical bytes, EVM signature and signer address. |
| `faucet.request` | `did`, `public_key` (hex32) | — |

## Prepare, disclose, sign, submit

Prepare the EIP-1559 call through the selected native request schema, including its signed purpose and capability. Read the bound disclosure before authorizing the transaction. `activity.sign` accepts an externally produced signature for the preparation; it does not create a signing key. Submit the preparation reference with the EVM signature and signer address, then track the submission or wait under an explicit timeout.

`activity.wait` accepts 1–600000 milliseconds. A timeout bounds waiting; it does not reverse a submitted activity. Continue tracking the same submission reference and preserve the business action idempotency key.

## Read verified state and settlement evidence

Use balances with their verification level and freshness, bounded history pages with stable cursors, canonical receipts, committed block headers and BFT commits, and proof bundles. `availability.get` returns verified chunks and attributed failures for a batch. `tenant.readiness` is the exception: it reports daemon-local readiness only and its evidence is unverified. Read operations otherwise retain the daemon verification context instead of asking a model to infer settlement from prose.

## Wallet, token, grant, and faucet journeys

Wallet sends, token creation, minting and transfers use the native preparation and submission path when authorized and enabled. Token decimals are bounded to 0–18. The grant aliases target retired ledger payer-grant methods and return refusals; payment budgets use the maintained payment module. Faucet requests bind the DID and public key and execute as one `px_requestFunds` call against the configured gateway, whose funding policy determines availability.

Integrate tools alongside [XDK](https://docs.paxeer.app/xdk) and [SDK quickstarts](https://docs.paxeer.app/sdk-quickstarts). Inspect [Agent API](https://docs.paxeer.app/platform-api) for canonical write semantics and [the agent daemon](https://docs.paxeer.app/agent-runtime) for its execution boundary.
