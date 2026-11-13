import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/spot-exchange')

export default function SpotExchange() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Spot Exchange</h1><p className="page-description">Trade asset pairs on Paxeer X with deterministic order books, price-time matching, escrowed liquidity, and immediate two-leg settlement.</p></div>
    <h2>Markets and ownership</h2>
    <p>The LayerX spot module gives each market a base asset, quote asset, tick size, lot size, and administrator. A bid buys the base asset with quote units; an ask sells base units for quote units. Market identifiers, asset identifiers, account identifiers, and order identifiers are 32-byte values. Prices and quantities use unsigned 128-bit integer values: integrations apply the assets&apos; unit conventions before constructing an activity.</p>
    <p>Spot runs in the LayerX execution kernel as module <code>10</code>. It settles through the asset module and uses the same deterministic price-time book engine as perpetuals. Native spot activities and the EVM exchange intent interface are distinct integration surfaces within Paxeer X. A separate Cosmos Go spot module (<code>modules/spot</code>, with swap, vault, shares, stop-order, and TWAP keepers behind <code>precompiles/spot</code>) is a third surface; this page covers the native kernel book and the EVM intent interface.</p>
    <h2>Choose an order</h2>
    <table><thead><tr><th>Order</th><th>Price</th><th>Time in force</th><th>Remaining quantity</th></tr></thead><tbody>
      <tr><td>Limit GTC</td><td>Nonzero limit</td><td>Good till cancelled</td><td>Rests on the book with escrow after matching</td></tr>
      <tr><td>Limit IOC</td><td>Nonzero limit</td><td>Immediate or cancel</td><td>Does not rest</td></tr>
      <tr><td>Market</td><td>Zero in the payload</td><td>Immediate or cancel</td><td>Does not rest</td></tr>
    </tbody></table>
    <p>For native spot, <code>side</code> is bid or ask, <code>kind</code> is <code>1</code> for limit and <code>2</code> for market, and limit time in force is <code>1</code> for GTC or <code>2</code> for IOC. A market order executes against available opposing liquidity. Use a limit IOC when the strategy needs an explicit price boundary.</p>
    <h2>Matching and fills</h2>
    <p>The matching engine selects the best opposing price first, the earliest global sequence second, and the order identifier as the final deterministic tie breaker. Every fill uses the resting maker&apos;s price. Matching reduces the maker&apos;s remaining quantity and escrow; a completely filled maker leaves the book.</p>
    <h3>Worked example</h3>
    <p>An ask offers 5 base units at a price of 100 quote units per base unit. An incoming limit bid for 3 units at 105 crosses that ask. The trade executes at the maker price of 100: the buyer receives 3 base units, the seller receives 300 quote units, and the ask retains 2 units. This example uses whole units for readability; encoded values follow the market&apos;s asset-unit, tick, and lot conventions.</p>
    <h2>Escrow and settlement</h2>
    <p>A resting ask escrows its remaining base quantity. A resting bid escrows the quote amount required by its price and remaining quantity. Each fill settles the base transfer and quote transfer immediately as <code>lxp_transfer_leg</code> entries with reason <code>LXP_REASON_ESCROW_CAPTURE</code> (<code>5</code>); locking escrow uses <code>LXP_REASON_ESCROW_LOCK</code> (<code>4</code>) and refunds use <code>LXP_REASON_ESCROW_RELEASE</code> (<code>6</code>). Spot keeps no separate shadow balance: asset-module ownership, pause rules, account namespaces, and debit authorization apply to settlement.</p>
    <p>After cancellation or execution, reconcile the order&apos;s remaining quantity and the asset account state together. An accepted submission identifies an attempted activity; the resulting receipt and state establish the execution outcome. An agent should record its order identifier before submission and retain the corresponding activity and receipt evidence.</p>
    <h2>Native activity reference</h2>
    <table><thead><tr><th>Activity</th><th>Type</th><th>Canonical payload</th></tr></thead><tbody>
      <tr><td>Create market</td><td><code>0x000A0001</code></td><td>Market, base asset, quote asset, tick size, lot size, administrator</td></tr>
      <tr><td>Place order</td><td><code>0x000A0002</code></td><td>Market, order, base account, quote account, side, kind, time in force, price, quantity</td></tr>
      <tr><td>Cancel order</td><td><code>0x000A0003</code></td><td>Market and order identifiers</td></tr>
      <tr><td>Halt market</td><td><code>0x000A0004</code></td><td>Market identifier</td></tr>
      <tr><td>Resume market</td><td><code>0x000A0005</code></td><td>Market identifier</td></tr>
    </tbody></table>
    <pre><code>{`ORDER_PLACE: 163 bytes
market_id[32] || order_id[32] || base_account_id[32] || quote_account_id[32]
|| side:u8 || kind:u8 || time_in_force:u8 || price:u128be || quantity:u128be

ORDER_CANCEL: 64 bytes
market_id[32] || order_id[32]`}</code></pre>
    <p>Market creation uses a 160-byte payload; halt and resume use 32 bytes. Escrow identifiers are derived from the market and asset rather than supplied by the caller. Encode canonical big-endian integers and exact field widths.</p>
    <h2>Agent trading workflow</h2>
    <ol><li>Resolve the market&apos;s base and quote assets, trading increments, and halted state.</li><li>Check the strategy&apos;s authorized accounts and available asset balances.</li><li>Choose a price boundary, quantity, order kind, and time in force; allocate and persist an order identifier.</li><li>Submit the canonical native activity through the LayerX client and retain its activity identifier.</li><li>Verify the receipt, account changes, fills, and residual order state before updating inventory.</li><li>Cancel a residual quote using its market and order identifiers. Reconcile the cancellation result before treating its reserved inventory as available for another order.</li></ol>
    <p>Market-making agents can quote both sides, cancel stale quotes, and adjust inventory using the same accounts and matching rules as other principals. Strategy decisions remain outside consensus; order authorization, matching, and settlement follow deterministic protocol rules.</p>
    <h2>EVM exchange intents</h2>
    <p>Contracts use <code>ILayerXExchange</code> at <code>0x0000000000000000000000000000000000001015</code> to deposit or withdraw margin, place or cancel orders, request settlement, and inspect intents. These calls record pending exchange intents for LayerX routing. Their EVM events are submission evidence; LayerX executes the corresponding trading operation.</p>
    <p>The interface&apos;s state views require a batch number and native state witness. They verify the witness against the batch&apos;s finalized anchored state root. Its order interface uses its own time-in-force encoding: <code>0</code> GTC, <code>1</code> IOC, <code>2</code> fill or kill, and <code>3</code> post-only. Preserve each surface&apos;s encoding when building an adapter; do not copy native spot enum values into this ABI.</p>
    <h2>Capacity and operational refusals</h2>
    <p>The native module supports 64 markets and 256 resting orders per market. One activity produces at most 8 fills and 18 transfer legs. These are compile-time execution bounds: strategies split large operations and reconcile each result rather than assuming unlimited book traversal.</p>
    <p>A duplicate market returns <code>LXP_ERR_MARKET_ALREADY_EXISTS</code>; order placement on a halted market returns <code>LXP_ERR_MARKET_HALTED</code>. Asset mismatch, paused assets, unauthorized debits, unknown account namespaces, sequence reuse, and noncanonical payloads also refuse execution. Correct the underlying account, market, or encoding condition before submitting a new operation.</p>
    <h2>Continue building</h2>
    <p>See <Link href="/market-makers">Market Makers</Link> for quoting and ramp inventory workflows, <Link href="/precompiles">Precompiles</Link> for contract integration, and <Link href="/modules">Modules</Link> for execution capabilities.</p>
    <div className="source-note"><strong>Protocol references:</strong> <code>include/layerx/lx_spot.h</code>, <code>src/modules/spot/lx_spot_market.c</code>, <code>src/modules/spot/lx_spot_book.c</code>, <code>src/modules/spot/lx_spot_dispatch.c</code>, and <code>precompiles/layerxexchange/LayerXExchange.sol</code>.</div>
  </DocsLayout>
}
