import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/node-troubleshooting')

export default function Guide() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Troubleshoot a Node</h1>
        <p className="page-description">Diagnose Paxeer X sync stalls, peer failures, RPC errors, database locks, and AppHash mismatches before changing node state.</p>
      </div>
      <p>Collect a small set of evidence before changing configuration: the running binary, configured network, current height, peer count, and recent logs. This lets you distinguish a local service problem from a network-wide halt or an application-state mismatch.</p>
      <h2>Diagnose kernel and application dependencies</h2>
      <p>A healthy EVM RPC confirms one execution surface. Trace the affected Paxeer X operation across ingress, identity, kernel execution, receipt verification, checkpoint authority, and chain settlement. Keep the activity ID, receipt identity, batch number, checkpoint ID, and chain transaction hash correlated.</p>
      <table><thead><tr><th>Symptom</th><th>Evidence to inspect</th><th>Recovery direction</th></tr></thead><tbody>
        <tr><td>Gateway rejects a request</td><td>Public method, authentication, registry route, dependency readiness, and receipt verification error</td><td>Correct the failing authorization or upstream dependency</td></tr>
        <tr><td>Node or hosted service fails startup</td><td>Generated configuration, signed genesis, network identity, state-directory ownership and lock. The former <code>layerxd --check-config</code> path is retired</td><td>Use the matching artifacts and one process per state directory</td></tr>
        <tr><td>Sequencer advances, replica lags</td><td>Replica input history, retained logs, connectivity, receipt and root agreement</td><td>Restore the replica path from its verified history</td></tr>
        <tr><td>Checkpoint awaits attestations</td><td>Availability retrieval, replay errors, authorized sequencer range, guarantor membership and threshold</td><td>Restore verification and attestor connectivity before publication</td></tr>
        <tr><td>Checkpoint registered, publication pending</td><td>Publication authorization, owner signatures, registered certificate, and finality evidence</td><td>Supply the required valid authorization through its protected signing path</td></tr>
        <tr><td>Human or agent operation stalls</td><td>Identity, owner approval, provider readiness, key-management channel, and canonical receipt</td><td>Repair the responsible service boundary and resume the recorded operation</td></tr>
        <tr><td>Relay/archive falls behind</td><td>Relay/archive is retired (see <Link href="/relay-archive">relay/archive status</Link>); for chain history lag, use the <code>paxd</code> sync checks below</td><td>Diagnose the <code>paxd</code> node&apos;s sync state instead</td></tr>
      </tbody></table>
      <p>A failed guarantor replay stops signing; preserve that evidence and resolve the disagreement. A refused or invalid publication signature is different from a missing authorization that remains pending. Do not substitute a transaction receipt for proof of checkpoint finality or a completed withdrawal. Continue with <Link href="/settlement">settlement</Link> and <Link href="/operators">operator responsibilities</Link>.</p>

      <h2>Confirm the process and endpoints</h2>
      <p>The following examples assume a Linux systemd service named <code>paxd</code> and default local ports. Substitute your service name and bound ports if they differ.</p>
      <pre><code className="language-bash">{"paxd version --long\nsystemctl status paxd --no-pager\nsystemctl show paxd -p ExecStart -p User\nss -lntp\ncurl --fail --silent --show-error http://127.0.0.1:26657/status | jq '.result | {node_info, sync_info}'\ncurl --fail --silent --show-error http://127.0.0.1:26657/net_info | jq '.result.n_peers'\ncurl --fail-with-body --silent --show-error http://127.0.0.1:8545   -H 'Content-Type: application/json'   --data '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"eth_chainId\",\"params\":[]}'"}</code></pre>
      <p>For the Paxeer X network documented here, compare the Cosmos network with <code>hyperpax_125-1</code> and the EVM result with <code>0x7d</code>. A local development cluster can use a different identifier. The binary found in your shell can differ from the absolute path in the service’s <code>ExecStart</code>.</p>
      <h2>Match the symptom to the next check</h2>
      <table><thead><tr><th>Symptom</th><th>Inspect</th><th>Next action</th></tr></thead><tbody><tr><td>Connection refused</td><td>Service status and listening sockets</td><td>Confirm process startup and the bound address before investigating peers</td></tr><tr><td>Zero peers</td><td>Network ID, configured peer IDs, DNS, and P2P connectivity</td><td>Use peers supplied for that exact network and release</td></tr><tr><td>Peers present, height stalled</td><td>Earliest error before the stall, upgrade height, disk and memory pressure</td><td>Compare the same height with a known-good node</td></tr><tr><td>Cosmos status works, EVM RPC fails</td><td>EVM enablement, port, and RPC namespace configuration</td><td>Check <Link href="/configuration">application configuration</Link></td></tr><tr><td>Historical query fails</td><td>Pruning and state retention at the requested height</td><td>Use an archive-capable endpoint with that state</td></tr></tbody></table>
      <h2>Capture a sync stall</h2>
      <p>Capture the height and latest block time twice, a short interval apart. Compare both samples with a known-good node on the same network. A changing height with increasing lag points toward throughput; an unchanged height with a repeating error points toward a specific blocker.</p>
      <pre><code className="language-bash">{"curl --fail --silent --show-error http://127.0.0.1:26657/status   | jq '.result.sync_info | {latest_block_height, latest_block_time, catching_up}'\njournalctl -u paxd --since '15 minutes ago' --no-pager -o short-iso\ndf -h\ndf -i\nfree -h"}</code></pre>
      <p>Check storage capacity and inode exhaustion separately. For slow disks, use the host’s I/O monitoring to compare read latency, write latency, and saturation with the stalled period. Avoid changing several performance settings at once; preserve one baseline to compare against.</p>
      <h2>Investigate AppHash mismatches</h2>
      <p>An AppHash mismatch means the node’s computed application state disagrees with the block header. Record the exact height, expected and actual hashes, binary revision, genesis identity, and snapshot provenance. Use the binary assigned to that network’s upgrade schedule at the failing height.</p>
      <pre><code className="language-bash">{"journalctl -u paxd --since '1 hour ago' --no-pager   | rg -n -i 'apphash|app hash|panic|corrupt|upgrade|handshake'\n\n# Set PAXEER_NODE_HOME to the data directory used by the service.\nsha256sum \"${PAXEER_NODE_HOME:?Set the active node home}/config/genesis.json\""}</code></pre>
      <p>Handshake errors can follow a state mismatch; start with the earliest application error rather than repeatedly rotating peers. Reconcile the release, upgrade schedule, and snapshot with a known-good operator before choosing recovery steps.</p>
      <h2>Handle database locks and recovery</h2>
      <p>A database lock often means another process still holds the same data directory. Inspect the service and process list, including containers, before attempting another start. Keep one writer per data directory; stop the process holding the database before recovery.</p>
      <p>Rollback, snapshot restore, and reset commands change durable state and depend on the storage engine and release. Preserve a backup and use the recovery procedure for that exact node version. For validator recovery, preserve the existing signing state and confirm that only one signer is active.</p>
      <p>For a duplicate-signing alert, stop the affected signer and establish which instance owns signing before restarting. Retain the validator keys and signing history throughout recovery.</p>
      <h2>Prepare an actionable report</h2>
      <ul><li>Node role, network identifiers, binary version and checksum, and startup command.</li><li>Stalled or failing height and a comparison with a healthy node.</li><li>At least the logs leading into the first failure, including the complete panic stack when present.</li><li>Disk, memory, peer count, recent upgrade or restore activity, and snapshot checksum.</li><li>The relevant configuration changes, with RPC credentials and signing material removed.</li></ul>
      <p>Use <Link href="/operators">Operators Guide</Link> for ongoing monitoring, <Link href="/configuration">Configuration</Link> for settings, and <Link href="/run-node">Run a Node</Link> for initial setup.</p>
      <div className="source-note"><strong>Paxeer source:</strong> monorepo root — <code>daemon/paxd/cmd/root.go</code> (command registration), <code>node/app.go</code> (DefaultNodeHome), and <code>consensus/internal/rpc/core/status.go</code> (Status).</div>
      <div className="prev-next">
        <Link href="/operators"><div className="prev-next-label">Previous</div><div className="prev-next-title">Operators Guide</div></Link>
        <Link href="/configuration"><div className="prev-next-label">Next</div><div className="prev-next-title">Configuration</div></Link>
      </div>
    </DocsLayout>
  )
}
