import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import Link from 'next/link'

export const metadata = metadataFor('/checkpoints')

export default function Checkpoints() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Checkpoints</h1>
        <p className="page-description">The LayerX checkpoint anchor at 0x1014: its historical ABI, and its retirement at the layerx-merge upgrade.</p>
      </div>
      <DocArt name="state-snapshots" />
      <h2>Retired at layerx-merge (v7.0)</h2>
      <p>The checkpoint anchor is retired. Do not build new flows on checkpoint submission, finalisation, challenges, or guarantor attestation against it.</p>
      <ul>
        <li><code>SubmitCheckpoint</code>, <code>SubmitAvailabilityAttestation</code>, and <code>SubmitEquivocation</code> in the anchor keeper unconditionally return a module-retired error.</li>
        <li>Non-view selectors of the <code>0x1014</code> precompile were retired at the <code>v7.0</code> merge upgrade. Between that upgrade and the later <code>v7.0-anchor-store-delete</code> upgrade, view selectors return zero values; after it, all 21 selectors are retired.</li>
        <li>The ABI below is retained for reading historical data and verifying old evidence. A submitted or finalised status from this surface is not a live settlement step.</li>
      </ul>
      <p>For the live ordering and evidence path, see <Link href="/sequencing">Sequencing &amp; Finality</Link>. Custody operations are covered under <Link href="/custody">Custody</Link>.</p>
      <h2>What the header binds</h2>
      <p>A checkpoint committed an ordered range of LayerX activity and its resulting state. The registration carried header commitments; individual activity envelopes, payloads, and receipts remained in the replayable batch and its availability material.</p>
      <table><thead><tr><th>Commitment</th><th>Fields</th><th>What a consumer checks</th></tr></thead><tbody>
        <tr><td>Domain</td><td>Protocol version and network ID</td><td>The header belongs to the expected network and supported protocol.</td></tr>
        <tr><td>Ordering</td><td>Epoch, batch number, first sequence, last sequence</td><td>Authority term, consecutive batch order, and an uninterrupted activity range.</td></tr>
        <tr><td>State transition</td><td>Previous state root and resulting state root</td><td>The batch continues the accepted history and commits its resulting state.</td></tr>
        <tr><td>Record commitments</td><td>Activity, receipt, event, and oracle roots</td><td>The relevant ordered records reproduce their independent roots.</td></tr>
        <tr><td>Availability</td><td>Data-availability root</td><td>Metadata-bound ordered chunks reconstruct the complete availability bundle.</td></tr>
        <tr><td>Producer</td><td>Timestamp and sequencer ID</td><td>Header-relative freshness and an authorized sequencer signature.</td></tr>
      </tbody></table>
      <p>The native anchor accepted the canonical 354-byte batch header and its 64-byte Ed25519 sequencer signature. Obtain commitments from verified canonical header bytes; copying a root from an unverified JSON response loses the signature binding.</p>
      <h2>Historical flow</h2>
      <ol>
        <li>Activities were accepted, deterministic transitions executed, and receipts issued.</li>
        <li>The batch was sealed and the canonical header signed, fixing the activity order and committed roots.</li>
        <li>Guarantors downloaded the complete batch and availability bundle, verified signatures and authority, and independently replayed every transition.</li>
        <li>Guarantors recomputed all committed roots and attested matching results. A mismatch withheld the attestation.</li>
        <li>A submitter presented the header, signature, and guarantor certificate to the Paxeer anchor.</li>
        <li>The anchor finalised after threshold, continuity, availability, and challenge conditions cleared.</li>
      </ol>
      <table><thead><tr><th>Stage</th><th>Evidence</th><th>Established property</th></tr></thead><tbody>
        <tr><td>Accepted / L0</td><td>Execution receipt</td><td>Activity execution and ordering in the current history.</td></tr>
        <tr><td>Sealed / L1</td><td>Signed canonical header</td><td>Fixed commitments from an identified sequencer.</td></tr>
        <tr><td>Distributed / L2</td><td>Verifiable availability material</td><td>Batch bytes can be retrieved and reconstructed.</td></tr>
        <tr><td>Attested / L3 (retired)</td><td>Guarantor certificate</td><td>Bonded guarantors attested matching independent execution.</td></tr>
        <tr><td>Settled / L4 (retired)</td><td>Finalised Paxeer checkpoint</td><td>The checkpoint satisfied settlement admission and finality rules.</td></tr>
      </tbody></table>
      <h2>Guarantor certificates and availability (ABI reference)</h2>
      <p>A guarantor signature attested correctness and possession of the required data classes. The certificate carried the header, validity-proof field, ordered attestations, threshold, and settlement reference. The native anchor supported up to 32 attestations in ascending guarantor-identifier order. Threshold attestations were a bonded economic guarantee; they did not replace independent verification with a cryptographic validity proof.</p>
      <p>The availability commitment covered five classes: activities and oracle inputs, tagged receipts and events, canonical state differences, and recovery metadata. Chunks used the canonical size of 65,536 bytes and bound class, ordering, and metadata. Activity, receipt, event, and oracle roots remained independent commitments, so a matching availability root alone did not establish correct execution.</p>
      <h2>Read the Paxeer anchor (historical)</h2>
      <p>The checkpoint anchor is <code>0x0000000000000000000000000000000000001014</code>. Read methods return both identities and status so applications can bind proofs to the intended checkpoint. After the store-delete upgrade these selectors are retired; before it, views return zero values.</p>
      <pre><code>{`interface ICheckpointReads {
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
}`}</code></pre>
      <p>Status <code>0</code> meant unknown, <code>1</code> submitted, and <code>2</code> final. <code>checkpoint(batchNumber)</code> returned roots, sequence range, sequencer, signer count, availability mask, open challenges, and submitted/finalized heights. <code>checkpointGuarantors(batchNumber)</code> returned admitted guarantor identities in certificate order. Use the complete <code>LayerXAnchor.sol</code> ABI for bindings.</p>
      <h2>Challenges and bonded accountability (retired)</h2>
      <p><code>openChallenge(batchNumber, kind, evidenceHash)</code> required the challenge bond and distinguished fraud (<code>0</code>) from data availability (<code>1</code>). Open challenges prevented finalisation. <code>finalize(batchNumber)</code> advanced a submitted checkpoint only when its challenge and timing conditions permitted. The keeper&apos;s finalisable check covered status, the declared threshold, the bonded guarantor count, open challenges, the challenge window, and that the batch was exactly the latest finalised batch plus one.</p>
      <p>Conflicting 274-byte attestations by the same guarantor for different checkpoints in one batch supported <code>submitEquivocation(evidenceA, evidenceB)</code>. That submission path is retired along with the rest of the anchor write surface.</p>
      <h2>Consumer verification checklist (historical evidence)</h2>
      <ol>
        <li>Verify endpoint TLS trust, the expected LayerX network, and supported encoding versions.</li>
        <li>Decode the canonical header and verify its sequencer signature and authority range.</li>
        <li>Check checkpoint identity, sequential history, roots, and the registered certificate.</li>
        <li>Read the checkpoint&apos;s finality status and the finality flag accompanying the required root.</li>
        <li>Verify native Merkle paths with exact counts, indices, positional siblings, and account authority bindings.</li>
        <li>Apply the custody claim&apos;s debit, delay, limit, eligibility, and nullifier rules.</li>
        <li>Retain the verified proof and confirm payout events and consumed claim state.</li>
      </ol>
      <p>Public activity commitments use <code>executed</code>, <code>batched</code>, and <code>finalised</code>. Preserve these meanings in application interfaces. Process readiness, a wallet signature, or a historically submitted checkpoint does not justify presenting a claim as settled through the retired anchor.</p>
      <p>Continue with <Link href="/settlement">settlement evidence encodings</Link>, <Link href="/custody">custody and exit workflows</Link>, <Link href="/consensus">Paxeer consensus</Link>, and <Link href="/precompiles">native EVM interfaces</Link>.</p>
    </DocsLayout>
  )
}
