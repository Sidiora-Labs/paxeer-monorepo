import { expect, test } from 'vitest';

import type { TransferLive } from '@/components/bridge/transfer-actions';
import { historyStatusOf, phaseOf, stepOf } from './phase';
import type { BridgeTransfer } from './transfers';

const deposit: BridgeTransfer = {
  id: '0x1234:in',
  direction: 'in',
  foreignChainKey: 'base',
  foreignChainId: '8453',
  symbol: 'USDC',
  decimals: 6,
  amount: '1000000',
  assetId: '0x0000000000000000000000000000000000000001',
  account: '0xabcdefabcdefabcdefabcdefabcdefabcdefabcd',
  recipient: '0xabcdefabcdefabcdefabcdefabcdefabcdefabcd',
  sourceTxHash: '0x1234',
  requiredConfirmations: 12,
  submittedAt: 1000,
};

const finalLive: TransferLive = {
  confirmations: 12,
  requiredConfirmations: 12,
  credited: false,
  released: false,
  attestationReady: false,
  sourceFailed: false,
};

function stepOfKeepsAttestingAndReadyApart(): void {
  expect(stepOf('attesting')).toBe('attesting');
  expect(stepOf('ready')).toBe('ready');
  for (const direction of [ 'in', 'out' ] as const) {
    const transfer = { ...deposit, direction };
    expect(stepOf(phaseOf(transfer, finalLive, false))).toBe('attesting');
    expect(stepOf(phaseOf(transfer, finalLive, true))).toBe('ready');
  }
  expect(stepOf('confirming')).toBe('confirming');
  expect(stepOf('settling')).toBe('submitted');
  expect(stepOf('settled')).toBe('credited');
  expect(stepOf('failed')).toBe('failed');
}

function historyStatusOfReportsAFailedSource(): void {
  for (const direction of [ 'in', 'out' ] as const) {
    const transfer = { ...deposit, direction };
    const reverted: TransferLive = { ...finalLive, sourceFailed: true };
    expect(phaseOf(transfer, reverted, false)).toBe('failed');
    expect(historyStatusOf(phaseOf(transfer, reverted, false))).toBe('failed');
    expect(historyStatusOf(phaseOf(transfer, reverted, true))).toBe('failed');
    expect(historyStatusOf(phaseOf(transfer, undefined, false))).toBe('processing');
    expect(historyStatusOf(phaseOf(transfer, finalLive, false))).toBe('processing');
    expect(historyStatusOf(phaseOf(transfer, finalLive, true))).toBe('processing');
  }
  expect(historyStatusOf('settling')).toBe('processing');
  expect(historyStatusOf(phaseOf(deposit, { ...finalLive, credited: true }, false))).toBe('completed');
  expect(historyStatusOf(phaseOf({ ...deposit, claimedAt: 0 }, undefined, false))).toBe('completed');
  expect(historyStatusOf(phaseOf({ ...deposit, claimTxHash: '0xabcd' }, undefined, false))).toBe('completed');
  expect(historyStatusOf(phaseOf({ ...deposit, direction: 'out', releaseTxHash: '0xabcd' }, undefined, false))).toBe('completed');
}

test(stepOfKeepsAttestingAndReadyApart);
test(historyStatusOfReportsAFailedSource);
