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
        <p className="page-description">Block commitment, trusted state-sync checkpoints, and the disabled checkpoint anchor interface.</p>
      </div>
      <DocArt name="state-snapshots" />
      <h2>Block commitment</h2>
      <p>Paxeer mainnet-beta is one BFT chain with EVM chain ID 125. Its kernel proposes blocks within the network. A receipt is final when its block is committed; a separate checkpoint submission is not part of application settlement.</p>
      <ul>
        <li><code>SubmitCheckpoint</code>, <code>SubmitAvailabilityAttestation</code>, and <code>SubmitEquivocation</code> in modules/layerxanchor/keeper/ unconditionally return a module-retired error.</li>
        <li>The <code>0x1014</code> precompile disables writes when <code>v7.0</code> is active and disables every selector when <code>v7.0-anchor-store-delete</code> is active.</li>
        <li>The ABI below declares the checkpoint read signatures. These calls do not provide a settlement path on the merged network.</li>
      </ul>
      <p>For the live ordering and evidence path, see <Link href="/sequencing">Sequencing &amp; Finality</Link>. Custody operations are covered under <Link href="/custody">Custody</Link>.</p>
      <h2>What a block header binds</h2>
      <p>Consensus headers identify the chain and block, link the block history, and commit transaction, validator and application data. The header and commit types are defined in consensus/types/block.go.</p>
      <table><thead><tr><th>Commitment</th><th>Fields</th><th>What a consumer checks</th></tr></thead><tbody>
        <tr><td>Domain</td><td>Chain ID and block version</td><td>The header belongs to the expected chain and block format.</td></tr>
        <tr><td>Ordering</td><td>Height and previous block ID</td><td>The height and previous-block reference continue the trusted chain.</td></tr>
        <tr><td>State transition</td><td>Previous-height application hash and result hash</td><td>The commitment covers the state after the preceding height; match the proof height accordingly.</td></tr>
        <tr><td>Record commitments</td><td>Transaction data hash and evidence hash</td><td>The block data and evidence match their header commitments.</td></tr>
        <tr><td>Validator authority</td><td>Validator and next-validator hashes</td><td>The validator sets agree with the trusted consensus history.</td></tr>
        <tr><td>Consensus</td><td>Timestamp, proposer and last commit</td><td>Header time and validator signatures satisfy the consensus verification rules.</td></tr>
      </tbody></table>
      <p>Obtain commitments from verified block headers and validator commits. A root copied from an unverified response is not a trust anchor. The light client in consensus/light/ verifies headers against a trusted validator set and trust period.</p>
      <h2>Transaction and block flow</h2>
      <ol>
        <li>A signed transaction is submitted to the node and checked for mempool admission.</li>
        <li>The kernel proposes a block within the Paxeer consensus network.</li>
        <li>Validators evaluate the proposal under the configured consensus protocol.</li>
        <li>The application executes the ordered transactions and records their results.</li>
        <li>Consensus commits the block and the application commits its state.</li>
        <li>Applications read the transaction receipt and check its status and committed block identity.</li>
      </ol>
      <table><thead><tr><th>Stage</th><th>Evidence</th><th>Established property</th></tr></thead><tbody>
        <tr><td>Submitted</td><td>Transaction hash</td><td>The transaction has a reconciliation identity.</td></tr>
        <tr><td>Proposed</td><td>Proposed block</td><td>The transaction has a proposed position in the block.</td></tr>
        <tr><td>Executed</td><td>Application result</td><td>The transaction has an execution outcome.</td></tr>
        <tr><td>Committed</td><td>Consensus commit</td><td>The block and its receipts are final.</td></tr>
        <tr><td>Successful</td><td>Successful transaction receipt</td><td>The committed transaction executed successfully.</td></tr>
      </tbody></table>
      <h2>Validator signatures and state sync</h2>
      <p>The light client verifies headers using trusted validator information. Non-adjacent verification checks trusted-set overlap and requires more than two-thirds of the new validator voting power to sign the new header. Trust-period and clock-drift checks also apply.</p>
      <p>State sync uses an independently trusted block height and hash with application snapshots. Snapshot production is configured in app.toml under [state-sync]; snapshot consumption is configured in config.toml under [statesync]. Configure the joining node with trusted RPC servers and a valid trust period.</p>
      <h2>Checkpoint read ABI</h2>
      <p>The checkpoint anchor address is <code>0x0000000000000000000000000000000000001014</code>. These signatures are declared in precompiles/layerxanchor/LayerXAnchor.sol. With the store-delete upgrade active, every selector returns a disabled-module error.</p>
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
      <p>The ABI declares status values <code>0</code>, <code>1</code>, and <code>2</code> for unknown, submitted and final. The declarations <code>checkpoint(batchNumber)</code> and <code>checkpointGuarantors(batchNumber)</code> describe checkpoint records. They do not supply live block finality. The interface file is <code>precompiles/layerxanchor/LayerXAnchor.sol</code>.</p>
      <h2>Checkpoint write availability</h2>
      <p>The anchor ABI declares <code>openChallenge(batchNumber, kind, evidenceHash)</code> with kind <code>0</code> for fraud and <code>1</code> for availability, and declares <code>finalize(batchNumber)</code>. These write selectors are disabled on the merged network; application receipt finality comes from the BFT block commit.</p>
      <p>The anchor declaration <code>submitEquivocation(evidenceA, evidenceB)</code> is also disabled. Validator equivocation is handled by the consensus evidence and slashing modules.</p>
      <h2>Consumer verification checklist</h2>
      <ol>
        <li>Verify endpoint TLS trust and the expected Paxeer chain identity.</li>
        <li>Verify the block header and validator commit against an independent trust anchor.</li>
        <li>Check the transaction hash, block identity and committed height.</li>
        <li>Read the transaction receipt and check whether execution succeeded.</li>
        <li>Verify state proofs against the application commitment for the requested height.</li>
        <li>Check application authorization, asset units and replay protection for the operation.</li>
        <li>Retain verified evidence and reconcile the resulting events and balances.</li>
      </ol>
      <p>For EVM transactions, read <code>eth_getTransactionReceipt</code> and inspect <code>status</code> and <code>blockHash</code>. A submission hash identifies a transaction; successful execution in a committed block establishes the application result.</p>
      <p>Continue with <Link href="/settlement">settlement evidence encodings</Link>, <Link href="/custody">native account funding</Link>, <Link href="/consensus">Paxeer consensus</Link>, and <Link href="/precompiles">native EVM interfaces</Link>.</p>
    </DocsLayout>
  )
}
