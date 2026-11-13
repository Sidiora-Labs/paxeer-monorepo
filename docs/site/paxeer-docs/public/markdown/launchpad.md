<!-- Source: https://docs.paxeer.app/launchpad/ -->

# Launchpad

Create fixed-supply instruments, open virtual-reserve markets, trade with explicit limits, and direct accumulated fees.

## From issuance to an active market

Paxeer X Launchpad deploys a CREATE2 `SidioraERC20` token, creates its SidioraPool, mints an NFT fee right, and opens a virtual-reserve curve in one market-creation workflow. The market's native denomination is `launchpad/` followed by the pool address hex. Human and agent-controlled businesses can issue instruments while trading agents buy and sell those instruments through explicit transaction permissions.

The instrument represents the issuer's declared terms. Creating a market does not establish equity, revenue rights, or a legal claim automatically. Keep the business identity, creator account, token address, and instrument description visible in your application.

1. Choose a name, symbol, fee strategy, and optical address (zero address when unused); fund the creator with the configured quote asset and transaction gas.
2. Submit `createMarket(name, symbol, feeStrategy, optical)` to the Launchpad precompile; it returns `(token, pool, nftId)`. A permit variant, `createMarketWithPermit`, takes the same arguments plus permit parameters.
3. Read the confirmed `MarketCreated` event and retain the token address, pool address, NFT id, and native denomination.
4. Inspect reserves and a fresh buy or sell quote before signing an order.
5. Operate fee distribution and market pause controls under the appropriate authority.

## Creation and supply

The creation fee is paid in the configured quote token. Launchpad deploys the token with its fixed supply, binds it to the pool with `bindPool`, and transfers the supply to the pool. Market indices start at one and increase per market; the denomination is `launchpad/<pool hex>`, not a numbered subdenomination. The creator begins as guardian and fee-rights holder, and the fee right is represented by the minted NFT (`nftId`).

Names contain at most 64 bytes and symbols at most 16. Both are nonblank UTF-8 without surrounding whitespace or control characters. Market supply and initial reserve settings come from protocol parameters rather than a per-call supply argument.

## Curve reserves and executable prices

A market tracks virtual quote reserve, real quote balance, and token reserve. Effective quote reserve is the sum of virtual and real quote reserves. Virtual liquidity shapes the pricing curve; it is not a funded quote balance that can be withdrawn. Sells remain constrained by real quote liquidity.

The market record includes total supply, cumulative volume, creation time, accumulated fees, pause state, and an eight-slot price-snapshot ring. Use `getPoolPrice(pool)` for a reference price, `getPoolStats(pool)` for reserves, volume, current fee, and market cap, and `quoteExactInput(pool, amountIn, isBuy)` for an amount-specific estimate returning amount out, fee amount, and price impact. `quoteMultihop` quotes token-to-token routes through the quote asset.

## Trade with slippage and expiry limits

The precompile is `0x0000000000000000000000000000000000001017`. `buy(pool, usdlAmountIn, minTokensOut, deadline)` and `sell(pool, tokenAmountIn, minUsdlOut, deadline)` are keyed by pool address and take no recipient argument; output goes to the caller. Permit variants (`buyWithPermit`, `sellWithPermit`) and the multihop `swapTokenForToken(tokenIn, tokenOut, amountIn, minAmountOut, deadline)` are also exposed. Both calls are nonpayable: EVM native value is not the quote payment.

```
// launchpad is an ethers Contract using the Launchpad ABI and signer.
// pool and quoteIn come from the selected market and user input.
const quote = await launchpad.quoteExactInput(pool, quoteIn, true)
const minOut = quote.amountOut * 9900n / 10000n // 1% tolerance
const deadline = BigInt(Math.floor(Date.now() / 1000) + 120)
const tx = await launchpad.buy(pool, quoteIn, minOut, deadline)
await tx.wait()
```

This template assumes the connected account has the required quote balance (and allowance, or a permit) and gas. The pool address identifies the market; do not substitute the token or quote asset address. A stale quote does not reserve liquidity. The signed minimum output and deadline protect execution when reserves or fees change.

### Buy and sell fee behavior

Fees are charged in the input asset. On a buy, the protocol share of quote fees goes to the treasury and the remainder accumulates for the market's fee-rights holder. On a sell, token fees remain in the token reserve. Dynamic fees use market age and volatility, clamped to configured bounds; a quote supplies the effective rate for that trade.

## Choose where accumulated fees go

| Value | Strategy | Execution |
| --- | --- | --- |
| 0 | Claim | `claimFees(nftId)` pays accumulated quote fees to the fee-rights holder; the accumulator-level `claim(pool, recipient)` pays a chosen recipient. |
| 1 | Burn | `executeBurn(nftId)` sends accumulated quote fees to the dead account. |
| 2 | Airdrop | `executeAirdrop(nftId)` opens a holder distribution epoch. |
| 3 | LP rewards | `executeLpRewards(nftId)` adds accumulated quote fees to real curve reserves. |

The fee-rights holder changes strategy with `setFeeStrategy(nftId, newStrategy)` and executes the selected strategy. Fee functions on this surface are keyed by `nftId`; accumulator-level functions are keyed by pool (for example `getAccumulatedFees(pool)`). Burn here directs quote fees to the dead account; it does not burn the launched token's fixed supply. LP rewards reinforce the market's quote backing rather than issuing a separate LP token.

### Holder airdrop epochs

Each airdrop fixes its distribution amount, holder history, supply, payout denomination, and block height. Entitlement is the integer floor of epoch amount multiplied by the holder's boundary balance divided by boundary supply. Later transfers do not rewrite the historical entitlement. A holder claims the current epoch through `claimAirdrop(nftId)` (or the accumulator-level `claimAirdrop(pool, holder)`); each holder claims an epoch once. Use `claimAirdropEpoch(nftId, epoch)` to claim a specified opened epoch.

## Permissions and market controls

| Actor | Authority |
| --- | --- |
| Creator | Funds creation and starts as guardian and fee-rights holder. |
| Guardian | Pauses and unpauses market trading. |
| Fee-rights holder | Changes fee strategy and executes fee disposition. |
| Trader | Spends its own input balance and selects minimum output and deadline; output is paid to the caller. |
| Token holder | Claims eligible airdrop entitlement. |
| Governance authority | Updates protocol parameters through the governance-authorized parameter message. |

Paused markets reject swaps. Orders also reject expired deadlines, zero input, insufficient balances or real liquidity, and output below the signed minimum. These are transaction conditions your interface should explain before prompting for a signature.

## Read markets and track receipts

`getAllPools(offset, limit)` paginates discovery; `getPoolCount()` returns the total; `getPoolsByCreator(creator)` groups issuance by creator; and `getPoolByToken(token)`, `getNftIdByPool(pool)`, and `getPoolMetadata(pool)` resolve one market. `getPoolStats`, `getPoolPrice`, and `getMarketCap` support trading views. Markets exist in Generation 1 and Generation 2 SidioraPool forms; new markets are created as Generation 2.

Index `MarketCreated`, `Swap`, `FeeFlow`, `FeeDistributed`, `FeeStrategyChanged`, `PoolRegistered`, `PoolStateUpdated`, and `TokenDeployed`. Reconcile event observations against the confirmed market record and balances before showing a completed issuance, trade, or claim.

## Protocol settings

The default quote token is the USDL ERC-20 at `0x85FcD13735F4309833A503EE804ea32395851479`. Reference defaults set virtual quote reserve to `10000000000`, token supply (virtual token default) to `1000000000000000` base units, and creation fee to `1000000` quote base units. Fee bounds start at 10–300 basis points with a base of 30, and the protocol receives 1000 basis points of buy fees. Governance can update these values; read current settings (for example `creationFee()`, `baseFeeBps()`, `virtualUsdlDefault()`) before presenting a launch cost or fee estimate.

Continue with [asset registration and supply](https://docs.paxeer.app/assets), [exchange trading](https://docs.paxeer.app/spot-exchange), [precompile integration](https://docs.paxeer.app/precompiles), and [SDKs](https://docs.paxeer.app/sdk).
