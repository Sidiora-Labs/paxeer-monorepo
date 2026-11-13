import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/perpetuals')

export default function Perpetuals() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Perpetual Futures</h1><p className="page-description">Trade long and short exposure through the native order book, USDL cross margin, governed market parameters and committed transaction receipts.</p></div>
    <h2>A market is a complete risk configuration</h2>
    <p>The perpetuals venue is implemented in <code>modules/perps</code> and exposed to EVM callers at <code>0x1015</code>. Its ABI is <code>precompiles/perps/abi/abi.json</code>; <code>precompiles/perps/Perps.sol</code> declares the Solidity interface. Collateral is the configured USDL ERC-20 with 6 decimal places. Prices, USD notionals, P&amp;L and fees use 18-decimal fixed point. Every precompile method is nonpayable.</p>
    <table><thead><tr><th>Parameter group</th><th>What it controls</th></tr></thead><tbody>
      <tr><td>Tick size, lot size and order-value bounds</td><td>Valid price and base-size increments and permitted order notionals.</td></tr>
      <tr><td>Leverage limits and margin tiers</td><td>Initial margin from leverage and maintenance requirements from the market tiers.</td></tr>
      <tr><td>Liquidation fees, slices and cooldown</td><td>Fees and bounded closing orders used to resolve an undercollateralized cross account.</td></tr>
      <tr><td>Premium sampling and funding cap</td><td>Sampling cadence, interest and premium adjustments, and the hourly rate cap.</td></tr>
      <tr><td>Oracle symbol and price freshness</td><td>The pricefeed index source and whether current prices permit risk-taking operations.</td></tr>
      <tr><td>Market enabled state and venue pause</td><td>Whether a market is active and which venue operations are permitted.</td></tr>
    </tbody></table>
    <p>Read <code>getMarketConfig</code>, <code>getMarginTiers</code>, <code>getMarkPriceDetails</code> and <code>getFeeTier</code> before sizing an order. <code>getMarketConfig</code> returns the market leverage cap as a plain integer; order leverage arguments use 18-decimal fixed point. Margin is shared across the account&apos;s positions.</p>
    <h2>Open, increase, and close exposure</h2>
    <ol><li>Use <code>getActiveMarketIds</code> and inspect the selected market configuration and price freshness.</li><li>Hold the configured collateral token, approve the precompile to spend it, and call <code>deposit</code> to fund the cross account.</li><li>Submit <code>placeOrder</code> with the complete order tuple, including side, size, leverage, time in force, price constraints and deadline.</li><li>Inspect fills, order status and <code>getPositionDetails</code>. Use <code>addCollateral</code> to increase collateral without changing position size.</li><li>Monitor <code>getAccount</code> and funding. Close through <code>closePosition</code>, <code>partialClose</code> or <code>closePositionWithParams</code>, then withdraw eligible collateral.</li></ol>
    <p>The <code>isLong</code> field selects long or short exposure. Cancel an order by <code>cancelOrder(orderId)</code> or <code>cancelByCloid(marketId, cloid)</code>. A successful order submission can leave a resting order or a partial fill; inspect the order and fill events to determine exposure.</p>
    <h3>Leverage and initial margin</h3>
    <p>Leverage is notional exposure divided by posted margin. For example, 10,000 USD of notional against 1,000 USD of collateral represents 10× exposure; against 2,000 USD it represents 5×. These illustrative values do not define a market limit. Initial margin, maintenance tiers and existing cross-account exposure constrain an actual order.</p>
    <h3>Profit and loss</h3>
    <p>For a long position, price P&amp;L follows <code>(mark price − entry price) × sizeBase ÷ 10^18</code> with 18-decimal prices and base size; the sign reverses for a short. Use <code>getPositionDetails</code> for unrealized P&amp;L and funding, and <code>getAccount</code> for account equity and withdrawable collateral. Keep USD fixed-point values separate from 6-decimal USDL amounts.</p>
    <h2>Funding follows block time</h2>
    <p>The venue samples the order-book premium against the pricefeed index using block time. At each new whole hour, <code>SettleFunding</code> in <code>modules/perps/keeper/funding.go</code> updates the cumulative funding index for elapsed hours. Positions realize the index difference when they are settled.</p>
    <p>The hourly rate combines the mean premium with the governed interest and premium clamp, divides the eight-hour rate into hourly units, and applies the governed hourly cap. Funding settlement converts USD values to USDL with payments rounded up and receipts rounded down. The rounding residue stays in the venue.</p>
    <p><code>getCurrentFundingRate</code>, <code>getFundingDetails</code> and <code>getPositionFunding</code> expose funding state. The position funding result is negative when the position pays and positive when it receives. Include funding separately from price P&amp;L when displaying equity.</p>
    <h2>Maintenance, liquidation, and insurance</h2>
    <p>Liquidation operates on the cross account when equity falls below its maintenance requirement. <code>checkLiquidatable</code> and <code>getAccount</code> expose the account risk state. <code>liquidate(positionId)</code> identifies an account through a position, settles its funding, cancels open orders and closes exposure against the order book. Large positions use governed slices and cooldowns.</p>
    <ol><li>Read fresh market prices and recompute cross-account equity and maintenance requirements.</li><li>Cancel open orders and attempt reduce-only market closes within the configured bounds.</li><li>Settle realized P&amp;L, funding and fees against account collateral.</li><li>Handle deficits through insurance, backstop exposure and bounded auto-deleveraging.</li></ol>
    <p>Insurance and auto-deleveraging are handled in <code>modules/perps/keeper/insurance.go</code> and <code>modules/perps/keeper/adl.go</code>. <code>getInsuranceBalance</code> and <code>getADLRank</code> expose their state. End-block processing bounds scans, liquidations and closing slices. The caller recorded in a liquidation event receives no keeper payout.</p>
    <h2>EVM method reference</h2>
    <table><thead><tr><th>Method</th><th>Purpose</th></tr></thead><tbody>
      <tr><td><code>getMarketConfig / getActiveMarketIds</code></td><td>Read governed market parameters and active markets.</td></tr>
      <tr><td><code>getMarkPriceDetails</code></td><td>Read mark, index, midpoint and price freshness.</td></tr>
      <tr><td><code>placeOrder / cancelOrder</code></td><td>Submit a typed order or cancel an owned order.</td></tr>
      <tr><td><code>openPosition / addSize / closePosition</code></td><td>Manage exposure through the order-book compatibility methods.</td></tr>
      <tr><td><code>getFundingDetails / getPositionFunding</code></td><td>Read cumulative funding and the position amount owed or received.</td></tr>
      <tr><td><code>liquidate / getADLRank</code></td><td>Liquidate an undercollateralized account or inspect position deleveraging rank.</td></tr>
    </tbody></table>
    <p>Use the exact tuple layouts and parameterless custom errors in <code>precompiles/perps/Perps.sol</code>. Methods reject attached native value, delegatecall and writes under staticcall. Check the error selector on a reverted call. Verify successful fills and the containing committed block before treating a trade as final.</p>
    <h2>Continue building</h2><ul><li><Link href="/oracle-risk">Oracle acceptance and market risk controls</Link></li><li><Link href="/fees">Fees and settlement</Link></li><li><Link href="/modules">Paxeer X modules</Link></li></ul>
  </DocsLayout>
}
