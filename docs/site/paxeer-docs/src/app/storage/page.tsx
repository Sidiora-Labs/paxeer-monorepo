import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/storage')

export default function Storage() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Storage</h1>
        <p className="page-description">State commitment, historical queries, receipt retention, and backend configuration in Paxeer.</p>
      </div>

      <h2>Overview</h2>
      <p>Paxeer storage separates active state commitment from historical state queries and receipt storage. Each has its own configuration and retention behavior. The implementation is under <code>storage/</code> at the repository root.</p>
      <h2>State Commitment</h2>
      <p>The state-commit layer under <code>storage/state_db/sc/</code> includes MemIAVL, FlatKV, and a composite store that routes state operations. It participates in application commitment, snapshot import/export, and supported proof queries.</p>
      <p>The default write mode is <code>memiavl_only</code>. Other named modes support staged migration to FlatKV. Select routing modes according to the existing data layout and the chain&apos;s migration plan.</p>
      <pre><code>{`# app.toml
[state-commit]
sc-enable = true
sc-write-mode = "memiavl_only"`}</code></pre>
      <div className="source-note"><strong>Source:</strong> <code>storage/config/sc_config.go</code>, <code>storage/config/write_mode.go</code>, <code>storage/state_db/sc/composite/store.go</code>.</div>
      <h2>Historical State Store</h2>
      <p>The state-store layer maintains versioned key/value data for historical queries. Configure imported data and retention for the historical range you need. Commitment proofs also require the corresponding commitment-store history.</p>
      <pre><code>{`# app.toml
[state-store]
ss-enable = true
ss-backend = "pebbledb"
ss-keep-recent = 100000`}</code></pre>
      <p>The default backend is Pebble and the default retention is 100,000 versions. <code>ss-keep-recent = 0</code> retains all available versions. The config also supports a separate EVM historical store through <code>evm-ss-split</code>; its data routing and missing-key behavior differ from the unsplit store.</p>
      <div className="source-note"><strong>Source:</strong> <code>storage/config/ss_config.go</code>, <code>storage/config/toml.go</code>, <code>storage/state_db/ss/</code>.</div>
      <h2>RocksDB Backend</h2>
      <p>The optional RocksDB backend uses user-defined timestamps and column families for versioned state. Build RocksDB and install the node from the repository root:</p>
      <pre><code>{`make build-rocksdb
make install-rocksdb`}</code></pre>
      <p>After installing the compatible binary, set <code>ss-backend = "rocksdb"</code> under <code>[state-store]</code>. When changing backends, import or restore data in the target backend&apos;s format.</p>
      <p>Compare backends with the same revision, hardware, retained history, and request workload. Measure historical-query latency, iteration time, disk usage, and write throughput for the operations your application uses.</p>
      <div className="source-note"><strong>Source:</strong> <code>chain.mk</code> — <code>build-rocksdb</code>, <code>install-rocksdb</code> (the root <code>Makefile</code> delegates to <code>chain.mk</code> through <code>PAXEER_MAKE</code>); <code>storage/db_engine/rocksdb/mvcc/opts.go</code>.</div>
      <h2>Receipts and Blocks</h2>
      <pre><code>{`# app.toml: root setting must precede section headers
min-retain-blocks = 100000

[receipt-store]
rs-backend = "pebbledb"`}</code></pre>
      <p>The receipt configuration accepts <code>pebbledb</code> or its <code>pebble</code> alias. Set receipt retention with the root <code>min-retain-blocks</code> setting and historical state retention with <code>ss-keep-recent</code>.</p>
      <p><code>--mode archive</code> configures block and SS retention to zero, preserving imported history and subsequent state. Start with a dataset covering the historical range you plan to serve.</p>
      <div className="source-note"><strong>Source:</strong> <code>storage/config/receipt_config.go</code>, <code>node/params/config.go</code> — <code>setArchiveTypeAppConfig</code>.</div>
      <h2>Snapshots and Recovery</h2>
      <p>Application snapshots, commitment-store snapshots, and filesystem backups are distinct. Use the mechanism and directory layout matching your build. Stop a node before replacing its data, preserve validator last-sign state, and keep a compatible copy of the configuration and genesis.</p>
      <p>Changelogs and write-ahead logs support storage replay and recovery. Follow the recovery procedure for the selected store, and preserve consensus signer state alongside the validator key.</p>
      <div className="source-note"><strong>Source:</strong> <code>storage/wal/</code>, <code>storage/state_db/sc/composite/</code>, <code>consensus/privval/file.go</code>.</div>
      <h2>Inspection Tools</h2>
      <p>The PaxDB command provides storage inspection and benchmark operations. Build or run it from the repository root, using its help to select the correct database path and height:</p>
      <pre><code>{`go run ./storage/tools/cmd/paxdb --help
go run ./storage/tools/cmd/paxdb dump-iavl --help
go run ./storage/tools/cmd/paxdb dump-db --help`}</code></pre>
      <p>Beyond inspection, <code>paxdb</code> provides <code>dump-flatkv</code> and <code>import-flatkv-from-memiavl</code> for FlatKV export and migration, <code>migrate-evm-status</code> for EVM status migration, <code>prune</code> for version pruning, and <code>replay-changelog</code> for changelog replay. Run these operations through <code>paxdb</code>. Use an offline copy when an operation needs exclusive database access.</p>
      <div className="source-note"><strong>Source:</strong> <code>storage/tools/cmd/paxdb/main.go</code>, <code>storage/tools/cmd/paxdb/operations/</code>.</div>
      <h2>Next Steps</h2>
      <ul><li><Link href="/configuration">Configure node storage</Link></li><li><Link href="/operators">Operate and recover a node</Link></li><li><Link href="/consensus">Understand block commitment</Link></li></ul>

      <div className="prev-next">
        <Link href="/evm"><div className="prev-next-label">Previous</div><div className="prev-next-title">EVM</div></Link>
        <Link href="/modules"><div className="prev-next-label">Next</div><div className="prev-next-title">Modules Overview</div></Link>
      </div>
    </DocsLayout>
  )
}
