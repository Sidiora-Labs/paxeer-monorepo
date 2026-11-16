// @vitest-environment jsdom

import React from 'react';

import type { ChainIndicatorId } from 'types/homepage';

import { render } from 'ui/shared/layout/testWrapper';
import { expect, test } from 'vitest';
import { fireEvent, screen } from 'vitest/lib';

import ChainIndicatorItem from './ChainIndicatorItem';

const selections: Array<ChainIndicatorId> = [];

function handleSelection(id: ChainIndicatorId) {
  selections.push(id);
}

test('indicatorSelectionUsesNamedNativeButton', function indicatorSelectionUsesNamedNativeButton() {
  selections.length = 0;
  const { rerender } = render(
    <ul>
      <ChainIndicatorItem
        indicator={{ id: 'daily_txs', title: 'Daily transactions', value: '125', icon: <span aria-hidden="true">T</span> }}
        isSelected={ false }
        isLoading={ false }
        onClick={ handleSelection }
      />
    </ul>,
  );
  const button = screen.getByRole('button', { name: 'Daily transactions' });
  expect(button.tagName).toBe('BUTTON');
  expect(button.getAttribute('type')).toBe('button');
  expect(button.getAttribute('aria-pressed')).toBe('false');
  button.focus();
  expect(document.activeElement).toBe(button);
  fireEvent.click(button);
  expect(selections).toEqual([ 'daily_txs' ]);

  rerender(
    <ul>
      <ChainIndicatorItem
        indicator={{ id: 'daily_txs', title: 'Daily transactions', value: '125', icon: <span aria-hidden="true">T</span> }}
        isSelected
        isLoading
        onClick={ handleSelection }
      />
    </ul>,
  );
  const loading = screen.getByRole('button', { name: 'Daily transactions' });
  expect(loading.getAttribute('aria-pressed')).toBe('true');
  expect((loading as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(loading);
  expect(selections).toEqual([ 'daily_txs' ]);
});
