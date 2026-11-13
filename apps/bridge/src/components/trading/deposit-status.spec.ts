// @vitest-environment jsdom

import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { getAddress, keccak256, toHex, zeroAddress } from 'viem';
import { expect, test } from 'vitest';

import type { TransferLive, TransferPhase } from '@/components/bridge/transfer-actions';
import { DepositStatus, TransferSteps } from '@/components/trading/deposit-status';
import type { TransferStatusItem, TransferStep } from '@/components/trading/deposit-status';
import depositFixture from '@/lib/bridge/fixtures/deposit-ready.json';
import { phaseOf, stepOf } from '@/lib/bridge/phase';
import { settlementBroadcastPatch } from '@/lib/bridge/settlement';
import { listTransfers, saveTransfer, transferIdOf, updateTransfer } from '@/lib/bridge/transfers';
import type { BridgeTransfer, TransferDirection } from '@/lib/bridge/transfers';

const phaseCases = {
  confirming: { step: 'confirming', confirmations: 3, ready: false, current: 1, completed: 1 },
  attesting: { step: 'attesting', confirmations: 12, ready: false, current: 2, completed: 2 },
  ready: { step: 'ready', confirmations: 12, ready: true, current: 3, completed: 3 },
  settling: { step: 'submitted', confirmations: 12, ready: true, current: 0, completed: 0 },
  settled: { step: 'credited', confirmations: 12, ready: true, current: -1, completed: 5 },
  failed: { step: 'failed', confirmations: 3, ready: false, current: 1, completed: 1 },
} as const satisfies Record<TransferPhase, {
  step: TransferStep;
  confirmations: number;
  ready: boolean;
  current: number;
  completed: number;
}>;

function transferFixture(direction: TransferDirection): BridgeTransfer {
  const sourceTxHash = keccak256(toHex(`registry-transfer-step:${ direction }`));
  const account = getAddress(`0x${ depositFixture.recipient.slice(-40) }`);
  return {
    id: transferIdOf(direction, sourceTxHash),
    direction,
    sourceTxHash,
    foreignChainKey: 'ethereum',
    foreignChainId: depositFixture.chainId,
    symbol: 'ETH',
    decimals: 18,
    amount: depositFixture.amount,
    assetId: zeroAddress,
    vault: getAddress(depositFixture.vault),
    account,
    recipient: account,
    requiredConfirmations: 12,
    submittedAt: 1000,
  };
}

function displayItem(transfer: BridgeTransfer, live: TransferLive): TransferStatusItem {
  return {
    id: transfer.id,
    type: transfer.direction === 'in' ? 'deposit' : 'withdraw',
    token: transfer.symbol,
    amount: Number(transfer.amount) / 10 ** transfer.decimals,
    step: stepOf(phaseOf(transfer, live, live.attestationReady)),
    confirmations: live.confirmations,
    requiredConfirmations: live.requiredConfirmations,
  };
}

function renderStatus(item: TransferStatusItem) {
  const container = document.createElement('div');
  container.innerHTML = renderToStaticMarkup(createElement(DepositStatus, { items: [ item ], open: true }));
  return container;
}

function registryTransferStepDisplaysEveryProducedPhase() {
  const originalStorage = window.localStorage.getItem('paxeer-bridge:transfers:v1');
  try {
    for (const direction of [ 'in', 'out' ] as const) {
      const producedSteps = new Set<TransferStep>();
      for (const expectedPhase of Object.keys(phaseCases) as Array<TransferPhase>) {
        const scenario = phaseCases[expectedPhase];
        const submitted = transferFixture(direction);
        saveTransfer(submitted);
        if (expectedPhase === 'settling') {
          updateTransfer(submitted.id, settlementBroadcastPatch(submitted, keccak256(toHex('settlement')), 2000));
        }
        const transfer = listTransfers(submitted.account).find((entry) => entry.id === submitted.id);
        expect(transfer).toBeDefined();
        if (!transfer) throw new Error('The production ledger did not return the submitted transfer.');
        const live: TransferLive = {
          confirmations: scenario.confirmations,
          requiredConfirmations: transfer.requiredConfirmations,
          credited: direction === 'in' && expectedPhase === 'settled',
          released: direction === 'out' && expectedPhase === 'settled',
          attestationReady: scenario.ready,
          sourceFailed: expectedPhase === 'failed',
        };
        expect(phaseOf(transfer, live, live.attestationReady)).toBe(expectedPhase);
        const item = displayItem(transfer, live);
        expect(item.step).toBe(scenario.step);
        producedSteps.add(item.step);

        const container = renderStatus(item);
        const progress = container.querySelector('ol[aria-label="Progress"]');
        expect(progress).not.toBeNull();
        const steps = Array.from(progress?.children ?? []);
        const labels = [
          'Submitted', `Confirming ${ scenario.confirmations }/12`, 'Attesting', 'Ready', direction === 'in' ? 'Credited' : 'Sent',
        ];
        if (expectedPhase === 'failed') labels[scenario.current] = 'Failed';
        expect(steps.map((step) => step.textContent)).toEqual(labels);
        expect(steps.map((step) => step.getAttribute('aria-current')))
          .toEqual(labels.map((_, index) => index === scenario.current ? 'step' : null));
        expect(progress?.querySelectorAll('svg.lucide-check')).toHaveLength(scenario.completed);
        expect(progress?.querySelectorAll('svg.lucide-x')).toHaveLength(expectedPhase === 'failed' ? 1 : 0);
        const summary = container.querySelector('button[aria-expanded="true"]');
        const summaryText = expectedPhase === 'settled' ? 'You have a successful transaction' :
          expectedPhase === 'failed' ? 'A transaction failed' : 'You have a pending transaction';
        expect(summary?.textContent).toContain(summaryText);
      }
      expect([ ...producedSteps ].sort()).toEqual([ 'submitted', 'confirming', 'attesting', 'ready', 'credited', 'failed' ].sort());
    }
  } finally {
    if (originalStorage === null) window.localStorage.removeItem('paxeer-bridge:transfers:v1');
    else window.localStorage.setItem('paxeer-bridge:transfers:v1', originalStorage);
  }
}

function registryTransferStepFailureShowsWhereProgressStopped() {
  for (const confirmations of [ 0, 3 ]) {
    const transfer = transferFixture('in');
    const item = displayItem(transfer, {
      confirmations,
      requiredConfirmations: transfer.requiredConfirmations,
      credited: false,
      released: false,
      attestationReady: false,
      sourceFailed: true,
    });
    expect(item.step).toBe('failed');
    const container = renderStatus(item);
    const steps = Array.from(container.querySelectorAll('ol[aria-label="Progress"] > li'));
    const stoppedAt = confirmations === 0 ? 0 : 1;
    expect(steps.findIndex((step) => step.getAttribute('aria-current') === 'step')).toBe(stoppedAt);
    expect(steps[stoppedAt].textContent).toBe('Failed');
    expect(steps[stoppedAt].querySelector('svg.lucide-x')).not.toBeNull();
    expect(container.querySelectorAll('svg.lucide-check')).toHaveLength(stoppedAt);
    expect(container.textContent).toContain('A transaction failed');
    expect(container.textContent).not.toContain('You have a successful transaction');
  }
}

function registryTransferStepRejectsAnUnknownConsumerState() {
  const item = displayItem(transferFixture('in'), {
    confirmations: 12,
    requiredConfirmations: 12,
    credited: false,
    released: false,
    attestationReady: true,
    sourceFailed: false,
  });
  expect(item.step).toBe('ready');
  const malformed: TransferStatusItem = JSON.parse(JSON.stringify({ ...item, step: 'unknown' }));
  expect(() => renderStatus(malformed)).toThrow('Invalid transfer progress step.');
  expect(() => renderToStaticMarkup(createElement(TransferSteps, { item: malformed })))
    .toThrow('Invalid transfer progress step.');
}

test(registryTransferStepDisplaysEveryProducedPhase.name, registryTransferStepDisplaysEveryProducedPhase);
test(registryTransferStepFailureShowsWhereProgressStopped.name, registryTransferStepFailureShowsWhereProgressStopped);
test(registryTransferStepRejectsAnUnknownConsumerState.name, registryTransferStepRejectsAnUnknownConsumerState);
