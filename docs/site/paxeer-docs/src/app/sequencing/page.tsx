import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/sequencing')

export default function Sequencing() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Sequencing &amp; Finality</h1>
        <p className="page-description">Follow a signed transaction through ordered execution, receipt verification, and BFT block commitment.</p>
      </div>
      <p>Paxeer mainnet-beta is one BFT chain. Its kernel participates in block execution and proposal, and the committed chain records the resulting order. A signed receipt can be available before its covering block is committed.</p>
      <p>The proposer places <code>MsgSequenceBatch</code> at transaction index zero of each block at or above the sequencing activation height. Validators verify that batch and re-execute its transactions. See <Link href="/checkpoints">block commitment</Link> for the evidence boundary.</p>
      <h2>Global ordering and sequence batches</h2>
      <p>A nonempty batch starts at the stored last sequence plus one and binds the preceding sequence hash. The keeper rejects a discontinuity, an invalid receipt-key signature, a mismatched height, or a second sequence batch in one block. EVM account nonces separately enforce sender ordering.</p>
      <table><thead><tr><th>Batch commitment</th><th>Fields</th></tr></thead><tbody>
        <tr><td>Block height</td><td><code>Height</code>, <code>FromSeq</code>, <code>ToSeq</code></td></tr>
        <tr><td>Sequence continuity</td><td><code>PrevSeqHash</code>, <code>LastSeqHash</code>, <code>TxRoot</code></td></tr>
        <tr><td>Receipt-key authority</td><td><code>KeyId</code>, <code>Sig</code></td></tr>
        <tr><td>Hash widths</td><td><code>PrevSeqHash</code>, <code>LastSeqHash</code>, <code>TxRoot</code> are each 32 bytes.</td></tr>
        <tr><td>Key and signature widths</td><td><code>KeyId</code> is 32 bytes; <code>Sig</code> is a 64-byte Ed25519 signature.</td></tr>
        <tr><td>Canonical encoding</td><td>The domain prefix is followed by big-endian positions, hashes, key ID, and signature; <code>Height</code> names the covering block.</td></tr>
      </tbody></table>
      <p>The canonical sequence-batch transaction is 241 bytes, including its domain prefix and signature. For an empty batch, <code>ToSeq</code> equals <code>FromSeq</code> minus one, the sequence hash is unchanged, and the transaction root is SHA-256 of the empty input.</p>
      <h2>Evidence stages</h2>
      <table><thead><tr><th>Level</th><th>Evidence</th><th>Responsibility</th></tr></thead><tbody>
        <tr><td>Submitted</td><td>The node holds the transaction without a signed receipt.</td><td>Track its transaction hash.</td></tr>
        <tr><td>Receipted</td><td>A signed receipt binds the raw transaction to its execution outcome.</td><td>Authenticate the receipt key and check the outcome.</td></tr>
        <tr><td>Committed</td><td>A committed block covers the receipt sequence.</td><td>Verify block inclusion and the consensus commit.</td></tr>
      </tbody></table>
      <p>A <code>submitted</code> result carries no execution receipt. A <code>receipted</code> result authenticates execution when verified under a trusted receipt key. Finality is established when the covering block is committed by BFT consensus.</p>
      <h2>Replay</h2>
      <ol><li>Retrieve the exact signed transaction and its canonical execution receipt.</li><li>Authenticate the receipt signature using a trusted key with a height range covering the receipt.</li><li>Check the sequence-batch signature, transaction order, and sequence-hash continuity.</li><li>Authenticate the covering block and its consensus commit before accepting committed inclusion.</li></ol>
      <p>A failed replay or sequence check rejects the proposal. Consensus execution uses the ordered transaction input and block context so validators can reproduce the transition before committing it.</p>
      <h2>Public receipt states</h2>
      <p>The node receipt API reports <code>submitted</code>, <code>receipted</code>, and <code>committed</code>. These values describe transaction progress; applications verify the evidence behind the reported value.</p>
      <table><thead><tr><th>Receipt state</th><th>Required evidence</th></tr></thead><tbody>
        <tr><td><code>submitted</code></td><td>Transaction hash, nonce, and progress state.</td></tr>
        <tr><td><code>receipted</code></td><td>Canonical signed receipt, authenticated signing key, and matching execution view.</td></tr>
        <tr><td><code>committed</code></td><td>Receipt evidence plus the covering block number and block hash; independent finality verification authenticates inclusion and the consensus commit.</td></tr>
      </tbody></table>
      <p>Retrieve transaction progress with <code>pax_sequencerReceipt</code>. Preserve the original signed transaction and obtain <code>px_getSequenceBatch</code> and <code>px_getCommit</code> evidence for independent committed inclusion checks. A transaction hash or successful HTTP response alone is not a receipt. The final rung is <code>committed</code>.</p>
      <h2>Data availability and recovery</h2>
      <p>Committed inclusion verification requires the exact transaction, receipt, covering sequence batch, block inclusion path, consensus commit, and a trusted validator set. A root without authenticated reconstruction and inclusion evidence is insufficient.</p>
      <p>Node recovery restores committed sequence state from the module store. The proposer executor also keeps a write-ahead log of accepted transactions and sealed blocks so recovery can reconcile uncommitted execution with the committed head.</p>
      <h3>Proposal validation</h3>
      <p>The signed sequence batch binds its height, ordered range, transaction root, and sequence hashes. Validators check the proposer against the governed leader schedule and reject inconsistent sequence or replay results.</p>
      <h2>Execution in one network</h2>
      <p>Transfers, program calls, service operations, and market operations execute within the same network. Applications inspect the operation result and affected account state in addition to its transaction position.</p>
      <p>If consensus cannot commit a block, an available execution receipt remains evidence of signed execution. Clients retain that receipt and wait for authenticated committed inclusion before treating its effects as final.</p>
      <h2>Continue reading</h2>
      <ul><li><Link href="/layerx-kernel">Deterministic kernel execution</Link></li><li><Link href="/checkpoints">Block commitment</Link></li><li><Link href="/data-availability">Data availability and reconstruction</Link></li><li><Link href="/bridge">Deposits, withdrawals, and exits</Link></li><li><Link href="/receipts">Receipt and proof verification</Link></li><li><Link href="/consensus">Paxeer consensus</Link></li></ul>
    </DocsLayout>
  )
}
