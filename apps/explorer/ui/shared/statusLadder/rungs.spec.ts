import { PAXEER_X_STATUS_RUNGS } from 'types/api/paxeerXLists';

import { expect, test } from 'vitest';

import { RUNG_ORDER, RUNGS } from './rungs';

function rung_order_is_three_rungs() {
  expect(PAXEER_X_STATUS_RUNGS).toEqual([ 'submitted', 'receipted', 'committed' ]);
  expect(RUNG_ORDER.map((item) => item.rung)).toEqual([ 'submitted', 'receipted', 'committed' ]);
  expect(Object.keys(RUNGS)).toEqual([ 'submitted', 'receipted', 'committed' ]);
  expect(RUNG_ORDER.map(({ label, icon, colorPalette }) => ({ label, icon, colorPalette }))).toEqual([
    { label: 'Submitted', icon: 'status/pending', colorPalette: 'gray' },
    { label: 'Receipted', icon: 'clock', colorPalette: 'blue' },
    { label: 'Committed', icon: 'status/success', colorPalette: 'green' },
  ]);
  expect(RUNGS.submitted.description).toContain('submitted to the node');
  expect(RUNGS.submitted.description).toContain('awaiting a receipt');
  expect(RUNGS.submitted.description).toContain('committed block');
  expect(RUNGS.receipted.description).toBe('A transaction receipt is available, and finality requires its block to be committed.');
  expect(RUNGS.committed.description).toBe('The transaction receipt is final because its block is committed.');
}

test(rung_order_is_three_rungs.name, rung_order_is_three_rungs);
