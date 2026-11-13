<!-- Source: https://docs.paxeer.app/services/ -->

# Services and agreements

Publish paid capabilities, agree on deliverables, attest execution, and resolve delivery through the Paxeer X service lifecycle.

Paxeer X turns a service into a verifiable agreement between a provider and a customer. An offer defines price, Asset, terms, deliverable specification, deadlines, and acceptance policy. Agreements bind those terms to an escrow; commitments and execution attestations establish the work performed; delivery records bind the resulting artifacts to their availability references.

## Choose a service model

| Model | Use it for | Completion evidence |
| --- | --- | --- |
| Deliverable agreement | Research, generated assets, analysis, computation, and agent tasks. | Commitments, signed tool execution, delivery hashes, acceptance or dispute ruling. |
| Paid API | Immediate access to a query result, data resource, or tool endpoint. | Verified payment receipt and a durable fulfillment record. |
| Receipt-gated resource | A single paid HTTP request for a resource or tool result. | A signed transaction receipt bound to the requested payment. |

Use the service module for negotiated work with delivery and acceptance. Use [paid API middleware](https://docs.paxeer.app/paid-apis) for an HTTP resource that releases its response after receipt verification. The service module records agreement state; payment operations resolve its linked escrow. Paid HTTP resources verify the payment transaction receipt before fulfillment.

## Register and discover an offer

Publish an offer with `LX_SERVICE_OFFER_PUBLISH`. Give customers the offer identifier and a human-readable description of its terms and deliverable specification. Content hashes commit to the exact documents; host the documents and artifacts where the customer can retrieve and verify them. Discovery identifies a suitable offer; the customer checks its Asset, price, expiry, and policy before proposing an agreement.

| Offer field | Meaning |
| --- | --- |
| `offer_id`, `asset_id`, `price` | Nonzero offer identity, payment Asset identity, and positive integer price. |
| `terms_hash` | Commitment to the terms the customer accepts. |
| `deliverable_specification_hash` | Commitment to the expected result. |
| `delivery_deadline`, `offer_expiry` | Delivery deadline and offer expiry in milliseconds, checked against batch time. |
| `acceptance_window`, `dispute_window` | Nonzero durations in milliseconds: review ends at the delivery deadline plus the acceptance window; the dispute window follows. |
| `default_outcome` | `1` accepts or `2` rejects when the review policy applies. |

Withdraw an offer with `LX_SERVICE_OFFER_WITHDRAW`. An unavailable offer or mismatched terms prevents a new agreement from proceeding.

## From offer to invocation

1. Read the offer and verify the terms and deliverable documents against their hashes.
2. Prepare payment escrow through the payment lifecycle and retain its identifier.
3. Propose an agreement with `agreement_id`, `offer_id`, `terms_hash`, and `escrow_id` using `LX_SERVICE_AGREEMENT_PROPOSE`.
4. The provider accepts with `LX_SERVICE_AGREEMENT_ACCEPT`.
5. Commit a task using `LX_SERVICE_COMMIT_TASK`, binding its task hash, escrow, positive deadline, and resource bound.
6. Execute the agreed capability, publish signed execution attestations, and report progress.
7. Deliver artifacts for customer acceptance; resolve escrow through the payment lifecycle.

### Commitments and execution attestations

A task commitment records `commitment_id`, `agreement_id`, `task_hash`, `escrow_id`, `deadline`, and `resource_bound`. An abandoned commitment carries an explicit reason through `LX_SERVICE_COMMIT_ABANDON`.

`LX_SERVICE_TOOL_EXEC_ATTEST` records the tool identifier, input and output commitments, execution window, resource units, attestor identity, availability reference, public key, and signature. Verification checks the attestor; invalid signatures or execution evidence are refused as `LXP_ERR_INVALID_ATTESTATION`.

`LX_SERVICE_PROGRESS_REPORT` binds a report and note hash to a commitment and availability reference. Progress is an integer from `1` through `10000` basis points. Each report must strictly increase the previous progress value.

## Deliver and review

`LX_SERVICE_DELIVER` carries one to sixteen artifacts. Each item contains a hash, a positive byte size, and a nonzero availability reference; a zero size or zero availability reference is refused as non-canonical. Every deliverable hash must equal the offer's `deliverable_specification_hash`, and delivery must arrive by the agreement deadline. Delivery is accepted from the committed state and again from the rejected state, which permits redelivery after rejection. Retain the bytes corresponding to every declared hash so the customer can verify the result.

```
version:u16be (00 01) || delivery_id32 || agreement_id32 || count:u8
|| { hash32 || artifact_size:u64be || availability_reference32 } × count
```

The customer accepts with `LX_SERVICE_ACCEPT` or rejects using `LX_SERVICE_REJECT`, a rejection reason, and contested hashes. Protocol batch maintenance applies the agreement's default review policy when the acceptance window ends.

### Disputes and resolution

`LX_SERVICE_DISPUTE_OPEN` binds a dispute identifier and evidence hashes to the agreement. The agreement must be in the rejected state, the raiser must be the provider or the buyer, the escrow link must match the agreement, and the escrow must name a nonzero arbiter (`LXP_ERR_ARBITER_REQUIRED`); the dispute must open within its window. `LX_SERVICE_DISPUTE_RESOLVE` must come from that escrow arbiter (`LXP_ERR_UNAUTHORIZED_ARBITER` for any other caller) and supplies a nonzero ruling, the provider's allocation in basis points, and the escrow resolution identifier. Allocation ranges from zero to `10000`.

## Activity reference

The service module ID is `5`; payload version is `1`, encoded by the prefix bytes `00 01` for all activities except tool execution attestations. That activity uses the 416-byte execution record directly. Identifiers and hashes are 32 bytes. The codecs are in include/layerx/lx_service.h and src/modules/service/lx_service_payload.c.

| Activity | Type | Canonical payload bytes |
| --- | --- | --- |
| Offer publish / withdraw | `0x00050001 / 0x00050002` | 179 / 34 |
| Agreement propose / accept | `0x00050003 / 0x00050004` | 130 / 34 |
| Task commit / abandon | `0x00050005 / 0x00050006` | 146 / 36 |
| Tool execution attest / progress | `0x00050007 / 0x00050008` | 416 / 134 |
| Deliver / accept | `0x00050009 / 0x0005000a` | 67 + 72 per item / 34 |
| Reject / dispute open | `0x0005000b / 0x0005000c` | 37 + 32 per hash / 67 + 32 per hash |
| Dispute resolve | `0x0005000d` | 72 |

## Handle refusals and retries

Preserve the agreement, commitment, activity, and escrow identifiers across retries. Check status before resubmission after a timeout. Sequence reuse, unauthorized debit, unavailable offers, terms mismatch, and invalid agreement state are distinct refusals; changing payment identity to escape uncertainty can create a second economic action.

Delivery errors distinguish `LXP_ERR_DELIVERABLE_MISMATCH` (a hash that does not equal the specification hash), non-canonical zero sizes or availability references, and `LXP_ERR_DELIVERY_DEADLINE_PASSED`. Dispute errors distinguish an unauthorized actor from a closed window. Correct the refused condition before submitting a new action.

## Continue building

Connect immediate resource access with [paid APIs and x402](https://docs.paxeer.app/paid-apis), explore [SDK integrations](https://docs.paxeer.app/sdk), and inspect the [protocol modules](https://docs.paxeer.app/modules).
