<!-- Source: https://docs.paxeer.app/payments/ -->

# Payments

Build transfers, paid APIs, merchant checkout, metered usage, and subscriptions on Paxeer X with explicit approval, bounded fees, and verifiable settlement.

## One payment lifecycle

Paxeer X settles native Asset payments on one BFT chain. Applications quote a move, commit the accepted quote under a stable idempotency key, and track a journey whose outcome is backed by evidence. The native wallet prepares a ledger precompile call, discloses its terms, signs an EIP-1559 transaction, submits it to the node and verifies receipt progress.

1. **Quote:** identify payer, recipient, Asset or currency, amount, mechanism, fee ceiling, expected arrival, and irreversible legs.
2. **Approve and commit:** bind approval to those terms. An expired quote returns `quote-expired` rather than silently changing the payment.
3. **Track:** retain the journey or activity identifier and idempotency key through interruptions.
4. **Verify and fulfil:** validate canonical evidence and payment facts before delivering goods or releasing a protected resource.

## Choose a payment experience

| Experience | Flow | Settlement rule |
| --- | --- | --- |
| Wallet or application transfer | Quote and commit a move, or sign a ledger payment transaction | Verify the receipt and selected commitment |
| Paid HTTP resource | x402 exact offer and receipt-backed retry | Release only after the offer is paid and verified |
| Merchant cart | Catalog quote, persistent order, payment, fulfilment | Bind order and settlement to the same request digest |
| Metered usage | Ledger grant and authorized executor draws | Validate purpose, grant limits, expiry, and each draw |
| Subscription | Recurring grant with a declared billing window | Signed draw and distinct period key for every renewal |

## Journey states

| State | Application behavior |
| --- | --- |
| `getting-ready` | The payment is accepted and its legs are being prepared. |
| `sending` | At least one leg is in flight. |
| `waiting-for-you` | Request the required human decision before its deterministic expiry. |
| `processing` | The payment is submitted and settling. |
| `still-checking` | The outcome is unknown. Keep duplicate-capable controls locked and resolve the original payment. |
| `done` | Settlement is backed by a verified receipt or finality proof. |
| `done-finalised` | The human API uses this label for receipt-backed completion with committed evidence. Inspect the evidence as well as the journey state; a `done` journey can also carry committed evidence. |
| `refused` | Inspect `refused_by` and `money_left` before reporting the result. |
| `awaiting-approval` | The native engine exposes its awaiting-approval leg phase while the journey awaits approval. It is not receipt-backed completion. |
| `closed` | A closed journey is read-only and cannot resume signing or execution. Inspect its evidence to determine the movement outcome. |

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

A metered or subscription challenge binds `extra.layerx.payer`, `purposeHash`, and `commitment`; subscriptions also bind `windowSeconds`. A payer authorizes a ledger grant for a specific executor. The executor signs an EIP-1559 `draw` call to `0x101A` for each charge. The seller checks the active grant, executor, source account, recipient, Asset, amount, purpose and network against the offer, then verifies the receipt and payment log.

Grant limits and expiry are enforced by the ledger. Keep one persistent idempotency key for each usage charge or subscription period. The middleware binds that key to the signed draw. A pending payment returns HTTP 202 while the original transaction is tracked; release the resource after the required evidence is verified.

## Commitments and evidence

| Commitment | Required evidence |
| --- | --- |
| `submitted` | An accepted transaction hash; execution is not yet established. |
| `receipted` | A verified signed protocol receipt bound to the transaction, successful result and expected payment log. |
| `committed` | Receipt inclusion in a committed block, its signed consensus commit and a validator set matched to an independently trusted validator hash. |

x402 accepts `receipted` or `committed` in `extra.layerx.commitment`; it defaults to `receipted` when the LayerX terms are absent. Only committed block evidence establishes finality. Successful settlement uses the `lxp:<receipt_digest>` reference. The middleware verifies the signed raw transaction, receipt signature, receipt digest and payment facts, plus consensus evidence when committed settlement is requested.

## Retries, sequences, and unknown outcomes

Choose one stable key per purchase or order. Repeating a human-plane mutation under that key returns its original journey; a different body under the same key is `idempotency-conflict`. Native wallet sends consume the EVM sender nonce and bind debit authorization to payment account state. Retain the signed transaction and its hash when a response is lost. Recover its receipt and journaled submission before creating a new payment.

On the human plane, retriabilities are `retriable`, `retriable-after`, `structural`, and `final`. The SDK `ProductionClient` reports its own retry classes: `never`, `safe`, `after`, and `unknown-outcome`. Map between the two vocabularies explicitly rather than treating them as one list. Respect carried rate-limit timing. An `unknown-outcome` result or human-plane `still-checking` requires tracking and evidence recovery, with duplicate controls locked.

## Fees and refusal

EVM payment transactions pay gas in the native asset. The CLI queries gas prices and estimates the exact ledger call at pending state, then checks the estimate against `--fee-limit` in wei. Keep enough native balance for gas as well as enough Asset balance for the transfer.

A transaction hash or pending response does not establish a successful debit. Inspect the execution result, payment event and requested receipt rung. An executed EVM transaction can consume its nonce and gas even when its call reverts. Program fees and occupancy charges have their own accounting rules; see the fee guide.

## Continue building

- [Make your first payment](https://docs.paxeer.app/payments-quickstart)

- [Assets and token accounts](https://docs.paxeer.app/assets)

- [Public payment API](https://docs.paxeer.app/platform-api)

- [x402 transport](https://docs.paxeer.app/paid-apis)

- [Fee schedules](https://docs.paxeer.app/fees)
