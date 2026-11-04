import { Interface, formatUnits, isAddress, type Provider } from 'ethers';
import type { SwapIntent } from '@/widgets/swap/swapJournal';

const transferCodec = new Interface([
  'event Transfer(address indexed from, address indexed to, uint256 value)',
]);

const verifiedResult = Symbol('verified swap receipt result');

export interface SwapReceiptResult {
  readonly [verifiedResult]: true;
  readonly quoteId: string;
  readonly chainId: string;
  readonly txHash: string;
  readonly blockHash: string;
  readonly fromAmount: string;
  readonly fromSymbol: string;
  readonly fromIconUrl?: string;
  readonly toAmount: string;
  readonly toAmountRaw: string;
  readonly toAddress: string;
  readonly toSymbol: string;
  readonly toIconUrl?: string;
}

/** Revalidate the executed journal identity and canonical receipt before displaying output. */
export async function resolveSwapReceiptResult(
  executed: SwapIntent,
  provider: Provider
): Promise<SwapReceiptResult> {
  // The caller may refresh or mutate its quote while receipt reads are in flight.
  const intent = structuredClone(executed);
  const { quote, params } = intent;
  const swaps = intent.transactions.filter((tx) => tx.kind === 'swap');
  if (
    !intent.id ||
    swaps.length !== intent.expectedLegs ||
    swaps.length === 0 ||
    intent.transactions.some((tx) => tx.receipt?.status !== 1) ||
    params.amountIn !== quote.amountIn ||
    params.tokenOut.address.toLowerCase() !== quote.tokenOut.address.toLowerCase() ||
    params.tokenOut.decimals !== quote.tokenOut.decimals ||
    params.tokenIn.address.toLowerCase() !== quote.tokenIn.address.toLowerCase() ||
    params.recipient.toLowerCase() !== intent.account.toLowerCase()
  )
    throw new Error('Swap execution is incomplete or differs from its approved quote.');
  if ((await provider.getNetwork()).chainId.toString() !== intent.chainId)
    throw new Error('Swap receipt provider is on a different chain.');
  if (quote.tokenOut.isNative || !isAddress(quote.tokenOut.address))
    throw new Error('Native swap output requires verified balance reconciliation.');
  if (
    !Number.isInteger(quote.tokenOut.decimals) ||
    quote.tokenOut.decimals < 0 ||
    quote.tokenOut.decimals > 80
  )
    throw new Error('Swap output precision is invalid.');

  let finalReceipt = null;
  for (const tx of swaps) {
    if (!tx.hash) throw new Error('Swap execution has no transaction hash.');
    const transaction = await provider.getTransaction(tx.hash);
    const receipt = await provider.getTransactionReceipt(tx.hash);
    if (
      !transaction ||
      !receipt ||
      receipt.status !== 1 ||
      transaction.hash.toLowerCase() !== tx.hash.toLowerCase() ||
      transaction.chainId.toString() !== intent.chainId ||
      transaction.from.toLowerCase() !== intent.account.toLowerCase() ||
      transaction.to?.toLowerCase() !== tx.to.toLowerCase() ||
      transaction.data.toLowerCase() !== tx.data.toLowerCase() ||
      transaction.value.toString() !== tx.value ||
      transaction.nonce !== tx.nonce ||
      receipt.hash.toLowerCase() !== tx.hash.toLowerCase() ||
      receipt.blockHash.toLowerCase() !== tx.receipt!.blockHash.toLowerCase() ||
      receipt.blockNumber !== tx.receipt!.blockNumber
    )
      throw new Error('Swap receipt does not match the executed transaction.');
    const block = await provider.getBlock(receipt.blockNumber);
    if (
      !block ||
      block.hash !== receipt.blockHash ||
      !block.transactions.some((hash) => hash.toLowerCase() === tx.hash!.toLowerCase())
    )
      throw new Error('Swap receipt is not in the canonical block.');
    finalReceipt = receipt;
  }
  if (!finalReceipt) throw new Error('Swap has no verified receipt.');
  let received = 0n;
  let matched = false;
  for (const log of finalReceipt.logs) {
    if (log.address.toLowerCase() !== quote.tokenOut.address.toLowerCase()) continue;
    if (log.topics[0] !== transferCodec.getEvent('Transfer')!.topicHash) continue;
    const event = transferCodec.parseLog(log);
    if (!event) throw new Error('Swap receipt output could not be decoded.');
    const from = String(event.args.from).toLowerCase();
    const to = String(event.args.to).toLowerCase();
    const recipient = params.recipient.toLowerCase();
    if (to === recipient) {
      received += BigInt(event.args.value);
      matched = true;
    }
    if (from === recipient) received -= BigInt(event.args.value);
  }
  if (!matched || received < 0n) throw new Error('Swap receipt has no matching received output.');
  return Object.freeze({
    [verifiedResult]: true as const,
    quoteId: intent.id,
    chainId: intent.chainId,
    txHash: finalReceipt.hash,
    blockHash: finalReceipt.blockHash,
    fromAmount: formatUnits(quote.amountIn, quote.tokenIn.decimals),
    fromSymbol: quote.tokenIn.symbol,
    fromIconUrl: quote.tokenIn.iconUrl,
    toAmount: formatUnits(received, quote.tokenOut.decimals),
    toAmountRaw: received.toString(),
    toAddress: quote.tokenOut.address,
    toSymbol: quote.tokenOut.symbol,
    toIconUrl: quote.tokenOut.iconUrl,
  });
}

export function isSwapReceiptResult(value: SwapReceiptResult): boolean {
  return value[verifiedResult] === true && Object.isFrozen(value);
}
