import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/launchpad')

export default function Launchpad() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Launchpad</h1><p className="page-description">Create fixed-supply instruments, open virtual-reserve markets, trade with explicit limits, and direct accumulated fees.</p></div>
    <h2>From issuance to an active market</h2>
    <p>Paxeer X Launchpad deploys a CREATE2 <code>SidioraERC20</code> token, installs its SidioraPool surface, mints an NFT fee right, and opens a virtual-reserve curve in one transaction. The market denomination is <code>launchpad/</code> followed by the pool&apos;s checksummed <code>0x</code>-prefixed address. The implementation is in <code>modules/launchpad/</code> and <code>precompiles/launchpad/</code>.</p>
    <p>The instrument represents the issuer&apos;s declared terms. Creating a market does not establish equity, revenue rights, or a legal claim automatically. Keep the business identity, creator account, token address, and instrument description visible in your application.</p>
    <ol><li>Choose a name, symbol, fee strategy, and optical hook address (zero when unused; a nonzero hook must be approved); fund the creator with the configured quote asset and transaction gas.</li><li>Submit <code>createMarket(name, symbol, feeStrategy, optical)</code> to the Launchpad precompile; it returns <code>(token, pool, nftId)</code>. A permit variant, <code>createMarketWithPermit</code>, takes the same arguments plus permit parameters.</li><li>Read the committed <code>MarketCreated</code> and <code>PoolRegistered</code> events and retain the token address, pool address, NFT id, and market denomination. The NFT id is supplied by <code>PoolRegistered</code> or a registry query.</li><li>Inspect reserves and a fresh buy or sell quote before signing an order.</li><li>Operate fee distribution and market pause controls under the appropriate authority.</li></ol>
    <h2>Creation and supply</h2>
    <p>The creation fee is paid in the configured quote token. Launchpad deploys the token with its fixed supply, binds it to the pool with <code>bindPool</code>, and transfers the supply to the pool. Market indices start at one and increase per market; the denomination is <code>launchpad/0x&lt;checksummed pool hex&gt;</code>. The creator begins as guardian and fee-rights holder, and the fee right is represented by the minted NFT (<code>nftId</code>).</p>
    <p>Names contain at most 64 bytes and symbols at most 16. Both are nonblank UTF-8 without surrounding whitespace or control characters. Market supply and initial reserve settings come from protocol parameters rather than a per-call supply argument.</p>
    <h2>Curve reserves and executable prices</h2>
    <p>A market tracks virtual quote reserve, real quote balance, and token reserve. Effective quote reserve is the sum of virtual and real quote reserves. Virtual liquidity shapes the pricing curve; it is not a funded quote balance that can be withdrawn. Sells remain constrained by real quote liquidity.</p>
    <p>The market record includes total supply, cumulative volume, creation time, accumulated fees, pause state, and an eight-slot price-snapshot ring. Use <code>getPoolPrice(pool)</code> for a reference price, <code>getPoolStats(pool)</code> for reserves, volume, current fee, and market cap, and <code>quoteExactInput(pool, amountIn, isBuy)</code> for an amount-specific estimate returning amount out, fee amount, and price impact. <code>quoteMultihop</code> quotes token-to-token routes through the quote asset.</p>
    <h2>Trade with slippage and expiry limits</h2>
    <p>The precompile is <code>0x0000000000000000000000000000000000001017</code>. <code>buy(pool, usdlAmountIn, minTokensOut, deadline)</code> and <code>sell(pool, tokenAmountIn, minUsdlOut, deadline)</code> are keyed by pool address and take no recipient argument; output goes to the caller. Permit variants (<code>buyWithPermit</code>, <code>sellWithPermit</code>) and the multihop <code>swapTokenForToken(tokenIn, tokenOut, amountIn, minAmountOut, deadline)</code> are also exposed. Both calls are nonpayable: EVM native value is not the quote payment.</p>
    <pre><code>{`// launchpad is an ethers Contract using the Launchpad ABI and signer.
// pool and quoteIn come from the selected market and user input.
const quote = await launchpad.quoteExactInput(pool, quoteIn, true)
const minOut = quote.amountOut * 9900n / 10000n // 1% tolerance
const deadline = BigInt(Math.floor(Date.now() / 1000) + 120)
const tx = await launchpad.buy(pool, quoteIn, minOut, deadline)
await tx.wait()`}</code></pre>
    <p>This template assumes the connected account has the required quote balance (and allowance, or a permit) and gas. The pool address identifies the market; do not substitute the token or quote asset address. A stale quote does not reserve liquidity. The signed minimum output and deadline protect execution when reserves or fees change.</p>
    <h3>Buy and sell fee behavior</h3>
    <p>Fees are charged in the input asset. On a buy, the protocol share of quote fees goes to the treasury and the remainder accumulates for the market&apos;s fee-rights holder. On a sell, token fees remain in the token reserve. Dynamic fees use market age and volatility, clamped to configured bounds; a quote supplies the effective rate for that trade.</p>
    <h2>Choose where accumulated fees go</h2>
    <table><thead><tr><th>Value</th><th>Strategy</th><th>Execution</th></tr></thead><tbody>
      <tr><td>0</td><td>Claim</td><td><code>claimFees(nftId)</code> pays accumulated quote fees to the fee-rights holder; the accumulator-level <code>claim(pool, recipient)</code> requires the fees-router role and pays a chosen recipient.</td></tr>
      <tr><td>1</td><td>Burn</td><td><code>executeBurn(nftId)</code> sends accumulated quote fees to the dead account.</td></tr>
      <tr><td>2</td><td>Airdrop</td><td><code>executeAirdrop(nftId)</code> opens a holder distribution epoch.</td></tr>
      <tr><td>3</td><td>LP rewards</td><td><code>executeLpRewards(nftId)</code> adds accumulated quote fees to real curve reserves.</td></tr>
    </tbody></table>
    <p>The fee-rights holder changes strategy with <code>setFeeStrategy(nftId, newStrategy)</code> and executes the selected strategy. Fee functions on this surface are keyed by <code>nftId</code>; accumulator-level functions are keyed by pool (for example <code>getAccumulatedFees(pool)</code>). Burn here directs quote fees to the dead account; it does not burn the launched token&apos;s fixed supply. LP rewards reinforce the market&apos;s quote backing rather than issuing a separate LP token.</p>
    <h3>Holder airdrop epochs</h3>
    <p>Each airdrop records its distribution amount, eligible supply, and snapshot block. Eligible supply excludes the pool and dead-account balances. A claim normally receives the integer floor of epoch amount multiplied by the holder&apos;s snapshot balance divided by eligible supply; the final eligible claimant receives the remaining rounding residue. The snapshot is the block before the epoch opens, so later transfers do not rewrite entitlement. A holder claims the current epoch through <code>claimAirdrop(nftId)</code> (or the accumulator-level <code>claimAirdrop(pool, holder)</code>); each holder claims an epoch once. Use <code>claimAirdropEpoch(nftId, epoch)</code> to claim a specified opened epoch.</p>
    <h2>Permissions and market controls</h2>
    <table><thead><tr><th>Actor</th><th>Authority</th></tr></thead><tbody>
      <tr><td>Creator</td><td>Funds creation and starts as guardian and fee-rights holder.</td></tr><tr><td>Guardian</td><td>Pauses and unpauses market trading.</td></tr><tr><td>Fee-rights holder</td><td>Changes fee strategy and executes fee disposition.</td></tr><tr><td>Trader</td><td>Spends its own input balance and selects minimum output and deadline; output is paid to the caller.</td></tr><tr><td>Token holder</td><td>Claims eligible airdrop entitlement.</td></tr><tr><td>Governance authority</td><td>Updates protocol parameters through the governance-authorized parameter message.</td></tr>
    </tbody></table>
    <p>Paused markets reject swaps. Orders also reject expired deadlines, zero input, insufficient balances or real liquidity, and output below the signed minimum. These are transaction conditions your interface should explain before prompting for a signature.</p>
    <h2>Read markets and track receipts</h2>
    <p><code>getAllPools(offset, limit)</code> paginates discovery; <code>getPoolCount()</code> returns the total; <code>getPoolsByCreator(creator)</code> groups issuance by creator; and <code>getPoolByToken(token)</code>, <code>getNftIdByPool(pool)</code>, and <code>getPoolMetadata(pool)</code> resolve one market. <code>getPoolStats</code>, <code>getPoolPrice</code>, and <code>getMarketCap</code> support trading views. Markets exist in Generation 1 and Generation 2 SidioraPool forms; new markets are created as Generation 2.</p>
    <p>Index <code>MarketCreated</code>, <code>Swap</code>, <code>FeeFlow</code>, <code>FeeDistributed</code>, <code>FeeStrategyChanged</code>, <code>PoolRegistered</code>, <code>PoolStateUpdated</code>, and <code>TokenDeployed</code>. Use <code>precompiles/launchpad/abi.json</code> for the event family and the role ABIs under <code>precompiles/launchpad/abi/</code> for role events: their signatures differ, and the Router&apos;s <code>MarketCreated</code> includes the NFT id. Reconcile observations against the committed market record and balances before showing a completed issuance, trade, or claim.</p>
    <h2>Protocol settings</h2>
    <p>The default quote token is the USDL ERC-20 at <code>0x85FcD13735F4309833A503EE804ea32395851479</code>. Defaults in <code>modules/launchpad/types/params.go</code> set virtual quote reserve to <code>10000000000</code>, token supply to <code>1000000000000000</code> base units, and creation fee to <code>1000000</code> quote base units. Fee bounds are 10–300 basis points with a base of 30, and the protocol receives 1000 basis points of buy fees. Read current settings through <code>creationFee()</code>, <code>baseFeeBps()</code>, and <code>virtualUsdlDefault()</code> before estimating costs. Parameter updates require governance authority; EVM administrator setters are refused.</p>
    <p>Continue with <Link href="/assets">asset registration and supply</Link>, <Link href="/spot-exchange">exchange trading</Link>, <Link href="/precompiles">precompile integration</Link>, and <Link href="/sdk">SDKs</Link>.</p>
  </DocsLayout>
}
