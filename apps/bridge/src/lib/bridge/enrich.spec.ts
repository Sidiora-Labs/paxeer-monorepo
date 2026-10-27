import { encodeAbiParameters, encodeEventTopics, zeroHash } from 'viem';
import type { Address, Hex, Log, TransactionReceipt } from 'viem';
import { expect, test } from 'vitest';

import { LAYERX_BRIDGE_ADDRESS, layerxBridgeAbi, paxeerXVaultAbi } from './contracts';
import { paxeerRecipient } from './digests';
import { enrichRecord } from './enrich';
import type { BridgeTransfer } from './transfers';

const vault: Address = '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
const account: Address = '0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';
const asset: Address = '0xcccccccccccccccccccccccccccccccccccccccc';
const recipient: Address = '0xdddddddddddddddddddddddddddddddddddddddd';
const other: Address = '0x000000000000000000000000000000000000dead';

const transfer: BridgeTransfer = {
  id: `${ zeroHash }:in`,
  direction: 'in',
  foreignChainKey: 'base',
  foreignChainId: '8453',
  symbol: 'TOKEN',
  decimals: 6,
  amount: '1250000',
  assetId: asset,
  vault,
  account,
  recipient,
  sourceTxHash: zeroHash,
  requiredConfirmations: 10,
  submittedAt: 0,
};

const receipt: TransactionReceipt = {
  blockHash: zeroHash,
  blockNumber: 400n,
  contractAddress: null,
  cumulativeGasUsed: 95000n,
  effectiveGasPrice: 1000000000n,
  from: account,
  gasUsed: 95000n,
  logs: [],
  logsBloom: `0x${ '00'.repeat(256) }`,
  status: 'success',
  to: vault,
  transactionHash: zeroHash,
  transactionIndex: 0,
  type: 'eip1559',
};

function receiptLog(address: Address, topics: ReturnType<typeof encodeEventTopics>, data: Hex, logIndex: number): Log {
  const [ signature, ...indexed ] = topics;
  return {
    address,
    topics: [ signature, ...indexed.map((topic) => {
      if (typeof topic !== 'string') throw new Error('Receipt topics must be concrete values');
      return topic;
    }) ],
    data,
    logIndex,
    blockHash: receipt.blockHash,
    blockNumber: receipt.blockNumber,
    transactionHash: receipt.transactionHash,
    transactionIndex: receipt.transactionIndex,
    removed: false,
  };
}

function depositLog(destination: Address, depositAsset: Address = asset): Log {
  return receiptLog(
    vault,
    encodeEventTopics({
      abi: paxeerXVaultAbi,
      eventName: 'BridgeDeposit',
      args: { asset: depositAsset, sender: account, paxeerRecipient: paxeerRecipient(destination) },
    }),
    encodeAbiParameters([ { type: 'uint256' }, { type: 'uint64' } ], [ BigInt(transfer.amount), 9n ]),
    7,
  );
}

function bridgeOutLog(address: Address, nonce: bigint): Log {
  return receiptLog(
    address,
    encodeEventTopics({
      abi: layerxBridgeAbi,
      eventName: 'BridgeOut',
      args: { chain: 8453n, asset, nonce },
    }),
    encodeAbiParameters([ { type: 'uint256' }, { type: 'address' } ], [ BigInt(transfer.amount), recipient ]),
    8,
  );
}

function enrichRecordIgnoresADepositLogForAnotherRecipient() {
  const wrongRecipient = depositLog(other);
  const ignored = enrichRecord(transfer, { ...receipt, logs: [ wrongRecipient ] });
  expect(ignored.logIndex).toBeUndefined();
  expect(ignored.nonce).toBeUndefined();
  expect(ignored).toBe(transfer);

  const wrongAsset = depositLog(recipient, other);
  expect(enrichRecord(transfer, { ...receipt, logs: [ wrongAsset ] })).toBe(transfer);

  const matching = depositLog(recipient);
  const enriched = enrichRecord(transfer, { ...receipt, logs: [ wrongRecipient, wrongAsset, matching ] });
  expect(enriched).toEqual({ ...transfer, logIndex: '7', nonce: '9' });
  expect(transfer.logIndex).toBeUndefined();
  expect(enrichRecord(enriched, { ...receipt, logs: [ depositLog(other) ] })).toBe(enriched);
  expect(enrichRecord(transfer, receipt)).toBe(transfer);
  expect(enrichRecord(transfer, { ...receipt, logs: [ { ...matching, logIndex: null } ] })).toBe(transfer);
}

function enrichRecordReadsTheBridgeOutNonce() {
  const withdrawal: BridgeTransfer = { ...transfer, id: `${ zeroHash }:out`, direction: 'out' };
  const impostor = bridgeOutLog(other, 5n);
  const bridgeReceipt: TransactionReceipt = {
    ...receipt,
    to: LAYERX_BRIDGE_ADDRESS,
    logs: [ impostor, depositLog(recipient), bridgeOutLog(LAYERX_BRIDGE_ADDRESS, 42n) ],
  };

  expect(LAYERX_BRIDGE_ADDRESS).toBe('0x0000000000000000000000000000000000001016');
  const enriched = enrichRecord(withdrawal, bridgeReceipt);
  expect(enriched).toEqual({ ...withdrawal, bridgeOutNonce: '42' });
  expect(withdrawal.bridgeOutNonce).toBeUndefined();
  expect(enrichRecord(withdrawal, { ...bridgeReceipt, logs: [ impostor ] })).toBe(withdrawal);
  expect(enrichRecord(withdrawal, receipt)).toBe(withdrawal);
  expect(enrichRecord(enriched, { ...bridgeReceipt, logs: [ bridgeOutLog(LAYERX_BRIDGE_ADDRESS, 99n) ] })).toBe(enriched);
  expect(enrichRecord(withdrawal, { ...bridgeReceipt, logs: [ bridgeOutLog(LAYERX_BRIDGE_ADDRESS, 0n) ] }).bridgeOutNonce).toBe('0');
}

test(enrichRecordIgnoresADepositLogForAnotherRecipient);
test(enrichRecordReadsTheBridgeOutNonce);
