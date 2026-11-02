import { verify as edVerify } from 'node:crypto';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { keccak256, parseTransaction, toHex, type Hex } from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { RpcPool, type SequencerReceiptResult } from '../../src/rpc/pool.js';
import { readReceipt } from '../../src/provision/bind.js';
import { TestChain } from './chain.js';

const CHAIN_ID = 125;
const RECIPIENT = '0x000000000000000000000000000000000000c0de';
const account = privateKeyToAccount(`0x${'12'.repeat(32)}`);
const chain = new TestChain(CHAIN_ID);
let rpc: RpcPool;

beforeAll(async () => {
  await chain.start();
  rpc = new RpcPool({
    urls: [chain.url], sequencerUrl: chain.url, chainId: CHAIN_ID,
    lagThresholdBlocks: 20, timeoutMs: 3_000, healthIntervalMs: 60_000,
  });
  chain.balances.set(account.address.toLowerCase(), 10n ** 18n);
});

afterAll(async () => {
  rpc?.stop();
  await chain.stop();
});

async function sequencerReceiptReturnsNullForUnknownHash(): Promise<void> {
  const hash: Hex = `0x${'ff'.repeat(32)}`;
  expect(await rpc.sequencerReceipt(hash)).toBeNull();
  expect(await readReceipt(rpc, hash)).toBeNull();
}

async function sequencerReceiptMatchesAppliedTransactions(): Promise<void> {
  const results: Extract<SequencerReceiptResult, { rung: 'receipted' }>[] = [];
  for (const nonce of [0, 1, 2]) {
    const before = BigInt(Math.floor(Date.now() / 1_000));
    const maxFeePerGas = chain.baseFee + (nonce === 1 ? chain.tip / 2n : chain.tip * 2n);
    const effectiveGasPrice = nonce === 1 ? maxFeePerGas : chain.baseFee + chain.tip;
    const raw = await account.signTransaction({
      type: 'eip1559', chainId: CHAIN_ID, nonce, to: RECIPIENT, value: 1n,
      gas: 21_000n, maxFeePerGas, maxPriorityFeePerGas: chain.tip,
      data: nonce === 2 ? '0x01' : '0x',
    });
    const hash = await rpc.sendRawTransaction(raw);
    const applied = await rpc.request<{ status: Hex; gasUsed: Hex; blockNumber: Hex }>('eth_getTransactionReceipt', [hash]);
    const result = await rpc.sequencerReceipt(hash);
    expect(hash).toBe(keccak256(raw));
    expect(result?.rung).toBe('receipted');
    if (!result || result.rung !== 'receipted') throw new Error('applied transaction must be receipted');
    expect(result).toMatchObject({
      txHash: hash, nonce: toHex(parseTransaction(raw).nonce!),
      from: account.address.toLowerCase(), to: RECIPIENT,
      seq: toHex(nonce + 1), height: applied.blockNumber, index: '0x0',
      status: nonce === 2 ? '0x0' : '0x1', returnData: '0x', logs: [],
      gasUsed: applied.gasUsed, cumulativeGasUsed: applied.gasUsed,
      effectiveGasPrice: toHex(effectiveGasPrice), feeDenom: 'PAX',
      feeAmount: toHex(BigInt(applied.gasUsed) * effectiveGasPrice),
    });
    expect(result.status).toBe(applied.status);
    expect(BigInt(result.timestamp)).toBeGreaterThanOrEqual(before);
    expect(BigInt(result.timestamp)).toBeLessThanOrEqual(BigInt(Math.floor(Date.now() / 1_000)));
    expect(result.keyId).toBe(keccak256(toHex(chain.sequencerPublicKey.export({ type: 'spki', format: 'der' }))));
    const { receipt, ...signed } = result;
    const signature = Buffer.from(receipt.slice(2), 'hex');
    expect(signature).toHaveLength(64);
    expect(edVerify(null, Buffer.from(JSON.stringify(signed)), chain.sequencerPublicKey, signature)).toBe(true);
    expect(edVerify(null, Buffer.from(JSON.stringify({ ...signed, status: '0x2' })), chain.sequencerPublicKey, signature)).toBe(false);
    expect(await readReceipt(rpc, hash)).toEqual({
      status: nonce === 2 ? 'reverted' : 'success',
      gasUsed: BigInt(applied.gasUsed), blockNumber: BigInt(applied.blockNumber),
    });
    results.push(result);
    chain.baseFee += 1n;
  }
  for (const result of results) expect(await rpc.sequencerReceipt(result.txHash)).toEqual(result);
}

test(sequencerReceiptReturnsNullForUnknownHash);
test(sequencerReceiptMatchesAppliedTransactions);
