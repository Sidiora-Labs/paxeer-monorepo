import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/operators')

export default function Operators() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Operators Guide</h1>
        <p className="page-description">Manage Paxeer X node services, validator accounts, monitoring, and recovery.</p>
      </div>

      <p>For stalled heights, peer failures, or state mismatches, start with <Link href="/node-troubleshooting">Troubleshoot a Node</Link>. Record the binary revision and failing height before changing state.</p>
      <h2>Operate the node and application services</h2>
      <p>Paxeer mainnet-beta is one BFT chain with EVM chain ID 125. Kernel execution is embedded in <code>paxd</code> and participates in block proposal. A receipt is final when its block is committed. Hosted gateways and user services support applications around the chain. Assign each deployed service its own health checks, storage policy, and recovery procedure.</p>
      <table><thead><tr><th>Role</th><th>Responsibility</th><th>Operational evidence</th></tr></thead><tbody>
        <tr><td>Chain node and validator</td><td>Execute EVM and Cosmos transactions; enforce active exchange, bridge, launchpad, ledger, and XWeb rules</td><td>Chain identity, synchronization, peers, application hash, upgrade version, and RPC health</td></tr>
        <tr><td>Embedded kernel</td><td>Execute ordered activities as part of chain block processing</td><td>Committed blocks, execution results, state roots, and receipts</td></tr>
        <tr><td>Receipt authority</td><td>Provide verified receipt material to hosted clients</td><td>Authorized receipt keys, transaction binding, and receipt digests</td></tr>
        <tr><td>Consensus validator set</td><td>Vote on blocks and establish BFT commits</td><td>Validator membership, voting power, commit signatures, and block identity</td></tr>
        <tr><td>Gateway and registry</td><td>Verify public receipts, route requests, and publish discovery records</td><td>Dependency readiness, receipt verification, registry identity, and routing health</td></tr>
        <tr><td>Agent and Human services</td><td>Serve identity, delegated execution, owner approval, security, movement, and key-management boundaries</td><td>Authorization outcomes, signed activity and receipt continuity, provider readiness</td></tr>
        <tr><td>Historical queries</td><td>Serve retained node state; see <Link href="/relay-archive">history access</Link></td><td>Earliest available height and archive retention settings</td></tr>
      </tbody></table>
      <p>Applications access node JSON-RPC or the hosted gateway methods appropriate to their integration. Core administration, signing APIs, and provider internals have separate authentication boundaries. Consensus and kernel execution remain within the node.</p>
      <p>See <Link href="/layerx-kernel">kernel execution</Link>, <Link href="/settlement">settlement</Link>, <Link href="/platform-api">public APIs</Link>, and <Link href="/configuration">configuration</Link>.</p>

      <h2>Choose the Node Profile</h2>
      <p>Full nodes serve queries and retain recent history. Archive nodes preserve available historical state. Validators use a signing profile with public application APIs disabled, and seed nodes serve peer discovery. Hardware sizing depends on traffic, retained history, and the selected stores.</p>
      <p>Use <Link href="/configuration">Configuration</Link> to choose a node profile and its enabled services. Validator participation also depends on on-chain registration and the active set.</p>
      <h2>HPX Node Management</h2>
      <p>HPX manages a native <code>paxd</code> systemd service and verifies installation artifacts against the selected distribution mirror’s checksum manifest. The mirror distributes installation material.</p>
      <p>From the monorepo root, inspect and run the checked-out bootstrap script:</p>
      <pre><code>{`less tools/hpx/get-hpx.sh
sudo bash tools/hpx/get-hpx.sh
# If setup was skipped during bootstrap:
sudo env HPX_TYPE=fullnode hpx setup`}</code></pre>
      <p>The bootstrap script offers the setup wizard after installation. Select <code>HPX_TYPE=validator</code> instead of <code>fullnode</code> for validator setup. Run the HPX management commands below from a root shell. The selected artifact set includes the binary, architecture-specific Wasm runtimes, genesis, and configuration. Use the selected mirror&apos;s manifest to confirm the release and artifact checksums.</p>
      <pre><code>{`hpx                 # interactive menu
hpx status
hpx info
hpx logs
hpx start
hpx stop
hpx restart
hpx peers show
hpx statesync
hpx validator keygen
hpx validator stake
hpx validator status
hpx help`}</code></pre>
      <p><code>hpx start|stop|restart</code> control the systemd service. <code>hpx validator keygen</code> generates this node&apos;s operator account, <code>hpx validator stake</code> submits create-validator from the funded operator account, and <code>hpx validator status</code> reports validator state. <code>hpx peers refresh</code> changes the peer configuration (a peers-refresh systemd timer is also installed); <code>hpx register</code> announces the node to the registry. Use <code>hpx update</code> as a planned version change after checking network compatibility.</p>
      <div className="source-note"><strong>Source:</strong> <code>tools/hpx/get-hpx.sh</code>, <code>tools/hpx/hpx</code>, <code>tools/hpx/README.md</code>.</div>
      <h2>Local Development Cluster</h2>
      <p>From the monorepo root:</p>
      <pre><code>{`DOCKER_DETACH=true make -f chain.mk docker-cluster-start
# Or start the cluster with monitoring:
DOCKER_DETACH=true make -f chain.mk docker-cluster-start-monitoring`}</code></pre>
      <p>The targets in <code>chain.mk</code> stop the corresponding local cluster and replace generated configuration before startup. See <Link href="/docker">Docker</Link> for compose services and generated paths.</p>
      <h2>Validator Accounts and Keys</h2>
      <p>The operator account submits staking and governance transactions. The consensus key signs blocks. Use an existing operator account, or create one with the following commands:</p>
      <pre><code>{`paxd keys add operator
paxd keys show operator -a
paxd keys show operator --bech val -a
paxd tendermint show-validator`}</code></pre>
      <p>A <code>pax1…</code> account address, <code>paxvaloper1…</code> validator address, and consensus public key are distinct arguments. Use the form required by each command.</p>
      <h3>Register a Validator</h3>
      <p>After the node is synchronized and the operator is funded, register it with <code>create-validator</code>. This example uses one PAX in base units. Choose your stake, moniker, and commission settings, taking the network&apos;s active validator set into account.</p>
      <pre><code>{`paxd tx staking create-validator \\
  --amount=1000000uhpx \\
  --pubkey="$(paxd tendermint show-validator)" \\
  --moniker="My Validator" \\
  --chain-id=hyperpax_125-1 \\
  --commission-rate="0.10" \\
  --commission-max-rate="0.20" \\
  --commission-max-change-rate="0.01" \\
  --min-self-delegation="1" \\
  --gas=auto \\
  --gas-adjustment=1.5 \\
  --gas-prices=0.02uhpx \\
  --from=operator`}</code></pre>
      <p><code>--min-self-delegation</code> is expressed in base units. Set gas prices for the connected network. Choose either <code>--gas-prices</code> with a gas limit or an explicit <code>--fees</code> amount.</p>
      <h3>Inspect Validator State</h3>
      <pre><code>{`paxd query staking validator "$(paxd keys show operator --bech val -a)"
paxd query staking delegations-to "$(paxd keys show operator --bech val -a)"
paxd query slashing signing-info "$(paxd tendermint show-validator)"
paxd query staking params
paxd query slashing params`}</code></pre>
      <p>The staking module also exposes <code>edit-validator</code>, <code>delegate</code>, <code>redelegate</code>, and <code>unbond</code>. <code>paxd tx slashing unjail</code> is subject to the keeper&apos;s eligibility checks; it cannot reverse permanent tombstoning.</p>
      <div className="source-note"><strong>Source:</strong> <code>sdk/x/staking/client/cli/tx.go</code>, <code>sdk/x/staking/client/cli/query.go</code>, <code>sdk/x/slashing/client/cli/query.go</code>, <code>sdk/client/tx/factory.go</code>.</div>
      <h2>Monitoring</h2>
      <pre><code>{`curl -sS "\${CONSENSUS_HTTP_URL:?Set the consensus HTTP origin}/status" | jq '.result.sync_info'
curl -sS "\${CONSENSUS_HTTP_URL:?Set the consensus HTTP origin}/net_info" | jq '.result.n_peers'
journalctl -u paxd -f`}</code></pre>
      <p>Enable the configured Prometheus listener as shown in <Link href="/configuration">Configuration</Link>. Consensus metrics include block height, last-signed height, missed-block gauges, and syncing state. Build alerts from the emitted metric names and choose thresholds for your workload. Treat missed-block values as gauges.</p>
      <h2>Peer and Signer Configuration</h2>
      <p>Use <code>bootstrap-peers</code>, <code>persistent-peers</code>, and <code>private-peer-ids</code> under <code>[p2p]</code>. The peer ID can be read with <code>paxd tendermint show-node-id</code>. Keep signer access separate from public query services.</p>
      <p>Remote signers connect through the configured <code>[priv-validator]</code> socket or gRPC interface. For a hardware-backed signer, use an integration compatible with that interface and verify signing before activating the validator.</p>
      <h2>Backup and Recovery</h2>
      <table><thead><tr><th>Data</th><th>Purpose</th></tr></thead><tbody>
        <tr><td><code>config/priv_validator_key.json</code></td><td>Consensus identity and signing key</td></tr>
        <tr><td><code>data/priv_validator_state.json</code></td><td>Last signed height, round, step, and signature</td></tr>
        <tr><td><code>config/node_key.json</code></td><td>Peer identity</td></tr>
        <tr><td>Selected keyring backend</td><td>Operator account keys</td></tr>
        <tr><td>Genesis, TOML configuration, and configured stores</td><td>Matching chain and state data</td></tr>
      </tbody></table>
      <p>The node defaults to <code>~/.paxeer</code>. HPX defaults to <code>/root/.paxeer</code> and reads <code>HPX_HOME</code> to override that home. Stop the node before replacing its stores. Preserve the key and last-sign state together and ensure the original signer is offline before activating a replacement. Never start a second active signer with the same validator key.</p>
      <p>For snapshot recovery, verify the archive revision, checksum, backend, and layout before extraction. Reserve <code>unsafe-reset-all</code> for a planned state reset with complete backups. Use retention settings for routine pruning and <code>paxd rollback</code> for supported stopped-node state reconciliation.</p>
      <div className="source-note"><strong>Source:</strong> <code>consensus/config/config.go</code>, <code>consensus/privval/file.go</code>, <code>sdk/server/rollback.go</code>, <code>sdk/x/evidence/keeper/infraction.go</code>.</div>
      <h2>Planned Upgrades</h2>
      <p>Match the network&apos;s upgrade plan and the release revision before replacing a binary. Record the installed version, preserve the previous binary and required state, stop the service, install the compatible release, and confirm synchronization after restart. Check the release&apos;s upgrade and snapshot compatibility requirements before installation.</p>
      <p>Governance upgrade proposals use the registered <code>software-upgrade</code> command. Expedited proposal support uses <code>--is-expedited</code>; query current deposit and voting parameters before submitting.</p>

      <div className="prev-next">
        <Link href="/configuration"><div className="prev-next-label">Previous</div><div className="prev-next-title">Configuration</div></Link>
        <Link href="/consensus"><div className="prev-next-label">Next</div><div className="prev-next-title">Consensus</div></Link>
      </div>
    </DocsLayout>
  )
}
