<!-- Source: https://docs.paxeer.app/sequencing/ -->

# Sequencing & Finality

Follow a signed transaction through ordered execution, receipt verification, and BFT block commitment.

Paxeer mainnet-beta is one BFT chain. Its kernel participates in block execution and proposal, and the committed chain records the resulting order. A signed receipt can be available before its covering block is committed.

The proposer places `MsgSequenceBatch` at transaction index zero of each block at or above the sequencing activation height. Validators verify that batch and re-execute its transactions. See [block commitment](https://docs.paxeer.app/checkpoints) for the evidence boundary.

## Global ordering and sequence batches

A nonempty batch starts at the stored last sequence plus one and binds the preceding sequence hash. The keeper rejects a discontinuity, an invalid receipt-key signature, a mismatched height, or a second sequence batch in one block. EVM account nonces separately enforce sender ordering.

| Batch commitment | Fields |
| --- | --- |
| Block height | `Height`, `FromSeq`, `ToSeq` |
| Sequence continuity | `PrevSeqHash`, `LastSeqHash`, `TxRoot` |
| Receipt-key authority | `KeyId`, `Sig` |
| Hash widths | `PrevSeqHash`, `LastSeqHash`, `TxRoot` are each 32 bytes. |
| Key and signature widths | `KeyId` is 32 bytes; `Sig` is a 64-byte Ed25519 signature. |
| Canonical encoding | The domain prefix is followed by big-endian positions, hashes, key ID, and signature; `Height` names the covering block. |

The canonical sequence-batch transaction is 241 bytes, including its domain prefix and signature. For an empty batch, `ToSeq` equals `FromSeq` minus one, the sequence hash is unchanged, and the transaction root is SHA-256 of the empty input.

## Evidence stages

| Level | Evidence | Responsibility |
| --- | --- | --- |
| Submitted | The node holds the transaction without a signed receipt. | Track its transaction hash. |
| Receipted | A signed receipt binds the raw transaction to its execution outcome. | Authenticate the receipt key and check the outcome. |
| Committed | A committed block covers the receipt sequence. | Verify block inclusion and the consensus commit. |

A `submitted` result carries no execution receipt. A `receipted` result authenticates execution when verified under a trusted receipt key. Finality is established when the covering block is committed by BFT consensus.

## Replay

1. Retrieve the exact signed transaction and its canonical execution receipt.
2. Authenticate the receipt signature using a trusted key with a height range covering the receipt.
3. Check the sequence-batch signature, transaction order, and sequence-hash continuity.
4. Authenticate the covering block and its consensus commit before accepting committed inclusion.

A failed replay or sequence check rejects the proposal. Consensus execution uses the ordered transaction input and block context so validators can reproduce the transition before committing it.

## Public receipt states

The node receipt API reports `submitted`, `receipted`, and `committed`. These values describe transaction progress; applications verify the evidence behind the reported value.

| Receipt state | Required evidence |
| --- | --- |
| `submitted` | Transaction hash, nonce, and progress state. |
| `receipted` | Canonical signed receipt, authenticated signing key, and matching execution view. |
| `committed` | Receipt evidence plus the covering block number and block hash; independent finality verification authenticates inclusion and the consensus commit. |

Retrieve transaction progress with `pax_sequencerReceipt`. Preserve the original signed transaction and obtain `px_getSequenceBatch` and `px_getCommit` evidence for independent committed inclusion checks. A transaction hash or successful HTTP response alone is not a receipt. The final rung is `committed`.

## Data availability and recovery

Committed inclusion verification requires the exact transaction, receipt, covering sequence batch, block inclusion path, consensus commit, and a trusted validator set. A root without authenticated reconstruction and inclusion evidence is insufficient.

Node recovery restores committed sequence state from the module store. The proposer executor also keeps a write-ahead log of accepted transactions and sealed blocks so recovery can reconcile uncommitted execution with the committed head.

### Proposal validation

The signed sequence batch binds its height, ordered range, transaction root, and sequence hashes. Validators check the proposer against the governed leader schedule and reject inconsistent sequence or replay results.

## Execution in one network

Transfers, program calls, service operations, and market operations execute within the same network. Applications inspect the operation result and affected account state in addition to its transaction position.

If consensus cannot commit a block, an available execution receipt remains evidence of signed execution. Clients retain that receipt and wait for authenticated committed inclusion before treating its effects as final.

## Continue reading

- [Deterministic kernel execution](https://docs.paxeer.app/layerx-kernel)

- [Block commitment](https://docs.paxeer.app/checkpoints)

- [Data availability and reconstruction](https://docs.paxeer.app/data-availability)

- [Deposits, withdrawals, and exits](https://docs.paxeer.app/bridge)

- [Receipt and proof verification](https://docs.paxeer.app/receipts)

- [Paxeer consensus](https://docs.paxeer.app/consensus)
