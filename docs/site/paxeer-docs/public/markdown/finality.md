<!-- Source: https://docs.paxeer.app/finality/ -->

# Finality & Commitment

Follow a transaction from submission through verified execution to BFT block commitment.

## Transaction progress on one network

Paxeer is one BFT network. Its kernel participates in block production, and an execution receipt becomes final when its block is committed. EVM chain ID is `125`. The node implementation lives in `node/`; receipt progress is exposed by `rpc/sequencer_receipt.go`. See [Commitment evidence](https://docs.paxeer.app/checkpoints).

| Stage | Name | Evidence and responsibility |
| --- | --- | --- |
| Submission | Submitted | The node holds the transaction; no execution receipt is attached. |
| Execution | Receipted | A signed receipt records the outcome before block commitment. |
| Commitment | Committed | A committed block covers the receipt sequence. |

A submission acknowledgement establishes that the transaction was received. A signed receipt establishes its execution outcome. Committed block evidence establishes finality on the same network.

## Receipt commitment rungs

`pax_sequencerReceipt` reports three exact `rung` values. The hosted gateway relays this node method without promoting its evidence level.

| Rung | Required evidence | Application decision |
| --- | --- | --- |
| `submitted` | Transaction hash and nonce without a receipt | Track the existing submission. |
| `receipted` | Signed execution receipt and consistent decoded fields | Inspect success or failure and the recorded effects. |
| `committed` | Receipt covered by a committed block | Treat the verified outcome as final. |

A submitted result contains `txHash` and `nonce`. Receipted results add signed receipt bytes and their decoded fields. Committed results also contain `blockNumber` and `blockHash` for the block covering the receipt sequence.

### Use the exact wire vocabulary

The current names are `submitted`, `receipted`, and `committed`. In the Human API, their verification labels are `unverified`, `receipt-verified`, and `checkpoint-finalised`.

## Recover an unknown outcome

If the outcome is unknown, track the existing transaction and obtain its evidence before taking another economic action:

1. Keep the signed transaction, hash, and nonce.
2. Read `pax_sequencerReceipt` for that transaction hash.
3. Verify the signed receipt against the governed receipt keys from `px_getReceiptKeys`.
4. Check the covering committed block before treating the outcome as final.

A new transaction can repeat the intended payment. Retain the original transaction hash and reconcile that transaction first.

## Payment release policies

Release a paid resource only after checking the receipt against the expected payer, recipient, asset, amount, and execution result, together with the block commitment required by your application.

A payment reference identifies an outcome; it does not establish the signer's authority or block inclusion. Verify the signed material and trusted receipt-key configuration as well as the response fields.

## Replay and execution

The kernel executes canonical activities, program calls, program transfers, and storage occupancy accounting. Replay checks deterministic state changes and receipts against the same ordered history.

## Verify receipt and block evidence

`SequencerReceiptResult.Verify` checks the signed receipt against its decoded fields and output digests. Supply trusted receipt-key pins; a signing key returned beside a receipt is insufficient to establish authority. Committed results must identify the covering block.

## Pending transactions

If a transaction remains submitted or receipted, retain that state until commitment evidence is available. A timeout does not establish failure or finality. See [Account and custody boundaries](https://docs.paxeer.app/custody).

Continue with [Receipts & Proofs](https://docs.paxeer.app/proofs), [Data Availability](https://docs.paxeer.app/data-availability) and [Consensus](https://docs.paxeer.app/consensus).
