<!-- Source: https://docs.paxeer.app/checkpoints/ -->

# Checkpoints

The LayerX checkpoint anchor at 0x1014: its historical ABI, and its retirement at the layerx-merge upgrade.

## Retired at layerx-merge (v7.0)

The checkpoint anchor is retired. Do not build new flows on checkpoint submission, finalisation, challenges, or guarantor attestation against it.

- `SubmitCheckpoint`, `SubmitAvailabilityAttestation`, and `SubmitEquivocation` in the anchor keeper unconditionally return a module-retired error.

- Non-view selectors of the `0x1014` precompile were retired at the `v7.0` merge upgrade. Between that upgrade and the later `v7.0-anchor-store-delete` upgrade, view selectors return zero values; after it, all 21 selectors are retired.

- The ABI below is retained for reading historical data and verifying old evidence. A submitted or finalised status from this surface is not a live settlement step.

For the live ordering and evidence path, see [Sequencing & Finality](https://docs.paxeer.app/sequencing). Custody operations are covered under [Custody](https://docs.paxeer.app/custody).

## What the header binds

A checkpoint committed an ordered range of LayerX activity and its resulting state. The registration carried header commitments; individual activity envelopes, payloads, and receipts remained in the replayable batch and its availability material.

| Commitment | Fields | What a consumer checks |
| --- | --- | --- |
| Domain | Protocol version and network ID | The header belongs to the expected network and supported protocol. |
| Ordering | Epoch, batch number, first sequence, last sequence | Authority term, consecutive batch order, and an uninterrupted activity range. |
| State transition | Previous state root and resulting state root | The batch continues the accepted history and commits its resulting state. |
| Record commitments | Activity, receipt, event, and oracle roots | The relevant ordered records reproduce their independent roots. |
| Availability | Data-availability root | Metadata-bound ordered chunks reconstruct the complete availability bundle. |
| Producer | Timestamp and sequencer ID | Header-relative freshness and an authorized sequencer signature. |

The native anchor accepted the canonical 354-byte batch header and its 64-byte Ed25519 sequencer signature. Obtain commitments from verified canonical header bytes; copying a root from an unverified JSON response loses the signature binding.

## Historical flow

1. Activities were accepted, deterministic transitions executed, and receipts issued.
2. The batch was sealed and the canonical header signed, fixing the activity order and committed roots.
3. Guarantors downloaded the complete batch and availability bundle, verified signatures and authority, and independently replayed every transition.
4. Guarantors recomputed all committed roots and attested matching results. A mismatch withheld the attestation.
5. A submitter presented the header, signature, and guarantor certificate to the Paxeer anchor.
6. The anchor finalised after threshold, continuity, availability, and challenge conditions cleared.

| Stage | Evidence | Established property |
| --- | --- | --- |
| Accepted / L0 | Execution receipt | Activity execution and ordering in the current history. |
| Sealed / L1 | Signed canonical header | Fixed commitments from an identified sequencer. |
| Distributed / L2 | Verifiable availability material | Batch bytes can be retrieved and reconstructed. |
| Attested / L3 (retired) | Guarantor certificate | Bonded guarantors attested matching independent execution. |
| Settled / L4 (retired) | Finalised Paxeer checkpoint | The checkpoint satisfied settlement admission and finality rules. |

## Guarantor certificates and availability (ABI reference)

A guarantor signature attested correctness and possession of the required data classes. The certificate carried the header, validity-proof field, ordered attestations, threshold, and settlement reference. The native anchor supported up to 32 attestations in ascending guarantor-identifier order. Threshold attestations were a bonded economic guarantee; they did not replace independent verification with a cryptographic validity proof.

The availability commitment covered five classes: activities and oracle inputs, tagged receipts and events, canonical state differences, and recovery metadata. Chunks used the canonical size of 65,536 bytes and bound class, ordering, and metadata. Activity, receipt, event, and oracle roots remained independent commitments, so a matching availability root alone did not establish correct execution.

## Read the Paxeer anchor (historical)

The checkpoint anchor is `0x0000000000000000000000000000000000001014`. Read methods return both identities and status so applications can bind proofs to the intended checkpoint. After the store-delete upgrade these selectors are retired; before it, views return zero values.

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

Status `0` meant unknown, `1` submitted, and `2` final. `checkpoint(batchNumber)` returned roots, sequence range, sequencer, signer count, availability mask, open challenges, and submitted/finalized heights. `checkpointGuarantors(batchNumber)` returned admitted guarantor identities in certificate order. Use the complete `LayerXAnchor.sol` ABI for bindings.

## Challenges and bonded accountability (retired)

`openChallenge(batchNumber, kind, evidenceHash)` required the challenge bond and distinguished fraud (`0`) from data availability (`1`). Open challenges prevented finalisation. `finalize(batchNumber)` advanced a submitted checkpoint only when its challenge and timing conditions permitted. The keeper's finalisable check covered status, the declared threshold, the bonded guarantor count, open challenges, the challenge window, and that the batch was exactly the latest finalised batch plus one.

Conflicting 274-byte attestations by the same guarantor for different checkpoints in one batch supported `submitEquivocation(evidenceA, evidenceB)`. That submission path is retired along with the rest of the anchor write surface.

## Consumer verification checklist (historical evidence)

1. Verify endpoint TLS trust, the expected LayerX network, and supported encoding versions.
2. Decode the canonical header and verify its sequencer signature and authority range.
3. Check checkpoint identity, sequential history, roots, and the registered certificate.
4. Read the checkpoint's finality status and the finality flag accompanying the required root.
5. Verify native Merkle paths with exact counts, indices, positional siblings, and account authority bindings.
6. Apply the custody claim's debit, delay, limit, eligibility, and nullifier rules.
7. Retain the verified proof and confirm payout events and consumed claim state.

Public activity commitments use `executed`, `batched`, and `finalised`. Preserve these meanings in application interfaces. Process readiness, a wallet signature, or a historically submitted checkpoint does not justify presenting a claim as settled through the retired anchor.

Continue with [settlement evidence encodings](https://docs.paxeer.app/settlement), [custody and exit workflows](https://docs.paxeer.app/custody), [Paxeer consensus](https://docs.paxeer.app/consensus), and [native EVM interfaces](https://docs.paxeer.app/precompiles).
