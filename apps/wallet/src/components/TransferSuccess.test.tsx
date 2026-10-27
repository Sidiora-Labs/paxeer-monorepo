// @vitest-environment jsdom
import { expect, test } from 'vitest';
import { statusLadder, submittedTransfer, type TransferObservation } from '@paxeer/wallet';
import { mount } from '@/account/test/render';
import { TransferSuccessResult } from './TransferSuccess';

async function transfer_success_completes_only_at_the_committed_rung() {
  const identity = { hash: `0x${'ab'.repeat(32)}`, chainId: 125 };
  const submitted = submittedTransfer(identity);
  const receipted = statusLadder.fromExplorer('receipted');
  const committed = statusLadder.fromExplorer('committed');
  const included: TransferObservation = {
    ...submitted,
    truth: 'included',
    blockNumber: 42,
    blockHash: `0x${'cd'.repeat(32)}`,
  };
  const cases: ReadonlyArray<{
    observation: TransferObservation;
    title: string;
    completed: boolean;
    terminal: boolean;
  }> = [
    { observation: submitted, title: 'Transfer Submitted', completed: false, terminal: false },
    {
      observation: { ...submitted, truth: 'pending' },
      title: 'Transfer Pending',
      completed: false,
      terminal: false,
    },
    {
      observation: { ...submitted, truth: 'pending', steps: [receipted] },
      title: 'Transfer Pending',
      completed: false,
      terminal: false,
    },
    { observation: included, title: 'Transfer Included', completed: false, terminal: false },
    {
      observation: { ...included, steps: [statusLadder.fromExplorer('submitted')] },
      title: 'Transfer Included',
      completed: false,
      terminal: false,
    },
    {
      observation: { ...included, steps: [receipted] },
      title: 'Receipted',
      completed: false,
      terminal: false,
    },
    {
      observation: { ...included, steps: [receipted, statusLadder.fromExplorer('submitted')] },
      title: 'Receipted',
      completed: false,
      terminal: false,
    },
    {
      observation: { ...included, steps: [committed] },
      title: 'Transfer Completed',
      completed: true,
      terminal: true,
    },
    {
      observation: { ...included, steps: [committed, receipted] },
      title: 'Transfer Completed',
      completed: true,
      terminal: true,
    },
    {
      observation: { ...included, truth: 'unknown', steps: [committed] },
      title: 'Transfer Status Unknown',
      completed: false,
      terminal: false,
    },
    {
      observation: {
        ...submitted,
        truth: 'unknown',
        lastVerified: { blockNumber: 42, blockHash: `0x${'cd'.repeat(32)}`, steps: [committed] },
      },
      title: 'Transfer Status Unknown',
      completed: false,
      terminal: false,
    },
    {
      observation: { ...included, truth: 'reverted', steps: [] },
      title: 'Transfer Reverted',
      completed: false,
      terminal: true,
    },
    {
      observation: { ...included, truth: 'replaced', steps: [] },
      title: 'Transfer Replaced',
      completed: false,
      terminal: true,
    },
  ];

  for (const { observation, title, completed, terminal } of cases) {
    const dismissed: boolean[] = [];
    const view = await mount(
      <TransferSuccessResult
        fromLabel="Sender"
        fromAmount="1"
        fromSymbol="PAX"
        toLabel="Recipient"
        transfer={identity}
        observation={observation}
        error=""
        onDone={(value) => dismissed.push(value)}
      />
    );
    try {
      expect(view.text('h2')).toBe(title);
      expect(view.text('button')).toBe(completed ? 'Done' : 'Close');
      expect(view.all('svg title').some((element) => element.textContent === 'Success')).toBe(
        completed
      );
      await view.click('button');
      expect(dismissed).toEqual([terminal]);
    } finally {
      await view.unmount();
    }
  }
}

test(transfer_success_completes_only_at_the_committed_rung);
