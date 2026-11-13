<!-- Source: https://docs.paxeer.app/checkpoints/ -->

# Checkpoints

Block commitment, trusted state-sync checkpoints, and the disabled checkpoint anchor interface.

## Block commitment

Paxeer mainnet-beta is one BFT chain with EVM chain ID 125. Its kernel proposes blocks within the network. A receipt is final when its block is committed; a separate checkpoint submission is not part of application settlement.

- `SubmitCheckpoint`, `SubmitAvailabilityAttestation`, and `SubmitEquivocation` in modules/layerxanchor/keeper/ unconditionally return a module-retired error.

- The `0x1014` precompile disables writes when `v7.0` is active and disables every selector when `v7.0-anchor-store-delete` is active.

- The ABI below declares the checkpoint read signatures. These calls do not provide a settlement path on the merged network.

For the live ordering and evidence path, see [Sequencing & Finality](https://docs.paxeer.app/sequencing). Custody operations are covered under [Custody](https://docs.paxeer.app/custody).

## What a block header binds

Consensus headers identify the chain and block, link the block history, and commit transaction, validator and application data. The header and commit types are defined in consensus/types/block.go.

| Commitment | Fields | What a consumer checks |
| --- | --- | --- |
| Domain | Chain ID and block version | The header belongs to the expected chain and block format. |
| Ordering | Height and previous block ID | The height and previous-block reference continue the trusted chain. |
| State transition | Previous-height application hash and result hash | The commitment covers the state after the preceding height; match the proof height accordingly. |
| Record commitments | Transaction data hash and evidence hash | The block data and evidence match their header commitments. |
| Validator authority | Validator and next-validator hashes | The validator sets agree with the trusted consensus history. |
| Consensus | Timestamp, proposer and last commit | Header time and validator signatures satisfy the consensus verification rules. |

Obtain commitments from verified block headers and validator commits. A root copied from an unverified response is not a trust anchor. The light client in consensus/light/ verifies headers against a trusted validator set and trust period.

## Transaction and block flow

1. A signed transaction is submitted to the node and checked for mempool admission.
2. The kernel proposes a block within the Paxeer consensus network.
3. Validators evaluate the proposal under the configured consensus protocol.
4. The application executes the ordered transactions and records their results.
5. Consensus commits the block and the application commits its state.
6. Applications read the transaction receipt and check its status and committed block identity.

| Stage | Evidence | Established property |
| --- | --- | --- |
| Submitted | Transaction hash | The transaction has a reconciliation identity. |
| Proposed | Proposed block | The transaction has a proposed position in the block. |
| Executed | Application result | The transaction has an execution outcome. |
| Committed | Consensus commit | The block and its receipts are final. |
| Successful | Successful transaction receipt | The committed transaction executed successfully. |

## Validator signatures and state sync

The light client verifies headers using trusted validator information. Non-adjacent verification checks trusted-set overlap and requires more than two-thirds of the new validator voting power to sign the new header. Trust-period and clock-drift checks also apply.

State sync uses an independently trusted block height and hash with application snapshots. Snapshot production is configured in app.toml under [state-sync]; snapshot consumption is configured in config.toml under [statesync]. Configure the joining node with trusted RPC servers and a valid trust period.

## Checkpoint read ABI

The checkpoint anchor address is `0x0000000000000000000000000000000000001014`. These signatures are declared in precompiles/layerxanchor/LayerXAnchor.sol. With the store-delete upgrade active, every selector returns a disabled-module error.

```
interface ICheckpointReads {
    function latestFinalized()
        external view returns (uint64 batchNumber, bool exists);
    function statusOf(uint64 batchNumber)
        external view returns (uint8);
    function finalizedStateRoot(uint64 batchNumber)
        external view returns (bytes32 stateRoot, bool finalized);
    function finalizedReceiptRoot(uint64 batchNumber)
        external view returns (bytes32 receiptRoot, bool finalized);
    function checkpointBatch(bytes32 checkpointId)
        external view returns (uint64 batchNumber, uint8 status);
}
```

The ABI declares status values `0`, `1`, and `2` for unknown, submitted and final. The declarations `checkpoint(batchNumber)` and `checkpointGuarantors(batchNumber)` describe checkpoint records. They do not supply live block finality. The interface file is `precompiles/layerxanchor/LayerXAnchor.sol`.

## Checkpoint write availability

The anchor ABI declares `openChallenge(batchNumber, kind, evidenceHash)` with kind `0` for fraud and `1` for availability, and declares `finalize(batchNumber)`. These write selectors are disabled on the merged network; application receipt finality comes from the BFT block commit.

The anchor declaration `submitEquivocation(evidenceA, evidenceB)` is also disabled. Validator equivocation is handled by the consensus evidence and slashing modules.

## Consumer verification checklist

1. Verify endpoint TLS trust and the expected Paxeer chain identity.
2. Verify the block header and validator commit against an independent trust anchor.
3. Check the transaction hash, block identity and committed height.
4. Read the transaction receipt and check whether execution succeeded.
5. Verify state proofs against the application commitment for the requested height.
6. Check application authorization, asset units and replay protection for the operation.
7. Retain verified evidence and reconcile the resulting events and balances.

For EVM transactions, read `eth_getTransactionReceipt` and inspect `status` and `blockHash`. A submission hash identifies a transaction; successful execution in a committed block establishes the application result.

Continue with [settlement evidence encodings](https://docs.paxeer.app/settlement), [native account funding](https://docs.paxeer.app/custody), [Paxeer consensus](https://docs.paxeer.app/consensus), and [native EVM interfaces](https://docs.paxeer.app/precompiles).
