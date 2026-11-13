import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/settlement')

export default function Settlement() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Checkpoint settlement &amp; evidence</h1>
        <p className="page-description">Follow LayerX activities from ordered receipts to bonded replay, finalized Paxeer checkpoints, and verifiable custody claims.</p>
      </div>
      <h2>Execution and settlement form one history</h2>
      <p>Paxeer X settles LayerX activity on EVM chain ID <code>125</code>. The sequencer orders activities; guarantors independently replay sealed batches; Paxeer records the resulting checkpoints and enforces custody releases. Payments, program calls, program-owned transfers, and storage occupancy charges participate in the same deterministic history.</p>
      <table><thead><tr><th>Level</th><th>Evidence</th><th>Meaning</th></tr></thead><tbody>
        <tr><td>L0 · Accepted</td><td>Sequencer receipt</td><td>The activity is ordered in the current batch and bound to a state transition.</td></tr>
        <tr><td>L1 · Sealed</td><td>Fixed batch history</td><td>Ordering is fixed and the batch is ready for distribution.</td></tr>
        <tr><td>L2 · Distributed</td><td>Batch and availability bundle</td><td>Guarantors receive the material needed for independent replay.</td></tr>
        <tr><td>L3 · Attested</td><td>Bonded guarantor certificate</td><td>Guarantors attest byte-identical replay results.</td></tr>
        <tr><td>L4 · Settled</td><td>Paxeer checkpoint</td><td>Settlement custody enforces finality, challenge windows, withdrawal limits, and exits.</td></tr>
      </tbody></table>
      <p>A guarantor attestation is bonded re-execution evidence. Its security combines bonds, challenges, availability checks, and release limits. Applications retain the receipt for the fast path and follow the same activity into settlement evidence before presenting settled funds.</p>
      <h2>Native checkpoint anchor</h2>
      <p>The <code>layerxanchor</code> precompile at <code>0x0000000000000000000000000000000000001014</code> records checkpoints, finality, availability attestations, and guarantor bonds. Anyone can submit a self-verifying checkpoint certificate. Finalization requires the configured threshold of active bonded guarantors, continuity with the finalized chain, no open challenge, and completion of the challenge window.</p>
      <table><thead><tr><th>Method</th><th>Purpose</th></tr></thead><tbody>
        <tr><td><code>submitCheckpoint(header, headerSignature, certificate)</code></td><td>Admit a canonical 354-byte header, 64-byte sequencer signature, and guarantor certificate.</td></tr>
        <tr><td><code>submitAvailabilityAttestation(attestation)</code></td><td>Record a 274-byte attestation for a known checkpoint.</td></tr>
        <tr><td><code>finalize(batchNumber)</code></td><td>Finalize a checkpoint after its challenge conditions clear.</td></tr>
        <tr><td><code>latestFinalized()</code>, <code>checkpoint(batchNumber)</code></td><td>Read finalized progress and full checkpoint metadata.</td></tr>
        <tr><td><code>finalizedStateRoot(batchNumber)</code>, <code>finalizedReceiptRoot(batchNumber)</code></td><td>Retrieve roots together with their finality flags.</td></tr>
        <tr><td><code>openChallenge(batchNumber, kind, evidenceHash)</code></td><td>Open a bonded fraud or data-availability challenge.</td></tr>
        <tr><td><code>submitEquivocation(evidenceA, evidenceB)</code></td><td>Present conflicting guarantor attestations for slashing.</td></tr>
      </tbody></table>
      <p>Anchor status is <code>0</code> unknown, <code>1</code> submitted, or <code>2</code> final. Read the finality flag with a root; the existence of a root alone does not establish payout eligibility. The exact interface is <code>precompiles/layerxanchor/LayerXAnchor.sol</code>.</p>
      <h2>Deposit-root registration</h2>
      <p>The custody precompile (<code>ILayerXCustody</code>) records the deposit root of a finalized checkpoint with <code>registerDepositRoot(registration, signature, leafOrdering)</code>. Only the account that submitted the checkpoint to the anchor may call it. No settlement publication contract exposing an <code>EVIDENCE_VERSION</code> or withdrawal/balance witness vectors exists in the current contracts; do not build against one.</p>
      <table><thead><tr><th>Item</th><th>Contents</th><th>Commitment</th></tr></thead><tbody>
        <tr><td>Deposit root registration</td><td>Registration bytes, deposit-root authority signature, and positional leaf ordering. The ordering is committed to, not verified by the precompile.</td><td><code>SHA256(abi.encode(uint16(2), registration, signature, leafOrdering))</code>, read back with <code>depositRegistrationDigest(checkpointId)</code></td></tr>
      </tbody></table>
      <p>Registration records evidence; payout still requires the custody checks on the claim path. Use <code>depositRootRegistered(checkpointId)</code> and <code>depositRootAuthority()</code> to inspect registration state.</p>
      <h3>Independent signing roles</h3>
      <p>A separately configured Ed25519 deposit-root authority signs the registration. Guarantors attest replay. A guarantor key cannot substitute for the deposit-root authority.</p>
      <pre><code>{`Deposit-root registration:
"LX:PAXEER:DEPOSIT:ROOT:v1" || checkpoint_id32
|| checkpoint_state_root32 || deposit_root32 || custody_reference32
|| network_id:u32 (4 bytes) || protocol_version:u16 (2 bytes)`}</code></pre>
      <p>The <code>signature</code> argument is the deposit-root authority&apos;s Ed25519 signature over those registration bytes. An Ed25519 public key is not converted into an EVM address.</p>
      <h2>Producer and consumer workflow</h2>
      <ol>
        <li>Replay native history independently and construct the deposit evidence for the checkpoint.</li>
        <li>Obtain the configured deposit-root authority&apos;s signature over the registration bytes.</li>
        <li>Submit and finalize the checkpoint at the anchor, then call <code>registerDepositRoot</code> from the checkpoint submitter account.</li>
        <li>Verify <code>depositRootRegistered</code> and <code>depositRegistrationDigest</code> before treating the deposit root as recorded.</li>
        <li>Apply claim-specific custody checks and verify the final payout on the custody path.</li>
      </ol>
      <h2>Verify the transport and the money</h2>
      <p>Settlement clients verify endpoint TLS trust and chain identity before consuming evidence. A healthy process or matching chain ID establishes connectivity, while finalized checkpoint evidence establishes the settlement claim. Do not elevate one finality level merely because an earlier stage succeeded.</p>
      <p>Continue with <Link href="/custody">deposits, withdrawals, and emergency exits</Link>, <Link href="/consensus">Paxeer consensus</Link>, and <Link href="/precompiles">native EVM interfaces</Link>.</p>
    </DocsLayout>
  )
}
