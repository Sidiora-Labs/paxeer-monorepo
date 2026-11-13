# x402 transport

`layerx-x402` implements the x402 v2 buyer, seller and facilitator roles. Its
adapter id is `x402`, descriptor version is `2.0.0`, and upstream specification
pin is `7d5363a6d51750dc246041f2b0ed5819dd46a0d7`.
[x402_adapter_descriptor(conformance)](../../services/interop/crates/layerx-x402/src/lib.rs)
requires a caller-supplied conformance suite and checks the vendored
[specification](../../services/interop/specs/vendor/x402/x402-specification-v2.md)
against `X402_SPEC_SHA256`.

The adapter translates payment semantics; it does not write balances. Payment
execution belongs to the merged Paxeer network: one BFT chain with EVM chain id
`125` (`0x7d`), whose kernel participates in block proposal. A receipt is final
once its block is committed. Signed receipt evidence and proof of commitment
are distinct verification steps within that network.

This page covers the Rust adapter, its hosted interop routes, and the related
seller middleware. [Interop](Interop.md) indexes the service;
[AP2 mandates](Ap2Mandates.md) and [Mirrors](Mirrors.md) describe other consumers
of signed receipt evidence.

---

## Payment-required flow

1. The seller validates a `PaymentRequired` declaration and issues HTTP 402 with
   a `PAYMENT-REQUIRED` header and the same declaration in the body.
2. The buyer selects the first offer whose `(scheme, network)` is in its
   configured support list, preserves the selected requirements and required
   extensions, and asks its payment plane to construct a JSON object payload.
   The payload must contain a nonzero 32-byte hexadecimal `idempotencyKey`
   matching the key supplied to `Buyer::build_payment`. The buyer returns the
   resulting `PaymentPayload` in `PAYMENT-SIGNATURE`.
3. The seller matches `accepted` against an issued offer before requesting
   payment. The facilitator's `/verify` operation is read-only; `/settle`
   crosses the payment-plane boundary.
4. Submitted transactions remain pending. Seller/facilitator success requires
   matching signed receipt evidence and a gateway `ReceiptVerified` record.
   Fulfillment requiring finality must additionally verify committed evidence.
5. The buyer verifies the returned `PAYMENT-RESPONSE` locally, including payment
   facts, transaction identity, receipt digest and the returned evidence level.
   Consumers requiring finality must require the captured rung to be `committed`.

The role implementations are
[seller.rs](../../services/interop/crates/layerx-x402/src/seller.rs),
[buyer.rs](../../services/interop/crates/layerx-x402/src/buyer.rs), and
[facilitator.rs](../../services/interop/crates/layerx-x402/src/facilitator.rs).

### Public transaction submission and recovery

Use the configured node's HTTPS root for JSON-RPC; check `eth_chainId` returns
`0x7d` before signing. The [public RPC reference](../site/paxeer-docs/src/app/public-rpc/page.tsx)
lists the node endpoints. Hosted interop HTTP calls use the configured interop
service's origin.

Submit the payer-signed EVM transaction with `eth_sendRawTransaction`. Retain
the exact raw bytes, transaction hash and payment identity before sending.
Recover using the original hash: `eth_getTransactionReceipt` returns the EVM
receipt, while `pax_sequencerReceipt` returns the node's signed receipt evidence
and its `submitted`, `receipted`, or `committed` rung. The method name is the
current node API; the signing authority is configured independently by the
consumer. Verify the receipt signature, successful result, transaction binding
and signed payment logs. For commitment, verify the corresponding block and
consensus evidence under configured trust.

The interop [ChainRelay](../../services/interop/crates/layerx-interop-service/src/relay.rs)
journals raw transactions and hashes in Redis before submission. A retry reads
the existing record and recovers evidence for that hash; replacement bytes under
the same identity are refused. An uncertain response is not evidence of failure.
Retry the same payment rather than signing another debit.

## 402LXP Asset grants

The Rust receipt verifier and seller middleware recognize `exact`, `metered`,
and `subscription` payment logs. This does not imply that every payment plane
can execute every scheme. The hosted service's current transaction decoder
accepts `payExact`; its route implementation does not relay a grant draw.
Consult `/supported` and the selected plane's implementation before choosing an
offer.

Grant draws use the ledger precompile at
`0x000000000000000000000000000000000000101a`. The current TypeScript
[grant codec](../../services/agent/sdk/typescript/src/x402/grant.ts) decodes the
228-byte ABI calldata for
`draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)`.
It binds grant id, recipient, amount, idempotency key, receiver sequence,
purpose hash and context hash to the selected offer. The transaction wrapper
binds chain, target and sender. Grant allowance and expiry remain on-chain
authorization constraints.

[PreparedGrantDraws](../../services/agent/sdk/typescript/src/x402/draw.ts)
durably registers principal, request digest, raw transaction, calldata, hash and
optional subscription period key before execution. It recovers using the same
transaction hash and refuses conflicting registrations. Subscription execution
requires a period key; reuse of a recurring allowance alone does not enforce
one charge per period. A new period needs its own registered draw identity and
correct receiver sequence.

### Offer and payload contract

The core [PaymentRequirements model](../../services/interop/crates/layerx-x402/src/model.rs)
contains `scheme`, `network`, `amount`, `asset`, `payTo`, `maxTimeoutSeconds`,
and optional `extra`. Mainnet offers use `eip155:125`.
`asset` and `payTo` are 32-byte identifiers, not 20-byte EVM addresses.

`AtomicAmount` serializes as a canonical unsigned decimal JSON string and uses
`u128` internally. Empty, negative, fractional, exponent, padded and overflowing
amounts are refused. Parsing permits zero, but a payment requirement must be
positive. `maxTimeoutSeconds` is a positive `u32`; it is not itself a verified
transaction expiry. The service checks the signed `expires_at` separately.

The Rust buyer, seller and facilitator require `extra.layerx.account` and
`extra.layerx.currency` through `layerx_terms()`. Account references support
`agent:<did>:main` and `agent:<did>:asset:<asset>`, with an asset suffix matching
the offered asset. Currency is bounded uppercase ASCII/digits. `payTo` must
match one of the account identifiers returned by `account_identifiers`:

- `SHA256("LXP/v1/account-id\0" || account_bytes)`;
- `SHA256("LX:ACCOUNT:v1" || u32be(account_byte_length) || account_bytes)`.

The [seller middleware](../../services/platform/middleware/seller/src/index.ts)
validates its own offer profile. Its
[commitment contract](../../services/platform/middleware/seller/src/commitment.ts)
uses `receipted` or `committed` for `extra.layerx.commitment`; a missing profile
defaults to receipt-level verification. Set `committed` when fulfillment requires
finality. Metered/subscription offers require nonzero payer and purpose hash.
The draw codec additionally requires `grantId`, `receiverSequence` and
`contextHash`. Subscription requires a positive `windowSeconds` below `2^64`;
metered forbids that field. Preserve each implementation's accepted profile;
Rust adapter and middleware payloads are separate contracts.

The hosted exact-payment payload contains `layerxTransaction` (signed raw
transaction hex) and `layerxIdempotencyKey` (32-byte reference). Its
[payer transaction decoder](../../services/interop/crates/layerx-interop-service/src/payer_tx.rs)
requires an EIP-1559 transaction targeting the ledger precompile and
`payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)` calldata. The service
checks the reference, recipient, asset, amount and expiry before relaying it.
These fields do not grant the edge authority to create a payer signature.

---

## Header and body encodings

| Wire header | Core value |
| --- | --- |
| `PAYMENT-REQUIRED` | `PaymentRequired` |
| `PAYMENT-SIGNATURE` | `PaymentPayload` |
| `PAYMENT-RESPONSE` | `SettlementResponse` |

HTTP payment headers contain RFC 4648 standard-base64 JSON. The
[codec](../../services/interop/crates/layerx-x402/src/codec.rs) bounds decoded JSON
at 64 KiB and refuses empty header text or encoded text over 128 KiB. Bad
base64/JSON is `Decode`; size violations are `Bounds`.

The [transport module](../../services/interop/crates/layerx-x402/src/transport.rs)
uses `TransportValue::HttpHeader` for HTTP signals and `TransportValue::Json`
for MCP/A2A, with a 64 KiB JSON bound. Representation/header-name mismatches
are refused. Facilitator `/verify`, `/settle`, and `/supported` messages are
JSON bodies on all three transports. Transport changes representation, not
payment identity or verification requirements.

Core wire structs use camelCase and deny unknown fields. A `PaymentPayload`
has `x402Version: 2`, an object `payload`, `accepted`, optional `resource`, and
`extensions`. A facilitator request wraps it as `paymentPayload` alongside
`x402Version` and `paymentRequirements`; `accepted` must equal those requirements.

Successful Rust settlement responses contain:

| Field | Value |
| --- | --- |
| `success` | `true` |
| `errorReason` | absent |
| `transaction` | Nonzero 32-byte transaction hash, `0x` plus 64 lowercase hex digits |
| `network` | Accepted network |
| `amount` | Exact accepted decimal-string amount |
| `payer` | Payer identifier from the verified payment log |
| `extensions.layerx.receipt` | Standard-base64 canonical signed receipt |
| `extensions.layerx.receiptDigest` | Canonical receipt digest, hex |
| `extensions.layerx.verificationLevel` | `receipted` or `committed` |
| `extensions.layerx.logs` | Receipt-bound address/topics/data/logIndex values |

Grant settlements additionally repeat the offered `purposeHash`. Committed
settlements add `blockNumber`, `blockHash` and `proof`. The receipt digest is
a separate evidence identifier; it is not the settlement transaction hash.

Pending facilitator responses use `success: false`,
`errorReason: "settlement_pending"`, the original transaction hash, and
`verificationLevel: "submitted"`. Other refusals have a safe error reason and
empty `transaction`. Neither an HTTP 200 nor a syntactically valid response
establishes successful payment.

---

## Roles

| Role | Boundary |
| --- | --- |
| Seller | Matches an issued offer and required extensions, calls `PaymentPlane`, verifies receipt/payment evidence, then records the gateway result |
| Buyer | Selects a supported offer, calls `BuyerPaymentPlane`, preserves extensions and verifies settlement locally |
| Facilitator | Declares support, keeps `/verify` read-only and gates `/settle` on verified evidence |

[TRANSPORT_MATRIX](../../services/interop/crates/layerx-x402/src/transport.rs)
declares all three roles for HTTP, MCP and A2A. The versioned hosted routes are
listed on [Interop](Interop.md#gateway-routes). Within x402, the settle routes
perform payment settlement. Route aliases share handlers; they do not bypass
authentication, idempotency or evidence checks.

The package `layerx-interop-service` builds the `layerx-interop-gateway` binary.
Its [HTTP implementation](../../services/interop/crates/layerx-interop-service/src/server.rs)
and [configuration](../../services/interop/crates/layerx-interop-service/src/config.rs)
compose the adapter with the relay, supported offers, receipt keys and durable
operation storage. Facilitator availability depends on the configured relay
and evidence backends.

## Typed refusals

The Rust `X402Error` vocabulary includes:

| Refusal | Meaning |
| --- | --- |
| `Decode`, `Encode`, `Bounds`, `WrongVersion` | Wire representation, serialization, size or version refusal |
| `InvalidAmount`, `InvalidRequirements`, `InvalidPayload` | Noncanonical amount, invalid offer or invalid payment object |
| `PaymentIdentityMissing`, `PaymentIdentityMalformed`, `PaymentIdentityMismatch` | Buyer construction payload omits a native `idempotencyKey`, encodes an invalid or zero key, or supplies a different key |
| `ProfileMissing`, `ProfileMismatch` | Missing or inconsistent account/currency/payee terms |
| `UnsupportedOffer` | No supported scheme/network combination |
| `RequirementsMismatch`, `ExtensionsMismatch` | Selected terms or required extensions changed |
| `PaymentRefused` | Payment-plane refusal or attempted capture of an unsuccessful settlement |
| `TransactionForm` | Transaction reference is not a canonical nonzero hash |
| `EvidenceMissing`, `EvidenceMismatch` | Missing, unauthorized, malformed or incorrectly bound evidence |
| `Gateway(_)` | Translation/store/idempotency refusal |

Pending is represented by seller/plane outcomes and `settlement_pending`;
`PaymentPending` is also a declared error variant. Safe refusal reasons contain
only lowercase ASCII and underscores, length 1–64; unsafe reason text is
rendered as `payment_refused`.

The service has its own HTTP error envelope and reason codes. Missing
transaction/reference inputs produce `typed_intent_required` or
`protocol_idempotency_required`; mismatched reference or terms produce
`protocol_idempotency_mismatch` or `payment_requirements_mismatch`. Invalid
transaction target, selector or signature has a corresponding `transaction_*`
refusal. Expired payment is `activity_time_window_refused`. Backend unavailability
remains unavailable; it does not authorize fulfillment.

The SDK grant-draw recovery path treats only JSON-RPC `-32001` with object
`data.state == "pending"` as the explicit retryable protocol error. A null
receipt or `submitted` result also remains pending. Other RPC, transport,
parsing and evidence errors propagate; they are not silently relabeled pending.

---

## Receipt binding

The shared [evidence model](../../services/interop/crates/layerx-interop-gateway/src/evidence.rs)
is `SequencerEvidence { transaction_hash, signed_receipt, logs, rung }`.
Its rungs mean:

| Rung | Established evidence |
| --- | --- |
| `submitted` | Transaction identity; no successful receipt yet |
| `receipted` | Authenticated signed receipt and bound logs |
| `committed` | Receipt evidence plus the relevant committed block coordinate; x402 committed capture additionally verifies a consensus proof |

`Seller::settled` verifies the canonical receipt under independently configured
height-scoped receipt keys, a successful result and a transaction id matching
`evidence.transaction_hash`. It authenticates logs against signed effects and
requires exactly one ledger payment event. `exact` binds reference, payer,
recipient, asset and amount through `PaymentExact`; grant schemes use
`PaymentDrawn`, bind grant id and reference, and check the offer's payment facts.
Purpose and context authorization for a draw also require the matching signed
transaction and grant validation in the execution path.

[GatewayCore::settle_with_sequencer_evidence](../../services/interop/crates/layerx-interop-gateway/src/gateway.rs)
refuses submitted evidence, verifies signatures and logs, requires one matching
payment reference, and stores the canonical receipt digest as
`TranslationStatus::ReceiptVerified`. The same identity and digest replay
idempotently; another digest produces `IdempotencyConflict`. This shared status
records receipt verification rather than independently certifying block finality.

For a committed x402 response, `Seller::settled` also calls
`layerx_committed_proof::committed::verify_committed`, checks that the proof
contains the same signed receipt, and binds the returned block number/hash.
`Buyer::capture_settlement` checks receipt-level responses using its own keys.
Committed capture requires `Buyer::capture_settlement_with_consensus` and an
independently trusted consensus anchor for the offered network. Response-supplied
validator material cannot replace that anchor. The buyer verifies inclusion,
commit, block identity, payment terms, network, payer and receipt digest; a
committed label without proof is refused.

The hosted relay exports `receipted` evidence even when the node reports
`committed`. It checks the reported block hash against a node block read to
manage journal retention; that observation is separate from portable commitment
evidence. Hosted x402 settlement therefore returns receipt-level success.
Committed fulfillment requires a producer supplying a `FacilitatorCommitment`
proof and a consumer with independently configured consensus trust.

Keep retry identity independent of delivery transport. Facilitator
`SettlementIdentity::derive` binds principal, caller's stable identity, canonical
request semantics and `SettlementStep` (`Single`, `EscrowDeposit`, or
`EscrowCharge`). Transport is absent from that identity. The hosted HTTP
operation record additionally binds the exact method, path and request body;
resume that operation unchanged. In the seller middleware, durable fulfillment
binds the request and unique receipt digest before releasing the resource.

---

## Conformance vectors

The [compatibility matrix](../../services/interop/crates/layerx-x402/COMPATIBILITY.md)
describes local offline coverage, not an independently operated facilitator or
upstream reference-implementation certification. The executable test sources
include:

| Source | Coverage |
| --- | --- |
| [vectors.rs](../../services/interop/crates/layerx-x402/tests/vectors.rs) | Wire models, canonical amounts, validation and transport encoding |
| [pinned_spec.rs](../../services/interop/crates/layerx-x402/tests/pinned_spec.rs) | Vendored specification integrity and altered-byte refusal |
| [transports.rs](../../services/interop/crates/layerx-x402/tests/transports.rs) | Role representation parity, identity, duplicate delivery and recovery |
| [facilitator_evidence.rs](../../services/interop/crates/layerx-x402/tests/facilitator_evidence.rs) | Signed receipt settlement, pending hashes and committed evidence |
| [sequenced_vectors.rs](../../services/interop/crates/layerx-x402/tests/sequenced_vectors.rs) | Recorded receipt/payment bindings and committed proof cases |
| [buyer_sequencer.rs](../../services/interop/crates/layerx-x402/tests/buyer_sequencer.rs) | Buyer-side receipt capture |

See the [portable verification guide](../interop/PORTABILITY.md) for offline
receipt verification and its trust boundaries.

Vector parsing establishes only the assertions in those tests. A local pass is
not evidence that a public endpoint is live, that a merchant fulfills resources,
or that a third-party facilitator interoperates with this deployment.

## `make interop-test-x402`

The existing Makefile target runs:

```text
cargo test --manifest-path services/interop/Cargo.toml --locked -p layerx-x402
```

`INTEROP_CARGO` selects the Cargo executable. This target runs the x402 crate's
tests; it is not a workspace-wide interop or live-service qualification gate.

## Sources

- [Rust x402 crate](../../services/interop/crates/layerx-x402/src/lib.rs)
- [Gateway route contract](../../services/interop/crates/layerx-interop-gateway/src/server.rs)
- [Gateway receipt verification](../../services/interop/crates/layerx-interop-gateway/src/gateway.rs)
- [Interop HTTP implementation](../../services/interop/crates/layerx-interop-service/src/server.rs)
- [Durable transaction relay](../../services/interop/crates/layerx-interop-service/src/relay.rs)
- [Seller middleware](../../services/platform/middleware/seller/src/index.ts)
- [Middleware commitment verification](../../services/platform/middleware/seller/src/commitment.ts)
- [SDK grant draws](../../services/agent/sdk/typescript/src/x402/draw.ts)

[Interop](Interop.md) · [AP2 mandates](Ap2Mandates.md) · [Mirrors](Mirrors.md) · [Home](Home.md)
