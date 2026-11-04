# Visa Trusted Agent Protocol

`layerx-visa-tap` verifies Visa Trusted Agent Protocol credentials and binds
them to Paxeer agent identities. Its adapter id is `visa-tap`; the crate is
`services/interop/crates/layerx-visa-tap`. The protocol description is the
vendor repository's `README.md`, pinned at revision
`16d59bdf3f8a542bc538d0962edbb80ea30a02af` and vendored under
`services/interop/specs/vendor/visa-tap/`.

A verified TAP credential authenticates an agent and its browse/pay intent.
It grants no signing power, capability, budget or permission to debit an
account. Payment still requires the network's native transaction authority
and verified payment evidence. See [UCP checkout](ucp.md) and
[x402 receipt binding](x402.md#receipt-binding).

## What the crate binds

The signature dictionary label is `sig2` and its covered components are
`("@authority" "@path")`. `AgentIntent::Browse` uses `agent-browser-auth`;
`AgentIntent::Pay` uses `agent-payer-auth`. The verifier supports Ed25519 and
RSA-PSS-SHA256, checks the registered algorithm and key status, and binds the
signature to the canonical merchant authority and query-free path.

Freshness uses server-owned time. A signature window may be at most eight
minutes; configured clock skew may be at most
`MAX_CLOCK_SKEW_SECONDS = 5 * 60`. `TapVerifier::verify_credential` verifies
without consuming replay state so a service can consume the nonce atomically
in durable storage. `TapVerifier::verify` instead uses the caller's in-memory
`NonceWindow` with zero clock skew; that helper alone cannot preserve replay
protection across a service restart.

`verified_agent_binding` requires the verified registry entry to name the
same nonzero agent. `bind_verified_agent` stores the association under the
authenticated principal using domain
`LayerX/interop/visa-tap/binding/v1\0`. `prepare_trusted_intent` hands the
authenticated meaning to the existing typed-intent boundary; the credential
binding itself remains non-authoritative.

### Merchant routes and execution

The interop gateway registers these HTTP wrappers:

| Method | Path | Behavior |
| --- | --- | --- |
| `POST` | `/v1/http/visa-tap/intents/verify` | Verify the credential and agent binding; return `credential-verified` without executing a payment |
| `POST` | `/v1/http/visa-tap/intents/execute` | Require a pay credential and a signed native payment, consume durable replay state, then relay that payment |

The service checks the target against the authenticated principal's
`visa_targets` entry and the agent against that principal's signer identity.
The verify route refuses an attached activity. Execution requires an
EIP-1559 transaction in the request's `activity` field, targeting the ledger
precompile's `payExact` call. It binds durable nonce consumption to the
credential, target, operation and payment intent before relaying it. A
conflicting replay returns HTTP `409` with `visa_tap_replayed`.

The service checks the signed native transaction's recipient, asset, amount
and payment reference against the verified result. A submitted payment
remains pending; a missing, failed or mismatched receipt cannot produce
`receipt-verified`.

### Payment evidence

`MerchantOperationResult::from_receipt(evidence, receipt_keys,
payment_reference)` consumes `SequencerEvidence` and deployment-pinned
`&[SequencerAuthorization]` keys. It verifies the native receipt signature,
transaction hash and sequence binding, successful result, and logs bound to
the signed effects. It requires exactly one `PaymentExact` log from
`0x000000000000000000000000000000000000101a`, matching the nonzero payment
reference, and extracts the payer, recipient, asset and positive atomic
amount. Invalid evidence returns `TapError::ReceiptMismatch`.

`MerchantOperationResult::ReceiptVerified` contains the receipt digest and
those payment facts. The caller must compare them with the intended payment;
the service's execution path performs that comparison. The digest identifies
the canonical receipt; the transaction hash identifies the submitted payment.

Paxeer is one BFT chain, EVM chain id `125` (`0x7d`), with its kernel inside
the network participating in block proposal. A payment is final when its
block commits. This adapter's receipt verification accepts signed evidence
at `receipted` or `committed` and does not independently verify a consensus
commit proof. Receipt verification alone must not be presented as proof of
block commitment. See [x402 evidence stages](x402.md#receipt-binding).

## Deployment inputs

`services/interop/deploy/gateway/README.md` describes the protocol network,
authoritative module registry, receipt-key pins, server-owned TAP clock skew,
`visa_agents` with explicit `active` or `revoked` status, and per-principal
canonical `visa_targets`. Missing, expired, revoked or mismatched trust
material is refused. Configure these from trusted deployment data.

Node JSON-RPC uses the root URLs in [public RPC endpoints](../reference/public-rpc.md).
The interop HTTP routes above belong to the configured interop service origin;
they are not additional methods on every public node.

## Status

The crate includes pinned-document, credential, middleware and native receipt
tests. First-party vectors are under
`services/interop/specs/conformance/visa-tap/`. Their presence establishes
neither a public merchant hostname nor external Visa TAP qualification.

Sources: `services/interop/crates/layerx-visa-tap/src/lib.rs`,
`services/interop/crates/layerx-interop-service/src/server.rs`, and
`services/interop/crates/layerx-interop-gateway/src/server.rs`.

[Interop](index.md) · [UCP](ucp.md) · [x402](x402.md)
