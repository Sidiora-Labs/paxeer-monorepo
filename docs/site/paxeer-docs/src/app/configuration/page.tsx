import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import Link from 'next/link'

export const metadata = metadataFor('/configuration')

export default function Configuration() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Configuration</h1>
        <p className="page-description">The current paxd configuration files, node modes, RPC settings, and storage controls.</p>
      </div>

      <DocArt name="configuration" />
      <h2>Configuration across the unified network</h2>
      <p>The chain configuration below controls <code>paxd</code>. Hosted service configuration is maintained separately under <code>services/platform/hosted/</code> and the component workspaces. Give each component the same intended network identity and the correct settlement binding while retaining separate service roles and credentials.</p>
      <table><thead><tr><th>Surface</th><th>Configure</th><th>Boundary</th></tr></thead><tbody>
        <tr><td>Core boundary</td><td>Network ID, node and replica connections, TLS material, and separate admin credentials</td><td>Core and admin listeners are separate planes</td></tr>
        <tr><td>Gateway</td><td>Public TLS, upstream dependencies, receipt verification, request policy, and shared coordination storage</td><td>Public application ingress</td></tr>
        <tr><td>Registry</td><td>Published service identities, network records, route destinations, and signing policy</td><td>Discovery records identify the authorized network services</td></tr>
        <tr><td>Guarantor</td><td>Sequencer identity and batch range, persistent replay state, attestation peers, bonded identity, and settlement submitter</td><td>Verification and attestation authority remain role-specific</td></tr>
        <tr><td>Agent and Human</td><td>Provider destinations, identity and approval policy, key-management channels, and receipt dependencies</td><td>User-facing authorization reaches protected provider boundaries</td></tr>
      </tbody></table>
      <p>The former <code>layerxd --check-config</code> validation path is retired with the <code>layerxd</code> daemon; validate <code>paxd</code> configuration by generating it with the binary you will run (below) and checking startup logs. Persistent data and runtime socket directories are distinct. Keep administrative listeners and signer channels off public ingress; route applications through the <Link href="/platform-api">gateway API</Link>. See <Link href="/network-parameters">on-chain parameters</Link> for governance-controlled economic settings.</p>

      <h2>Configuration Files</h2>
      <p><code>paxd init</code> generates configuration under <code>~/.paxeer/config/</code>. An explicit <code>--home</code> changes that location. Generate files using the binary revision you will run, then edit the settings you need.</p>
      <table><thead><tr><th>File</th><th>Purpose</th></tr></thead><tbody>
        <tr><td><code>config.toml</code></td><td>Consensus, P2P, mempool, consensus RPC, state-sync client, and validator signer</td></tr>
        <tr><td><code>app.toml</code></td><td>Application APIs, EVM RPC, execution, snapshots, and storage</td></tr>
        <tr><td><code>client.toml</code></td><td>CLI node, chain, output, and keyring defaults</td></tr>
        <tr><td><code>genesis.json</code></td><td>Initial chain identity, consensus parameters, and application state</td></tr>
      </tbody></table>
      <h2>Node Modes</h2>
      <pre><code>{`paxd init my-node --chain-id hyperpax_125-1 --mode full`}</code></pre>
      <table><thead><tr><th>Mode</th><th>Application profile</th><th>History</th></tr></thead><tbody>
        <tr><td><code>full</code></td><td>API, gRPC, EVM RPC, and KV transaction index</td><td>100,000 recent SS versions and consensus blocks by default</td></tr>
        <tr><td><code>archive</code></td><td>Full-node services; consensus mode is written as <code>full</code></td><td>SS and block retention set to zero, meaning keep available history</td></tr>
        <tr><td><code>validator</code></td><td>API, gRPC, EVM RPC, SS queries, and transaction indexing disabled by this profile</td><td>State needed to validate and execute</td></tr>
        <tr><td><code>seed</code></td><td>Discovery profile with a 1,000-connection setting; public application APIs disabled</td><td>Peer discovery role</td></tr>
      </tbody></table>
      <p>The default init mode is <code>full</code>. It binds consensus RPC and P2P to all interfaces. Validator and seed profiles retain the base loopback listeners; configure the intended peer topology explicitly.</p>
      <div className="source-note"><strong>Source:</strong> <code>daemon/paxd/cmd/init.go</code>; <code>node/params/config.go</code> — <code>SetTendermintConfigByMode</code>, <code>SetAppConfigByMode</code>, <code>SetEVMConfigByMode</code>.</div>

      <h2>Consensus and P2P</h2>
      <p>These example settings belong in <code>config.toml</code>. Use the current hyphenated keys and real peer IDs from your network.</p>
      <pre><code>{`log-level = "info"
log-format = "text"

[rpc]
laddr = "tcp://127.0.0.1:26657"

[p2p]
laddr = "tcp://0.0.0.0:26656"
bootstrap-peers = ""
persistent-peers = ""
private-peer-ids = ""
max-connections = 100

[mempool]
broadcast = true
size = 5000
max-txs-bytes = 1073741824
max-tx-bytes = 1048576`}</code></pre>
      <p><code>max-txs-bytes</code> limits total mempool bytes; <code>max-tx-bytes</code> limits one transaction. Use <code>bootstrap-peers</code> for initial discovery, <code>persistent-peers</code> for maintained connections, and <code>max-connections</code> for the combined peer limit.</p>
      <div className="source-note"><strong>Source:</strong> <code>consensus/config/config.go</code> and <code>consensus/config/toml.go</code>.</div>

      <h2>Application and EVM RPC</h2>
      <p>These settings belong in <code>app.toml</code>. EVM options use underscores, unlike the consensus options above. The host and port are separate EVM fields.</p>
      <pre><code>{`minimum-gas-prices = "0.01uhpx"
min-retain-blocks = 100000
occ-enabled = true

[api]
enable = true
swagger = true
address = "tcp://127.0.0.1:1317"

[grpc]
enable = true
address = "127.0.0.1:9090"

[evm]
http_enabled = true
http_address = "127.0.0.1"
http_port = 8545
ws_enabled = true
ws_port = 8546
simulation_gas_limit = 10000000`}</code></pre>
      <p>The example binds HTTP query services locally. Set <code>http_address</code> and <code>http_port</code> for the EVM HTTP listener, and <code>ws_port</code> for the WebSocket listener. Configure public routing to match your deployment.</p>
      <p>The default worker count is twice the detected CPU count, clamped to 10–128. Override <code>concurrency-workers</code> only after measuring your workload. The node admission setting <code>minimum-gas-prices</code> is separate from the EVM module&apos;s on-chain fee parameters.</p>
      <div className="source-note"><strong>Source:</strong> <code>rpc/config/config.go</code>, <code>rpc/server.go</code>, <code>sdk/server/config/config.go</code>, <code>daemon/paxd/cmd/root.go</code>.</div>

      <h2>Storage and Retention</h2>
      <p>Keep root-level settings before TOML section headers. State-store keys have the <code>ss-</code> prefix; state-commit keys have <code>sc-</code>.</p>
      <pre><code>{`# app.toml root setting
min-retain-blocks = 100000

[state-commit]
sc-enable = true
sc-write-mode = "memiavl_only"

[state-store]
ss-enable = true
ss-backend = "pebbledb"
ss-keep-recent = 100000

[receipt-store]
rs-backend = "pebbledb"`}</code></pre>
      <p>Archive mode sets both <code>min-retain-blocks</code> and <code>ss-keep-recent</code> to zero to preserve available history. Start from data covering the historical range you need. RocksDB is an optional SS backend that requires a compatible build; receipt storage uses Pebble.</p>
      <p>Storage routing modes control state migration. Select the mode specified by your chain&apos;s migration plan and existing data layout. See <Link href="/storage">Storage</Link>.</p>
      <div className="source-note"><strong>Source:</strong> <code>storage/config/toml.go</code>, <code>storage/config/sc_config.go</code>, <code>storage/config/ss_config.go</code>, <code>storage/config/receipt_config.go</code>.</div>

      <h2>Snapshots and State Sync</h2>
      <p>A snapshot provider sets <code>[state-sync]</code> in <code>app.toml</code>. A joining node enables <code>[statesync]</code> in <code>config.toml</code>. These are different roles.</p>
      <pre><code>{`# app.toml: application snapshot production
[state-sync]
snapshot-interval = 0
snapshot-keep-recent = 2

# config.toml: snapshot consumption
[statesync]
enable = false
rpc-servers = ""
trust-height = 0
trust-hash = ""`}</code></pre>
      <p>Snapshot interval zero disables periodic application snapshots. Before enabling a joining node, supply trusted RPC servers, a valid checkpoint height and hash, and the required trust period. See <Link href="/run-node">Run a Node</Link>.</p>
      <h2>Metrics</h2>
      <pre><code>{`# config.toml
[instrumentation]
prometheus = true
prometheus-listen-addr = "127.0.0.1:26660"`}</code></pre>
      <p>Read <code>/metrics</code> on the configured listener. Application telemetry is configured separately in <code>app.toml</code> under <code>[telemetry]</code>.</p>
      <h2>Validation</h2>
      <p>Review generated files using the selected binary&apos;s help and configuration templates. Start the configured node with <code>paxd start</code>, check its startup logs, then confirm its chain identity and synchronization with <code>paxd status</code>.</p>

      <div className="prev-next">
        <Link href="/run-node"><div className="prev-next-label">Previous</div><div className="prev-next-title">Run a Node</div></Link>
        <Link href="/operators"><div className="prev-next-label">Next</div><div className="prev-next-title">Operators Guide</div></Link>
      </div>
    </DocsLayout>
  )
}
