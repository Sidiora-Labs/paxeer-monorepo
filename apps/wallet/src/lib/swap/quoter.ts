import { ethers } from 'ethers';
import {
  TOKENS,
  getQuote as pecorGetQuote,
  quoteBuy,
  quoteSell,
  quoteMultihop,
  formatAmount,
} from '@/lib/swap/sdk';
import { toCentralSwapAddress, isSidioraToken } from './constants';
import type { SwapToken } from './constants';
import type { SwapQuote, SwapParams } from './types';
import {
  getSwapNetwork,
  quoteNetwork,
  assertQuoteNetwork,
  resolveTokenCapability,
  tokenCapability,
} from './network';
import type { QuoteNetwork } from './network';

// ── Helpers ──────────────────────────────────────────────────────────────────

function fmt(raw: bigint, decimals: number): string {
  return formatAmount(raw, decimals, 6);
}

function isUsdl(token: SwapToken): boolean {
  return !token.isNative && token.address.toLowerCase() === TOKENS.USDL.address.toLowerCase();
}

// ── 1. PECOR V3 — non-launchpad pairs ────────────────────────────────────────

async function getPecorQuote(
  params: SwapParams,
  provider: ethers.Provider
): Promise<SwapQuote | null> {
  const { tokenIn, tokenOut, amountIn } = params;
  if (isSidioraToken(tokenIn) || isSidioraToken(tokenOut)) return null;

  const addrIn = toCentralSwapAddress(tokenIn);
  const addrOut = toCentralSwapAddress(tokenOut);

  try {
    const quote = await pecorGetQuote(addrIn, addrOut, BigInt(amountIn), provider);
    if (!quote.sufficientLiquidity) return null;
    if (quote.priceStaleIn || quote.priceStaleOut) {
      console.warn(
        '[PECOR] Stale oracle price for',
        quote.priceStaleIn ? tokenIn.symbol : tokenOut.symbol
      );
    }
    return {
      protocol: 'pecor',
      protocolLabel: 'PECOR Exchange',
      tokenIn,
      tokenOut,
      amountIn,
      amountOut: quote.amountOut.toString(),
      amountOutDisplay: fmt(quote.amountOut, tokenOut.decimals),
      fee: quote.feeAmount.toString(),
      feeBps: Number(quote.effectiveFeeBps),
      priceImpact: Number(quote.priceImpactBps) / 100,
      sufficient: true,
    };
  } catch (err) {
    console.warn('[PECOR] Quote failed:', err);
    return null;
  }
}

// ── 2. Sidiora direct — USDL ↔ Launchpad ────────────────────────────────────

async function getSidioraDirectQuote(
  params: SwapParams,
  provider: ethers.Provider
): Promise<SwapQuote | null> {
  const { tokenIn, tokenOut, amountIn } = params;

  const usdlIn = isUsdl(tokenIn) && isSidioraToken(tokenOut);
  const usdlOut = isUsdl(tokenOut) && isSidioraToken(tokenIn);
  if (!usdlIn && !usdlOut) return null;

  const launchpadToken = isSidioraToken(tokenIn) ? tokenIn : tokenOut;
  const amountInBn = BigInt(amountIn);

  try {
    const pool = poolFor(launchpadToken);
    if (!pool) return null;

    if (usdlIn) {
      const q = await quoteBuy(pool, amountInBn, provider);
      if (q.amountOut === BigInt(0)) return null;
      return {
        protocol: 'sidiora',
        protocolLabel: 'Sidiora Launchpad',
        tokenIn,
        tokenOut,
        amountIn,
        amountOut: q.amountOut.toString(),
        amountOutDisplay: fmt(q.amountOut, tokenOut.decimals),
        fee: q.feeAmount.toString(),
        feeBps: 100,
        priceImpact: Number(q.priceImpactBps) / 100,
        sufficient: true,
        poolAddress: pool,
      };
    } else {
      const q = await quoteSell(pool, amountInBn, provider);
      if (q.amountOut === BigInt(0)) return null;
      return {
        protocol: 'sidiora',
        protocolLabel: 'Sidiora Launchpad',
        tokenIn,
        tokenOut,
        amountIn,
        amountOut: q.amountOut.toString(),
        amountOutDisplay: fmt(q.amountOut, tokenOut.decimals),
        fee: q.feeAmount.toString(),
        feeBps: 100,
        priceImpact: Number(q.priceImpactBps) / 100,
        sufficient: true,
        poolAddress: pool,
      };
    }
  } catch (err) {
    console.warn('[Sidiora] Direct quote failed:', err);
    return null;
  }
}

// ── 3. Sidiora multihop — Launchpad ↔ Launchpad ──────────────────────────────

async function getSidioraMultihopQuote(
  params: SwapParams,
  provider: ethers.Provider
): Promise<SwapQuote | null> {
  const { tokenIn, tokenOut, amountIn } = params;
  if (!isSidioraToken(tokenIn) || !isSidioraToken(tokenOut)) return null;

  try {
    const q = await quoteMultihop(tokenIn.address, tokenOut.address, BigInt(amountIn), provider);
    if (q.amountOut === BigInt(0)) return null;
    return {
      protocol: 'sidiora-multihop',
      protocolLabel: 'Sidiora Multihop',
      tokenIn,
      tokenOut,
      amountIn,
      amountOut: q.amountOut.toString(),
      amountOutDisplay: fmt(q.amountOut, tokenOut.decimals),
      fee: (q.sellFeeAmount + q.buyFeeAmount).toString(),
      feeBps: 150,
      priceImpact: Number(q.combinedPriceImpactBps) / 100,
      sufficient: true,
    };
  } catch (err) {
    console.warn('[Sidiora] Multihop quote failed:', err);
    return null;
  }
}

// ── 4. PECOR V4 multihop — ERC20 ↔ Launchpad (via USDL bridge) ───────────────
// Route: ERC20 →[VaultAdapter]→ USDL →[SidioraAdapter]→ Launchpad (and reverse)
// Executed atomically via PECORRouter.swapMultiHop.
// Native PAX is excluded (V4 Router is not payable).

async function getV4MultihopQuote(
  params: SwapParams,
  provider: ethers.Provider
): Promise<SwapQuote | null> {
  const { tokenIn, tokenOut, amountIn } = params;

  const inIsSidiora = isSidioraToken(tokenIn);
  const outIsSidiora = isSidioraToken(tokenOut);
  if (inIsSidiora === outIsSidiora) return null; // both or neither

  const launchpad = inIsSidiora ? tokenIn : tokenOut;
  const pecorSide = inIsSidiora ? tokenOut : tokenIn;

  // USDL handled by Sidiora direct; native PAX not supported (non-payable router)
  if (isUsdl(pecorSide) || pecorSide.isNative) return null;

  const amountInBn = BigInt(amountIn);
  const p = provider;

  try {
    const pool = poolFor(launchpad);
    if (!pool) return null;

    if (!inIsSidiora) {
      // ERC20 → USDL (PECOR) → Launchpad (Sidiora)
      const addrIn = toCentralSwapAddress(tokenIn);
      const pecorQ = await pecorGetQuote(addrIn, TOKENS.USDL.address, amountInBn, p);
      if (!pecorQ.sufficientLiquidity || pecorQ.amountOut === BigInt(0)) return null;

      const sidQ = await quoteBuy(pool, pecorQ.amountOut, p);
      if (sidQ.amountOut === BigInt(0)) return null;

      return {
        protocol: 'pecor-v4-multihop',
        protocolLabel: 'PECOR → Sidiora',
        tokenIn,
        tokenOut,
        amountIn,
        amountOut: sidQ.amountOut.toString(),
        amountOutDisplay: fmt(sidQ.amountOut, tokenOut.decimals),
        fee: (pecorQ.feeAmount + sidQ.feeAmount).toString(),
        feeBps: Number(pecorQ.feeBps) + 100,
        priceImpact: Number(sidQ.priceImpactBps) / 100,
        sufficient: true,
        intermediateAmount: pecorQ.amountOut.toString(),
        poolAddress: pool,
      };
    } else {
      // Sidiora token → USDL (Sidiora) → ERC20 (PECOR)
      const sidQ = await quoteSell(pool, amountInBn, p);
      if (sidQ.amountOut === BigInt(0)) return null;

      const addrOut = toCentralSwapAddress(tokenOut);
      const pecorQ = await pecorGetQuote(TOKENS.USDL.address, addrOut, sidQ.amountOut, p);
      if (!pecorQ.sufficientLiquidity || pecorQ.amountOut === BigInt(0)) return null;

      return {
        protocol: 'pecor-v4-multihop',
        protocolLabel: 'Sidiora → PECOR',
        tokenIn,
        tokenOut,
        amountIn,
        amountOut: pecorQ.amountOut.toString(),
        amountOutDisplay: fmt(pecorQ.amountOut, tokenOut.decimals),
        fee: (sidQ.feeAmount + pecorQ.feeAmount).toString(),
        feeBps: 100 + Number(pecorQ.feeBps),
        priceImpact: Number(sidQ.priceImpactBps) / 100,
        sufficient: true,
        intermediateAmount: sidQ.amountOut.toString(),
        poolAddress: pool,
      };
    }
  } catch (err) {
    console.warn('[V4 Multihop] Quote failed:', err);
    return null;
  }
}

// ── Aggregator ───────────────────────────────────────────────────────────────

export async function getSwapQuotes(params: SwapParams): Promise<NetworkSwapQuote[]> {
  params = { ...params, tokenIn: { ...params.tokenIn }, tokenOut: { ...params.tokenOut } };
  const source = quoteNetwork();
  const provider = getSwapNetwork().provider;
  const [input, output] = await Promise.all([
    resolveTokenCapability(params.tokenIn),
    resolveTokenCapability(params.tokenOut),
  ]);
  assertQuoteNetwork(source);
  if (input.state === 'unsupported' || output.state === 'unsupported') return [];
  const { tokenIn, tokenOut } = params;
  const inIsSidiora = isSidioraToken(tokenIn);
  const outIsSidiora = isSidioraToken(tokenOut);

  const quoters: Promise<SwapQuote | null>[] = [];

  if (!inIsSidiora && !outIsSidiora) {
    // Pure PECOR pairs
    quoters.push(getPecorQuote(params, provider));
  } else if (inIsSidiora && outIsSidiora) {
    // Sidiora ↔ Sidiora — multihop via USDL bridge
    quoters.push(getSidioraMultihopQuote(params, provider));
  } else {
    // Mixed: one side is Sidiora
    quoters.push(getSidioraDirectQuote(params, provider)); // USDL ↔ Sidiora token
    quoters.push(getV4MultihopQuote(params, provider)); // ERC20 ↔ Sidiora token via V4
  }

  const results = await Promise.allSettled(quoters);
  assertQuoteNetwork(source);
  const quotes: NetworkSwapQuote[] = [];
  for (const r of results) {
    if (r.status === 'fulfilled' && r.value) quotes.push({ ...r.value, network: source });
  }

  quotes.sort((a, b) => {
    const ao = BigInt(a.amountOut);
    const bo = BigInt(b.amountOut);
    return bo > ao ? 1 : bo < ao ? -1 : 0;
  });

  return quotes;
}

export function getBestQuote(quotes: SwapQuote[]): SwapQuote | null {
  const best = quotes[0];
  if (!best) return null;
  if (best.network) assertQuoteNetwork(best.network);
  return best;
}

export type NetworkSwapQuote = SwapQuote & { network: QuoteNetwork };

function poolFor(token: SwapToken): string | null {
  const capability = tokenCapability(token);
  return capability.state === 'launchpad' ? capability.poolAddress : null;
}
