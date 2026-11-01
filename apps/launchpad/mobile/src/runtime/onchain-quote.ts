import {
  keccak256,
  parseAbi,
  toHex,
  type Address,
  type PublicClient,
} from 'viem';
import { decodeContract } from '../../../../../sdk/launchpad/packages/sdk/src/validate.ts';
import type { Quote } from '../../../../../sdk/launchpad/packages/sdk/src/types.gen.ts';

const quoterAbi = parseAbi([
  'function quoteExactInput(address pool,uint256 amountIn,bool isBuy) view returns((uint256 amountOut,uint256 feeAmount,uint256 priceImpactBps))',
]);

export async function onchainQuote(
  client: Pick<PublicClient, 'getChainId' | 'getBlock' | 'readContract'>,
  quoter: Address,
  {
    pool,
    side,
    inputRaw,
    slippageBps,
  }: {
    pool: Address;
    side: Quote['side'];
    inputRaw: Quote['inputRaw'];
    slippageBps: number;
  },
  signal: AbortSignal,
): Promise<Quote> {
  if (!Number.isInteger(slippageBps) || slippageBps < 1 || slippageBps > 1000)
    throw new RangeError('Slippage must be an integer between 1 and 1000 bps');
  const amountIn = BigInt(inputRaw);
  if (amountIn <= 0n) throw new RangeError('Quote input must be positive');
  signal.throwIfAborted();
  const chainId = await client.getChainId();
  signal.throwIfAborted();
  if (chainId !== 125)
    throw new Error('Chain 125 is required for mobile quotes');
  const block = await client.getBlock({ blockTag: 'latest' });
  signal.throwIfAborted();
  if (block.number === null || block.hash === null)
    throw new Error('A mined block is required for mobile quotes');
  const result = await client.readContract({
    address: quoter,
    abi: quoterAbi,
    functionName: 'quoteExactInput',
    args: [pool, amountIn, side === 'buy'],
    blockNumber: block.number,
  });
  signal.throwIfAborted();
  const id = keccak256(
    toHex(
      JSON.stringify([
        quoter.toLowerCase(),
        pool.toLowerCase(),
        side,
        amountIn.toString(),
        slippageBps,
        block.hash,
      ]),
    ),
  ).slice(2);
  return decodeContract('Quote', {
    quoteId: `${id.slice(0, 8)}-${id.slice(8, 12)}-8${id.slice(13, 16)}-8${id.slice(17, 20)}-${id.slice(20, 32)}`,
    poolAddress: pool,
    side,
    inputRaw: amountIn.toString(),
    outputRaw: result.amountOut.toString(),
    feeRaw: result.feeAmount.toString(),
    minOutputRaw: (
      (result.amountOut * BigInt(10000 - slippageBps)) /
      10000n
    ).toString(),
    validUntil: new Date(Number(block.timestamp + 30n) * 1000).toISOString(),
    watermark: {
      chainId: '125',
      height: block.number.toString(),
      blockHash: block.hash,
      integrity: 'healthy',
      observedAt: new Date(Number(block.timestamp) * 1000).toISOString(),
      projectionRevision: '1',
    },
  });
}
