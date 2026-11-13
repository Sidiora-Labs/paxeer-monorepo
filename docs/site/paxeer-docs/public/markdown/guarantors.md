<!-- Source: https://docs.paxeer.app/guarantors/ -->

# Validators and commitment

BFT block commitment, signed execution receipts, and independent evidence verification.

Paxeer validators commit blocks on one network. The kernel participates in block production. Applications verify execution receipts and the committed blocks covering them; finality comes from BFT commitment.

## Verify the execution outcome

1. Confirm the chain ID and retain the signed transaction.
2. Retrieve the transaction receipt and its progress rung.
3. Obtain the governed receipt keys and check their authorized height ranges.
4. Verify the receipt signature, decoded fields, and output digests.
5. Verify the committed block covering the receipt sequence.

A receipt signature authenticates execution only when its signing key is authorized for the receipt height. The node exposes the governed key ranges through `px_getReceiptKeys`, implemented in `rpc/sequencer_receipt.go`.

### Exact evidence bindings

Verification checks the exact signed bytes and their bindings. Receipt output data must reproduce the digests in the signed receipt. Missing evidence leaves the outcome at its observed progress level.

## Receipt and commitment evidence

`pax_sequencerReceipt` exposes the transaction hash, nonce, progress rung, and available receipt material. Its committed result identifies the block whose sequence batch covers the receipt.

| Evidence | Meaning |
| --- | --- |
| Submission acknowledgement | The node holds the transaction. |
| Signed execution receipt | The authorized signer records the execution outcome. |
| Receipt-key range | Governance authorizes a receipt key over stated heights. |
| Committed block | BFT commitment makes the included outcome final. |

`SequencerReceiptResult.Verify` checks receipt signatures, decoded fields, output digests, and required block fields. Pass the trusted receipt keys to authenticate signing authority. Consistency under an untrusted supplied key is insufficient.

## Consensus membership

Validator membership is part of the BFT network. The hosted gateway declares `px_getValidators` and `px_getCommit` in `services/platform/hosted/gateway/openrpc.json`; use their verified chain data when inspecting consensus evidence.

The public EVM chain ID is `125`. Bind transaction signatures and receipt verification to the intended chain and obtain current consensus state from that network.

## Block confirmation

The `committed` rung means the receipt sequence is covered by a committed block. Its `blockNumber` and `blockHash` identify that block. Verify these bindings together with the execution result.

A successful submission response is an acknowledgement. A receipt may record a reverted execution. Check status and effects before presenting a committed outcome as a successful payment.

## Application responsibilities

Retain the original signed transaction, its hash, receipt bytes, trusted receipt-key ranges, and covering block evidence. These records let an application verify the outcome independently of a progress indicator.

If evidence is unavailable, keep the transaction pending and reconcile its existing hash. Reissuing a payment with another transaction can create a second economic effect.

## Continue

See [Finality](https://docs.paxeer.app/finality), [Governance](https://docs.paxeer.app/governance), and [Security and trust boundaries](https://docs.paxeer.app/security).
