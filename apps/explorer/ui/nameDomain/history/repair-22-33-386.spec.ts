// @vitest-environment jsdom

import { fireEvent, within } from '@testing-library/react';
import React from 'react';

import type * as bens from '@blockscout/bens-types';

import 'configs/app';
import { SettingsContextProvider } from 'lib/contexts/settings';
import { ensDomainEventA, ensDomainEventB } from 'mocks/ens/events';
import { render } from 'ui/shared/layout/testWrapper';
import { describe, expect, it } from 'vitest';

import NameDomainHistoryTable from './NameDomainHistoryTable';
import type { Sort, SortField } from './utils';
import { getNextSortValue, sortFn } from './utils';

function HistoryControls({ history }: { history: bens.ListDomainEventsResponse }) {
  const [ sort, setSort ] = React.useState<Sort>('default');
  const handleSortToggle = React.useCallback((field: SortField) => {
    setSort(getNextSortValue(field));
  }, []);

  return React.createElement(SettingsContextProvider, null,
    React.createElement(NameDomainHistoryTable, {
      history,
      domain: undefined,
      sort,
      onSortToggle: handleSortToggle,
    }),
  );
}

function assertHistoryRows(table: HTMLElement, events: Array<bens.DomainEvent>) {
  const rows = within(table).getAllByRole('row').slice(1);
  expect(rows).toHaveLength(events.length);
  rows.forEach((row, index) => {
    const cells = within(row).getAllByRole('cell');
    expect(within(cells[0]).getByRole('link').getAttribute('href')).toBe(`/tx/${ events[index].transaction_hash }`);
    expect(cells[3].textContent).toBe(events[index].action);
  });
}

function exerciseHistoryControls(events: Array<bens.DomainEvent>) {
  const history: bens.ListDomainEventsResponse = { items: events.slice() };
  const originalItems = history.items.slice();
  const view = render(React.createElement(HistoryControls, { history }));
  const table = view.getByRole('table');
  const timestamp = within(table).getByText('Timestamp');

  assertHistoryRows(table, originalItems);

  fireEvent.click(timestamp);
  assertHistoryRows(table, [ ensDomainEventB, ensDomainEventA ]);

  fireEvent.click(timestamp);
  assertHistoryRows(table, [ ensDomainEventA, ensDomainEventB ]);

  fireEvent.click(timestamp);
  assertHistoryRows(table, originalItems);
  return history;
}

describe('domain-history-sort-reversed', () => {
  it('cycles descending, ascending and default from oldest first', function domainHistorySortControlsFromOldestFirst() {
    const events = [ ensDomainEventA, ensDomainEventB ];
    const history = exerciseHistoryControls(events);
    expect(history.items).toEqual(events);
  });

  it('cycles descending, ascending and default from newest first', function domainHistorySortControlsFromNewestFirst() {
    const events = [ ensDomainEventB, ensDomainEventA ];
    const history = exerciseHistoryControls(events);
    expect(history.items).toEqual(events);
  });

  it('preserves equal timestamps and default order', function domainHistorySortPreservesEqualTimestampsAndDefaultOrder() {
    const sameTime: bens.DomainEvent = { ...ensDomainEventB, timestamp: ensDomainEventA.timestamp };
    expect(sortFn('timestamp-asc')(ensDomainEventA, sameTime)).toBe(0);
    expect(sortFn('timestamp-desc')(ensDomainEventA, sameTime)).toBe(0);
    expect(sortFn('default')(ensDomainEventA, ensDomainEventB)).toBe(0);
    expect(sortFn(undefined)(ensDomainEventB, ensDomainEventA)).toBe(0);
  });
});
