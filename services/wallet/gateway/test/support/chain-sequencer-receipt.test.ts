import { afterAll, beforeAll, expect, test } from 'vitest';
import { hexToBytes, keccak256, parseTransaction, toHex, type Hex } from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { RpcPool, type SequencerReceiptResult } from '../../src/rpc/pool.js';
import { readReceipt } from '../../src/provision/bind.js';
import { verifySequencerReceipt } from '../../../../agent/sdk/typescript/src/verifier.js';
import { encodeSequencerReceipt } from '../../../../agent/sdk/typescript/src/sequencer-receipt-codec.js';
import {
  nodeReceiptVectors, TestChain, verifyChainSequencerReceipt,
  type SignedChainReceipt,
} from './chain.js';

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
    const before = BigInt(Date.now());
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
      effectiveGasPrice: toHex(effectiveGasPrice), paymentDenom: 'uhpx', feeDenom: 'uhpx',
      feeAmount: toHex(BigInt(applied.gasUsed) * effectiveGasPrice),
    });
    expect(result.status).toBe(applied.status);
    expect(BigInt(result.timestamp)).toBeGreaterThanOrEqual(before);
    expect(BigInt(result.timestamp)).toBeLessThanOrEqual(BigInt(Date.now()));
    expect(result.keyId).toBe(chain.receiptKeyPins[0]!.keyId);
    const canonical = await verifySequencerReceipt(hexToBytes(result.receipt), hexToBytes(raw), chain.receiptKeyPins.map((pin) => ({
      keyId: hexToBytes(pin.keyId), publicKey: hexToBytes(pin.publicKey),
      first_height: BigInt(pin.firstHeight), last_height: BigInt(pin.lastHeight),
    })));
    expect(toHex(encodeSequencerReceipt(canonical.receipt))).toBe(result.receipt);
    expect(canonical.receipt.sequencerSignature).toHaveLength(64);
    expect(canonical.receipt.activityId).toEqual(hexToBytes(hash));
    expect(canonical.receipt.resultCode).toBe(nonce === 2 ? 1 : 0);
    expect(await verifyChainSequencerReceipt(result as SignedChainReceipt, raw, chain.receiptKeyPins)).toEqual(result);
    await expect(verifyChainSequencerReceipt({ ...result, status: '0x2' } as SignedChainReceipt, raw, chain.receiptKeyPins)).rejects.toThrow();
    expect(await readReceipt(rpc, hash)).toEqual({
      status: nonce === 2 ? 'reverted' : 'success',
      gasUsed: BigInt(applied.gasUsed), blockNumber: BigInt(applied.blockNumber),
    });
    results.push(result);
    chain.baseFee += 1n;
  }
  for (const result of results) expect(await rpc.sequencerReceipt(result.txHash)).toEqual(result);
}

async function sequencerReceiptMatchesNodeVectors(): Promise<void> {
  expect(nodeReceiptVectors.provenance.generator).toBe('rpc/sequencer_receipt_vectors_test.go');
  const vectors = nodeReceiptVectors.cases.filter((entry) => entry.vector === 'V1' || entry.vector === 'V2');
  expect(vectors).toHaveLength(6);
  for (const vector of vectors) {
    await chain.recordSequencerReceipt(vector);
    expect(await rpc.sendRawTransaction(vector.rawTransaction)).toBe(vector.receiptJson.txHash);
    const result = await rpc.sequencerReceipt(vector.receiptJson.txHash);
    expect(result).toStrictEqual(vector.receiptJson);
    expect(result).toMatchObject({ receipt: vector.signedReceipt, signingKey: vector.pinnedKey.publicKey });
    expect(await rpc.request('px_getReceiptKeys')).toEqual({ keys: [vector.pinnedKey] });
    expect(await rpc.request('eth_getRawTransactionByHash', [vector.receiptJson.txHash])).toBe(vector.rawTransaction);
    expect(await readReceipt(rpc, vector.receiptJson.txHash)).toEqual({
      status: vector.vector === 'V2' ? 'reverted' : 'success',
      gasUsed: BigInt(vector.receiptJson.gasUsed), blockNumber: BigInt(vector.receiptJson.height),
    });
  }
}

async function sequencerReceiptRefusesMutatedNodeFieldsAndWrongKeys(): Promise<void> {
  const vector = nodeReceiptVectors.cases.find((entry) => entry.id === 'V1-erc20-approve')!;
  const view = vector.receiptJson;
  const wrongKey = nodeReceiptVectors.cases[0]!.receiptJson.txHash;
  const mutations: Partial<SignedChainReceipt>[] = [
    { status: '0x0' }, { nonce: '0x2' }, { seq: '0x3' }, { height: '0x3e9' },
    { index: '0x0' }, { timestamp: '0x1' }, { gasUsed: '0x1' }, { cumulativeGasUsed: '0x1' },
    { effectiveGasPrice: '0x1' }, { feeAmount: '0x1' }, { paymentDenom: 'PAX' }, { feeDenom: 'PAX' },
    { returnData: '0x' }, { vmError: 'execution reverted' }, { from: view.to }, { to: view.from },
    { sponsor: view.from }, { contractAddress: view.to }, { txHash: wrongKey },
    { keyId: wrongKey }, { signingKey: wrongKey }, { logs: [] },
    { logs: view.logs.map((log) => ({ ...log, data: `${log.data}00` })) },
    { logs: view.logs.map((log) => ({ ...log, topics: [] })) },
    { logs: view.logs.map((log) => ({ ...log, address: view.from, logIndex: '0x1' })) },
  ];
  for (const mutation of mutations) {
    await expect(verifyChainSequencerReceipt({ ...view, ...mutation } as SignedChainReceipt, vector.rawTransaction, chain.receiptKeyPins), JSON.stringify(mutation)).rejects.toThrow();
  }
  await expect(verifyChainSequencerReceipt(view, vector.rawTransaction, [])).rejects.toMatchObject({ check: 'unpinned_key' });
  await expect(verifyChainSequencerReceipt(view, vector.rawTransaction, [{ ...vector.pinnedKey, publicKey: wrongKey }])).rejects.toMatchObject({ check: 'bad_signature' });
  const height = BigInt(view.height);
  for (const [firstHeight, lastHeight] of [[height + 1n, height + 2n], [1n, height - 1n]]) {
    await expect(verifyChainSequencerReceipt(view, vector.rawTransaction, [{
      ...vector.pinnedKey, firstHeight: firstHeight.toString(), lastHeight: lastHeight.toString(),
    }])).rejects.toMatchObject({ check: 'key_out_of_range' });
  }
  expect(await verifyChainSequencerReceipt(view, vector.rawTransaction, [{
    ...vector.pinnedKey, firstHeight: height.toString(), lastHeight: height.toString(),
  }])).toEqual(view);
}

test(sequencerReceiptReturnsNullForUnknownHash.name, sequencerReceiptReturnsNullForUnknownHash);
test(sequencerReceiptMatchesAppliedTransactions.name, sequencerReceiptMatchesAppliedTransactions);
test(sequencerReceiptMatchesNodeVectors.name, sequencerReceiptMatchesNodeVectors);
test(sequencerReceiptRefusesMutatedNodeFieldsAndWrongKeys.name, sequencerReceiptRefusesMutatedNodeFieldsAndWrongKeys);
