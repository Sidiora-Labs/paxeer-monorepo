import { expect, test } from 'vitest';
import { hexToBytes, keccak256, toHex, type Hex } from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { RpcPool } from '../src/rpc/pool.js';
import { readReceipt } from '../src/provision/bind.js';
import { verifySequencerReceipt } from '../../../agent/sdk/typescript/src/verifier.js';
import {
  nodeReceiptVectors, TestChain, type NodeReceiptVector, type SignedChainReceipt,
} from './support/chain.js';
import './support/chain-sequencer-receipt.test.js';

/* Task 22.33.210 repair record:
 * json-signature-contract-test -> sequencerReceiptMatchesAppliedTransactions,
 *   sequencerReceiptMatchesNodeVectors, sequencerReceiptRefusesMutatedNodeFieldsAndWrongKeys.
 * json-signature-chain-fixture -> jsonSignatureChainFixtureUsesCanonicalReceipt,
 *   jsonSignatureChainFixtureRejectsMalformedSignedViews,
 *   jsonSignatureChainFixtureRefusesNodeVector (one named case per refusal id).
 * Baseline was not run: the explicit implementation/build/hf-done order forbids it;
 * no pre-change execution or failure is claimed. Gate evidence is recorded by hf-done.
 * Inherited attempt 2: five of six tests passed; the refusal loop in
 *   jsonSignatureChainFixtureRejectsMalformedSignedViews failed. V3-seq-hash-mismatch
 *   must refuse a previousStateRoot that differs from the recording's prevSeqHash.
 * Qualification finding: Vitest 4 treats test(function) as a todo; every declared
 *   test is registered with its function name and callback so assertions execute.
 * Wiring request (wallet/domain 12): test/sponsor-submit-receipt.test.ts must replace
 *   its remaining JSON signature assertion with canonical receipt verification.
 * Wiring request (wallet/domain 12): src/rpc/pool.ts SequencerReceiptView must declare
 *   the node's signingKey and paymentDenom fields (paths relative to this gateway).
 */

function receiptPool(chain: TestChain): RpcPool {
  return new RpcPool({
    urls: [chain.url], sequencerUrl: chain.url, chainId: 125,
    lagThresholdBlocks: 20, timeoutMs: 3_000, healthIntervalMs: 60_000,
  });
}

async function jsonSignatureChainFixtureUsesCanonicalReceipt(): Promise<void> {
  const chain = new TestChain(125);
  await chain.start();
  const rpc = receiptPool(chain);
  const account = privateKeyToAccount(`0x${'12'.repeat(32)}`);
  const recipient = '0x000000000000000000000000000000000000c0de';
  try {
    chain.balances.set(account.address.toLowerCase(), 10n ** 18n);
    let previousRoot = new Uint8Array(32);
    for (const nonce of [0, 1]) {
      const raw = await account.signTransaction({
        type: 'eip1559', chainId: 125, nonce, to: recipient, value: 1n, gas: 21_000n,
        maxFeePerGas: chain.baseFee + chain.tip, maxPriorityFeePerGas: chain.tip,
        data: nonce === 0 ? '0x' : '0x01',
      });
      const hash = await rpc.sendRawTransaction(raw);
      const view = await rpc.sequencerReceipt(hash);
      if (!view || view.rung === 'submitted') throw new Error('missing signed receipt');
      const keys = await rpc.request<{ keys: NodeReceiptVector['pinnedKey'][] }>('px_getReceiptKeys');
      expect(keys.keys).toEqual(chain.receiptKeyPins);
      const verified = await verifySequencerReceipt(hexToBytes(view.receipt), hexToBytes(raw), keys.keys.map((pin) => ({
        keyId: hexToBytes(pin.keyId), publicKey: hexToBytes(pin.publicKey),
        first_height: BigInt(pin.firstHeight), last_height: BigInt(pin.lastHeight),
      })));
      expect(view.receipt.slice(0, 14)).toBe('0x000252010002');
      expect(verified.level).toBe('sequencer-signed');
      expect(verified.receipt.previousStateRoot).toEqual(previousRoot);
      expect(verified.receipt.activityId).toEqual(hexToBytes(keccak256(raw)));
      expect(verified.receipt.fromSequence).toBe(BigInt(nonce));
      expect(verified.receipt.globalSequence).toBe(BigInt(nonce + 1));
      expect(verified.receipt.resultCode).toBe(nonce === 0 ? 0 : 1);
      expect(view).toMatchObject({
        keyId: keys.keys[0]!.keyId, signingKey: keys.keys[0]!.publicKey,
        paymentDenom: 'uhpx', feeDenom: 'uhpx', status: nonce === 0 ? '0x1' : '0x0',
      });
      expect(await readReceipt(rpc, hash)).toEqual({
        status: nonce === 0 ? 'success' : 'reverted', gasUsed: 21_000n, blockNumber: verified.height,
      });
      expect(await rpc.request('eth_getRawTransactionByHash', [hash])).toBe(raw);
      const malformed = { ...view, gasUsed: '0x1' } as SignedChainReceipt;
      chain.sequencerReceipts.set(hash, malformed);
      await expect(rpc.sequencerReceipt(hash)).rejects.toMatchObject({ code: -32000 });
      await expect(readReceipt(rpc, hash)).rejects.toThrow('signed receipt view mismatch');
      chain.sequencerReceipts.set(hash, view as SignedChainReceipt);
      expect(await rpc.sequencerReceipt(hash)).toEqual(view);
      previousRoot = verified.receipt.resultingStateRoot;
    }
    expect(await rpc.sequencerReceipt(`0x${'ff'.repeat(32)}`)).toBeNull();
  } finally {
    rpc.stop();
    await chain.stop();
  }
}

async function jsonSignatureChainFixtureRejectsMalformedSignedViews(): Promise<void> {
  const source = nodeReceiptVectors.cases.find((entry) => entry.id === 'V1-erc20-approve')!;
  const chain = new TestChain(125);
  await chain.start();
  const rpc = receiptPool(chain);
  try {
    await chain.recordSequencerReceipt(source);
    const view = await rpc.sequencerReceipt(source.receiptJson.txHash);
    expect(view).toStrictEqual(source.receiptJson);
    const bytes = hexToBytes(source.signedReceipt);
    bytes[bytes.length - 1] ^= 1;
    const corruptSignature = toHex(bytes);
    const mutations: Record<string, unknown>[] = [
      { status: '0x0' }, { receipt: corruptSignature },
      { receipt: `${source.signedReceipt}00` }, { receipt: source.signedReceipt.slice(0, -2) },
      { receipt: `0x${'00'.repeat(64)}` }, { receipt: '0xgg' },
      { signingKey: `0x${'ab'.repeat(32)}` }, { keyId: `0x${'ab'.repeat(32)}` },
      { signingKey: undefined }, { paymentDenom: undefined }, { paymentDenom: 'PAX' },
      { feeDenom: 'PAX' }, { height: '0x3e7' }, { nonce: '0x00' },
      { logs: source.receiptJson.logs.map((log) => ({ ...log, data: `${log.data}00` })) },
      { rung: 'submitted' }, { blockNumber: '0x3e8', blockHash: `0x${'ab'.repeat(32)}` },
    ];
    for (const mutation of mutations) {
      const malformed = { ...source.receiptJson, ...mutation } as SignedChainReceipt;
      await expect(chain.recordSequencerReceipt({ ...source, receiptJson: malformed }), JSON.stringify(mutation)).rejects.toThrow();
      expect(await rpc.sequencerReceipt(source.receiptJson.txHash)).toStrictEqual(source.receiptJson);
      chain.sequencerReceipts.set(source.receiptJson.txHash, malformed);
      await expect(rpc.sequencerReceipt(source.receiptJson.txHash), JSON.stringify(mutation)).rejects.toMatchObject({ code: -32000 });
      chain.sequencerReceipts.set(source.receiptJson.txHash, structuredClone(source.receiptJson));
    }
    const wrongRaw = nodeReceiptVectors.cases.find((entry) => entry.id === 'V1-pax-transfer')!.rawTransaction;
    chain.rawTransactions.set(source.receiptJson.txHash, wrongRaw as Hex);
    await expect(rpc.sequencerReceipt(source.receiptJson.txHash)).rejects.toMatchObject({ code: -32000 });
  } finally {
    rpc.stop();
    await chain.stop();
  }
}

async function jsonSignatureChainFixtureRefusesNodeVector(id: string): Promise<void> {
  const source = nodeReceiptVectors.cases.find((entry) => entry.id === 'V1-erc20-approve')!;
  const refusal = nodeReceiptVectors.cases.find((entry) => entry.id === id)!;
  const chain = new TestChain(125);
  await chain.start();
  const rpc = receiptPool(chain);
  try {
    await chain.recordSequencerReceipt(source);
    await expect(chain.recordSequencerReceipt(refusal)).rejects.toThrow(
      id === 'V3-seq-hash-mismatch' ? 'node recording prevSeqHash mismatch' : undefined,
    );
    expect(await rpc.sequencerReceipt(source.receiptJson.txHash)).toStrictEqual(source.receiptJson);
    expect(chain.sequencerReceipts.size).toBe(1);
    expect(chain.rawTransactions.size).toBe(1);
    expect(chain.receipts.size).toBe(1);
  } finally {
    rpc.stop();
    await chain.stop();
  }
}

test(jsonSignatureChainFixtureUsesCanonicalReceipt.name, jsonSignatureChainFixtureUsesCanonicalReceipt);
test(jsonSignatureChainFixtureRejectsMalformedSignedViews.name, jsonSignatureChainFixtureRejectsMalformedSignedViews);
test.each([
  'V3-key-not-pinned', 'V3-height-outside-key-range', 'V3-activity-id-not-raw-hash',
  'V3-seq-hash-mismatch', 'V3-tampered-log', 'V3-non-canonical-bytes',
])(`${jsonSignatureChainFixtureRefusesNodeVector.name} %s`, jsonSignatureChainFixtureRefusesNodeVector);
