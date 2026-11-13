import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/perpetuals')

export default function Perpetuals() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Perpetual Futures</h1><p className="page-description">Trade long and short exposure with explicit margin accounts, deterministic funding, signed market prices, and accountable liquidation settlement.</p></div>
    <h2>A market is a complete risk configuration</h2>
    <p>Paxeer X perpetual markets combine an order book with position, margin, funding, insurance, and liquidation state. Every market identifies its quote asset, administrator, liquidity account, long and short funding accounts, and insurance account. Losses, fees, funding, and margin releases settle through conserved transfer legs in the shared ledger.</p>
    <table><thead><tr><th>Parameter group</th><th>What it controls</th></tr></thead><tbody>
      <tr><td>Contract size, tick size, lot size, price scale</td><td>Contract units, valid price and quantity increments, and integer price interpretation.</td></tr>
      <tr><td>Initial and maintenance margin ratios</td><td>Collateral required to open exposure and the threshold used by maintenance checks.</td></tr>
      <tr><td>Liquidation fee and liquidator share</td><td>The fee charged during liquidation and its division between the liquidator and insurance.</td></tr>
      <tr><td>Funding interval and maximum rate</td><td>Accrual cadence and the permitted funding-rate magnitude.</td></tr>
      <tr><td>Oracle keys, freshness, bounds, deviation</td><td>Which signed observations are acceptable for the market.</td></tr>
      <tr><td>Parameter version and halt state</td><td>The configuration revision and whether risk-taking operations are permitted.</td></tr>
    </tbody></table>
    <p>Initial margin must exceed maintenance margin. Market accounts are distinct, and the permitted oracle set contains up to eight canonical keys. Read the selected market configuration before sizing an order; ratios, intervals, and limits belong to the market rather than a universal exchange default.</p>
    <h2>Open, increase, and close exposure</h2>
    <ol><li>Select a market and inspect its quote asset, price scale, lot size, margin ratios, oracle observation, and halt state.</li><li>Fund your main account with the required quote asset and choose a dedicated margin account.</li><li>Place a buy or sell order with a price and quantity, or submit a position-open activity with side, size, entry notional, and margin amount.</li><li>Opening transfers posted margin from the owner main account into the position margin account. Increase an existing position with explicit size, notional, and margin deltas.</li><li>Monitor accepted prices, accrued funding, margin, and the market maintenance threshold. Close the position to settle exposure and release eligible remaining margin.</li></ol>
    <p>Buy positions express long exposure; sell positions express short exposure. Orders have explicit owner and order identifiers, and cancellations identify both the market and order. A rejected activity does not become an executed position: inspect its receipt before updating a trading interface.</p>
    <h3>Leverage and initial margin</h3>
    <p>Leverage is notional exposure divided by posted margin. A hypothetical market with a 1,000-basis-point initial margin ratio requires at least 1,000 quote units for 10,000 quote units of opening notional, corresponding to 10× exposure at that minimum. Posting 2,000 quote units lowers that ratio to 5×. These figures illustrate the calculation; the selected market defines the actual requirements.</p>
    <h3>Profit and loss</h3>
    <p>For a long position, price profit or loss follows <code>(mark price − entry price) × size ÷ price scale</code>; the sign reverses for a short. Integer settlement rounds positive profit down and loss magnitude up when division leaves a remainder. A long with one normalized unit entered at 100 and marked at 105 has five quote units of price profit before funding and fees.</p>
    <h2>Funding follows sealed batch time</h2>
    <p>Funding ticks use the sealed batch timestamp. Only whole configured funding intervals accrue; a tick before the next complete interval is refused. The cumulative funding index advances by the rate multiplied by elapsed intervals, and a position records the index at entry so its accrued funding can be computed from the difference.</p>
    <p>The funding-rate calculation measures the signed difference between oracle and reference prices relative to the reference price, then caps its magnitude at the market maximum. Positive funding transfers from the long funding account to the short funding account; negative funding reverses that direction. At the position level, positive index movement makes longs pay and shorts receive. Funding payments are rounded conservatively when integer units leave a remainder.</p>
    <p>For example, 10,000 quote units of entry notional and a positive cumulative change of 20 basis points produce 20 quote units of funding owed by a long, before any rounding effects. The same index movement credits a short of equal entry notional.</p>
    <h2>Maintenance, liquidation, and insurance</h2>
    <p>The maintenance check computes mark notional from current price and size, rounds required maintenance collateral upward, and compares it with the supplied margin balance. A balance strictly below that requirement is liquidatable; equality satisfies that check. Applications should keep price P&amp;L, funding, fees, and collateral accounting visible rather than inferring a liquidation price from leverage alone.</p>
    <ol><li>Trading loss is paid from the position margin account to market liquidity, up to the available margin.</li><li>A remaining trading-loss deficit draws on the market insurance account.</li><li>The liquidation fee is limited by margin remaining after losses and split between the liquidator and insurance account.</li><li>Any residual margin returns to the owner main account.</li></ol>
    <p>Insurance is a funded account, not an unlimited guarantee. A liquidation whose deficit exceeds the available insurance balance is refused. Auto-deleveraging accepts an explicitly ordered list of position identifiers. The codec capacity is 128 (<code>LX_PERPS_ADL_CAPACITY</code>), but dispatch refuses more than <code>LX_PERPS_DISPATCH_MAX_ADL</code> (16) positions per activity; canonical sorted ordering makes processing deterministic.</p>
    <h2>Activity reference</h2>
    <table><thead><tr><th>Activity</th><th>Purpose</th></tr></thead><tbody>
      <tr><td><code>MARKET_CREATE / MARKET_HALT</code></td><td>Create a market configuration or change its halt state.</td></tr>
      <tr><td><code>ORACLE_PUSH</code></td><td>Submit a signed market price observation.</td></tr>
      <tr><td><code>ORDER_PLACE / ORDER_CANCEL</code></td><td>Manage explicit buy and sell orders.</td></tr>
      <tr><td><code>POSITION_OPEN / POSITION_INCREASE / POSITION_CLOSE</code></td><td>Manage exposure and its margin lifecycle.</td></tr>
      <tr><td><code>FUNDING_TICK</code></td><td>Accrue completed funding intervals.</td></tr>
      <tr><td><code>LIQUIDATE / ADL</code></td><td>Resolve undercollateralized exposure and ordered deleveraging.</td></tr>
    </tbody></table>
    <p>Perpetual activities use module identifier <code>6</code> and the <code>0x0006xxxx</code> activity family: <code>0x00060001</code>–<code>0x0006000b</code> for the activities above, plus <code>LX_PERPS_MARGIN_REGISTER</code> at <code>0x0006000c</code>. A versioned 130-byte time-in-force order payload is also defined (<code>lx_perps_tif_module_iface</code>). Unknown ordinals, noncanonical payloads, zero amounts, insufficient margin, unauthorized debits, and halted markets produce explicit refusals.</p>
    <h2>Continue building</h2><ul><li><Link href="/oracle-risk">Oracle acceptance and market risk controls</Link></li><li><Link href="/fees">Fees and settlement</Link></li><li><Link href="/modules">Paxeer X modules</Link></li></ul>
  </DocsLayout>
}
