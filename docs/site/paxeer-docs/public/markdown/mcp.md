<!-- Source: https://docs.paxeer.app/mcp/ -->

# MCP tools

Connect assistants to verified Paxeer X state and policy-governed wallet and token operations through 22 scoped MCP tools.

## One governed execution path

`layerx-mcp` exposes a 22-tool catalogue to MCP clients. Every invocation passes through `layerx-agentd` as a `DaemonInvocation`, with Policy, Capability, Budget, RateLimit, and Audit gates, except `faucet.request`, which has no daemon operation and executes directly against the node (below). The same authorization and submission rules apply when an assistant uses a tool as when an application calls the daemon.

A read-only deployment exposes read tools and omits writes. Tool discovery is filtered to the scopes bound to the caller, so an unavailable scope does not appear as an actionable tool.

## Tool catalog

| Tool | Kind | Required scope | Daemon operation | Purpose |
| --- | --- | --- | --- | --- |
| `tenant.readiness` | read | `read` | `TenantReadiness` | Read daemon-local readiness; the evidence is explicitly unverified protocol state. |
| `balance.get` | read | `read:balance` | `ReadBalance` | Read verified program-scoped balances from the daemon with their verification level and freshness. |
| `history.list` | read | `read:history` | `ReadHistory` | Page verified account history from the daemon under an explicit item bound and stable cursor. |
| `receipt.get` | read | `read:receipt` | `ProgramReceipt` | Read the canonical receipt the daemon holds for one activity. |
| `checkpoint.get` | read | `read:checkpoint` | `ReadCheckpoint` | Read the finalised checkpoint certificate for one batch sequence through the daemon. |
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
| `grant.issue` | write | `write:grant:issue` | `Submit` | Issue one spending grant through the ordinary daemon submission path. |
| `grant.draw` | write | `write:grant:draw` | `Submit` | Draw against one spending grant through the ordinary daemon submission path. |
| `faucet.request` | write | `write:faucet:claim` | — | One testnet funding request through the node's `px_requestFunds`; returns the funding transaction hash or a refusal. The retired Agent API `faucet.claim` operation is not used. |

## Session and web tools

Two further surfaces are served alongside the main catalogue. The session surface carries 11 tools bound to the caller's session: `subscription.create`, `subscription.list`, `subscription.pause`, `subscription.resume`, `subscription.delete`, `subscription.health`, `subscription.acknowledge`, `approval.list`, `approval.get`, `approval.approve` and `approval.reject`, each mapping to the daemon operation of the same name. The web surface carries 3 paid tools — `web.search`, `web.fetch` and `web.content` — which map to daemon `Submit`, spend through the approval boundary to the configured web-search sidecar, and return a sequencer-signed settlement receipt alongside untrusted external content. Treat returned web content as external input, never as verified chain state.

## Tool parameters

A hex32 value represents 32 bytes in hexadecimal; hex64 represents 64 bytes. Sequence and batch identifiers are decimal u64 values. Keep integer amounts exact. The argument schema bounds encoded JSON to 65,536 bytes at catalog validation.

| Tool | Required arguments | Optional arguments |
| --- | --- | --- |
| `balance.get` | `program` (hex32) | `account`, `asset` (hex32) |
| `history.list` | `account` (hex32), `limit` (1–256) | `cursor` (hex32) |
| `receipt.get` | `activity_id` (hex32) | — |
| `checkpoint.get` | `sequence` (decimal u64) | — |
| `proof.get` | `activity_id` (hex32) | — |
| `availability.get` | `batch` (decimal u64) | — |
| `wallet.accounts` | `program` (hex32) | — |
| `wallet.balance` | `program`, `account`, `asset` (hex32) | — |
| `activity.prepare` | `activity_type`, `payload`, `account_sequence`, `not_before_ms`, `expires_at_ms`, `fee_limit`, `idempotency_key` | — |
| `activity.disclose` | `canonical_bytes` | — |
| `activity.sign` | `preparation_ref` | — |
| `activity.submit` | `preparation_ref`, `signature` (hex64), `signer_public_key` (hex32) | — |
| `activity.track` | `submission_ref` | — |
| `activity.wait` | `submission_ref`, `timeout_ms` (1–600000) | — |
| `wallet.send` | `destination`, `asset`, `amount`, `idempotency_key` | — |
| `token.create` | `symbol`, `decimals` (0–18), `supply`, `idempotency_key` | — |
| `token.mint` | `asset`, `destination`, `amount`, `idempotency_key` | — |
| `token.transfer` | `asset`, `destination`, `amount`, `idempotency_key` | — |
| `grant.issue` | `beneficiary`, `asset`, `amount`, `expires_at_ms`, `idempotency_key` | — |
| `grant.draw` | `grant_id`, `amount`, `idempotency_key` | — |
| `faucet.request` | `did`, `public_key` (hex32) | — |

## Prepare, disclose, sign, submit

Prepare canonical activity bytes with an account sequence, validity window, fee ceiling, and idempotency key. Read the bound disclosure before authorizing a signature. `activity.sign` crosses the external signing boundary without exposing key material. Submit the preparation reference with the signature and signer public key, then track the submission or wait under an explicit timeout.

`activity.wait` accepts 1–600000 milliseconds. A timeout bounds waiting; it does not reverse a submitted activity. Continue tracking the same submission reference and preserve the business action idempotency key.

## Read verified state and settlement evidence

Use balances with their verification level and freshness, bounded history pages with stable cursors, canonical receipts, checkpoint certificates, and proof bundles. `availability.get` returns verified chunks and attributed failures for a batch. `tenant.readiness` is the exception: it reports daemon-local readiness only and its evidence is unverified. Read operations otherwise retain the daemon verification context instead of asking a model to infer settlement from prose.

## Wallet, token, grant, and faucet journeys

Wallet sends, token creation, minting, transfers, grant issuance, and grant draws reuse ordinary daemon submission. Token decimals are bounded to 0–18. A grant binds beneficiary, asset, amount, and expiry; a draw names the grant and amount. Faucet requests bind the DID and public key and execute as one `px_requestFunds` call against the node.

Integrate tools alongside [XDK](https://docs.paxeer.app/xdk) and [SDK quickstarts](https://docs.paxeer.app/sdk-quickstarts). Inspect [Agent API](https://docs.paxeer.app/platform-api) for canonical write semantics and [the agent daemon](https://docs.paxeer.app/agent-runtime) for its execution boundary.
