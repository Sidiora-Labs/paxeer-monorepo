import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/run-node')

export default function RunNode() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Run a Node</h1>
        <p className="page-description">Initialize a Paxeer X node, choose its role, connect to a network, and verify synchronization.</p>
      </div>

      <h2>Chain node and hosted services</h2>
      <p>The chain node is <code>paxd</code>. Hosted services (gateway, core, authority, registry, Human and related components) live under <code>services/platform/hosted/</code> in the monorepo, with the hosted node under <code>services/platform/hosted/node/</code>; each has its own configuration, credentials, and state. This page covers the <code>paxd</code> chain node.</p>
      <p>The former <code>layerxd</code> C daemon (<code>cmd/layerxd</code>) is retired: its sources are absent from the build and retirement tests assert that absence. There is no <code>layerxd --check-config</code>, <code>--serve</code>, or <code>--authority-replica</code> startup path. Order startup so dependencies exist first, then verify synchronization and an end-to-end receipt before admitting traffic. See <Link href="/operators">operators</Link>, <Link href="/configuration">configuration</Link>, <Link href="/relay-archive">relay/archive status</Link>, and <Link href="/layerx-kernel">kernel architecture</Link>.</p>

      <h2>Before You Start</h2>
      <p>Install a <code>paxd</code> revision compatible with the network you intend to join. Obtain its genesis file, peer information, and any snapshot or state-sync checkpoint from that network&apos;s operator. Keep EVM RPC and consensus RPC endpoints separate.</p>
      <p>Size disk, memory, and CPU for your retained history and query workload. See <Link href="/installation">Installation</Link> for build requirements and <Link href="/configuration">Configuration</Link> for node profiles.</p>
      <h2>Node Types</h2>
      <ul>
        <li><strong>Full:</strong> Serves application and EVM queries and retains recent history by default.</li>
        <li><strong>Archive:</strong> Serves full-node APIs and retains available historical state and blocks.</li>
        <li><strong>Validator:</strong> Runs the signing node profile; active-set membership also requires an on-chain validator and sufficient stake.</li>
        <li><strong>Seed:</strong> Uses the peer-discovery profile.</li>
      </ul>
      <h2>Initialize Node</h2>
      <pre><code>{`paxd init my-node --chain-id hyperpax_125-1 --mode full`}</code></pre>
      <p>The default home is <code>~/.paxeer</code>. Init writes configuration and a local genesis. To join an existing network, install that network&apos;s verified genesis before starting. Validator operators should select <code>--mode validator</code> before configuring the signer and peer topology.</p>
      <h2>Genesis and Peers</h2>
      <p>Place the verified network genesis at <code>~/.paxeer/config/genesis.json</code>. Confirm its chain ID and expected checksum. Configure <code>bootstrap-peers</code> and <code>persistent-peers</code> under <code>[p2p]</code> in <code>config.toml</code> using the network&apos;s <code>node-id@host:26656</code> entries.</p>
      <p><Link href="/admin-hpx">HPX tooling</Link> provides distribution manifests, genesis, and peer registry handling for managed node setup.</p>
      <h2>Sync Methods</h2>
      <h3>Block Replay</h3>
      <p>A node with compatible genesis and an available block history can replay blocks. Use the correct binaries and upgrades for that history.</p>
      <h3>State Sync</h3>
      <p>State sync imports an application snapshot at a recent height. In <code>config.toml</code>, the <code>[statesync]</code> section accepts <code>rpc-servers</code>, <code>trust-height</code>, <code>trust-hash</code>, and <code>trust-period</code>. Use independent trusted RPC sources and a checkpoint within the trust period.</p>
      <p>Inspect a chosen RPC checkpoint before configuring it:</p>
      <pre><code>{`TM_RPC_URL=http://127.0.0.1:26657
curl -sS "$TM_RPC_URL/status" | jq '.result.sync_info'
curl -sS "$TM_RPC_URL/block?height=100000" | jq '.result.block.header, .result.block_id.hash'`}</code></pre>
      <p>Replace the example height with your trusted checkpoint height. JSON-RPC data is inside <code>result</code>. Check the response for a matching height and valid block hash before entering the checkpoint in your configuration.</p>
      <h3>Snapshot Restore</h3>
      <p>For a filesystem snapshot, stop the node first and verify the archive&apos;s layout, checksum, revision, and storage backend. Preserve the validator signing key and its last-sign state. Restore to the provider&apos;s documented directories, including any SC or SS paths configured outside the default home.</p>
      <p>Choose a snapshot covering the historical range your application needs. Archive retention preserves that imported history and subsequent state.</p>
      <h2>Start Node</h2>
      <pre><code>{`paxd start --home "$HOME/.paxeer"`}</code></pre>
      <p>For a service, use an explicit binary path, operating-system user, and home that match the initialized node. Create the user and directories shown in the example before starting the service.</p>
      <pre><code>{`[Unit]
Description=Paxeer Node
After=network-online.target

[Service]
Type=simple
User=paxeer
ExecStart=/usr/local/bin/paxd start --home /var/lib/paxeer
Restart=on-failure
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target`}</code></pre>
      <h2>Verify Synchronization</h2>
      <pre><code>{`paxd status --node tcp://127.0.0.1:26657
curl -sS http://127.0.0.1:26657/status | jq '.result.sync_info'
curl -sS http://127.0.0.1:26657/net_info | jq '.result.n_peers'
journalctl -u paxd -f`}</code></pre>
      <p>Confirm the expected chain identity, advancing block height, and completed synchronization before directing application traffic to the node.</p>
      <h2>Validator Setup</h2>
      <p>After synchronization, create or select the operator account and fund it with the intended stake and fees. The consensus signing key is separate from that account. Use <Link href="/operators">Operators Guide</Link> for registration and last-sign-state handling.</p>
      <h2>Local Docker Development</h2>
      <p>From the monorepo root (the repository is flattened; there is no <code>Paxeer-X-Network/</code> subdirectory):</p>
      <pre><code>{`make docker-cluster-start`}</code></pre>
      <p>See <Link href="/docker">Docker</Link> for the local cluster and monitoring targets.</p>
      <div className="source-note"><strong>Source:</strong> <code>daemon/paxd/cmd/init.go</code>, <code>node/params/config.go</code>, <code>consensus/config/config.go</code>, <code>consensus/rpc/jsonrpc/types/types.go</code>, <code>consensus/privval/file.go</code>.</div>

      <div className="prev-next">
        <Link href="/installation"><div className="prev-next-label">Previous</div><div className="prev-next-title">Installation & Build</div></Link>
        <Link href="/configuration"><div className="prev-next-label">Next</div><div className="prev-next-title">Configuration</div></Link>
      </div>
    </DocsLayout>
  )
}
