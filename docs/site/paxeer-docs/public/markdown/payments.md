<!-- Source: https://docs.paxeer.app/payments/ -->

# Payments

Build transfers, paid APIs, merchant checkout, metered usage, and subscriptions on Paxeer X with explicit approval, bounded fees, and verifiable settlement.

## One payment lifecycle

Paxeer X combines native Asset settlement with the LayerX payment platform. Applications quote a move, commit the accepted quote under a stable idempotency key, and track a journey whose outcome is backed by evidence. Native integrations prepare canonical activities, disclose their exact terms, sign, submit, and verify the requested commitment.

1. **Quote:** identify payer, recipient, Asset or currency, amount, mechanism, fee ceiling, expected arrival, and irreversible legs.
2. **Approve and commit:** bind approval to those terms. An expired quote returns `quote-expired` rather than silently changing the payment.
3. **Track:** retain the journey or activity identifier and idempotency key through interruptions.
4. **Verify and fulfil:** validate canonical evidence and payment facts before delivering goods or releasing a protected resource.

## Choose a payment experience

| Experience | Flow | Settlement rule |
| --- | --- | --- |
| Wallet or application transfer | Quote and commit a move, or sign a native Asset SEND | Verify the receipt and selected commitment |
| Paid HTTP resource | x402 exact offer and receipt-backed retry | Release only after the offer is paid and verified |
| Merchant cart | Catalog quote, persistent order, payment, fulfilment | Bind order and settlement to the same request digest |
| Metered usage | Non-recurring grant and authorized receiver draws | Validate purpose, allowance, expiry, and each receive |
| Subscription | Recurring grant with a declared billing window | New receive and distinct period key for every renewal |

## Journey states

| State | Application behavior |
| --- | --- |
| `getting-ready` | The payment is accepted and its legs are being prepared. |
| `sending` | At least one leg is in flight. |
| `waiting-for-you` | Request the required human decision before its deterministic expiry. |
| `processing` | The payment is submitted and settling. |
| `still-checking` | The outcome is unknown. Keep duplicate-capable controls locked and resolve the original payment. |
| `done` | Settlement is backed by a verified receipt or finality proof. |
| `done-finalised` | The receipt-backed outcome has committed evidence, represented by `checkpoint-finalised` verification. This retained label is not required for every committed journey: EVM-intent and native journey projections render `done` and expose evidence separately. |
| `refused` | Inspect `refused_by` and `money_left` before reporting the result. |
| `awaiting-approval` | The native engine exposes its awaiting-approval leg phase while the journey awaits approval. It is not receipt-backed completion. |
| `closed` | Retained wallet-binding, deposit, withdraw, and exit history reads as closed. Closed history cannot resume signing or execution and does not assert a successful movement. |

## Merchant checkout

`@sidiora/layerx-merchant-middleware` prices catalog lines and creates a `PAYMENT-REQUIRED` offer. A cart contains at most 256 lines, with unique SKUs and positive integer quantities. All lines share one Asset, recipient, scheme, and network. Amount multiplication and summation use integer arithmetic within the 128-bit amount range.

The order store opens the order before the seller decision. The quote digest binds its lines, total, Asset, recipient, scheme, network, and timeout. Repeating an unchanged checkout resolves the same order; a catalog price change produces a different digest. Store responses are checked for matching `checkoutKey` and `requestDigest`, with mismatches refused as `order-conflict`.

| Checkout result | Action |
| --- | --- |
| `payment-required` | Return HTTP 402 with the decision headers. |
| `pending` | Preserve the order while settlement completes. |
| `refused` | Record the refused order and expose its refusal. |
| `paid` | The order is `paid-verified` and carries the receipt digest and transaction. |

Late settlement uses verified webhooks. The merchant checks signature, freshness, replay protection, order identifier and request digest, then resolves and verifies canonical receipt evidence. It recomputes the receipt digest before marking the order paid.

## Buyer and HTTP payment flow

1. The seller responds with HTTP 402 and `PAYMENT-REQUIRED`.
2. The buyer selects an offered requirement unchanged and checks supported scheme, network, Asset, recipient, amount, and commitment.
3. The buyer pays under the purchase idempotency key and retries with `PAYMENT-SIGNATURE`.
4. The seller verifies the payment before release and supplies `PAYMENT-RESPONSE`.
5. The buyer verifies captured settlement against receipt evidence.

`@sidiora/layerx-buyer-middleware` provides `fetch(url, init)` for the complete loop, plus `parseOffer`, `prepare`, and `captureSettlement` for custom workflows. Unsupported requirements fail before payment. The human-plane transport uses HTTPS, protects bearer tokens with `SecretBytes`, and carries `Idempotency-Key` on mutations.

## Metered payments and subscriptions

A metered or subscription challenge binds `extra.layerx.payer`, `purposeHash`, and `commitment`; subscriptions also bind `windowSeconds`. The payer signs an ordinal-7 grant. Each draw uses a receiver-signed ordinal-6 receive with its own idempotency key. Before submission, the seller checks payer, recipient, Asset, amount, purpose, grant limits, expiry, network, and receiver authority.

Metered grants are non-recurring with a zero window. Subscription grants recur with the exact offered window. Every renewal uses current sequences and a distinct period key: an allowance alone does not establish one renewal per period. Pending draws return HTTP 202 without resource release; persistent draw recovery resolves the original activity identifier.

## Commitments and evidence

| Commitment | Required evidence |
| --- | --- |
| `executed` | Verified canonical receipt for the submitted activity. |
| `batched` | Execution plus an authenticated receipt proof whose canonical value equals the receipt byte for byte. |
| `finalised` | Batch evidence plus finalised checkpoint evidence whose canonical header equals the proof's signed batch header. |

A stronger requested commitment is never downgraded to a weaker success. HTTP 202, admission acknowledgements, and activity identifiers alone do not establish execution. Successful x402 settlement references `lxp:<receipt_digest>`, with the digest computed from the canonical receipt and the protocol Merkle-leaf domain. Verify configured authority, protocol/network, receipt facts, and commitment evidence together.

## Retries, sequences, and unknown outcomes

Choose one stable key per purchase or order. Repeating a human-plane mutation under that key returns its original journey; a different body under the same key is `idempotency-conflict`. Native activities also consume identity and account sequences, preventing replay. Retain signed bytes and the activity identifier when a response is lost. Resolve the original through receipt and status reads instead of generating another debit.

On the human plane, retriabilities are `retriable`, `retriable-after`, `structural`, and `final`. The SDK `ProductionClient` reports its own retry classes: `never`, `safe`, `after`, and `unknown-outcome`. Map between the two vocabularies explicitly rather than treating them as one list. Respect carried rate-limit timing. An `unknown-outcome` result or human-plane `still-checking` requires tracking and evidence recovery, with duplicate controls locked.

## Fees and refusal

The committed schedule prices activity type, encoded bytes, execution, storage, Asset operations, and module operations, then applies its basis-point multiplier with ceiling rounding. Fee estimates expose their schedule and snapshot but reserve nothing. Set an integer `fee_limit` and maintain spendable fee balance for that limit.

An admission refusal consumes no sequence and charges no fee. An admitted activity that fails consumes its sequence, emits a failure receipt, rolls back module effects, and may charge up to the fee limit. Protocol fees go to `system:fees`; Programs occupancy rent is a separate batch settlement.

## Continue building

- [Make your first payment](https://docs.paxeer.app/payments-quickstart)

- [Assets and token accounts](https://docs.paxeer.app/assets)

- [Public payment API](https://docs.paxeer.app/platform-api)

- [x402 transport](https://docs.paxeer.app/paid-apis)

- [Fee schedules](https://docs.paxeer.app/fees)
