<!-- Source: https://docs.paxeer.app/settlement/ -->

# Settlement & committed evidence

Verify signed execution, transaction inclusion, and committed block evidence on the Paxeer BFT chain.

## Execution and settlement form one history

Paxeer mainnet-beta has EVM chain ID `125`. Its kernel executes within the network, and receipts are final once their covering block is committed. Payments, program calls, transfers, and storage charges participate in the chain execution history.

| Level | Evidence | Meaning |
| --- | --- | --- |
| Submission | Transaction hash | The node has accepted a transaction for processing. |
| Signed execution | Canonical execution receipt | The receipt binds the transaction to its execution outcome. |
| Outcome verification | Receipt and execution outputs | The authenticated result matches the intended payment or program operation. |
| Block coverage | Sequence batch and transaction inclusion | The transaction is included in the identified block and ordered sequence. |
| Commitment | BFT consensus commit | The covering block is committed under the trusted validator set. |

A signed receipt authenticates an execution claim under a receipt key. To establish finality independently, authenticate the covering block, the transaction and sequence-batch inclusion evidence, and the BFT commit under a trusted validator set.

## Node receipt and consensus reads

The node provides `pax_sequencerReceipt` for execution progress and `px_getCommit` for consensus evidence. The receipt response binds the transaction hash to its signed receipt and, when committed, identifies the covering block.

| Method | Purpose |
| --- | --- |
| `eth_sendRawTransaction(raw)` | Submit a signed EVM transaction and obtain its hash. |
| `pax_sequencerReceipt(txHash)` | Read submitted, receipted, or committed progress and execution evidence. |
| `eth_getTransactionReceipt(txHash)` | Read the EVM execution receipt. |
| `px_getCommit(height)`, `px_getValidators(height)` | Acquire the signed consensus header and validator information. |
| `eth_getBlockByNumber(height, false)`, `eth_getTransactionByHash(txHash)` | Acquire block and transaction data for reconciliation. |
| `px_getSequenceBatch(height)` | Acquire the canonical sequence-batch transaction and its block inclusion proof. |
| `px_getReceiptKeys()` | Acquire governed receipt-key identifiers, public keys, and valid height ranges. |

The receipt rungs are `submitted`, `receipted`, and `committed`. Verify the evidence behind the reported rung. The response contract is implemented in `rpc/sequencer_receipt.go`.

## Sequence-batch coverage

Each sequenced block begins with `MsgSequenceBatch`. Retrieve its canonical transaction, activity identifiers, and block inclusion proof with `px_getSequenceBatch(height)`. Obtain the corresponding consensus header and commit with `px_getCommit(height)`.

| Item | Contents | Commitment |
| --- | --- | --- |
| Ordered activity commitment | The batch binds fixed-width activity identifiers in their execution order. | `SHA256(activity_id32 \|\| ...)`, stored in the batch TxRoot field. |

The batch commits to the ordered activity identifiers and sequence hashes. Use `px_getValidators(height)` to acquire validator data and `px_getReceiptKeys` for receipt-key data, authenticating both against independently trusted chain state.

### Receipt and consensus signing roles

Receipt-signing keys authenticate execution receipts and sequence batches within their governed height ranges. Consensus validator keys authenticate block commits. A receipt signature alone does not establish the validator quorum or committed inclusion.

```
Sequence-batch transaction: 241 bytes
"PAXEER/v1/sequence-batch" || 0x00 || Height:u64be || FromSeq:u64be || ToSeq:u64be
|| PrevSeqHash[32] || LastSeqHash[32] || TxRoot[32]
|| KeyId[32] || Sig[64]
```

The `Sig` field is an Ed25519 signature over the chain-bound sequence-batch digest. Verify it with the pinned receipt key valid at the batch height, then verify the batch transaction against the committed block.

## Committed inclusion workflow

1. Retain the signed transaction and verify its hash against the receipt.
2. Authenticate the receipt key, height range, signature, and execution outcome.
3. Acquire the covering sequence batch and verify its signature, sequence hashes, and ordered transaction identifiers.
4. Authenticate the covering block and consensus commit using a trusted validator set.
5. Check the expected account changes or payment effect before completing the application action.

## Verify the transport and the money

Clients check TLS trust and chain identity before consuming evidence. A healthy endpoint or matching chain ID establishes connectivity. Finality requires authenticated committed inclusion and a verified execution outcome for the intended transaction.

Continue with [account and custody interfaces](https://docs.paxeer.app/custody), [Paxeer consensus](https://docs.paxeer.app/consensus), and [native EVM interfaces](https://docs.paxeer.app/precompiles).
