import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/settlement')

export default function Settlement() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Settlement &amp; committed evidence</h1>
        <p className="page-description">Verify signed execution, transaction inclusion, and committed block evidence on the Paxeer BFT chain.</p>
      </div>
      <h2>Execution and settlement form one history</h2>
      <p>Paxeer mainnet-beta has EVM chain ID <code>125</code>. Its kernel executes within the network, and receipts are final once their covering block is committed. Payments, program calls, transfers, and storage charges participate in the chain execution history.</p>
      <table><thead><tr><th>Level</th><th>Evidence</th><th>Meaning</th></tr></thead><tbody>
        <tr><td>Submission</td><td>Transaction hash</td><td>The node has accepted a transaction for processing.</td></tr>
        <tr><td>Signed execution</td><td>Canonical execution receipt</td><td>The receipt binds the transaction to its execution outcome.</td></tr>
        <tr><td>Outcome verification</td><td>Receipt and execution outputs</td><td>The authenticated result matches the intended payment or program operation.</td></tr>
        <tr><td>Block coverage</td><td>Sequence batch and transaction inclusion</td><td>The transaction is included in the identified block and ordered sequence.</td></tr>
        <tr><td>Commitment</td><td>BFT consensus commit</td><td>The covering block is committed under the trusted validator set.</td></tr>
      </tbody></table>
      <p>A signed receipt authenticates an execution claim under a receipt key. To establish finality independently, authenticate the covering block, the transaction and sequence-batch inclusion evidence, and the BFT commit under a trusted validator set.</p>
      <h2>Node receipt and consensus reads</h2>
      <p>The node provides <code>pax_sequencerReceipt</code> for execution progress and <code>px_getCommit</code> for consensus evidence. The receipt response binds the transaction hash to its signed receipt and, when committed, identifies the covering block.</p>
      <table><thead><tr><th>Method</th><th>Purpose</th></tr></thead><tbody>
        <tr><td><code>eth_sendRawTransaction(raw)</code></td><td>Submit a signed EVM transaction and obtain its hash.</td></tr>
        <tr><td><code>pax_sequencerReceipt(txHash)</code></td><td>Read submitted, receipted, or committed progress and execution evidence.</td></tr>
        <tr><td><code>eth_getTransactionReceipt(txHash)</code></td><td>Read the EVM execution receipt.</td></tr>
        <tr><td><code>px_getCommit(height)</code>, <code>px_getValidators(height)</code></td><td>Acquire the signed consensus header and validator information.</td></tr>
        <tr><td><code>eth_getBlockByNumber(height, false)</code>, <code>eth_getTransactionByHash(txHash)</code></td><td>Acquire block and transaction data for reconciliation.</td></tr>
        <tr><td><code>px_getSequenceBatch(height)</code></td><td>Acquire the canonical sequence-batch transaction and its block inclusion proof.</td></tr>
        <tr><td><code>px_getReceiptKeys()</code></td><td>Acquire governed receipt-key identifiers, public keys, and valid height ranges.</td></tr>
      </tbody></table>
      <p>The receipt rungs are <code>submitted</code>, <code>receipted</code>, and <code>committed</code>. Verify the evidence behind the reported rung. The response contract is implemented in <code>rpc/sequencer_receipt.go</code>.</p>
      <h2>Sequence-batch coverage</h2>
      <p>Each sequenced block begins with <code>MsgSequenceBatch</code>. Retrieve its canonical transaction, activity identifiers, and block inclusion proof with <code>px_getSequenceBatch(height)</code>. Obtain the corresponding consensus header and commit with <code>px_getCommit(height)</code>.</p>
      <table><thead><tr><th>Item</th><th>Contents</th><th>Commitment</th></tr></thead><tbody>
        <tr><td>Ordered activity commitment</td><td>The batch binds fixed-width activity identifiers in their execution order.</td><td><code>SHA256(activity_id32 || ...)</code>, stored in the batch TxRoot field.</td></tr>
      </tbody></table>
      <p>The batch commits to the ordered activity identifiers and sequence hashes. Use <code>px_getValidators(height)</code> to acquire validator data and <code>px_getReceiptKeys</code> for receipt-key data, authenticating both against independently trusted chain state.</p>
      <h3>Receipt and consensus signing roles</h3>
      <p>Receipt-signing keys authenticate execution receipts and sequence batches within their governed height ranges. Consensus validator keys authenticate block commits. A receipt signature alone does not establish the validator quorum or committed inclusion.</p>
      <pre><code>{`Sequence-batch transaction: 241 bytes
"PAXEER/v1/sequence-batch" || 0x00 || Height:u64be || FromSeq:u64be || ToSeq:u64be
|| PrevSeqHash[32] || LastSeqHash[32] || TxRoot[32]
|| KeyId[32] || Sig[64]`}</code></pre>
      <p>The <code>Sig</code> field is an Ed25519 signature over the chain-bound sequence-batch digest. Verify it with the pinned receipt key valid at the batch height, then verify the batch transaction against the committed block.</p>
      <h2>Committed inclusion workflow</h2>
      <ol>
        <li>Retain the signed transaction and verify its hash against the receipt.</li>
        <li>Authenticate the receipt key, height range, signature, and execution outcome.</li>
        <li>Acquire the covering sequence batch and verify its signature, sequence hashes, and ordered transaction identifiers.</li>
        <li>Authenticate the covering block and consensus commit using a trusted validator set.</li>
        <li>Check the expected account changes or payment effect before completing the application action.</li>
      </ol>
      <h2>Verify the transport and the money</h2>
      <p>Clients check TLS trust and chain identity before consuming evidence. A healthy endpoint or matching chain ID establishes connectivity. Finality requires authenticated committed inclusion and a verified execution outcome for the intended transaction.</p>
      <p>Continue with <Link href="/custody">account and custody interfaces</Link>, <Link href="/consensus">Paxeer consensus</Link>, and <Link href="/precompiles">native EVM interfaces</Link>.</p>
    </DocsLayout>
  )
}
