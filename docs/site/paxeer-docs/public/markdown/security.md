<!-- Source: https://docs.paxeer.app/security/ -->

# Security and trust boundaries

What signatures, replay, checkpoint settlement, custody controls, and authority services establish in Paxeer X.

Paxeer X separates fast ordered execution from settlement and custody. A receipt establishes a specific execution claim; its inclusion evidence, sequencer authorization, guarantor certificate, and settlement state determine how that claim is verified. Applications preserve these distinctions when releasing goods, updating balances, or permitting withdrawals.

## The trust boundaries

| Boundary | Verification responsibility | Remaining dependency |
| --- | --- | --- |
| Actor to sequencer | Canonical activity encoding, actor signature, authority scope, sequence, and fee rules. | Key custody and correctly scoped delegation. |
| Sequencer to client | Authorized header signature, receipt inclusion, batch identity, and outcome. | Sequencer availability and the requested evidence level. |
| Sealed batch to guarantor | Availability proofs, deterministic replay, and root comparison. | Accessible batch data and eligible honest guarantors. |
| Checkpoint to Paxeer | Domain-bound eligible attestations and registry acceptance. | Settlement-chain execution and configured challenge and withdrawal rules. |
| Application to hosted authority | Authenticated request and pinned identities; verify the resulting receipt facts. | Authority and replica availability. |

## Evidence levels are consequential

Distinguish acceptance, execution, attestation, and settlement as separate evidence levels: acceptance is an ordering receipt, attestation adds bonded replay, and settlement registers the checkpoint on Paxeer. The gateway exposes executed-state handling; use the evidence level required by the application rather than interpreting every successful response as settlement. A canonical L0–L4 naming ladder and a complete commitment-level enum were not verified in source and are not asserted here.

Custody remains on Paxeer. Guarantor bonds, challenge windows, withdrawal limits, and emergency exits bound settlement risk under their configured rules. The checkpoint path uses bonded re-execution and does not claim a SNARK or another validity proof.

## Canonical execution and replay

The append-only activity log is the execution record. Indexes are derived views. Replay checks actor authority, receipts, state transitions, recovery metadata, and roots. Consensus execution excludes floating point, local-clock dependence, and unstable iteration so independent machines reproduce identical bytes.

Program monetary effects are represented as transfer sets in the same history; they are not private balance writes outside the protocol. Storage occupancy is also replay-checkable. Missing availability data or inconsistent replay results prevent a guarantor from signing.

## Verify hosted receipt authority

The hosted authority obtains a canonical receipt and requests evidence from a pinned replica. It checks the replica identity, pinned sequencer public key, signed header, Merkle inclusion, protocol version, sequence range, and re-derived execution batch identifier. Outcome verification then runs against these derived batch facts.

### Authorized batch facts

The hosted authority references an authenticated authorized-batches route (`GET /v1/authorized-batches/by-activity/{activity_id}`) for derived batch facts. Its exact response schema was not verified in authority source (only README/test references were found) and is not specified here; read the deployed service's contract before depending on field names.

1. Start with the exact activity identifier and the intended network and wire version.
2. Retrieve the canonical receipt and authorized evidence through the configured authenticated service.
3. Verify the receipt against the derived batch facts and the application's expected outcome.
4. Require the appropriate checkpoint and settlement evidence for the action being authorized.

A valid signature over a different batch does not establish inclusion for the requested activity. A root copied from an unverified response does not establish authorized execution.

### Refusals and retry behavior

Treat authentication failures, unknown activities, refused evidence, and unavailable dependencies as distinct outcomes: fix credentials for the first, confirm the identifier for the second, do not retry refused evidence as if it were transient, and retry dependency outages with backoff. Exact status codes and error strings for the authorized-batches route were not verified in source and are not listed here.

Service readiness probes the replica, and liveness reports process availability. Neither response certifies a particular activity or proves that the sequencer is reachable.

## Keys, roles, and custody

Owner signatures, session grants, delegated capabilities, and budget allowances carry distinct authority scopes. Revocation and primary-key rotation are ordered governance events. Grant only the operations, assets, budgets, and lifetime the application needs, and verify recovery and signer history when evaluating authority.

Guarantor membership, bond control, custody authority, checkpoint publication authority, and settlement submission are separate responsibilities. The live guarantor model is the Go `layerxanchor` Cosmos module (see [Guarantors](https://docs.paxeer.app/guarantors)), not a GuarantorBond contract. A funded submitter can publish an authorized certificate; funding does not give it the ability to manufacture eligible attestations or owner authorization.

## Emergency response and upgrade control

Protocol emergency controls identify their scope, trigger, exit conditions, epoch, and ordered sequence. Settlement timelocks restrict proposers, executors, guardians, target selectors, call value, delays, and execution windows. Layer X component attestations identify role, configuration, release, and layout; upgrade authority follows the specific component or proxy path.

During a stall, preserve the latest verified receipts, inclusion proofs, batch data, and checkpoint evidence. Determine the last established commitment level before proceeding with settlement or exit workflows. A loss of service availability must not be interpreted as a successful state transition.

## Integration review

- Pin network, protocol version, sequencer authorization, and settlement domain.

- Reject noncanonical encodings, unexpected response fields, and evidence bound to another activity or batch.

- Make retries idempotent and preserve the original activity identifier.

- Distinguish transport success, verified execution, checkpoint attestation, and settlement.

- Inspect deployed role assignments, thresholds, delays, and limits as part of the application's risk policy.

Continue with [Guarantors](https://docs.paxeer.app/guarantors), [Governance](https://docs.paxeer.app/governance), and [Finality](https://docs.paxeer.app/finality).
