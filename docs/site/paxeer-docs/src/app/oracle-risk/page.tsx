import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/oracle-risk')

export default function OracleRisk() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Oracles &amp; Market Risk</h1><p className="page-description">Understand how Paxeer X accepts signed price observations, rejects stale or anomalous inputs, and applies market-specific risk limits.</p></div>
    <h2>Prices enter the ledger as signed observations</h2>
    <p>Paxeer X carries external market data into deterministic execution through signed <code>ORACLE_PUSH</code> activities. Execution consumes accepted observations; it does not make an external network request to obtain a price. This gives replay the same ordered market input that original execution used.</p>
    <p>An observation identifies a market, observation sequence, unsigned integer price, observation timestamp, and source identifier. The signed transport appends a signature to the observation (see the format below); the signer key itself is checked against the market&apos;s permitted set rather than carried in the transport. A market defines the permitted keys, freshness window, minimum and maximum prices, and maximum deviation from its preceding accepted observation.</p>
    <h2>Price acceptance pipeline</h2>
    <table><thead><tr><th>Check</th><th>Acceptance rule</th><th>Refusal</th></tr></thead><tbody>
      <tr><td>Canonical encoding</td><td>The payload has the exact expected length and re-encodes to identical bytes.</td><td><code>LXP_ERR_NON_CANONICAL</code></td></tr>
      <tr><td>Authorized signer</td><td>The public key belongs to the market permission set and verifies the domain-separated observation signature.</td><td><code>LXP_ERR_UNAUTHORIZED_ORACLE</code></td></tr>
      <tr><td>Freshness</td><td>Observation time is no later than sealed batch time, and age does not exceed maximum staleness.</td><td><code>LXP_ERR_TIMESTAMP_REGRESSION</code> or <code>LXP_ERR_ORACLE_STALE</code></td></tr>
      <tr><td>Price range</td><td>The integer price lies between configured minimum and maximum, inclusive.</td><td><code>LXP_ERR_ORACLE_BOUNDS</code></td></tr>
      <tr><td>Observation ordering</td><td>The incoming sequence is strictly greater than the preceding accepted sequence for this market.</td><td><code>LXP_ERR_ORACLE_SEQUENCE</code></td></tr>
      <tr><td>Deviation</td><td>Absolute change does not exceed the prior accepted price multiplied by the deviation limit in basis points.</td><td><code>LXP_ERR_ORACLE_DEVIATION</code></td></tr>
    </tbody></table>
    <p>The deviation allowance is rounded down in integer price units. The first accepted observation has no preceding observation for a relative-deviation comparison, but it must still satisfy authorization, freshness, encoding, and configured bounds.</p>
    <h3>Concrete acceptance example</h3>
    <p>Suppose the last accepted price is 100,000 integer units and the permitted deviation is 200 basis points. The next observation may differ by at most 2,000 units. A price of 101,500 satisfies that deviation check; 103,000 does not. If maximum staleness is 5,000 milliseconds, an observation aged exactly 5,000 milliseconds satisfies the freshness boundary, while 5,001 milliseconds fails it. These are example parameters, not network-wide defaults.</p>
    <h2>Canonical observation format</h2>
    <p>The unsigned canonical observation is 72 bytes (<code>LX_ORACLE_OBSERVATION_BYTES</code>). The signed transport is 137 bytes (<code>LX_ORACLE_TRANSPORT_BYTES</code>): a 1-byte version (<code>LX_ORACLE_TRANSPORT_VERSION</code> = 1), the 72-byte observation, and a 64-byte signature, decoded by <code>lx_perps_oracle_transport_decode</code>. The transport carries no public key; activity authority supplies the signer identity, which must belong to the market&apos;s permitted set. Integer fields use big-endian encoding; price uses unsigned 128-bit units and timestamps use milliseconds. Verification binds the observation to the <code>LXP:ORACLE:OBSERVATION:v1</code> tag under the signature-preimage domain. These formats are defined in <code>include/layerx/lx_oracle.h</code> and <code>src/modules/perps/lx_perps_command.c</code>.</p>
    <pre><code>{`market_id:             32 bytes
observation_sequence:   8 bytes
price:                 16 bytes
observed_at:            8 bytes
source_identifier:      8 bytes`}</code></pre>
    <p>Keep the market price scale with every decoded price. A source identifier describes the submitted observation; it does not replace signer authorization or make an unsigned price acceptable.</p>
    <h2>Risk limits are market-specific</h2>
    <p>Initial margin, maintenance margin, liquidation fees, funding caps, permitted oracle keys, and price acceptance bounds are explicit market parameters. A parameter version identifies the configuration revision. Market creation validates nonzero units and intervals, distinct system accounts, sorted unique permitted keys, and coherent margin and price bounds.</p>
    <p>Market halt state is an additional control. Risk-taking operations and funding execution enforce halt checks. Treat the halt flag as an operation-specific refusal boundary: an integration should inspect the receipt of each action rather than assume that all account or recovery operations have identical halt behavior.</p>
    <h3>What accepted data proves</h3>
    <p>An accepted observation proves that its canonical payload passed the configured signature and market checks at its place in execution. It does not prove the external source was economically correct, guarantee future prices, or remove liquidity and insurance constraints. Application risk policies can impose stricter freshness or exposure limits than the protocol minimums.</p>
    <h2>Integration workflow</h2>
    <ol><li>Load the market identifier, price scale, parameter version, permitted signer set, and risk thresholds.</li><li>Read the latest accepted observation with its sequence, timestamp, source identifier, and price.</li><li>Display observation age and market status beside price-sensitive actions.</li><li>Submit actions using canonical payloads and inspect execution receipts for price, margin, authorization, and halt refusals.</li><li>On a stale, out-of-range, or deviation refusal, obtain a new accepted observation or reassess the request. Repeatedly submitting the same invalid observation cannot make it valid.</li></ol>
    <p>Registered oracle activities persist the latest accepted observation in the committed perps key-value state. The observation retains its sequence, timestamp, source identifier, and signer. Oracle ingestion does not transfer balances; authorized value movements use explicit ledger transfer sets. Keep observation acceptance and financial settlement separate in application accounting.</p>
    <h2>Related documentation</h2><ul><li><Link href="/perpetuals">Perpetual positions, funding, and liquidation</Link></li><li><Link href="/modules/oracle">Native Oracle module interfaces</Link></li><li><Link href="/modules">Module architecture</Link></li></ul>
  </DocsLayout>
}
