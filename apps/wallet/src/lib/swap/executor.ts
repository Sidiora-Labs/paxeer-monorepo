import { ethers } from 'ethers';
import {
  TOKENS,
  CONTRACTS,
  NATIVE_TOKEN,
  MAX_UINT256,
  PECOR_ABI,
  PECOR_ROUTER_ABI,
  ERC20_ABI,
  SIDIORA_ROUTER_ABI,
  approveVault,
  approveRouter as sidioraApproveRouter,
} from '@/lib/swap/sdk';
import {
  toCentralSwapAddress,
  isSidioraToken,
  VAULT_ADAPTER_ID,
  SIDIORA_ADAPTER_ID,
} from './constants';
import type { SwapQuote, SwapParams } from './types';

export interface SwapResult {
  hash: string;
  receipt: ethers.TransactionReceipt;
}

export class SwapRevertedError extends Error {
  constructor(public readonly receipt: ethers.TransactionReceipt) {
    super(`Swap transaction reverted: ${receipt.hash}`);
    this.name = 'SwapRevertedError';
  }
}

function swapResult(receipt: ethers.TransactionReceipt): SwapResult {
  if (receipt.status === 0) throw new SwapRevertedError(receipt);
  return { hash: receipt.hash, receipt };
}

async function awaitSwapReceipt(tx: ethers.TransactionResponse): Promise<SwapResult> {
  const receipt = await tx.wait();
  if (!receipt) throw new Error(`Swap transaction has no receipt: ${tx.hash}`);
  return swapResult(receipt);
}

// ── Approval helper (approves PECORRouter for ERC20 swaps) ─────────────────
export async function ensureApproval(
  signer: ethers.Signer,
  tokenAddress: string
): Promise<boolean> {
  return approveVault(tokenAddress, signer, MAX_UINT256);
}

// ── Execute swap — dispatches by protocol ───────────────────────────────────
//
// Accepts any ethers.Signer so the same SDK drives `EmbeddedSigner`
// (Paxeer-managed, delegates `sendTransaction` to
// connect.paxportwallet.com) and `FundedSigner`. Read calls
// inside this module hit `signer.provider` regardless of custody model.
export async function executeSwap(
  signer: ethers.Signer,
  quote: SwapQuote,
  params: SwapParams
): Promise<SwapResult> {
  try {
    switch (quote.protocol) {
      case 'pecor':
        return await executePecorSwap(signer, quote, params);
      case 'sidiora':
        return await executeSidioraSwap(signer, quote, params);
      case 'sidiora-multihop':
        return await executeSidioraMultihop(signer, quote, params);
      case 'pecor-v4-multihop':
        return await executeV4Multihop(signer, quote, params);
      default:
        throw new Error(`Unknown swap protocol: ${(quote as SwapQuote).protocol}`);
    }
  } catch (error) {
    if (ethers.isError(error, 'CALL_EXCEPTION') && error.receipt?.status === 0) {
      throw new SwapRevertedError(error.receipt);
    }
    throw error;
  }
}

// ── PECOR V3 ─────────────────────────────────────────────────────────────────

async function executePecorSwap(
  signer: ethers.Signer,
  quote: SwapQuote,
  params: SwapParams
): Promise<SwapResult> {
  const tokenIn = toCentralSwapAddress(quote.tokenIn);
  const tokenOut = toCentralSwapAddress(quote.tokenOut);
  const amountIn = BigInt(quote.amountIn);
  const slipBps = params.slippageBps;
  const dl = BigInt(Math.floor(Date.now() / 1000) + (params.deadline ?? 300));
  const isNativeIn = tokenIn === NATIVE_TOKEN.address;
  const isNativeOut = tokenOut === NATIVE_TOKEN.address;
  const actualIn = isNativeIn ? NATIVE_TOKEN.wrappedAddress : tokenIn;
  const actualOut = isNativeOut ? NATIVE_TOKEN.wrappedAddress : tokenOut;

  const amountOutMin = (BigInt(quote.amountOut) * BigInt(10000 - slipBps)) / BigInt(10000);

  const pecor = new ethers.Contract(CONTRACTS.PECOR, PECOR_ABI, signer);
  let tx: ethers.TransactionResponse;
  if (isNativeIn) {
    // Native PAX → ERC20: PECOR contract handles payable native swaps
    tx = await pecor.swapExactInNative(actualOut, amountOutMin, dl, { value: amountIn });
  } else if (isNativeOut) {
    // ERC20 → Native PAX: PECOR contract handles native output
    await approveVault(tokenIn, signer, amountIn);
    tx = await pecor.swapExactInToNative(tokenIn, amountIn, amountOutMin, dl);
  } else {
    // ERC20 → ERC20: use PECOR.swapExactIn — same path as PECORQuoter.quoteExactIn
    await approveVault(tokenIn, signer, amountIn);
    tx = await pecor.swapExactIn(actualIn, actualOut, amountIn, amountOutMin, dl);
  }
  return awaitSwapReceipt(tx);
}

// ── Sidiora direct buy/sell ───────────────────────────────────────────────

async function executeSidioraSwap(
  signer: ethers.Signer,
  quote: SwapQuote,
  params: SwapParams
): Promise<SwapResult> {
  const pool = quote.poolAddress;
  if (!pool) throw new Error('Sidiora quote missing pool address');
  const dl = BigInt(Math.floor(Date.now() / 1000) + (params.deadline ?? 300));
  const amountIn = BigInt(quote.amountIn);
  const slipBps = params.slippageBps;
  const minOut = (BigInt(quote.amountOut) * BigInt(10000 - slipBps)) / 10000n;
  const router = new ethers.Contract(CONTRACTS.Router, SIDIORA_ROUTER_ABI, signer);

  let tx: ethers.TransactionResponse;
  if (isSidioraToken(quote.tokenIn)) {
    // Sell: Sidiora token → USDL
    await sidioraApproveRouter(quote.tokenIn.address, signer);
    tx = await router.sell(pool, amountIn, minOut, dl);
  } else {
    // Buy: USDL → Launchpad
    await sidioraApproveRouter(TOKENS.USDL.address, signer);
    tx = await router.buy(pool, amountIn, minOut, dl);
  }
  return awaitSwapReceipt(tx);
}

// ── Sidiora token-to-token multihop ────────────────────────────────────────────

type SidioraMultihopArguments = [
  tokenIn: string,
  tokenOut: string,
  amountIn: bigint,
  minAmountOut: bigint,
  deadline: bigint,
];

type SidioraMultihopReturn = [amountOut: bigint, intermediateUsdl: bigint];

export function prepareSidioraMultihop(
  runner: ethers.ContractRunner,
  quote: SwapQuote,
  params: SwapParams
) {
  const router = new ethers.Contract(CONTRACTS.Router, SIDIORA_ROUTER_ABI, runner);
  const method: ethers.ContractMethod<SidioraMultihopArguments, SidioraMultihopReturn> =
    router.getFunction('swapTokenForToken');
  const args: SidioraMultihopArguments = [
    quote.tokenIn.address,
    quote.tokenOut.address,
    BigInt(quote.amountIn),
    (BigInt(quote.amountOut) * BigInt(10000 - params.slippageBps)) / 10000n,
    BigInt(Math.floor(Date.now() / 1000) + (params.deadline ?? 300)),
  ];
  return { router, method, args };
}

async function executeSidioraMultihop(
  signer: ethers.Signer,
  quote: SwapQuote,
  params: SwapParams
): Promise<SwapResult> {
  const { method, args } = prepareSidioraMultihop(signer, quote, params);
  await sidioraApproveRouter(quote.tokenIn.address, signer);
  const tx = await method.send(...args);
  return awaitSwapReceipt(tx);
}

// ── Atomic Sidiora ↔ PECOR swap via the USDL bridge ────────────────────────

type V4Hop = Readonly<{
  tokenIn: string;
  tokenOut: string;
  adapterId: string;
  minAmountOut: bigint;
}>;

type V4MultihopArguments = [
  hops: readonly V4Hop[],
  amountIn: bigint,
  amountOutMin: bigint,
  deadline: bigint,
];

export function prepareV4Multihop(
  runner: ethers.ContractRunner,
  quote: SwapQuote,
  params: SwapParams
) {
  const inIsSidiora = isSidioraToken(quote.tokenIn);
  const outIsSidiora = isSidioraToken(quote.tokenOut);
  const tokenIn = toCentralSwapAddress(quote.tokenIn);
  const tokenOut = toCentralSwapAddress(quote.tokenOut);
  if (
    quote.protocol !== 'pecor-v4-multihop' ||
    inIsSidiora === outIsSidiora ||
    tokenIn === NATIVE_TOKEN.address ||
    tokenOut === NATIVE_TOKEN.address ||
    tokenIn.toLowerCase() === TOKENS.USDL.address.toLowerCase() ||
    tokenOut.toLowerCase() === TOKENS.USDL.address.toLowerCase()
  ) {
    throw new Error('Invalid V4 multihop route');
  }
  if (
    !Number.isInteger(params.slippageBps) ||
    params.slippageBps < 0 ||
    params.slippageBps >= 10000
  ) {
    throw new Error('Invalid V4 multihop slippage');
  }
  const amountIn = BigInt(quote.amountIn);
  const amountOutMin = (BigInt(quote.amountOut) * BigInt(10000 - params.slippageBps)) / 10000n;
  if (amountIn <= 0n || amountOutMin <= 0n) {
    throw new Error('V4 multihop requires positive input and minimum output');
  }
  const now = Math.floor(Date.now() / 1000);
  const expiry = params.deadline ?? now + 300;
  if (!Number.isSafeInteger(expiry) || expiry <= now) {
    throw new Error('V4 multihop confirmation has expired or has an invalid expiry');
  }
  const hops: readonly V4Hop[] = Object.freeze([
    Object.freeze({
      tokenIn,
      tokenOut: TOKENS.USDL.address,
      adapterId: inIsSidiora ? SIDIORA_ADAPTER_ID : VAULT_ADAPTER_ID,
      minAmountOut: 0n,
    }),
    Object.freeze({
      tokenIn: TOKENS.USDL.address,
      tokenOut,
      adapterId: inIsSidiora ? VAULT_ADAPTER_ID : SIDIORA_ADAPTER_ID,
      minAmountOut: amountOutMin,
    }),
  ]);
  const args = Object.freeze([
    hops,
    amountIn,
    amountOutMin,
    BigInt(expiry),
  ] satisfies V4MultihopArguments);
  const router = new ethers.Contract(CONTRACTS.PECORRouter, PECOR_ROUTER_ABI, runner);
  const method: ethers.ContractMethod<V4MultihopArguments, bigint> =
    router.getFunction('swapMultiHop');
  return { router, method, args };
}

async function executeV4Multihop(
  signer: ethers.Signer,
  quote: SwapQuote,
  params: SwapParams
): Promise<SwapResult> {
  const { method, args } = prepareV4Multihop(signer, quote, params);
  const [hops, amountIn, , expiry] = args;
  const token = new ethers.Contract(hops[0].tokenIn, ERC20_ABI, signer);
  const owner = await signer.getAddress();
  const allowance: bigint = await token.allowance(owner, CONTRACTS.PECORRouter);
  if (allowance < amountIn) {
    await awaitSwapReceipt(await token.approve(CONTRACTS.PECORRouter, amountIn));
  }
  if (BigInt(Math.floor(Date.now() / 1000)) > expiry) {
    throw new Error('V4 multihop confirmation expired during approval');
  }
  return awaitSwapReceipt(await method.send(...args));
}
