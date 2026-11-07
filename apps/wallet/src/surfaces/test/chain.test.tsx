// @vitest-environment jsdom
import { expect, test } from 'vitest';
import { webcrypto } from 'node:crypto';
import { ethers } from 'ethers';
import { bytesToHex, hexToBytes } from 'viem';
import { verifySequencerReceipt } from '@sidiora/layerx-sdk/browser';
import * as layerxSdk from '@sidiora/layerx-sdk';
import {
  authenticateSequencerReceipt,
  decodeSequencerReceipt,
  encodeKernelReceipt,
  readReceiptKeyPins,
  readSequencerReceipt,
  requireAuthenticatedReceipt,
} from '@paxeer/wallet';
import { receiptStatus } from '../useSurface';
import { SurfaceChain, nodeReceiptCall, nodeReceiptVectors } from './chain';

Object.defineProperty(globalThis, 'crypto', { value: webcrypto, configurable: true });

async function surfaces_test_chain_answers_the_sequencer_receipt_of_a_sent_transaction() {
  const chain = new SurfaceChain();
  const entry = nodeReceiptVectors().find(({ id }) => id === 'V1-erc20-approve')!;
  const call = nodeReceiptCall(entry);
  const abi = new ethers.Interface([
    'function approve(address spender, uint256 amount)',
    'function allowance(address owner, address spender) view returns (uint256)',
  ]);
  const [spender, amount] = abi.decodeFunctionData('approve', call.data);
  for (const method of [
    'eth_getTransactionReceipt',
    'pax_sequencerReceipt',
    'eth_getRawTransactionByHash',
  ]) {
    expect(await chain.request({ method, params: [ethers.ZeroHash] })).toBeNull();
    expect(await chain.request({ method, params: [entry.receiptJson.txHash] })).toBeNull();
  }
  for (const [index, method] of [
    'eth_sendTransaction',
    'paxeer_sendTransactionReceipted',
  ].entries()) {
    const result = await chain.request({
      method,
      params: [
        {
          ...call,
          from: chain.wallet.address,
          to: ethers.getAddress(call.to),
          data: call.data.toUpperCase().replace('0X', '0x'),
        },
      ],
    });
    expect(result).toEqual(
      method === 'eth_sendTransaction' ? entry.receiptJson.txHash : entry.receiptJson
    );
    expect(chain.sent).toHaveLength(index + 1);
    expect(chain.sent[index]).toEqual(call);
    const answer = await chain.request({
      method: 'pax_sequencerReceipt',
      params: [entry.receiptJson.txHash],
    });
    expect(answer).toEqual(entry.receiptJson);
    const rpc = (method: string, params: readonly unknown[]) => chain.request({ method, params });
    const pins = await readReceiptKeyPins(rpc);
    const decoded = decodeSequencerReceipt(answer, entry.receiptJson.txHash, pins);
    expect(decoded.nonce).toBe(entry.receiptJson.nonce);
    expect(decoded.logs?.map((log) => log.logIndex)).toEqual(
      entry.receiptJson.logs!.map((log) => log.logIndex)
    );
    const authenticated = await authenticateSequencerReceipt(rpc, answer, entry.receiptJson.txHash);
    requireAuthenticatedReceipt(authenticated);
    expect(receiptStatus(authenticated)).toBe('confirmed');
    expect(
      await chain.request({
        method: 'eth_getTransactionReceipt',
        params: [entry.receiptJson.txHash],
      })
    ).toEqual({ transactionHash: entry.receiptJson.txHash, ...entry.receiptJson });
    expect(
      await chain.request({
        method: 'eth_call',
        params: [
          { to: call.to, data: abi.encodeFunctionData('allowance', [chain.address, spender]) },
        ],
      })
    ).toBe(abi.encodeFunctionResult('allowance', [amount]));
    await expect(
      chain.request({ method, params: [{ ...call, from: spender }] })
    ).rejects.toMatchObject({ message: 'unknown account', code: 4100 });
    expect(chain.sent).toHaveLength(index + 1);
    expect(
      await chain.request({ method: 'pax_sequencerReceipt', params: [entry.receiptJson.txHash] })
    ).toEqual(entry.receiptJson);
  }
}

test(
  surfaces_test_chain_answers_the_sequencer_receipt_of_a_sent_transaction.name,
  surfaces_test_chain_answers_the_sequencer_receipt_of_a_sent_transaction
);

async function surface_chain_preserves_node_execution_fields_and_governed_signatures() {
  const entries = nodeReceiptVectors();
  expect(entries.map(({ id }) => id)).toEqual([
    'V1-pax-transfer',
    'V1-erc20-approve',
    'V1-layerxledger-pay-exact',
    'V1-layerxbridge-bridge-out',
    'V1-layerxprograms-call',
    'V2-layerxledger-revert',
    'V2-pax-transfer-insufficient-funds',
  ]);
  for (const entry of entries) {
    const chain = new SurfaceChain();
    const call = nodeReceiptCall(entry);
    const hash = await chain.request({ method: 'eth_sendTransaction', params: [call] });
    expect(hash).toBe(entry.receiptJson.txHash);
    const receipt = await readSequencerReceipt(
      (method, params) => chain.request({ method, params }),
      entry.receiptJson.txHash
    );
    requireAuthenticatedReceipt(receipt);
    expect(receipt).toEqual({
      ...entry.receiptJson,
      authentication: {
        level: 'sequencer-signed',
        keyId: entry.pinnedKey.keyId,
        signingKey: entry.pinnedKey.publicKey,
        height: BigInt(entry.receiptJson.height!),
        receiptDigest: entry.receiptDigest,
        rawTransaction: entry.rawTransaction,
      },
    });
    expect(receiptStatus(receipt)).toBe(
      entry.receiptJson.status === '0x1' ? 'confirmed' : 'reverted'
    );
    const verified = await verifySequencerReceipt(
      hexToBytes(entry.signedReceipt),
      hexToBytes(entry.rawTransaction),
      [
        {
          keyId: hexToBytes(entry.pinnedKey.keyId),
          publicKey: hexToBytes(entry.pinnedKey.publicKey),
          first_height: BigInt(entry.pinnedKey.firstHeight),
          last_height: BigInt(entry.pinnedKey.lastHeight),
        },
      ]
    );
    expect(bytesToHex(encodeKernelReceipt(verified.receipt))).toBe(entry.signedReceipt);
    expect(bytesToHex(verified.receiptDigest)).toBe(entry.receiptDigest);
    expect(chain.methods).toContain('px_getReceiptKeys');
    expect(chain.methods).toContain('eth_getRawTransactionByHash');
    const answer = (await chain.request({
      method: 'pax_sequencerReceipt',
      params: [receipt.txHash],
    })) as Record<string, unknown>;
    Reflect.deleteProperty(answer, 'receipt');
    expect(
      await chain.request({ method: 'pax_sequencerReceipt', params: [receipt.txHash] })
    ).toEqual(entry.receiptJson);
  }
}

test(
  surface_chain_preserves_node_execution_fields_and_governed_signatures.name,
  surface_chain_preserves_node_execution_fields_and_governed_signatures
);

async function surface_chain_refuses_removed_or_mutated_authenticated_fields() {
  const chain = new SurfaceChain();
  const entry = nodeReceiptVectors().find(({ id }) => id === 'V1-layerxbridge-bridge-out')!;
  await chain.request({ method: 'eth_sendTransaction', params: [nodeReceiptCall(entry)] });
  const rpc = (method: string, params: readonly unknown[]) => chain.request({ method, params });
  for (const field of Object.keys(entry.receiptJson)) {
    const removed = structuredClone(entry.receiptJson);
    Reflect.deleteProperty(removed, field);
    await expect(
      authenticateSequencerReceipt(rpc, removed, entry.receiptJson.txHash),
      `removed ${field}`
    ).rejects.toThrow();
    const mutated: Record<string, unknown> = { ...structuredClone(entry.receiptJson) };
    const value = mutated[field];
    if (field === 'logs') mutated.logs = [];
    else if (field === 'receipt') {
      const bytes = hexToBytes(entry.signedReceipt);
      bytes[bytes.length - 1] ^= 1;
      mutated.receipt = bytesToHex(bytes);
    } else if (typeof value === 'string' && value.startsWith('0x')) {
      mutated[field] = /^0x[0-9a-f]{40}$/.test(value)
        ? `0x${'42'.repeat(20)}`
        : /^0x[0-9a-f]{64}$/.test(value)
          ? `0x${'42'.repeat(32)}`
          : value === '0x1'
            ? '0x2'
            : '0x1';
    } else mutated[field] = `${String(value)}-tampered`;
    await expect(
      authenticateSequencerReceipt(rpc, mutated, entry.receiptJson.txHash),
      `mutated ${field}`
    ).rejects.toThrow();
  }
  for (const field of ['address', 'topics', 'data', 'logIndex'] as const) {
    const mutated = structuredClone(entry.receiptJson);
    const log = mutated.logs![0]!;
    if (field === 'topics') log.topics[0] = `0x${'42'.repeat(32)}`;
    else if (field === 'address') log.address = `0x${'42'.repeat(20)}`;
    else if (field === 'data') log.data = '0x00';
    else log.logIndex = '0x1';
    await expect(
      authenticateSequencerReceipt(rpc, mutated, entry.receiptJson.txHash),
      `log ${field}`
    ).rejects.toThrow();
  }
  for (const keys of [
    [],
    [{ ...entry.pinnedKey, keyId: `0x${'42'.repeat(32)}` }],
    [{ ...entry.pinnedKey, publicKey: `0x${'42'.repeat(32)}` }],
    [{ ...entry.pinnedKey, firstHeight: entry.pinnedKey.lastHeight }],
    [{ ...entry.pinnedKey, lastHeight: entry.pinnedKey.firstHeight }],
  ]) {
    await expect(
      authenticateSequencerReceipt(
        (method, params) =>
          method === 'px_getReceiptKeys' ? Promise.resolve({ keys }) : rpc(method, params),
        entry.receiptJson,
        entry.receiptJson.txHash
      )
    ).rejects.toThrow();
  }
}

test(
  surface_chain_refuses_removed_or_mutated_authenticated_fields.name,
  surface_chain_refuses_removed_or_mutated_authenticated_fields
);

async function surface_chain_refuses_unrecorded_calls_and_invented_logs() {
  const chain = new SurfaceChain();
  const entry = nodeReceiptVectors().find(({ id }) => id === 'V1-erc20-approve')!;
  const call = nodeReceiptCall(entry);
  for (const method of ['eth_sendTransaction', 'paxeer_sendTransactionReceipted']) {
    for (const changed of [
      { ...call, to: ethers.ZeroAddress },
      { ...call, data: `${call.data}00` },
      { ...call, value: '0x1' },
    ]) {
      await expect(chain.request({ method, params: [changed] })).rejects.toThrow(
        'no node receipt vector'
      );
    }
    chain.logsFor = () => [{ address: call.to, topics: [], data: '0x' }];
    await expect(chain.request({ method, params: [call] })).rejects.toThrow('logs differ');
    expect(chain.sent).toHaveLength(0);
    expect(
      await chain.request({ method: 'pax_sequencerReceipt', params: [entry.receiptJson.txHash] })
    ).toBeNull();
  }
}

test(
  surface_chain_refuses_unrecorded_calls_and_invented_logs.name,
  surface_chain_refuses_unrecorded_calls_and_invented_logs
);

function surface_sdk_uses_the_browser_entry_without_sqlite() {
  expect(layerxSdk.verifySequencerReceipt).toBe(verifySequencerReceipt);
  expect(layerxSdk).not.toHaveProperty('PreparedGrantDraws');
  expect(layerxSdk.bridgeOutCall(1n, ethers.ZeroAddress, 1n, ethers.ZeroAddress).to).toBe(
    layerxSdk.LAYERX_BRIDGE_PRECOMPILE
  );
}

test(
  surface_sdk_uses_the_browser_entry_without_sqlite.name,
  surface_sdk_uses_the_browser_entry_without_sqlite
);
