import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import { DocArt } from '@/components/DocArt'
import Link from 'next/link'

export const metadata = metadataFor('/network-parameters')

export default function NetworkParameters() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Network Parameters</h1>
        <p className="page-description">Chain identity, denominations, configurable defaults, and governance controls across Paxeer X.</p>
      </div>

      <DocArt name="chain-configuration" />
      <h2>Unified execution and application parameters</h2>
      <p>Paxeer X retains chain ID 125 while joining EVM and Cosmos accounts to LayerX identity and kernel execution in one node. The address precompile’s <code>getUnifiedAccount</code> returns the related EVM address, Cosmos address, DID public key, and LayerX main account identifier. See the <Link href="/unified-network">network model</Link>.</p>
      <table><thead><tr><th>Parameter family</th><th>Controlled behavior</th></tr></thead><tbody>
        <tr><td>Bridge</td><td>Chain registry, attestor threshold, per-asset caps, and pause state</td></tr>
        <tr><td>Launchpad</td><td>Quote denomination, virtual reserves, creation fee, fee bounds, decay, volatility and concentration weights, and protocol fee share</td></tr>
        <tr><td>Fee tokens</td><td>Approved fee denominations, rate age, spread limit, enablement, and distribution policy</td></tr>
        <tr><td>XWeb</td><td>Attestors, signature threshold, request fee, delivery limits, and pause controls</td></tr>
        <tr><td>Tenant domains</td><td><code>modules/layerxchains</code> namespaces state within the shared network. Creating a domain does not require new node infrastructure.</td></tr>
      </tbody></table>
      <p>The pre-merge custody and anchor parameter families are not live governable families. <code>layerxVerify</code> (<code>0x1012</code>) and <code>layerxCustody</code> (<code>0x1013</code>) are retired at the merge. <code>layerxAnchor</code> (<code>0x1014</code>) rejects writes after the merge, and the retained anchor module refuses new checkpoint submissions. Guarantor threshold, bond, challenge, and unbonding parameters describe that retired protocol only.</p>
      <p><code>docs/system/chain-modules.md</code> in the monorepo lists parameter fields and <code>docs/system/governance.md</code> lists proposal types. Query the target network’s active values for financial decisions. See <Link href="/precompiles">native interfaces</Link>, <Link href="/assets">asset units</Link>, and <Link href="/bridge">bridge controls</Link>.</p>

      <h2>Chain Identifiers</h2>
      <p>Use these identifiers when configuring a wallet or client. Confirm the connected node&apos;s chain ID before signing a transaction.</p>
      <table>
        <thead><tr><th>Parameter</th><th>Value</th></tr></thead>
        <tbody>
          <tr><td>EVM chain ID</td><td><code>125</code> / <code>0x7d</code></td></tr>
          <tr><td>Consensus chain ID</td><td><code>hyperpax_125-1</code></td></tr>
          <tr><td>Account prefix</td><td><code>pax1…</code></td></tr>
          <tr><td>Validator operator prefix</td><td><code>paxvaloper1…</code></td></tr>
          <tr><td>Binary / default home</td><td><code>paxd</code> / <code>~/.paxeer</code></td></tr>
          <tr><td>Go module</td><td><code>github.com/Sidiora-Labs/Paxeer-X-Network</code></td></tr>
          <tr><td>Repository</td><td><code>Sidiora-Labs/paxeer-monorepo</code>. The Go module path retains the <code>Paxeer-X-Network</code> name.</td></tr>
        </tbody>
      </table>
      <pre><code>{`EVM_RPC_URL=http://127.0.0.1:8545
curl -sS "$EVM_RPC_URL" -H 'Content-Type: application/json' \\
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
paxd status --node tcp://127.0.0.1:26657`}</code></pre>
      <p>Public node JSON-RPC uses the root URL <code>https://api-mainnet-beta.paxeer.network</code> or the numbered nodes <code>https://api1.mainnet-beta.paxeer.network</code> through <code>https://api14.mainnet-beta.paxeer.network</code>. Expected <code>eth_chainId</code> result is <code>0x7d</code>. Public namespaces are <code>debug</code>, <code>echo</code>, <code>eth</code>, <code>net</code>, <code>pax</code>, <code>test</code> and <code>txpool</code>. Method availability depends on node configuration. Configure more than one node and compare <code>eth_blockNumber</code> when selecting one near the head. <code>node.hyperpaxeer.com</code> is only the hpx library mirror used by build and installation tooling, never a JSON-RPC endpoint. See <Link href="/public-rpc">Public RPC</Link>.</p>
      <div className="source-note"><strong>Source:</strong> <code>tools/hpx/publish.sh</code>, <code>node/params/config.go</code>, <code>node/const.go</code>, <code>rpc/info.go</code>.</div>

      <h2>Native Denominations</h2>
      <p>The native asset pays EVM gas. Node parameters name its human unit <code>hpx</code>. Cosmos CLI and bank APIs use <code>uhpx</code> for base-unit amounts. EVM clients use 18-decimal native amounts.</p>
      <pre><code>{`1 hpx = 1,000,000 uhpx
1 uhpx = 1,000,000,000,000 EVM wei
1 hpx = 1,000,000,000,000,000,000 EVM wei`}</code></pre>
      <p>Use <code>uhpx</code> amounts for Cosmos CLI transactions and 18-decimal native amounts for EVM transactions. Address association joins the supported wallet representations; see <Link href="/precompiles">address precompiles</Link>.</p>
      <div className="source-note"><strong>Source:</strong> <code>node/params/config.go</code> — <code>UhpxExponent</code>; <code>modules/evm/state/utils.go</code> — <code>UhpxToSweiMultiplier</code>.</div>

      <h2>Query Current Parameters</h2>
      <p>Query the connected network for its active governance, staking, slashing, and consensus parameters. Cosmos queries use the consensus RPC endpoint; EVM requests use the EVM JSON-RPC endpoint.</p>
      <pre><code>{`paxd query staking params --node tcp://127.0.0.1:26657
paxd query slashing params --node tcp://127.0.0.1:26657
paxd query gov params --node tcp://127.0.0.1:26657
curl -sS http://127.0.0.1:26657/consensus_params`}</code></pre>
      <table>
        <thead><tr><th>Default parameter</th><th>Value</th><th>Meaning</th></tr></thead>
        <tbody>
          <tr><td>Unbonding time</td><td>21 days</td><td>Default staking unbonding duration</td></tr>
          <tr><td>Unbonding / redelegation entries</td><td>7</td><td>Limit for an individual delegation pair</td></tr>
          <tr><td>Standard governance period</td><td>2 days</td><td>Default deposit and voting periods</td></tr>
          <tr><td>Expedited governance period</td><td>1 day</td><td>Default expedited voting duration</td></tr>
          <tr><td>Governance minimum deposit</td><td>10,000,000 base units</td><td>Query the active governance deposit requirement</td></tr>
          <tr><td>Expedited minimum deposit</td><td>20,000,000 base units</td><td>Query the active governance deposit requirement</td></tr>
          <tr><td>Slashing fractions</td><td>0 by default</td><td>Configured by chain governance</td></tr>
        </tbody>
      </table>
      <p>Governance approval requires a yes-vote ratio strictly greater than the configured threshold after abstentions are excluded. Validator jailing and equivocation handling apply alongside the configured slashing fractions.</p>
      <div className="source-note"><strong>Source:</strong> <code>sdk/x/staking/types/params.go</code>, <code>sdk/x/gov/types/params.go</code>, <code>sdk/x/gov/keeper/tally.go</code>, <code>sdk/x/slashing/types/params.go</code>.</div>

      <h2>Gas and Block Limits</h2>
      <p>Query <code>eth_gasPrice</code>, <code>eth_feeHistory</code>, and <code>eth_maxPriorityFeePerGas</code> for EVM fee information. The node also has local admission prices and simulation limits. Consensus <code>block.max_bytes</code> is a block limit; <code>mempool.max-tx-bytes</code> is a separate transaction limit.</p>
      <p>See <Link href="/json-rpc">JSON-RPC</Link> and <Link href="/configuration">Configuration</Link> for the distinct EVM, consensus, and node settings.</p>

      <h2>Performance and Endpoints</h2>
      <p>Track block interval using block timestamps, throughput using transactions per block, and RPC latency using request metrics. Record the node revision and workload when comparing measurements.</p>
      <p>Use the RPC endpoint supplied by your node operator. The <Link href="/admin-hpx">HPX distribution and peer registry</Link> supplies chain metadata and peer information for its configured network. The hosted gateway <code>px_</code> API is separate from public node JSON-RPC. Obtain its URL and credentials from the operator.</p>
      <h2>LayerX Integration</h2>
      <p>Paxeer chain parameters govern L1 transactions. For applications that also use LayerX activities and receipts, see <Link href="/paxeer-vs-layerx">Paxeer and LayerX</Link> for the integration architecture.</p>

      <div className="prev-next">
        <Link href="/paxeer-vs-layerx"><div className="prev-next-label">Previous</div><div className="prev-next-title">Paxeer and LayerX</div></Link>
        <Link href="/installation"><div className="prev-next-label">Next</div><div className="prev-next-title">Installation & Build</div></Link>
      </div>
    </DocsLayout>
  )
}
