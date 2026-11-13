import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/data-availability')

export default function DataAvailability() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Data Availability</h1><p className="page-description">Retrieve authenticated batch data, verify it against the committed availability root, and preserve independent evidence through archives and mirrors.</p></div>
    <h2>Availability makes replay possible</h2>
    <p>Paxeer X commits batch data in five availability classes. A verifier fetches these bytes in chunks, authenticates every chunk against the committed data-availability root, and can then independently replay the transition. An availability proof establishes committed data; replay establishes the computed result.</p>
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
    <ol><li>Obtain the committed data-availability root from evidence you already trust.</li><li>Fetch chunks for the batch from a provider.</li><li>For each chunk, recompute its digest and verify its inclusion path against the root. Refuse chunks from another batch, out-of-order chunks and digest mismatches.</li><li>Require complete coverage of the classes your decision depends on.</li><li>Replay activities and oracle inputs; compare state roots and the recomputed account diff.</li><li>Attest only after replay reaches the exact committed result.</li></ol>
    <h2>Agent availability API</h2>
    <p>The agent runtime exposes an <code>availability_fetch</code> operation. A request carries a selector string, a requested rung, and finite retrieval bounds: <code>maximum_bytes</code>, <code>maximum_chunks</code> and a <code>deadline</code>. An empty selector or a zero bound is refused.</p>
    <p>The report states whether retrieval completed from one provider or is partial. It carries a per-class report (completeness, verified chunk and byte counts, and any failure) and a per-provider report. Treat a partial report as missing data, not as an empty batch.</p>
    <div className="source-note"><strong>Source:</strong> <code>services/agent/crates/layerx-agent-api/src/read.rs</code> — <code>AvailabilityRequest</code>, <code>AvailabilityReport</code>; <code>services/agent/crates/layerx-agentd/src/human_runtime.rs</code> — <code>availability_fetch</code>.</div>
    <h2>Retired kernel bundle API</h2>
    <p>The earlier kernel bundle symbols (<code>lxp_batch_availability_root</code> and the <code>lxp_da_*</code> family) are retired. The chain test suite asserts these symbols are absent from the kernel archive. Do not build integrations against them; use the chunk verification and agent API surfaces above.</p>
    <div className="source-note"><strong>Source:</strong> <code>modules/lxphost/retired_da_bundle_test.go</code>.</div>
    <h2>Refusals and recovery</h2>
    <p>Keep storage and proof failures as typed refusals. Chunk verification distinguishes batch mismatch, index mismatch, digest mismatch, inclusion failure and ordering failure. Retry a retained selection or restore authenticated archive data; do not treat missing chunks as an empty batch or replace a verified root with an untrusted newer one.</p>
    <h2>Ethereum and Solana mirrors</h2>
    <p>Mirror archives extend independent access to batch history. <code>layerx-mirror-publisher</code> publishes archives to Ethereum or Solana, while <code>layerx-mirror-verify</code> checks a receipt obtained from an untrusted mirror archive without a hosted gateway.</p>
    <p>Verification validates the archive, binds its batch header to operator-configured trust, proves receipt inclusion and returns freshness. Archive provenance and freshness are separate from the receipt&apos;s signed outcome. Preserve the archive commitment, trusted header context, inclusion proof and checkpoint references for later audit.</p>
    <p>See <Link href="/proofs">Receipts &amp; Proofs</Link> for portable offline verification and native state witnesses, or <Link href="/finality">Finality &amp; Commitment</Link> for how data availability supports settlement.</p>
  </DocsLayout>
}
