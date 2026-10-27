import { encodeAbiParameters, encodeEventTopics, getAddress, zeroHash } from 'viem';
import type { Address, Hex, Log } from 'viem';
import { expect, test } from 'vitest';

import { LAYERX_BRIDGE_ADDRESS, layerxBridgeAbi, paxeerXVaultAbi } from './contracts';
import { paxeerRecipient } from './digests';
import { matchDepositLog, nonceFromBridgeOutLogs } from './import';
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
