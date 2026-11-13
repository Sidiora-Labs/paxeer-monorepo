<!-- Source: https://docs.paxeer.app/guarantors/ -->

# Guarantors

Independent batch replay, bonded attestations, checkpoint certificates, and settlement accountability.

Guarantors connect Layer X execution to Paxeer settlement. Each guarantor retrieves sealed batch data, independently executes it, compares the committed results, and signs an attestation only after verification. The certificate aggregates the required eligible attestations into evidence that the settlement registry can check.

## Replay before signing

1. Pin the network, sequencer identity and key, and authorized batch range.
2. Retrieve the signed candidate header and its availability chunks.
3. Verify request correlation, chunk indices, hashes, Merkle proofs, the terminating response, and the reconstructed availability root.
4. Load the verified genesis and registration state, validate actor signatures and authority, and replay the kernel and Programs runtime.
5. Compare receipts, state diffs, recovery metadata, and committed roots before retaining or signing evidence.

The producer verifies receipt signatures with the sequencer public key; it does not need the sequencer private key. Deterministic program calls, program-owned transfers, and storage-occupancy charges participate in the same replay. Replay failure stops attestation production.

### Data availability is part of verification

Candidate retrieval is bounded, and reconstruction must reproduce the signed header's availability root before signing. A missing terminator, unavailable data, or a root mismatch is a refusal to sign. Specific chunk-count, chunk-size, and availability-class limits were not verified against a live producer in the monorepo (the former C/Python guarantor paths are retired) and are not specified here.

## Attestations and certificates

Guarantors exchange canonical attestations through authenticated peer connections. An incoming attestation must pass signature and recovery verification, bonded-membership checks, and checkpoint-identifier recomputation from a locally verified header. The certificate assembler sorts guarantor identifiers and requires the configured threshold.

| Evidence | Meaning |
| --- | --- |
| Signed batch header | Sequencer commitment to batch ordering and roots. |
| Guarantor attestation | A bonded signer's statement about a verified checkpoint. |
| Checkpoint certificate | Canonical aggregation of the required eligible attestations. |
| Settlement registration | The registry accepts the certificate under the deployed domain and contract rules. |

Checkpoint-certificate and guarantor-attestation signatures use distinct domain separators: `LXP/v2/checkpoint-certificate` and `LXP/v2/guarantor-attestation`, each followed by a null byte. Certificates carry an empty validity-proof field. An attestation is bonded re-execution evidence, not a zero-knowledge validity proof.

## Bonded membership

The live implementation is the Go Cosmos module `modules/layerxanchor`, not a GuarantorBond contract; no `GuarantorBond.sol` exists in the monorepo. The module records guarantors with a signer and bond, and eligibility is narrow: a guarantor is eligible when its status is active and its bond is at least the module's `MinBond` parameter (default `1000000` in the bond denomination). Depositing tokens alone does not create membership authority.

Older descriptions of a minimum bond computed as custodied value times a basis-point requirement, epoch/jail/ejection/unresolved-slashing eligibility checks, and a minimum one-day unbonding delay describe the retired contract narrative and were not found in the live module. Unbonding delay, equivocation/slashing paths, and peer-transport (mTLS) details were not verified in a live replacement producer and are not specified here.

## Settlement confirmation

The submitter validates the deployed chain and registry binding, encodes the certificate, and submits a settlement transaction. Registration is confirmed against successful receipt status, transaction identity, the canonical block, and every checkpoint-registration event field. An existing registration is accepted only when it records the exact certificate.

The node independently verifies the finality bundle returned to it. The producer then compares the node's stored finality evidence with the submitted bundle. These checks connect locally verified execution, the registered certificate, and the evidence applications subsequently consume.

## Operator responsibilities

Maintain a distinct signing identity, exclusive persistent-state ownership, durable availability data, and retained attestations. Reuse stored attestations for a previously signed batch rather than generating replacement signatures. Peer-transport details were not verified in a live producer and are not specified here. Organizational and infrastructure independence require separate operator arrangements; two different keys alone do not establish that independence.

Replay supports the direct-owner authority path and explicitly refuses unsupported oracle inputs or incomplete rotated-signer history. Treat these refusals as verification boundaries. A guarantor must obtain sufficient replay evidence before attesting.

## Continue

See [Finality](https://docs.paxeer.app/finality), [Governance](https://docs.paxeer.app/governance), and [Security and trust boundaries](https://docs.paxeer.app/security).
