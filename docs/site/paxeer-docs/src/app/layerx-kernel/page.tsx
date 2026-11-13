import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/layerx-kernel')

export default function LayerXKernel() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">LayerX Kernel</h1>
        <p className="page-description">The deterministic C17 execution core of Paxeer X: one signed activity, one canonical history, and one reproducible result.</p>
      </div>
      <p>The LayerX kernel executes native activity across payments, escrow, budgets, streams, services, trading, governance, assets, bridges, and programs. It resolves authority, applies state transitions, produces receipts, and binds each result to the next state root. It runs inside <code>paxd</code> through <code>modules/lxphost/</code>. Native operations execute in the node&apos;s transaction context on the same chain as EVM operations; a receipt is final when its block commits.</p>
      <h2>Three execution invariants</h2>
      <table><thead><tr><th>Invariant</th><th>Meaning for applications</th></tr></thead><tbody>
        <tr><td>One canonical history</td><td>The node orders transactions and binds native activity results to global sequences and state roots. Block commitment establishes finality; query indexes derive from committed execution.</td></tr>
        <tr><td>One financial doorway</td><td>Modules do not write balances directly. Economic modules and programs propose validated transfer sets, and the only module-to-ledger entry is <code>lxp_kernel_apply_transfer_set</code>, preserving a common accounting boundary.</td></tr>
        <tr><td>One reproducible result</td><td>Independent replay produces identical receipts and roots from the same ordered input, parameter versions, and batch time.</td></tr>
      </tbody></table>
      <h2>Canonical activities</h2>
      <p>The kernel represents an operation with an <code>lxp_activity</code> envelope. Native precompiles resolve the EVM caller&apos;s authority and construct the kernel execution context. The activity codec uses fixed-width big-endian integers, explicit fields, and no floating point or maps. A valid decode must re-encode to the same bytes; extra trailing bytes are rejected.</p>
      <table><thead><tr><th>Field</th><th>Purpose</th></tr></thead><tbody>
        <tr><td><code>protocol_version</code>, <code>network_id</code></td><td>Select the enabled protocol rules and prevent cross-network replay. Version 2 includes occupancy, and version 3 includes state commitment. Use the version configured by the node.</td></tr>
        <tr><td><code>activity_type</code></td><td>The high 16 bits identify the module; the low 16 identify the operation ordinal.</td></tr>
        <tr><td><code>actor_did</code>, <code>authority</code></td><td>Identify the principal and primary key, session, or capability grant authorizing the action.</td></tr>
        <tr><td><code>account_sequence</code></td><td>Must exactly match the actor’s next sequence; gaps and stale sequences are refused.</td></tr>
        <tr><td><code>timestamp_bound</code></td><td>Bounds validity using the sealed batch timestamp.</td></tr>
        <tr><td><code>idempotency_key</code></td><td>Binds the retry identity of the activity. Retain the exact signed bytes and key for recovery; do not assume re-executing a duplicate returns the original receipt — recover the outcome through receipt and status reads.</td></tr>
        <tr><td><code>fee_limit</code></td><td>Caps the deterministic execution fee the actor authorizes.</td></tr>
        <tr><td><code>payload</code>, <code>payload_hash</code>, <code>signature</code></td><td>Bind the operation body to its canonical signing preimage. Direct signed activities use Ed25519; native precompiles supply authority from the authenticated transaction caller.</td></tr>
      </tbody></table>
      <h2>From admission to state transition</h2>
      <ol>
        <li>Decode and check the envelope, including protocol version, network, payload binding, and module enablement.</li>
        <li>Resolve the actor and authority, validate the applicable signature or native caller authorization, and enforce the account sequence.</li>
        <li>Evaluate timestamp bounds against batch time.</li>
        <li>Compute the fee against the active schedule and the activity’s fee limit.</li>
        <li>Open the state journal; dispatch module validation and execution.</li>
        <li>Commit successful module effects or roll them back on execution refusal.</li>
        <li>Apply required fee and sequence bookkeeping and produce a receipt chained to the resulting state root.</li>
      </ol>
      <h3>Admission refusal and execution failure</h3>
      <p>Malformed envelopes, wrong-network requests, and payload-hash mismatches fail admission without consuming the kernel account sequence or charging a kernel execution fee. Ordered terminal refusals have a separate receipt path. An admitted execution can roll back module effects while retaining sequence and fee bookkeeping under the active fee policy. EVM gas accounting is separate. Inspect the receipt’s result code and committed state before treating an action as successful.</p>
      <h2>Identity is part of execution</h2>
      <p>Agent DIDs identify principals whose account IDs are derived by the protocol account codec. The kernel enforces primary and session keys, scoped grants, rotation, recovery, revocation, and expiry. Delegation can narrow authority without widening it. See <Link href="/identity">Identity and delegated authority</Link> for the identity lifecycle.</p>
      <h2>Determinism across machines</h2>
      <p>Consensus execution uses C17 and deterministic protocol types. It excludes floating point, local wall-clock reads, unstable database iteration, and pointer-derived hashing. Time-dependent rules—including grant expiry, escrow deadlines, stream accrual, funding intervals, and governance activation—read the committed batch timestamp. Oracle observations arrive as signed ordered activities; execution never fetches an external HTTP response.</p>
      <h3>Programs share the accounting boundary</h3>
      <p>Module <code>0x09</code> executes deterministic WASM programs under the invoking activity’s authority. Program-owned accounts derive from public program identifiers and seeds. Derivation alone grants no debit authority. Nested spending grants narrow the permitted source, destination, asset, and amount. Program monetary effects pass through the same transfer-set entry as other modules, and persisted namespace occupancy is metered and settled as replayable transfer legs.</p>
      <h2>Receipts and replay</h2>
      <p>A <code>lxp_receipt</code> binds the activity identifier, global sequence, previous and resulting state roots, activity root, result code, effects, fee charged, batch identifier, and module and parameter versions. Program outcomes additionally bind terminal results, runtime and ABI versions, resource metering, transfer evidence, and call-graph commitments. Clients supply activities; the protocol produces receipt evidence.</p>
      <p>Replay uses the ordered history and committed parameters to reproduce receipts and roots. A root or receipt disagreement refuses the replay. Portable <code>layerx-receipt-proof-v2</code> evidence authenticates a signed receipt against independently trusted receipt keys and inclusion facts. A signed receipt records execution; finality on Paxeer X follows when the containing block commits.</p>
      <h3>Native integration surfaces</h3>
      <p>The native interface in <code>include/layerx/lxp_kernel.h</code> exposes <code>lxp_kernel_execute_activity</code> and <code>lxp_kernel_dispatch</code>. Prepared-batch APIs separate preparation, commit, and publication; batch-boundary and publication-digest interfaces bind the records being published. Protocol-3 batch maintenance runs escrow deadlines, budget rollover, service defaults, and occupancy finalization within the batch journal.</p>
      <h2>Application workflow</h2>
      <ol><li>Select the native precompile or signed-activity interface and the correct network, authority, sequence, fee limit, and validity bounds.</li><li>Sign the applicable transaction or activity and retain its exact bytes and retry identity.</li><li>Submit to the node and retain the transaction hash.</li><li>Verify receipt identity, result, fees, effects, and state-root binding.</li><li>Verify the containing block&apos;s commitment before releasing a resource that requires final execution.</li></ol>
      <h2>Continue reading</h2>
      <ul><li><Link href="/sequencing">Sequencing, checkpoints, and finality</Link></li><li><Link href="/programs">Programmable execution</Link></li><li><Link href="/identity">Identity and delegated authority</Link></li><li><Link href="/payments">Payments and accounting</Link></li><li><Link href="/receipts">Receipt verification</Link></li></ul>
    </DocsLayout>
  )
}
