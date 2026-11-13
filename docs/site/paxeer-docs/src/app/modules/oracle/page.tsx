import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/modules/oracle')

export default function Oracle() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Oracle Module</h1>
        <p className="page-description">Navigate the retired EVM Oracle API and the native interfaces for exchange rates, voting parameters, and price observations.</p>
      </div>

      <div className="source-note"><strong>Source:</strong> <code>Paxeer-X-Network/precompiles/oracle/oracle.go</code>, <code>modules/oracle/</code>, <code>node/wasmbinding/queries.go</code>, and <code>node/app.go</code>.</div>
      <h2>EVM price queries are retired</h2>
      <p>The Oracle precompile remains registered at <code>0x0000000000000000000000000000000000001008</code>, but both <code>getExchangeRates</code> and <code>getOracleTwaps</code> revert with:</p>
      <pre><code>oracle precompile is retired; oracle data queries are disabled</code></pre>
      <p>For EVM applications, choose an active price-data integration for your network and configure its asset identifiers, units, and freshness checks.</p>

      <h2>Native Oracle interfaces</h2>
      <p>The native Oracle module handles validator voting, aggregation, and queries. It also provides the keeper used by CosmWasm Oracle query plugins.</p>
      <table>
        <thead><tr><th>Component</th><th>Function</th></tr></thead>
        <tbody>
          <tr><td><code>modules/oracle/handler.go</code></td><td>Feed-consent delegation and aggregate exchange-rate vote messages.</td></tr>
          <tr><td><code>modules/oracle/abci.go</code></td><td><code>MidBlocker</code>: vote-period ballot aggregation, exchange-rate updates, and slash-window penalty processing.</td></tr>
          <tr><td><code>modules/oracle/keeper/</code></td><td>Stored rates, vote targets, validator voting state, parameters, and query services.</td></tr>
          <tr><td><code>modules/oracle/client/cli/query.go</code></td><td>Native exchange-rate, actives, snapshot-history, TWAP, parameter, feeder, and vote-penalty queries.</td></tr>
          <tr><td><code>node/wasmbinding/queries.go</code></td><td><code>HandleOracleQuery</code>: native <code>exchange_rates</code> and <code>oracle_twaps</code> custom-query handlers.</td></tr>
        </tbody>
      </table>
      <p>Query current vote targets and observations to identify the denominations available on your network. Check observation timestamps before using a rate in application logic.</p>

      <h2>Aggregation and parameters</h2>
      <p>Aggregation runs in the module&apos;s mid-blocker (<code>MidBlock</code> in <code>modules/oracle/module.go</code>), not in begin- or end-block. At the last block of each vote period it builds voting claims from bonded validators, organizes ballots by configured vote targets, applies threshold checks, and derives exchange rates through weighted-median processing (<code>WeightedMedianWithAssertion</code> for the reference denom, <code>Tally</code> with the reward band for cross rates). At the last block of each slash window it runs <code>SlashAndResetCounters</code> over the miss/abstain counters. Vote-period, reward-band, and slash-window behavior is controlled by module parameters.</p>
      <p>Read the live parameters and vote targets before interpreting a network&apos;s observations or validator obligations. Do not infer a fixed voting period, reward band, slash rate, or required asset list from an example or another network&apos;s settings.</p>

      <h2>Native query commands</h2>
      <p>These eight commands are registered by the native module. Point the CLI at the Cosmos RPC endpoint for the node you are inspecting:</p>
      <pre><code>{`paxd query oracle params
paxd query oracle vote-targets
paxd query oracle actives
paxd query oracle exchange-rates
paxd query oracle exchange-rates "$DENOM"
paxd query oracle price-snapshot-history
paxd query oracle twaps "$LOOKBACK_SECONDS"
paxd query oracle feeder "$VALIDATOR_ADDRESS"
paxd query oracle vote-penalty-counter "$VALIDATOR_ADDRESS"`}</code></pre>
      <p>The optional denomination belongs to <code>exchange-rates</code>; there is no separate singular <code>exchange-rate</code> command in this CLI. Validator operator addresses use the <code>paxvaloper</code> prefix.</p>

      <h2>CosmWasm query boundary</h2>
      <p>The <code>oracle</code> custom-query route still calls the native module handler. A CosmWasm custom querier payload for retained exchange rates has this shape:</p>
      <pre><code>{`{
  "route": "oracle",
  "query_data": { "exchange_rates": {} }
}`}</code></pre>
      <p>Issue this query from a CosmWasm contract through <Link href="/wasmbinding">WASM bindings</Link>. Check the returned denominations and observation freshness before using the results.</p>

      <h2>Related interfaces</h2>
      <ul>
        <li><Link href="/precompiles">Current precompile addresses and ABI behavior</Link></li>
        <li><Link href="/rest-grpc">Native REST and gRPC interfaces</Link></li>
        <li><Link href="/wasmbinding">Custom CosmWasm queries and messages</Link></li>
      </ul>
      <div className="prev-next">
        <Link href="/modules/mint"><div className="prev-next-label">Previous</div><div className="prev-next-title">Mint Module</div></Link>
        <Link href="/modules/store"><div className="prev-next-label">Next</div><div className="prev-next-title">Store Module</div></Link>
      </div>
    </DocsLayout>
  )
}
