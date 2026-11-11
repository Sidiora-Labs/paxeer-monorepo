import { createPublicClient, http, type Address, type Hash } from 'viem';
import { decodeContract } from '../../../../../sdk/launchpad/packages/sdk/src/validate.ts';
import type { TransactionRequest } from '../../../../../sdk/launchpad/packages/sdk/src/types.gen.ts';
import { validHomeChainId } from './chain-launch.ts';

export type TradeRequest = Omit<TransactionRequest, 'chainId'> & {
  chainId: string;
};
export function decodeTradeRequest(value: unknown): TradeRequest {
  if (
    !value ||
    typeof value !== 'object' ||
    Array.isArray(value) ||
    !('chainId' in value) ||
    typeof value.chainId !== 'string' ||
    !/^[1-9][0-9]*$/.test(value.chainId)
  )
    throw new Error('Invalid trade chain');
  if (!validHomeChainId(BigInt(value.chainId)))
    throw new Error('Invalid trade chain');
  const root = decodeContract('TransactionRequest', {
    ...value,
    chainId: '125',
  });
  return Object.freeze({ ...root, chainId: value.chainId });
}

export type TradeIdentity = Readonly<{
  version: 1;
  quoteId: string;
  nonce: number;
  request: Readonly<TradeRequest>;
}>;
export function decodeTradeIdentity(value: unknown): TradeIdentity {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    throw new Error('Invalid trade recovery identity');
  const p = value as Record<string, unknown>;
  if (
    Object.keys(p).length !== 4 ||
    p.version !== 1 ||
    typeof p.quoteId !== 'string' ||
    !/^[a-zA-Z0-9_-]{1,128}$/.test(p.quoteId) ||
    typeof p.nonce !== 'number' ||
    !Number.isSafeInteger(p.nonce) ||
    p.nonce < 0
  )
    throw new Error('Invalid trade recovery identity');
  return Object.freeze({
    version: 1,
    quoteId: p.quoteId,
    nonce: p.nonce,
    request: Object.freeze({
      ...decodeTradeRequest(p.request),
    }),
  });
}
function chainClient(url: string, signal: AbortSignal) {
  signal.throwIfAborted();
  return createPublicClient({
    transport: http(url, {
      retryCount: 0,
      timeout: 15000,
      fetchOptions: { signal, redirect: 'error', credentials: 'omit' },
    }),
  });
}
export async function prepareTradeIdentity(
  rpcURL: string,
  request: TradeRequest,
  quoteId: string,
  signal: AbortSignal,
): Promise<TradeIdentity> {
  const tx = decodeTradeRequest(request);
  const client = chainClient(rpcURL, signal);
  if ((await client.getChainId()) !== Number(tx.chainId))
    throw new Error('Wrong trade recovery chain');
  const nonce = await client.getTransactionCount({
    address: tx.from as Address,
    blockTag: 'pending',
  });
  signal.throwIfAborted();
  return decodeTradeIdentity({ version: 1, quoteId, request: tx, nonce });
}
export async function verifyTradeRecovery(
  rpcURL: string,
  identity: TradeIdentity,
  transactionHash: string,
  signal: AbortSignal,
): Promise<'pending' | 'confirmed' | 'reverted'> {
  const saved = decodeTradeIdentity(identity),
    tx = saved.request;
  if (!/^0x[0-9a-fA-F]{64}$/.test(transactionHash))
    throw new Error('Invalid trade transaction hash');
  const hash = transactionHash as Hash,
    client = chainClient(rpcURL, signal);
  const same = (a: string | null, b: string) =>
    a?.toLowerCase() === b.toLowerCase();
  if ((await client.getChainId()) !== Number(tx.chainId))
    throw new Error('Wrong trade recovery chain');
  const transaction = await client.getTransaction({ hash });
  signal.throwIfAborted();
  if (
    !same(transaction.hash, hash) ||
    transaction.chainId !== Number(tx.chainId) ||
    !same(transaction.from, tx.from) ||
    !same(transaction.to, tx.to) ||
    transaction.nonce !== saved.nonce ||
    !same(transaction.input, tx.data) ||
    transaction.value !== BigInt(tx.valueRaw)
  )
    throw new Error(
      'Trade transaction account, chain, nonce or action mismatch',
    );
  let receipt;
  try {
    receipt = await client.getTransactionReceipt({ hash });
  } catch (error) {
    let cause: unknown = error;
    for (let i = 0; i < 8 && cause && typeof cause === 'object'; i++) {
      if ('name' in cause && cause.name === 'TransactionReceiptNotFoundError')
        return 'pending';
      cause = 'cause' in cause ? cause.cause : undefined;
    }
    throw error;
  }
  const block = await client.getBlock({ blockNumber: receipt.blockNumber });
  signal.throwIfAborted();
  if (
    !same(receipt.transactionHash, hash) ||
    !same(receipt.from, tx.from) ||
    !same(receipt.to, tx.to) ||
    block.hash !== receipt.blockHash ||
    block.number !== receipt.blockNumber ||
    transaction.blockHash !== receipt.blockHash ||
    transaction.blockNumber !== receipt.blockNumber ||
    transaction.transactionIndex !== receipt.transactionIndex ||
    !same(block.transactions[receipt.transactionIndex] ?? null, hash) ||
    !['success', 'reverted'].includes(receipt.status)
  )
    throw new Error(
      'Trade receipt is not canonical or has a different identity',
    );
  return receipt.status === 'success' ? 'confirmed' : 'reverted';
}
