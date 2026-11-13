<!-- Source: https://docs.paxeer.app/layerx-kernel/ -->

# LayerX Kernel

The deterministic C17 execution core of Paxeer X: one signed activity, one canonical history, and one reproducible result.

The LayerX kernel executes native activity across payments, escrow, budgets, streams, services, trading, governance, assets, bridges, and programs. It resolves authority, applies state transitions, produces receipts, and binds each result to the next state root. It runs inside `paxd` through `modules/lxphost/`. Native operations execute in the node's transaction context on the same chain as EVM operations; a receipt is final when its block commits.

## Three execution invariants

| Invariant | Meaning for applications |
| --- | --- |
| One canonical history | The node orders transactions and binds native activity results to global sequences and state roots. Block commitment establishes finality; query indexes derive from committed execution. |
| One financial doorway | Modules do not write balances directly. Economic modules and programs propose validated transfer sets, and the only module-to-ledger entry is `lxp_kernel_apply_transfer_set`, preserving a common accounting boundary. |
| One reproducible result | Independent replay produces identical receipts and roots from the same ordered input, parameter versions, and batch time. |

## Canonical activities

The kernel represents an operation with an `lxp_activity` envelope. Native precompiles resolve the EVM caller's authority and construct the kernel execution context. The activity codec uses fixed-width big-endian integers, explicit fields, and no floating point or maps. A valid decode must re-encode to the same bytes; extra trailing bytes are rejected.

| Field | Purpose |
| --- | --- |
| `protocol_version`, `network_id` | Select the enabled protocol rules and prevent cross-network replay. Version 2 includes occupancy, and version 3 includes state commitment. Use the version configured by the node. |
| `activity_type` | The high 16 bits identify the module; the low 16 identify the operation ordinal. |
| `actor_did`, `authority` | Identify the principal and primary key, session, or capability grant authorizing the action. |
| `account_sequence` | Must exactly match the actor’s next sequence; gaps and stale sequences are refused. |
| `timestamp_bound` | Bounds validity using the sealed batch timestamp. |
| `idempotency_key` | Binds the retry identity of the activity. Retain the exact signed bytes and key for recovery; do not assume re-executing a duplicate returns the original receipt — recover the outcome through receipt and status reads. |
| `fee_limit` | Caps the deterministic execution fee the actor authorizes. |
| `payload`, `payload_hash`, `signature` | Bind the operation body to its canonical signing preimage. Direct signed activities use Ed25519; native precompiles supply authority from the authenticated transaction caller. |

## From admission to state transition

1. Decode and check the envelope, including protocol version, network, payload binding, and module enablement.
2. Resolve the actor and authority, validate the applicable signature or native caller authorization, and enforce the account sequence.
3. Evaluate timestamp bounds against batch time.
4. Compute the fee against the active schedule and the activity’s fee limit.
5. Open the state journal; dispatch module validation and execution.
6. Commit successful module effects or roll them back on execution refusal.
7. Apply required fee and sequence bookkeeping and produce a receipt chained to the resulting state root.

### Admission refusal and execution failure

Malformed envelopes, wrong-network requests, and payload-hash mismatches fail admission without consuming the kernel account sequence or charging a kernel execution fee. Ordered terminal refusals have a separate receipt path. An admitted execution can roll back module effects while retaining sequence and fee bookkeeping under the active fee policy. EVM gas accounting is separate. Inspect the receipt’s result code and committed state before treating an action as successful.

## Identity is part of execution

Agent DIDs identify principals whose account IDs are derived by the protocol account codec. The kernel enforces primary and session keys, scoped grants, rotation, recovery, revocation, and expiry. Delegation can narrow authority without widening it. See [Identity and delegated authority](https://docs.paxeer.app/identity) for the identity lifecycle.

## Determinism across machines

Consensus execution uses C17 and deterministic protocol types. It excludes floating point, local wall-clock reads, unstable database iteration, and pointer-derived hashing. Time-dependent rules—including grant expiry, escrow deadlines, stream accrual, funding intervals, and governance activation—read the committed batch timestamp. Oracle observations arrive as signed ordered activities; execution never fetches an external HTTP response.

### Programs share the accounting boundary

Module `0x09` executes deterministic WASM programs under the invoking activity’s authority. Program-owned accounts derive from public program identifiers and seeds. Derivation alone grants no debit authority. Nested spending grants narrow the permitted source, destination, asset, and amount. Program monetary effects pass through the same transfer-set entry as other modules, and persisted namespace occupancy is metered and settled as replayable transfer legs.

## Receipts and replay

A `lxp_receipt` binds the activity identifier, global sequence, previous and resulting state roots, activity root, result code, effects, fee charged, batch identifier, and module and parameter versions. Program outcomes additionally bind terminal results, runtime and ABI versions, resource metering, transfer evidence, and call-graph commitments. Clients supply activities; the protocol produces receipt evidence.

Replay uses the ordered history and committed parameters to reproduce receipts and roots. A root or receipt disagreement refuses the replay. Portable `layerx-receipt-proof-v2` evidence authenticates a signed receipt against independently trusted receipt keys and inclusion facts. A signed receipt records execution; finality on Paxeer X follows when the containing block commits.

### Native integration surfaces

The native interface in `include/layerx/lxp_kernel.h` exposes `lxp_kernel_execute_activity` and `lxp_kernel_dispatch`. Prepared-batch APIs separate preparation, commit, and publication; batch-boundary and publication-digest interfaces bind the records being published. Protocol-3 batch maintenance runs escrow deadlines, budget rollover, service defaults, and occupancy finalization within the batch journal.

## Application workflow

1. Select the native precompile or signed-activity interface and the correct network, authority, sequence, fee limit, and validity bounds.
2. Sign the applicable transaction or activity and retain its exact bytes and retry identity.
3. Submit to the node and retain the transaction hash.
4. Verify receipt identity, result, fees, effects, and state-root binding.
5. Verify the containing block's commitment before releasing a resource that requires final execution.

## Continue reading

- [Sequencing, checkpoints, and finality](https://docs.paxeer.app/sequencing)

- [Programmable execution](https://docs.paxeer.app/programs)

- [Identity and delegated authority](https://docs.paxeer.app/identity)

- [Payments and accounting](https://docs.paxeer.app/payments)

- [Receipt verification](https://docs.paxeer.app/receipts)
