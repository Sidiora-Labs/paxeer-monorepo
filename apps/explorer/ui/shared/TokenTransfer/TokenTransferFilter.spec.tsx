// @vitest-environment jsdom

import React from 'react';

import 'css.escape';

import { render } from 'ui/shared/layout/testWrapper';
import { expect, test, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from 'vitest/lib';

import TokenTransferFilter from './TokenTransferFilter';

test('tokenTransferFilterNamesItsControlsAndChangesDirection', async function tokenTransferFilterNamesItsControlsAndChangesDirection() {
  const onAddressFilterChange = vi.fn();
  render(
    <TokenTransferFilter
      defaultTypeFilters={ [] }
      onTypeFilterChange={ vi.fn() }
      withAddressFilter
      onAddressFilterChange={ onAddressFilterChange }
    />,
  );
  const trigger = screen.getByRole('button', { name: 'Filter token transfers' });
  trigger.focus();
  fireEvent.click(trigger);
  const dialog = await screen.findByRole('dialog', { name: 'Filter token transfers' });
  const directions = within(dialog).getByRole('radiogroup', { name: 'Transfer direction' });
  fireEvent.click(within(directions).getByRole('radio', { name: 'Outgoing transfers' }));
  await waitFor(() => expect(onAddressFilterChange).toHaveBeenCalledWith('from'));
  fireEvent.keyDown(dialog, { key: 'Escape' });
  await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Filter token transfers' })).toBeNull());
  await waitFor(() => expect(document.activeElement).toBe(trigger));
});
