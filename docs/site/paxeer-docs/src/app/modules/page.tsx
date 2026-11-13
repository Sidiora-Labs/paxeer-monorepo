import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/modules')

export default function Modules() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Paxeer X Modules</h1>
        <p className="page-description">
          Native execution, markets, issuance, and data modules in <code>modules/</code>, including retired settlement-compatibility directories.
        </p>
      </div>

      <div className="source-note">
        <strong>Source:</strong> <code>Paxeer-X-Network/modules/README.md</code>
      </div>

      <h2>Unified Paxeer X modules</h2>
      <p>Paxeer X combines EVM and Cosmos execution with the LayerX kernel. The chain modules below enforce the chain-side rules; the <Link href="/layerx-kernel">LayerX kernel</Link> is part of the same network, and a receipt is final when its block is committed. Addresses below are shortened; see <Link href="/precompiles">precompiles</Link> for full addresses and upgrade gates.</p>
      <table><thead><tr><th>Module</th><th>Role in Paxeer X</th><th>EVM entry point</th></tr></thead><tbody>
        <tr><td><code>lxphost</code></td><td>Shared cgo host for kernel validation and execution; also backs the <code>layerxprograms</code> precompile</td><td><code>0x101D</code></td></tr>
        <tr><td><code>layerxledger</code></td><td>Asset registration, derived accounts, transfers, and exact payments. The old payer-grant issue, revoke, and draw operations are retired</td><td><code>0x101A</code></td></tr>
        <tr><td><code>layerxauth</code></td><td>Capability and session grants, revocation, and SDK authorization integration</td><td><code>0x101C</code></td></tr>
        <tr><td><code>layerxpay</code></td><td>Escrows, budgets, streams, and service agreements, subject to configured family enablement and pause gates</td><td><code>0x101B</code></td></tr>
        <tr><td><code>layerxagent</code></td><td>Scheduled EVM calls, job deposits, and bounded end-block execution. Agent-function precompiles occupy <code>0x0901</code>–<code>0x0908</code>; the scheduler is at <code>0x0905</code></td><td><code>0x0901</code>–<code>0x0908</code></td></tr>
        <tr><td><code>layerxchains</code></td><td>Tenant-domain registry, funding, allowances, operator controls, and lifecycle. Sub-chains are database namespaces on the same network</td><td><code>0x101E</code></td></tr>
        <tr><td><code>sequencer</code></td><td>On-chain leader list, receipt keys, and sequence state used by block proposal and receipt machinery</td><td>—</td></tr>
        <tr><td><code>layerxgov</code></td><td>Governance proposal content (<code>LayerXProposal</code>) that dispatches packaged parameter and admin messages through the SDK <code>MsgServiceRouter</code></td><td>Governance proposals</td></tr>
        <tr><td><code>perps</code></td><td>Native perpetuals markets and synchronous trading: orders, positions, and margin</td><td><code>0x1015</code></td></tr>
        <tr><td><code>spot</code></td><td>Native spot markets, orders, stop orders, quoters, and vaults</td><td>—</td></tr>
        <tr><td><code>pricefeed</code></td><td>Validator-signed index and mark price observations with mid-block aggregation</td><td>—</td></tr>
        <tr><td><code>layerxbridge</code></td><td>Attested transfers between Paxeer and external chains: chain registry, attestor set, per-asset caps, and <code>bridgeIn</code>/<code>bridgeOut</code></td><td><code>0x1016</code></td></tr>
        <tr><td><code>launchpad</code></td><td>Native bonding-curve markets over <code>tokenfactory</code> denoms</td><td><code>0x1017</code></td></tr>
        <tr><td><code>xweb</code></td><td>Attested web data for EVM contracts and kernel programs; governance controls the attestor set, threshold, and pause state</td><td><code>0x1019</code></td></tr>
      </tbody></table>
      <p>Explore the <Link href="/unified-network">unified network</Link>, <Link href="/assets">asset model</Link>, <Link href="/settlement">settlement lifecycle</Link>, and <Link href="/bridge">bridge</Link>. <code>modules/README.md</code> describes module ownership.</p>

      <h2>Retired settlement compatibility</h2>
      <p>These directories retain legacy types, historical state handling, and retirement checks. They do not provide a separate execution or settlement domain:</p>
      <table><thead><tr><th>Module</th><th>Status</th><th>EVM entry point</th></tr></thead><tbody>
        <tr><td><code>layerxcustody</code></td><td>Legacy custody registry and retirement handling. Deposits, proof-carrying withdrawals, forced exits, and deposit-root publication are retired. Fund derived sub-accounts through native bank sends; use <code>layerxbridge</code> for outward transfers</td><td><code>0x1013</code></td></tr>
        <tr><td><code>layerxanchor</code></td><td>Retired checkpoint and bond machinery. Use committed blocks for finality and committed application state with per-key proofs for state verification</td><td><code>0x1014</code></td></tr>
        <tr><td><code>layerxexchange</code></td><td>Retired exchange-intent submission and proof-based settlement machinery. Use the native <code>perps</code> module at <code>0x1015</code> for orders, positions, and margin</td><td><code>0x1015</code> (superseded by <code>perps</code>)</td></tr>
      </tbody></table>

      <h2>Module Overview</h2>

      <p>
        Paxeer X-specific chain modules live under <code>Paxeer-X-Network/modules/</code>. These extend the Cosmos SDK with functionality unique to Paxeer X’s EVM L1 design. The pages in this section document <code>epoch</code>, <code>mint</code>, <code>oracle</code>, <code>store</code>, and <code>tokenfactory</code> in detail; the modules above are documented in their own sections or in <code>modules/README.md</code>.
      </p>

      <h2>Module List</h2>

      <table>
        <thead>
          <tr>
            <th>Module</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><Link href="/evm"><code>evm</code></Link></td>
            <td>Native EVM execution, address association, receipts, pointers, and precompile integration</td>
          </tr>
          <tr>
            <td><Link href="/modules/epoch"><code>epoch</code></Link></td>
            <td>Time-based hooks and epoch lifecycle management</td>
          </tr>
          <tr>
            <td><Link href="/modules/mint"><code>mint</code></Link></td>
            <td>Scheduled native-token minting, released daily through the epoch hook</td>
          </tr>
          <tr>
            <td><Link href="/modules/oracle"><code>oracle</code></Link></td>
            <td>Validator exchange-rate voting and weighted-median price aggregation (native interfaces; the EVM precompile is retired)</td>
          </tr>
          <tr>
            <td><Link href="/modules/store"><code>store</code></Link></td>
            <td>Single cached-context helper; not a registered chain module</td>
          </tr>
          <tr>
            <td><Link href="/modules/tokenfactory"><code>tokenfactory</code></Link></td>
            <td>Creation and admin management of native token denominations by any sender</td>
          </tr>
        </tbody>
      </table>

      <h2>Framework Modules</h2>

      <p>
        Framework-provided modules (bank, staking, distribution, governance, etc.) remain under <code>sdk/x/</code>. Interchain application modules live under <code>interchain/modules/</code>.
      </p>

      <h2>EVM Module</h2>

      <p>
        The <code>evm</code> module is Paxeer X’s core EVM execution layer:
      </p>

      <ul>
        <li><strong>EVM execution:</strong> Runs Ethereum transactions with full opcode compatibility</li>
        <li><strong>Address association:</strong> Maps Cosmos addresses to EVM addresses</li>
        <li><strong>Receipts:</strong> Generates EVM transaction receipts</li>
        <li><strong>Pointers:</strong> Bridging between Cosmos and EVM state</li>
        <li><strong>Precompiles:</strong> Integration with Paxeer X-specific precompiled contracts</li>
      </ul>

      <p>
        See <Link href="/evm">EVM documentation</Link> for details.
      </p>

      <h2>Epoch Module</h2>

      <p>
        The <code>epoch</code> module provides time-based lifecycle hooks:
      </p>

      <ul>
        <li><strong>Epoch boundaries:</strong> Fixed time periods (60s by default) checked in <code>BeginBlock</code></li>
        <li><strong>Hooks:</strong> Registered modules run <code>AfterEpochEnd</code> / <code>BeforeEpochStart</code> callbacks at each boundary. Only the mint keeper is wired in <code>node/app.go</code></li>
      </ul>

      <p>
        See <Link href="/modules/epoch">Epoch documentation</Link> for details.
      </p>

      <h2>Mint Module</h2>

      <p>
        The <code>mint</code> module manages native token issuance:
      </p>

      <ul>
        <li><strong>Release schedule:</strong> A <code>total_mint_amount</code> released between a start and end date</li>
        <li><strong>Minting trigger:</strong> Only the epoch <code>AfterEpochEnd</code> hook, which mints at most once per day (UTC date)</li>
        <li><strong>Destination:</strong> Newly minted coins are sent to the <code>fee_collector</code> account</li>
      </ul>

      <p>
        See <Link href="/modules/mint">Mint documentation</Link> for details.
      </p>

      <h2>Oracle Module</h2>

      <p>
        The <code>oracle</code> module enables validator-based price feeds on native interfaces:
      </p>

      <ul>
        <li><strong>Exchange rate voting:</strong> Validators submit price observations for configured vote targets</li>
        <li><strong>Aggregation:</strong> Weighted-median computation in the mid-blocker at the last block of each vote period</li>
        <li><strong>Penalties:</strong> Miss/abstain counters are processed at each slash window; this is not a check on whether a submitted price was &quot;bad data&quot;</li>
        <li><strong>EVM access:</strong> The oracle precompile at <code>0x…1008</code> is retired and reverts; use native or CosmWasm queries</li>
      </ul>

      <p>
        See <Link href="/modules/oracle">Oracle documentation</Link> for details.
      </p>

      <h2>Store Module</h2>

      <p>
        <code>modules/store/</code> is a single helper file, not a registered module:
      </p>

      <ul>
        <li><strong><code>GetCachedContext</code>:</strong> Returns a context backed by a <code>CacheMultiStore</code> plus the cache itself. It has no keeper, types, queries, or messages</li>
      </ul>

      <p>
        See <Link href="/modules/store">Store documentation</Link> for details.
      </p>

      <h2>Token Factory Module</h2>

      <p>
        The <code>tokenfactory</code> module enables native token creation:
      </p>

      <ul>
        <li><strong>Native denominations:</strong> Create new token types as <code>factory/&#123;creator&#125;/&#123;subdenom&#125;</code></li>
        <li><strong>Creation:</strong> <code>MsgCreateDenom</code> has no permission gate; any sender can create a denom and becomes its admin</li>
        <li><strong>Management:</strong> The admin can mint (to itself), burn (from itself), change admin, update the allow list, set metadata, and pause/unpause the denom</li>
      </ul>

      <p>
        See <Link href="/modules/tokenfactory">Token Factory documentation</Link> for details.
      </p>

      <h2>Module Integration</h2>

      <p>
        Modules are registered in the application through <code>node/app.go</code>:
      </p>

      <ul>
        <li><strong>Module manager:</strong> Coordinates module initialization and upgrades</li>
        <li><strong>Begin/end block:</strong> Modules register hooks for block lifecycle</li>
        <li><strong>Message routing:</strong> Module-specific transactions routed to handlers</li>
        <li><strong>Query routing:</strong> Module-specific queries routed to keepers</li>
      </ul>

      <div className="prev-next">
        <Link href="/storage">
          <div className="prev-next-label">Previous</div>
          <div className="prev-next-title">Storage</div>
        </Link>
        <Link href="/modules/epoch">
          <div className="prev-next-label">Next</div>
          <div className="prev-next-title">Epoch Module</div>
        </Link>
      </div>
    </DocsLayout>
  )
}
