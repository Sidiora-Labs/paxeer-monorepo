import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/data-availability')

export default function DataAvailability() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Data Availability</h1><p className="page-description">Read block availability through the agent runtime and verify supplied batch archives with the proof library.</p></div>
    <h2>Availability makes replay possible</h2>
    <p>The proof library defines five batch availability classes. Given an authenticated availability root and provider chunks, a verifier authenticates the data before replay. This format describes supplied batch evidence; the agent runtime reads merged-node blocks through a separate availability operation.</p>
    <table><thead><tr><th>Class</th><th>Contents</th></tr></thead><tbody>
      <tr><td>Activities (1)</td><td>Canonical accepted activities</td></tr>
      <tr><td>Receipts (2)</td><td>Canonical activity receipts</td></tr>
      <tr><td>Oracle (3)</td><td>Canonical signed oracle inputs</td></tr>
      <tr><td>StateDiff (4)</td><td>Canonical state-diff material</td></tr>
      <tr><td>Recovery (5)</td><td>Canonical recovery metadata</td></tr>
    </tbody></table>
    <p>Chunks carry at most 65,536 bytes. Each chunk states its batch number, flattened bundle index, class, byte offset inside the class section, exact bytes and claimed digest. A chunk is accepted only when its metadata digest recomputes correctly and its Merkle inclusion path verifies under the data-availability root. Verified chunks must appear in strict flattened-index order.</p>
    <div className="source-note"><strong>Source:</strong> <code>services/agent/crates/layerx-proof/src/availability.rs</code> — <code>AvailabilityClass</code>, <code>Chunk</code>, <code>VerifiedChunk</code>.</div>
    <h2>Fetch and verify workflow</h2>
    <ol><li>Obtain the committed data-availability root from evidence you already trust.</li><li>Fetch chunks for the batch from a provider.</li><li>For each chunk, recompute its digest and verify its inclusion path against the root. Refuse chunks from another batch, out-of-order chunks and digest mismatches.</li><li>Require complete bundle coverage and contiguous class offsets before complete-bundle verification.</li><li>Replay activities and oracle inputs; compare state roots and the recomputed account diff.</li><li>Attest only after replay reaches the exact committed result.</li></ol>
    <h2>Agent availability API</h2>
    <p>The agent runtime exposes an <code>availability_fetch</code> operation. Its selector is a decimal block height. The request also carries a requested rung, <code>maximum_bytes</code>, <code>maximum_chunks</code> and a <code>deadline</code> in Unix seconds. Empty selectors, zero bounds and expired deadlines are refused. The implementation bounds transaction bytes and transaction count for the selected block.</p>
    <p>For an available block, the runtime returns a complete <code>transactions</code> class with transaction count and byte count, plus the height, transaction root and receipt root. A pruned block produces a partial report with a provider failure of <code>pruned</code>. Missing data does not represent an empty block.</p>
    <div className="source-note"><strong>Source:</strong> <code>services/agent/crates/layerx-agent-api/src/read.rs</code> — <code>AvailabilityRequest</code>, <code>AvailabilityReport</code>; <code>services/agent/crates/layerx-agentd/src/human_runtime.rs</code> — <code>availability_fetch</code>.</div>
    <h2>Block availability reads</h2>
    <p>The merged-node reader checks the requested height against the node head and fetches that block. It returns the block&apos;s transaction root, receipt root and transaction count, or a pruned result when the block is absent. The runtime returns this block report separately from the proof library&apos;s five-class batch format.</p>
    <div className="source-note"><strong>Source:</strong> <code>services/agent/crates/layerx-agentd/src/read/availability.rs</code>.</div>
    <h2>Refusals and recovery</h2>
    <p>Keep storage and proof failures as typed refusals. Chunk verification distinguishes batch mismatch, index mismatch, digest mismatch, inclusion failure, ordering failure and incomplete bundle coverage. Retry a retained selection or restore authenticated archive data; do not treat missing chunks as an empty batch or replace a verified root with an untrusted newer one.</p>
    <h2>Ethereum and Solana mirrors</h2>
    <p>Mirror archives extend independent access to batch history. <code>layerx-mirror-publisher</code> publishes archives to Ethereum or Solana, while <code>layerx-mirror-verify</code> checks a receipt obtained from an untrusted mirror archive without a hosted gateway.</p>
    <p>Verification validates the archive, binds its batch header to operator-configured trust, proves receipt inclusion and returns freshness. Archive provenance and freshness are separate from the receipt&apos;s signed outcome. Preserve the archive commitment, trusted header context, inclusion proof and checkpoint references for later audit.</p>
    <p>See <Link href="/proofs">Receipts &amp; Proofs</Link> for portable offline verification and native state witnesses, or <Link href="/finality">Finality &amp; Commitment</Link> for how data availability supports settlement.</p>
  </DocsLayout>
}
