import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/relay-archive')

export default function RelayArchive() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Chain History &amp; Archives</h1><p className="page-description">Read retained chain history from full or archive nodes and verify exported receipt evidence.</p></div>
    <h2>Full and archive nodes</h2>
    <p>The chain node is <code>paxd</code>. Initialize with <code>--mode full</code> for recent query history or <code>--mode archive</code> to retain all available state history and blocks. The archive profile retains state history and blocks by setting its retention counts to zero; the generated application configuration includes <code>ss-keep-recent</code> under <code>[state-store]</code> and <code>min-retain-blocks</code>. Retention applies to the history the node has synchronized or restored.</p>
    <p>Choose retention for the queries your application needs. A recent read can succeed while a request for pruned state fails. A snapshot supplies only the history included in that snapshot.</p>
    <h2>Read chain history through node RPC</h2>
    <p>Use <code>eth_getBlockByNumber</code>, <code>eth_getTransactionReceipt</code> and bounded <code>eth_getLogs</code> queries at the node’s root JSON-RPC endpoint. Keep one transaction namespace throughout an indexer and retain block coordinates with each result. Use <code>pax_sequencerReceipt</code> to distinguish a signed execution result from one covered by a committed block.</p>
    <h2>Verify receipt archives</h2>
    <p>The mirror tooling under <code>services/interop/crates/layerx-mirror/</code> reads Ethereum or Solana archives under configured source trust. It checks archive commitments, committed block coordinates and receipt signatures. Preserve receipt-key pins and source provenance with the archive, and use its reported freshness when deciding whether the evidence is current.</p>
    <h2>Continue reading</h2>
    <p>For chain history and state, run a <code>paxd</code> full or archive node (see <Link href="/run-node">Run a Node</Link> and <Link href="/configuration">Configuration</Link>). For receipts and proofs, see <Link href="/proofs">Receipts &amp; Proofs</Link> and <Link href="/data-availability">Data Availability</Link>. For external mirrors, see <Link href="/mirrors">External Mirrors</Link>.</p>
    <div className="source-note"><strong>Source:</strong> <code>node/params/config.go</code>, <code>rpc/block.go</code>, <code>rpc/tx.go</code>, <code>rpc/filter.go</code>, <code>rpc/sequencer_receipt.go</code> and <code>services/interop/crates/layerx-mirror/src/verify.rs</code>.</div>
  </DocsLayout>
}
