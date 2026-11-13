import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/sequencing')

export default function Sequencing() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Sequencing &amp; Finality</h1>
        <p className="page-description">Follow an activity from deterministic ordering into a sealed, signed batch, and request the evidence your application needs.</p>
      </div>
      <p>Paxeer X combines the LayerX activity execution plane with Paxeer custody. Activities are ordered into signed batches and replicas preserve the sealed history for replay. A normal payment or agent action does not require a separate settlement-chain transaction.</p>
      <p>The layerx-merge upgrade (<code>v7.0</code>) retired the separate sequencer, checkpoint, and guarantor settlement plane. The native sequencer recovery, equivocation, loss, and handover interfaces no longer exist in the kernel, and checkpoint submission to the Paxeer anchor returns a module-retired error. Sections below describe the live ordering and batch-evidence path; see <Link href="/checkpoints">Checkpoints</Link> for the retired anchor surface.</p>
      <h2>Global ordering and signed batches</h2>
      <p>Admitted activities receive contiguous global sequence numbers. A batch begins immediately after the previous batch’s last sequence and extends the state-root chain. A sequence gap, a reversed range, or a mismatched previous root refuses the batch. Account sequences enforce each actor’s own ordering alongside the network-wide sequence.</p>
      <table><thead><tr><th>Header commitment</th><th>Fields</th></tr></thead><tbody>
        <tr><td>Protocol and network</td><td><code>protocol_version</code>, <code>network_id</code>, <code>epoch</code></td></tr>
        <tr><td>Ordered range</td><td><code>batch_number</code>, <code>first_sequence</code>, <code>last_sequence</code></td></tr>
        <tr><td>State transition</td><td><code>previous_state_root</code>, <code>resulting_state_root</code></td></tr>
        <tr><td>Execution evidence</td><td><code>activity_merkle_root</code>, <code>receipt_merkle_root</code>, <code>event_merkle_root</code></td></tr>
        <tr><td>Reconstruction inputs</td><td><code>data_availability_root</code>, <code>oracle_root</code></td></tr>
        <tr><td>Time and signer</td><td>Batch timestamp and <code>sequencer_id</code>; the native header stores time in <code>timestamp_ms</code>.</td></tr>
      </tbody></table>
      <p>The canonical header contains exactly these 15 fields, encodes to 354 bytes, and is signed by the key bound to the authorized sequencer identity. Batch time is the only time consulted during execution. Protocol-3 batches append an <code>LXP/batch-maintenance/v1</code> receipt and an <code>LXP/batch-maintenance-effects/v1</code> event leaf after activity leaves, including an empty effects frame when no maintenance module is enabled.</p>
      <h2>Evidence stages</h2>
      <table><thead><tr><th>Level</th><th>Evidence</th><th>Responsibility</th></tr></thead><tbody>
        <tr><td>L0 · Accepted</td><td>The ordered activity has a receipt chained to its state root.</td><td>Inclusion and ordering.</td></tr>
        <tr><td>L1 · Sealed</td><td>The signed batch fixes ordering and committed roots.</td><td>Replica preservation of sealed bytes.</td></tr>
        <tr><td>L2 · Distributed</td><td>The batch and availability bundle are retrievable for independent replay.</td><td>Availability of the material needed for replay.</td></tr>
      </tbody></table>
      <p>Earlier designs extended this ladder with bonded guarantor attestation (L3) and checkpoint registration on Paxeer (L4). That settlement plane is retired: a receipt’s existence does not imply Paxeer checkpoint settlement, and applications must not wait for or require anchor registration. Certificates and attestations in historical data used the distinct domains <code>LXP/v2/checkpoint-certificate</code> and <code>LXP/v2/guarantor-attestation</code>, each with its terminating zero byte.</p>
      <h2>Replay</h2>
      <ol><li>Retrieve the complete batch body: activities, receipts, events, oracle inputs, state-diff material, and recovery metadata.</li><li>Verify actor, grant, oracle, and sequencer signatures.</li><li>Replay from the previous state root using the committed protocol and parameters.</li><li>Recompute every committed root and compare the canonical results byte for byte.</li></ol>
      <p>Any disagreement stops acceptance of that replay. Deterministic execution excludes local clocks, floating point, and unstable iteration, so identical ordered input reproduces identical receipts and roots.</p>
      <h2>Public commitment levels</h2>
      <p>Submission uses the exact names <code>executed</code>, <code>batched</code>, and <code>finalised</code>. These are evidence requirements alongside the lifecycle stages.</p>
      <table><thead><tr><th>Requested level</th><th>Required evidence</th></tr></thead><tbody>
        <tr><td><code>executed</code></td><td>A verified canonical receipt for the submitted activity.</td></tr>
        <tr><td><code>batched</code></td><td>Execution evidence plus an authenticated receipt proof whose canonical value exactly matches the receipt.</td></tr>
        <tr><td><code>finalised</code></td><td>Batch evidence plus finalised checkpoint evidence whose canonical header exactly matches the proof’s signed batch header.</td></tr>
      </tbody></table>
      <p>The gateway never downgrades the requested assurance. If its bounded wait cannot establish the requested commitment, it returns an error rather than a weaker success. Preserve the original signed activity and recover through <code>lx_getActivityStatus</code>, <code>lx_getReceipt</code>, and the required proof reads. An acknowledgement, HTTP 202, or activity identifier alone is not an executed receipt. The accepted spelling is <code>finalised</code>.</p>
      <h2>Data availability and recovery</h2>
      <p>A state root without its reconstruction material is insufficient. Retrieval must re-hash to the committed availability roots. Missing data prevents independent replay from accepting the batch.</p>
      <p>After a crash, recovery reads the durable log head, loads a usable snapshot, replays committed records, compares roots, and rebuilds disposable projections.</p>
      <h3>Equivocation</h3>
      <p>Two different signed batches at one batch number, or two activities at one global sequence, are conflicting histories. The later conflicting batch is refused. There is no native sequencer equivocation, loss, recovery, or handover interface in the current kernel; those entry points were retired with the separate sequencer plane.</p>
      <h2>The Paxeer custody boundary</h2>
      <p>Economic rules for orders, service delivery, escrow, and ordinary transfers execute in LayerX. Custody operations on Paxeer consume proven facts from the executed history. Deposits credit against proven custody facts. Withdrawals debit LayerX first and pay on Paxeer at most once.</p>
      <p>If Paxeer is unreachable, ordinary LayerX execution continues. Paxeer payouts wait for settlement connectivity. Applications should choose their commitment requirement according to the value and reversibility of the action they release.</p>
      <h2>Continue reading</h2>
      <ul><li><Link href="/layerx-kernel">Deterministic kernel execution</Link></li><li><Link href="/checkpoints">Checkpoints (retired anchor)</Link></li><li><Link href="/data-availability">Data availability and reconstruction</Link></li><li><Link href="/bridge">Deposits, withdrawals, and exits</Link></li><li><Link href="/receipts">Receipt and proof verification</Link></li><li><Link href="/consensus">Paxeer consensus</Link></li></ul>
    </DocsLayout>
  )
}
