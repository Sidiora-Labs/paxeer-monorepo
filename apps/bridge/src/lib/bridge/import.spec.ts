import { encodeAbiParameters, encodeEventTopics, getAddress, zeroAddress, zeroHash } from 'viem';
import type { Address, Hex, Log, TransactionReceipt } from 'viem';
import { expect, test } from 'vitest';

import { bridgeChainByKey, type BridgeChain } from './chains';
import { LAYERX_BRIDGE_ADDRESS, layerxBridgeAbi, paxeerXVaultAbi } from './contracts';
import { paxeerRecipient } from './digests';
import { importDeposit, importWithdrawal, matchDepositLog, nonceFromBridgeOutLogs } from './import';
import type { BridgeTransfer } from './transfers';

const vault: Address = '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
const account: Address = '0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';
const asset: Address = '0xcccccccccccccccccccccccccccccccccccccccc';
const recipient: Address = '0xdddddddddddddddddddddddddddddddddddddddd';
const other: Address = '0x000000000000000000000000000000000000dead';

const transfer: BridgeTransfer = {
  id: `${ zeroHash }:7`,
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
    blockHash: zeroHash,
    blockNumber: BigInt(1),
    transactionHash: zeroHash,
    transactionIndex: 0,
    removed: false,
  };
}

function depositLog(logIndex: number, overrides: {
  address?: Address;
  asset?: Address;
  sender?: Address;
  recipient?: Address;
  amount?: bigint;
} = {}): Log {
  return receiptLog(
    overrides.address ?? vault,
    encodeEventTopics({
      abi: paxeerXVaultAbi,
      eventName: 'BridgeDeposit',
      args: {
        asset: overrides.asset ?? asset,
        sender: overrides.sender ?? account,
        paxeerRecipient: paxeerRecipient(overrides.recipient ?? recipient),
      },
    }),
    encodeAbiParameters([ { type: 'uint256' }, { type: 'uint64' } ], [ overrides.amount ?? BigInt(transfer.amount), BigInt(9) ]),
    logIndex,
  );
}

function bridgeOutLog(address: Address, nonce: bigint): Log {
  return receiptLog(
    address,
    encodeEventTopics({
      abi: layerxBridgeAbi,
      eventName: 'BridgeOut',
      args: { chain: BigInt(8453), asset, nonce },
    }),
    encodeAbiParameters([ { type: 'uint256' }, { type: 'address' } ], [ BigInt(transfer.amount), recipient ]),
    0,
  );
}

function matchDepositLogRequiresTheAssetAndRecipient() {
  const wrongAsset = depositLog(0, { asset: other });
  const wrongRecipient = depositLog(1, { recipient: other });
  const wrongVault = depositLog(2, { address: other });
  const wrongSender = depositLog(3, { sender: other });
  const wrongAmount = depositLog(4, { amount: BigInt(transfer.amount) + BigInt(1) });
  const mismatches = [ wrongAsset, wrongRecipient, wrongVault, wrongSender, wrongAmount ];

  for (const log of mismatches) {
    expect(matchDepositLog(transfer, [ log ])).toBeUndefined();
  }

  const matching = depositLog(7, { address: getAddress(vault) });
  const hit = matchDepositLog({ ...transfer, account: getAddress(account), assetId: getAddress(asset) }, [ ...mismatches, matching ]);
  expect(hit?.logIndex).toBe(7);
  expect(hit?.args.nonce).toBe(BigInt(9));
  expect(matchDepositLog(transfer, [])).toBeUndefined();
  expect(matchDepositLog({ ...transfer, vault: undefined }, [ matching ])).toBeUndefined();
}

function nonceFromBridgeOutLogsReadsTheNonce() {
  const log = bridgeOutLog(LAYERX_BRIDGE_ADDRESS, BigInt(9));
  expect(nonceFromBridgeOutLogs([ log ])).toBe(BigInt(9));
  expect(nonceFromBridgeOutLogs([])).toBeUndefined();
  expect(nonceFromBridgeOutLogs([ depositLog(0, { address: LAYERX_BRIDGE_ADDRESS }), log ])).toBe(BigInt(9));
}

function nonceFromBridgeOutLogsIgnoresAnotherAddress() {
  const impostor = bridgeOutLog(other, BigInt(5));
  expect(nonceFromBridgeOutLogs([ impostor ])).toBeUndefined();
  expect(nonceFromBridgeOutLogs([ impostor, bridgeOutLog(LAYERX_BRIDGE_ADDRESS, BigInt(9)) ])).toBe(BigInt(9));
}

test(matchDepositLogRequiresTheAssetAndRecipient);
test(nonceFromBridgeOutLogsReadsTheNonce);
test(nonceFromBridgeOutLogsIgnoresAnotherAddress);

const sourceReceipt: TransactionReceipt = {
  blockHash: '0x1111111111111111111111111111111111111111111111111111111111111111',
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
  transactionHash: '0x2222222222222222222222222222222222222222222222222222222222222222',
  transactionIndex: 2,
  type: 'eip1559',
};

const importChain: BridgeChain = {
  key: 'base',
  label: 'Base',
  kind: 'evm',
  chainId: 8453n,
  vault,
  finalityDepth: 1800,
  blockSeconds: 2,
  nativeSymbol: 'ETH',
  state: 'configured',
  assets: [ {
    key: 'token',
    symbol: 'TOKEN',
    decimals: 6,
    address: asset,
    assetId: asset,
    isNative: false,
    perTxCap: '10000000',
    totalCap: '100000000',
  } ],
};

function importDepositBuildsARecordFromTheVaultLog() {
  const vaultLog: Log = {
    address: getAddress(vault),
    topics: [
      '0x19a8713a4594d98824323571a687838aa76b4d730cfedde1785ee8bb92195190',
      '0x000000000000000000000000cccccccccccccccccccccccccccccccccccccccc',
      '0x000000000000000000000000bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
      '0x000000000000000000000000dddddddddddddddddddddddddddddddddddddddd',
    ],
    data: '0x000000000000000000000000000000000000000000000000000000000074cbb10000000000000000000000000000000000000000000000000000000000000009',
    logIndex: 7,
    blockHash: sourceReceipt.blockHash,
    blockNumber: sourceReceipt.blockNumber,
    transactionHash: sourceReceipt.transactionHash,
    transactionIndex: sourceReceipt.transactionIndex,
    removed: false,
  };
  const receipt: TransactionReceipt = {
    ...sourceReceipt,
    from: other,
    logs: [
      depositLog(1, { address: other, amount: 900n, recipient: other }),
      vaultLog,
    ],
  };
  const record = importDeposit(importChain, receipt);
  expect(record).toEqual({
    id: `${ sourceReceipt.transactionHash }:in`,
    direction: 'in',
    foreignChainKey: 'base',
    foreignChainId: '8453',
    symbol: 'TOKEN',
    decimals: 6,
    amount: '7654321',
    assetId: getAddress(asset),
    vault,
    account: getAddress(account),
    recipient: getAddress(recipient),
    sourceTxHash: sourceReceipt.transactionHash,
    requiredConfirmations: 1800,
    submittedAt: expect.any(Number),
    logIndex: '7',
    nonce: '9',
  });
  expect(matchDepositLog(record, receipt.logs)?.args.paxeerRecipient).toBe(paxeerRecipient(recipient).toLowerCase());
}

function importDepositRefusesAReceiptWithoutAVaultLog() {
  const impostorReceipt: TransactionReceipt = {
    ...sourceReceipt,
    logs: [ depositLog(7, { address: other }), bridgeOutLog(vault, 9n) ],
  };
  expect(() => importDeposit(importChain, impostorReceipt)).toThrow(`No BridgeDeposit log from vault ${ vault } was found.`);
  expect(() => importDeposit(importChain, { ...sourceReceipt, logs: [] })).toThrow(vault);
  expect(() => importDeposit(importChain, {
    ...sourceReceipt,
    status: 'reverted',
    logs: [ depositLog(7) ],
  })).toThrow('reverted');
}

function importWithdrawalReadsTheBridgeOutLog() {
  const base = bridgeChainByKey('base');
  if (!base) throw new Error('The Base bridge route must be registered.');
  const validLog: Log = {
    address: LAYERX_BRIDGE_ADDRESS,
    topics: [
      '0x3e990eb54009dcdca53d8fa87307210f07097f37dcf6185dee71a42f8e7d524e',
      '0x0000000000000000000000000000000000000000000000000000000000002105',
      '0x0000000000000000000000000000000000000000000000000000000000000000',
      '0x000000000000000000000000000000000000000000000000000000000000002a',
    ],
    data: '0x000000000000000000000000000000000000000000000000000000000012d687000000000000000000000000dddddddddddddddddddddddddddddddddddddddd',
    logIndex: 8,
    blockHash: sourceReceipt.blockHash,
    blockNumber: sourceReceipt.blockNumber,
    transactionHash: sourceReceipt.transactionHash,
    transactionIndex: sourceReceipt.transactionIndex,
    removed: false,
  };
  const receipt: TransactionReceipt = {
    ...sourceReceipt,
    to: LAYERX_BRIDGE_ADDRESS,
    logs: [ bridgeOutLog(other, 5n), depositLog(3, { address: LAYERX_BRIDGE_ADDRESS }), validLog ],
  };
  expect(importWithdrawal(receipt)).toEqual({
    id: `${ sourceReceipt.transactionHash }:out`,
    direction: 'out',
    foreignChainKey: 'base',
    foreignChainId: '8453',
    symbol: 'ETH',
    decimals: 18,
    amount: '1234567',
    assetId: zeroAddress,
    vault: base.vault,
    account,
    recipient: getAddress(recipient),
    sourceTxHash: sourceReceipt.transactionHash,
    requiredConfirmations: 1,
    submittedAt: expect.any(Number),
    bridgeOutNonce: '42',
  });
  expect(() => importWithdrawal({ ...receipt, logs: [ bridgeOutLog(other, 5n) ] })).toThrow(LAYERX_BRIDGE_ADDRESS);
  expect(() => importWithdrawal({ ...receipt, status: 'reverted' })).toThrow('reverted');
}

test(importDepositBuildsARecordFromTheVaultLog);
test(importDepositRefusesAReceiptWithoutAVaultLog);
test(importWithdrawalReadsTheBridgeOutLog);
