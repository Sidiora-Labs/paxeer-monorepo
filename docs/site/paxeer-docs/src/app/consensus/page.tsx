import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/consensus')

export default function Consensus() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Consensus</h1>
        <p className="page-description">Consensus service wiring, application execution, trusted headers, and validator signing state.</p>
      </div>

      <h2>Overview</h2>
      <p>Paxeer mainnet-beta is one BFT chain with EVM chain ID 125. Its kernel is part of the network and proposes blocks. A receipt is final when its block is committed. The tree includes a Tendermint-derived consensus stack and configurable Autobahn components; the application executes transactions and commits state. Network safety depends on the protocol&apos;s validator and trust assumptions, while observed block time and throughput depend on the deployed configuration.</p>
      <h2>Autobahn and Validator Committees</h2>
      <p>Autobahn uses application and lane proposals, votes, quorum certificates, committees, and timeouts. Their types are defined under <code>consensus/autobahn/types/</code>; the protocol state is coordinated under <code>consensus/internal/autobahn/</code>.</p>
      <p>Use the deployment&apos;s Autobahn configuration to inspect its lane and committee settings. Track throughput and block intervals through node metrics.</p>
      <div className="source-note"><strong>Source:</strong> <code>consensus/autobahn/types/committee.go</code>, <code>consensus/autobahn/types/lane_proposal.go</code>, <code>consensus/autobahn/types/commit_qc.go</code>, <code>consensus/config/config.go</code>.</div>
      <h2>Application Execution</h2>
      <p>The node application registers <code>ProcessProposalHandler</code> and <code>FinalizeBlocker</code>. A proposal may begin optimistic work before finalization; finalized execution and successful commit establish the application result. Mempool acceptance and a returned submission hash are earlier stages.</p>
      <p>Transaction execution can use the standard scheduler or the configured Giga executor. The standard scheduler records multiversion accesses, validates dependencies, and retries conflicting tasks. After 10 iterations it reverts to sequential processing of remaining unvalidated work.</p>
      <p>Consensus establishes transaction ordering, while the execution scheduler manages parallel work and dependency validation.</p>
      <div className="source-note"><strong>Source:</strong> <code>node/app.go</code> — <code>ProcessProposalHandler</code>, <code>FinalizeBlocker</code>, <code>ExecuteTxsConcurrently</code>; <code>sdk/tasks/scheduler.go</code> — <code>ProcessAll</code>.</div>
      <h2>Node Services</h2>
      <p>The <code>consensus/node/</code> package assembles peer networking, block and state stores, mempool, evidence handling, RPC, indexing, and application connections. The consensus configuration accepts <code>full</code>, <code>validator</code>, and <code>seed</code>. The paxd initializer also accepts archive, writes consensus mode full, and configures the application to retain available history.</p>
      <p>Sequencer mode cannot run with an <code>autobahn-config-file</code>; the configuration rejects that combination.</p>
      <p>See <Link href="/configuration">Configuration</Link> for node mode defaults and <Link href="/run-node">Run a Node</Link> for initialization.</p>
      <h2>State and Synchronization</h2>
      <p>Consensus state records validator sets, block identity, consensus parameters, and application commitment. State sync uses a trusted checkpoint and application snapshots to bootstrap a node without replaying all earlier blocks.</p>
      <p>Configure application snapshot production on provider nodes and state-sync consumption on joining nodes. Use the <Link href="/storage">storage configuration</Link> to retain the data and commitment history needed for queries and proofs.</p>
      <div className="source-note"><strong>Source:</strong> <code>consensus/config/config.go</code> — <code>StateSyncConfig</code>; <code>consensus/internal/statesync/</code>; <code>consensus/internal/state/</code>.</div>
      <h2>Light Client</h2>
      <p>The light client checks new headers against a trusted header, validator set, trust period, and clock-drift rules. Non-adjacent verification checks the configured trusted-set overlap and requires more than two-thirds of the new validator voting power to sign the new header.</p>
      <p>Maintain a trusted checkpoint within the configured trust period. For historical application proofs, connect to a node retaining the corresponding commitment state.</p>
      <div className="source-note"><strong>Source:</strong> <code>consensus/light/verifier.go</code> — <code>VerifyAdjacent</code>, <code>VerifyNonAdjacent</code>; <code>consensus/light/client.go</code>.</div>
      <h2>Validator Signing</h2>
      <p>File-based validators persist a signing key and a mutable last-sign state containing height, round, step, and signature data. Recovery must preserve both. Never run two active signers using the same validator key.</p>
      <p>Autobahn requires a local validator key and rejects remote signers. Do not configure a remote signer through the node&apos;s <code>[priv-validator]</code> configuration for an Autobahn validator.</p>
      <p>Equivocation handling can permanently tombstone a validator. Query the connected network&apos;s slashing parameters for its configured monetary penalties.</p>
      <div className="source-note"><strong>Source:</strong> <code>consensus/privval/file.go</code>, <code>consensus/node/setup.go</code> — <code>createPrivval</code>; <code>sdk/x/evidence/keeper/infraction.go</code>.</div>
      <h2>RPC and Metrics</h2>
      <pre><code>{`curl -sS "$CONSENSUS_RPC_URL/status"
curl -sS "$CONSENSUS_RPC_URL/net_info"
curl -sS "$CONSENSUS_RPC_URL/consensus_params"`}</code></pre>
      <p>Set CONSENSUS_RPC_URL in your shell to the consensus listener for these examples. Consensus RPC and EVM JSON-RPC are separate listeners; the default consensus RPC port is 26657. The consensus metrics include height, validator last-signed height, missed-block gauges, block/state-sync state, and block intervals. Use the series emitted by your configured namespace when constructing alerts.</p>
      <div className="source-note"><strong>Source:</strong> <code>consensus/internal/rpc/core/routes.go</code>; <code>consensus/internal/consensus/metrics.go</code>.</div>
      <h2>Next Steps</h2>
      <ul><li><Link href="/engine">EVM execution engine</Link></li><li><Link href="/operators">Validator operations</Link></li><li><Link href="/network-parameters">Network parameters</Link></li></ul>

      <div className="prev-next">
        <Link href="/operators"><div className="prev-next-label">Previous</div><div className="prev-next-title">Operators Guide</div></Link>
        <Link href="/engine"><div className="prev-next-label">Next</div><div className="prev-next-title">Engine</div></Link>
      </div>
    </DocsLayout>
  )
}
