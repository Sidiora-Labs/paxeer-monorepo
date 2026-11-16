// @vitest-environment jsdom

import { ChakraProvider } from '@chakra-ui/react';
import { cleanup, fireEvent, render } from '@testing-library/react';
import React from 'react';

import { PopoverRoot } from 'toolkit/chakra/popover';
import theme from 'toolkit/theme/theme';
import { afterEach, expect, test } from 'vitest';

import AgeFilter from './filters/AgeFilter';
import AmountFilter from './filters/AmountFilter';

afterEach(cleanup);

function RangeFilterSurface({ children }: { children: React.ReactNode }) {
  return <ChakraProvider value={ theme }><PopoverRoot defaultOpen>{ children }</PopoverRoot></ChakraProvider>;
}

function collectFilterChanges(changes: Array<[string, string | undefined]>) {
  return (field: string, value?: string) => {
    changes.push([ field, value ]);
  };
}

test('amountRangeRejectsNegativeAndReversedBounds', function amountRangeRejectsNegativeAndReversedBounds() {
  const changes: Array<[string, string | undefined]> = [];
  const handleFilterChange = collectFilterChanges(changes);
  const view = render(
    <RangeFilterSurface>
      <AmountFilter handleFilterChange={ handleFilterChange }/>
    </RangeFilterSurface>,
  );
  const from = view.getByLabelText('From');
  const to = view.getByLabelText('To');
  const apply = view.getByRole('button', { name: 'Filter' }) as HTMLButtonElement;
  expect(view.getByRole('button', { name: /^<10$/ }).getAttribute('aria-pressed')).toBe('false');

  fireEvent.change(from, { target: { value: '-1' } });
  expect(from.getAttribute('aria-invalid')).not.toBe('true');
  fireEvent.blur(from);
  expect(from.getAttribute('aria-invalid')).toBe('true');
  expect(view.getByText('Enter a non-negative amount')).toBeDefined();
  expect(apply.disabled).toBe(true);

  fireEvent.change(from, { target: { value: '2.125' } });
  fireEvent.change(to, { target: { value: '1.5' } });
  fireEvent.blur(to);
  expect(to.getAttribute('aria-invalid')).toBe('true');
  expect(view.getByText('Maximum amount must be at least the minimum amount')).toBeDefined();
  fireEvent.click(apply);
  expect(changes).toEqual([]);

  fireEvent.change(to, { target: { value: '3.25' } });
  expect(to.getAttribute('aria-invalid')).not.toBe('true');
  expect(apply.disabled).toBe(false);
  fireEvent.click(apply);
  expect(changes).toEqual([ [ 'amount_from', '2.125' ], [ 'amount_to', '3.25' ] ]);
});

test('ageRangeRejectsFutureAndReversedDates', function ageRangeRejectsFutureAndReversedDates() {
  const changes: Array<[string, string | undefined]> = [];
  const handleFilterChange = collectFilterChanges(changes);
  const view = render(
    <RangeFilterSurface>
      <AgeFilter columnName="Age" handleFilterChange={ handleFilterChange }/>
    </RangeFilterSurface>,
  );
  const from = view.getByLabelText('From');
  const to = view.getByLabelText('To');
  const apply = view.getByRole('button', { name: 'Filter' }) as HTMLButtonElement;
  expect(view.getByRole('button', { name: /^1h$/ }).getAttribute('aria-pressed')).toBe('false');

  fireEvent.change(from, { target: { value: '2099-01-02' } });
  expect(from.getAttribute('aria-invalid')).not.toBe('true');
  fireEvent.blur(from);
  expect(from.getAttribute('aria-invalid')).toBe('true');
  expect(view.getByText('Enter a valid date on or before today')).toBeDefined();
  expect(apply.disabled).toBe(true);

  fireEvent.change(from, { target: { value: '2020-01-02' } });
  fireEvent.change(to, { target: { value: '2020-01-01' } });
  fireEvent.blur(to);
  expect(to.getAttribute('aria-invalid')).toBe('true');
  expect(view.getByText('End date must be on or after the start date')).toBeDefined();
  fireEvent.click(apply);
  expect(changes).toEqual([]);

  fireEvent.change(to, { target: { value: '2020-01-03' } });
  expect(to.getAttribute('aria-invalid')).not.toBe('true');
  expect(apply.disabled).toBe(false);
  fireEvent.click(apply);
  expect(changes).toEqual([
    [ 'age_from', '2020-01-02T00:00:00.000Z' ],
    [ 'age_to', '2020-01-03T23:59:59.999Z' ],
    [ 'age', undefined ],
  ]);
});
