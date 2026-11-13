import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/finality')

export default function Finality() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Finality &amp; Commitment</h1><p className="page-description">Follow a transaction from submission through verified execution to BFT block commitment.</p></div>
    <h2>Transaction progress on one network</h2>
    <p>Paxeer is one BFT network. Its kernel participates in block production, and an execution receipt becomes final when its block is committed. EVM chain ID is <code>125</code>. The node implementation lives in <code>node/</code>; receipt progress is exposed by <code>rpc/sequencer_receipt.go</code>. See <Link href="/checkpoints">Commitment evidence</Link>.</p>
    <table><thead><tr><th>Stage</th><th>Name</th><th>Evidence and responsibility</th></tr></thead><tbody>
      <tr><td>Submission</td><td>Submitted</td><td>The node holds the transaction; no execution receipt is attached.</td></tr>
      <tr><td>Execution</td><td>Receipted</td><td>A signed receipt records the outcome before block commitment.</td></tr>
      <tr><td>Commitment</td><td>Committed</td><td>A committed block covers the receipt sequence.</td></tr>
    </tbody></table>
    <p>A submission acknowledgement establishes that the transaction was received. A signed receipt establishes its execution outcome. Committed block evidence establishes finality on the same network.</p>
    <h2>Receipt commitment rungs</h2>
    <p><code>pax_sequencerReceipt</code> reports three exact <code>rung</code> values. The hosted gateway relays this node method without promoting its evidence level.</p>
    <table><thead><tr><th>Rung</th><th>Required evidence</th><th>Application decision</th></tr></thead><tbody>
      <tr><td><code>submitted</code></td><td>Transaction hash and nonce without a receipt</td><td>Track the existing submission.</td></tr>
      <tr><td><code>receipted</code></td><td>Signed execution receipt and consistent decoded fields</td><td>Inspect success or failure and the recorded effects.</td></tr>
      <tr><td><code>committed</code></td><td>Receipt covered by a committed block</td><td>Treat the verified outcome as final.</td></tr>
    </tbody></table>
    <p>A submitted result contains <code>txHash</code> and <code>nonce</code>. Receipted results add signed receipt bytes and their decoded fields. Committed results also contain <code>blockNumber</code> and <code>blockHash</code> for the block covering the receipt sequence.</p>
    <h3>Use the exact wire vocabulary</h3>
    <p>The current names are <code>submitted</code>, <code>receipted</code>, and <code>committed</code>. In the Human API, their verification labels are <code>unverified</code>, <code>receipt-verified</code>, and <code>checkpoint-finalised</code>.</p>
    <h2>Recover an unknown outcome</h2>
    <p>If the outcome is unknown, track the existing transaction and obtain its evidence before taking another economic action:</p>
    <ol><li>Keep the signed transaction, hash, and nonce.</li><li>Read <code>pax_sequencerReceipt</code> for that transaction hash.</li><li>Verify the signed receipt against the governed receipt keys from <code>px_getReceiptKeys</code>.</li><li>Check the covering committed block before treating the outcome as final.</li></ol>
    <p>A new transaction can repeat the intended payment. Retain the original transaction hash and reconcile that transaction first.</p>
    <h2>Payment release policies</h2>
    <p>Release a paid resource only after checking the receipt against the expected payer, recipient, asset, amount, and execution result, together with the block commitment required by your application.</p>
    <p>A payment reference identifies an outcome; it does not establish the signer&apos;s authority or block inclusion. Verify the signed material and trusted receipt-key configuration as well as the response fields.</p>
    <h2>Replay and execution</h2>
    <p>The kernel executes canonical activities, program calls, program transfers, and storage occupancy accounting. Replay checks deterministic state changes and receipts against the same ordered history.</p>
    <h2>Verify receipt and block evidence</h2>
    <p><code>SequencerReceiptResult.Verify</code> checks the signed receipt against its decoded fields and output digests. Supply trusted receipt-key pins; a signing key returned beside a receipt is insufficient to establish authority. Committed results must identify the covering block.</p>
    <h2>Pending transactions</h2>
    <p>If a transaction remains submitted or receipted, retain that state until commitment evidence is available. A timeout does not establish failure or finality. See <Link href="/custody">Account and custody boundaries</Link>.</p>
    <p>Continue with <Link href="/proofs">Receipts &amp; Proofs</Link>, <Link href="/data-availability">Data Availability</Link> and <Link href="/consensus">Consensus</Link>.</p>
  </DocsLayout>
}
