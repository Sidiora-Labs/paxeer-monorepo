import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/finality')

export default function Finality() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Finality &amp; Commitment</h1><p className="page-description">Follow an activity from execution into a sealed batch, and request the evidence your application needs.</p></div>
    <h2>One history, increasing assurance</h2>
    <p>Paxeer X combines the fast LayerX activity path with Paxeer custody. Activities are ordered into sealed batches and replicas preserve the history for replay. The layerx-merge upgrade (<code>v7.0</code>) retired the separate guarantor and checkpoint settlement plane: checkpoint submission to the Paxeer anchor returns a module-retired error, so applications must not require anchor registration as a live step. See <Link href="/checkpoints">Checkpoints</Link>.</p>
    <table><thead><tr><th>Stage</th><th>Name</th><th>Evidence and responsibility</th></tr></thead><tbody>
      <tr><td>L0</td><td>Accepted</td><td>The activity is ordered into the current batch and issued a state-root-bound receipt.</td></tr>
      <tr><td>L1</td><td>Sealed</td><td>Batch ordering is fixed by the signed canonical header. Replicas retain sealed bytes.</td></tr>
      <tr><td>L2</td><td>Distributed</td><td>The sealed batch and availability bundle are retrievable for independent replay.</td></tr>
    </tbody></table>
    <p>Earlier designs added bonded guarantor attestation and Paxeer checkpoint registration as further stages. That plane is retired. A receipt proves execution at its declared evidence level; it does not by itself prove settlement. Before using evidence to release value, apply the configured settlement and withdrawal rules for the relevant network.</p>
    <h2>Public commitment levels</h2>
    <p>Activity submission and payment middleware expose three exact commitment names. These are evidence requirements rather than interchangeable progress labels. They do not replace the lifecycle stages.</p>
    <table><thead><tr><th>Commitment</th><th>Required evidence</th><th>Application decision</th></tr></thead><tbody>
      <tr><td><code>executed</code></td><td>A verified canonical receipt for the submitted activity</td><td>Inspect the authenticated outcome and its payment or program effects.</td></tr>
      <tr><td><code>batched</code></td><td>Executed evidence plus an authenticated receipt proof naming the same activity and carrying byte-identical canonical receipt bytes</td><td>Require signed-batch inclusion before releasing a resource.</td></tr>
      <tr><td><code>finalised</code></td><td>Batched evidence plus finalised checkpoint evidence whose canonical header exactly equals the proof&apos;s signed batch header</td><td>Require the checkpoint evidence bound to that exact batch.</td></tr>
    </tbody></table>
    <p><code>batch_evidence</code> carries the authenticated receipt proof and <code>checkpoint_evidence</code> carries the checkpoint. A different checkpoint or a newer head cannot substitute for the exact required header. Admission acknowledgements, queue entries, activity IDs and HTTP 202 responses do not establish execution.</p>
    <h3>Use the exact wire vocabulary</h3>
    <p>The spelling is <code>finalised</code>, alongside <code>executed</code> and <code>batched</code>. Do not substitute other spellings or progress labels.</p>
    <h2>Recover a pending commitment</h2>
    <p>When a bounded wait cannot establish the requested evidence, the gateway returns an error rather than silently returning a weaker success. Recover the existing outcome instead of submitting a duplicate:</p>
    <ol><li>Keep the original signed activity and activity ID.</li><li>Read <code>lx_getActivityStatus</code> and <code>lx_getReceipt</code> to recover the existing outcome.</li><li>Retrieve the required authenticated receipt and batch evidence.</li><li>Verify exact activity, canonical receipt and signed-header bindings before satisfying your release policy.</li></ol>
    <p>Creating another activity to escape an unknown result can duplicate the economic action.</p>
    <h2>Payment release policies</h2>
    <p>An x402 offer&apos;s LayerX terms under <code>extra.layerx</code> carry the payer <code>account</code> reference and <code>currency</code> code; the offer model has no LayerX commitment field. Choose the evidence your release policy requires at the application layer and verify it before releasing the resource.</p>
    <p>A successful settlement reference is <code>lxp:&lt;receipt_digest&gt;</code>. Validate the receipt, payment facts, configured authority and commitment evidence together. The reference or an HTTP success is insufficient on its own.</p>
    <h2>Replay covers the whole economic system</h2>
    <p>Replay covers payments, program calls, program-owned transfers and storage occupancy settlement from the same ordered history. Program monetary effects are explicit transfer sets applied through the kernel&apos;s single module-to-ledger entry, while occupancy charges remain replay-checkable batch evidence. Deterministic execution excludes local clocks, floating point and unstable iteration from consensus decisions.</p>
    <h2>Checkpoint finalisation checks (retired submission)</h2>
    <p>Checkpoint submission is retired and unconditionally returns a module-retired error. For historical checkpoints, the anchor keeper&apos;s finalisable check covers checkpoint status, the declared threshold, the bonded guarantor count, open challenges, the challenge window, and that the batch is exactly the latest finalised batch plus one. Do not build new flows on checkpoint registration.</p>
    <h2>Liveness and exits</h2>
    <p>If the ordinary activity path stalls, the last accepted history remains the replay anchor. Custody release on Paxeer remains governed by its applicable evidence, timing, limit, and eligibility rules; see <Link href="/custody">Custody</Link>.</p>
    <p>Continue with <Link href="/proofs">Receipts &amp; Proofs</Link>, <Link href="/data-availability">Data Availability</Link> and <Link href="/consensus">Consensus</Link>.</p>
  </DocsLayout>
}
