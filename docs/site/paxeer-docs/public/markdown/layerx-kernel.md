<!-- Source: https://docs.paxeer.app/layerx-kernel/ -->

# LayerX Kernel

The deterministic C17 execution core of Paxeer X: one signed activity, one canonical history, and one reproducible result.

The LayerX kernel executes agent activity across payments, escrow, budgets, streams, services, trading, governance, assets, bridges, and programs. It resolves authority, applies state transitions, produces receipts, and binds each result to the next state root. Paxeer provides the EVM settlement and custody layer; ordinary LayerX operations run through the activity protocol without requiring a separate Paxeer transaction.

## Three execution invariants

| Invariant | Meaning for applications |
| --- | --- |
| One canonical history | Global sequences order admitted activities. State roots chain per activity. The append-only log is authoritative; query indexes can be rebuilt. |
| One financial doorway | Modules do not write balances directly. Economic modules and programs propose validated transfer sets, and the only module-to-ledger entry is `lxp_kernel_apply_transfer_set`, preserving a common accounting boundary. |
| One reproducible result | Independent replay produces identical receipts and roots from the same ordered input, parameter versions, and batch time. |

## Canonical activities

Each state-changing action uses a signed `lxp_activity` envelope. The canonical binary encoding uses fixed-width big-endian integers, explicit fields, and no floating point or maps. A valid decode must re-encode to the same bytes; extra trailing bytes are rejected.

| Field | Purpose |
| --- | --- |
| `protocol_version`, `network_id` | Select the enabled protocol rules and prevent cross-network replay. Protocol 1 is legacy, 2 adds occupancy, and 3 adds state commitment. |
| `activity_type` | The high 16 bits identify the module; the low 16 identify the operation ordinal. |
| `actor_did`, `authority` | Identify the principal and primary key, session, or capability grant authorizing the action. |
| `account_sequence` | Must exactly match the actor’s next sequence; gaps and stale sequences are refused. |
| `timestamp_bound` | Bounds validity using the sealed batch timestamp. |
| `idempotency_key` | Binds the retry identity of the activity. Retain the exact signed bytes and key for recovery; do not assume re-executing a duplicate returns the original receipt — recover the outcome through receipt and status reads. |
| `fee_limit` | Caps the deterministic execution fee the actor authorizes. |
| `payload`, `payload_hash`, `signature` | Bind the operation body and its Ed25519 authorization to the canonical signing preimage. |

## From admission to state transition

1. Decode and check the envelope, including protocol version, network, payload binding, and module enablement.
2. Resolve the actor and authority, verify the signature, and enforce the exact account sequence.
3. Evaluate timestamp bounds against batch time.
4. Compute the fee against the active schedule and the activity’s fee limit.
5. Open the state journal; dispatch module validation and execution.
6. Commit successful module effects or roll them back on execution refusal.
7. Apply required fee and sequence bookkeeping and produce a receipt chained to the resulting state root.

### Admission refusal and execution failure

Malformed envelopes, wrong-network requests, and payload-hash mismatches fail admission without consuming the account sequence or charging an execution fee. A terminal rejection still occupies its global sequence, but it does not consume the actor’s account sequence and no fee is charged. Module effects roll back, while protocol bookkeeping persists. Applications must inspect the receipt’s result code before treating an action as successful.

## Identity is part of execution

Agent DIDs are native accounts. The kernel enforces primary and session keys, scoped grants, rotation, recovery, revocation, and expiry. Delegation can narrow authority without widening it. See [Identity and delegated authority](https://docs.paxeer.app/identity) for the identity lifecycle.

## Determinism across machines

Consensus execution uses C17 and deterministic protocol types. It excludes floating point, local wall-clock reads, unstable database iteration, and pointer-derived hashing. Time-dependent rules—including grant expiry, escrow deadlines, stream accrual, funding intervals, and governance activation—read the committed batch timestamp. Oracle observations arrive as signed ordered activities; execution never fetches an external HTTP response.

### Programs share the accounting boundary

Module `0x09` executes deterministic WASM programs under the invoking activity’s authority. Program-owned accounts derive from public program identifiers and seeds. Derivation alone grants no debit authority. Nested spending grants narrow the permitted source, destination, asset, and amount. Program monetary effects pass through the same transfer-set entry as other modules, and persisted namespace occupancy is metered and settled as replayable transfer legs.

## Receipts and replay

A `lxp_receipt` binds the activity identifier, global sequence, previous and resulting state roots, activity root, result code, effects, fee charged, batch identifier, and module and parameter versions. Program outcomes additionally bind terminal results, runtime and ABI versions, resource metering, transfer evidence, and call-graph commitments. Clients supply activities; the protocol produces receipt evidence.

Replicas replay the append-only history using its committed parameters. A root or receipt disagreement stops acceptance of that replay. Portable `layerx-receipt-proof-v2` evidence lets a verifier authenticate a receipt against independently trusted batch facts. A receipt proves execution at its declared evidence level; it does not by itself prove settlement.

### Native integration surfaces

The native interface exposes `lxp_kernel_execute_activity` and `lxp_kernel_dispatch`. Prepared-batch APIs separate preparation, commit, and publication; batch-boundary and publication-digest interfaces bind the records being published. Protocol-3 batch maintenance runs escrow deadlines, budget rollover, service defaults, and occupancy finalization within the batch journal. There is no `lxp_kernel_replay` entry point in the native interface.

## Application workflow

1. Build the canonical activity using the correct network, authority, next sequence, fee limit, and validity bounds.
2. Sign once and retain the exact bytes and idempotency key for retry and recovery.
3. Submit with the evidence level required by the business operation.
4. Verify receipt identity, result, fees, effects, and state-root binding.
5. Require matching batch evidence before releasing a resource that needs settled assurance.

## Continue reading

- [Sequencing, checkpoints, and finality](https://docs.paxeer.app/sequencing)

- [Programmable execution](https://docs.paxeer.app/programs)

- [Identity and delegated authority](https://docs.paxeer.app/identity)

- [Payments and accounting](https://docs.paxeer.app/payments)

- [Receipt verification](https://docs.paxeer.app/receipts)
