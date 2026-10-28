import type { Address, Hex } from 'viem';

import { expect, test } from 'vitest';

import { BRIDGE_CHAINS } from './chains';
import { depositRecord, withdrawalRecord } from './submitted';
import { transferIdOf, type BridgeTransfer } from './transfers';

const hash: Hex = '0xABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789AB';
const account: Address = '0x1111111111111111111111111111111111111111';
const vault: Address = '0x2222222222222222222222222222222222222222';
const recipient: Address = '0xAbCdEf0123456789aBcDeF0123456789aBCdEf01';

function depositRecordCarriesTheVaultAndFinalityDepth() {
  const foreign = BRIDGE_CHAINS.find((chain) => chain.key === 'base')!;
  const asset = foreign.assets[0];
  const amount = 1234567890123456789n;
  const startedAt = Date.now();

  for (const requiredConfirmations of [ foreign.finalityDepth, foreign.finalityDepth + 17 ]) {
    const record: BridgeTransfer = depositRecord({
      hash, foreign, asset, account, amount, vault, recipient, requiredConfirmations,
    });

    expect(record).toEqual({
      id: transferIdOf('in', hash),
      direction: 'in',
      foreignChainKey: foreign.key,
      foreignChainId: foreign.chainId.toString(),
      symbol: asset.symbol,
      decimals: asset.decimals,
      amount: '1234567890123456789',
      assetId: asset.assetId,
      vault,
      account,
      recipient,
      sourceTxHash: hash,
      requiredConfirmations,
      submittedAt: expect.any(Number),
    });
    expect(record.id).toBe(`${ hash.toLowerCase() }:in`);
    expect(record.submittedAt).toBeGreaterThanOrEqual(startedAt);
    expect(record.submittedAt).toBeLessThanOrEqual(Date.now());
    expect(() => JSON.stringify(record)).not.toThrow();
  }
}

function withdrawalRecordKeepsTheTypedDestination() {
  for (const key of [ 'base', 'solana' ]) {
    const foreign = { ...BRIDGE_CHAINS.find((chain) => chain.key === key)!, vault };
    const asset = foreign.assets[0];
    const destination = key === 'solana' ? 'So11111111111111111111111111111111111111112' : recipient;
    const amount = 9007199254740993n;
    const startedAt = Date.now();
    const record: BridgeTransfer = withdrawalRecord({
      hash, foreign, asset, account, amount, destination, requiredConfirmations: foreign.finalityDepth,
    });

    expect(record).toEqual({
      id: transferIdOf('out', hash),
      direction: 'out',
      foreignChainKey: key,
      foreignChainId: key === 'solana' ? '91600046870081' : '8453',
      symbol: asset.symbol,
      decimals: asset.decimals,
      amount: '9007199254740993',
      assetId: asset.assetId,
      account,
      recipient: destination,
      sourceTxHash: hash,
      requiredConfirmations: foreign.finalityDepth,
      submittedAt: expect.any(Number),
    });
    expect(record.id).toBe(`${ hash.toLowerCase() }:out`);
    expect(record).not.toHaveProperty('vault');
    expect(record.submittedAt).toBeGreaterThanOrEqual(startedAt);
    expect(record.submittedAt).toBeLessThanOrEqual(Date.now());
    expect(() => JSON.stringify(record)).not.toThrow();
  }
}

test(depositRecordCarriesTheVaultAndFinalityDepth.name, depositRecordCarriesTheVaultAndFinalityDepth);
test(withdrawalRecordKeepsTheTypedDestination.name, withdrawalRecordKeepsTheTypedDestination);
