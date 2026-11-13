// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import React from 'react';

import type { AddressFromToFilter } from 'types/api/address';

import type { ResourcePayload } from 'lib/api/resources';
import { getResourceKey } from 'lib/api/useApiQuery';
import dayjs from 'lib/date/dayjs';
import * as addressFixture from 'mocks/address/address';
import * as transactionFixture from 'mocks/txs/tx';
import CsvExport from 'ui/pages/CsvExport';
import { nextRouterModule, render, routerState } from 'ui/shared/layout/testWrapper';
import { expect, test, vi } from 'vitest';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_RE_CAPTCHA_APP_SITE_KEY: 'test-site-key',
  };
});

import AddressTxs from './AddressTxs';

async function assertDateRangeExportFromSecondPage(filter?: AddressFromToFilter) {
  const address = filter === 'from' ? transactionFixture.base.from.hash : addressFixture.hash;
  const cursor = { block_number: 29611751, index: 8, items_count: 50 };
  routerState.pathname = '/address/[hash]';
  routerState.query = {
    hash: address,
    tab: 'txs_local',
    page: '2',
    next_page_params: encodeURIComponent(JSON.stringify(cursor)),
    ...(filter ? { filter } : {}),
  };

  // Committed API fixtures in the real query store keep this UI regression independent of a running backend.
  const client = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity, retry: false } } });
  const secondPage: ResourcePayload<'general:address_txs'> = {
    items: [ transactionFixture.base ],
    next_page_params: null,
  };
  client.setQueryData(getResourceKey('general:address_txs', {
    pathParams: { hash: address },
    queryParams: { ...cursor, filter },
  }), secondPage);
  client.setQueryData(getResourceKey('general:address', { pathParams: { hash: address } }), { ...addressFixture.eoa, hash: address });
  client.setQueryData(getResourceKey('general:config_csv_export'), { limit: 10000 });

  const withRoute = (component: React.ReactElement) => React.createElement(
    RouterContext.Provider,
    { value: nextRouterModule().useRouter() },
    React.createElement(QueryClientProvider, { client }, component),
  );

  const view = render(withRoute(React.createElement(AddressTxs, { isQueryEnabled: false, txsCount: 100 })));
  try {
    expect(view.container.querySelector('[data-control="page"]')?.textContent).toBe('Page 2');
    expect(view.container.querySelector('[data-title]')?.textContent).toBe('Latest 1 from a total of 100 transactions');

    const action = view.container.querySelector('[data-actions] a[href^="/csv-export?"]');
    expect(action?.textContent).toBe('Export CSV by date range');
    const href = action?.getAttribute('href');
    expect(href).toBeTruthy();
    const destination = new URL(href!, 'http://localhost');
    expect(destination.pathname).toBe('/csv-export');
    expect(Object.fromEntries(destination.searchParams)).toEqual({
      address: address,
      type: 'transactions',
      filterType: 'address',
      ...(filter ? { filterValue: filter } : {}),
    });

    // Consume the actual link query in the production export page, including its date-range form.
    routerState.pathname = destination.pathname;
    routerState.query = Object.fromEntries(destination.searchParams);
    view.rerender(withRoute(React.createElement(CsvExport)));

    const from = view.container.querySelector<HTMLInputElement>('input[name="from"]');
    const to = view.container.querySelector<HTMLInputElement>('input[name="to"]');
    expect(from?.type).toBe('datetime-local');
    expect(to?.type).toBe('datetime-local');
    expect(dayjs(to?.value).diff(dayjs(from?.value), 'hour')).toBe(24);
    expect(view.container.textContent).toContain('Exports are limited to the last 10K transactions.');
    if (filter) {
      expect(view.container.textContent).toContain(`with applied filter by address (${ filter })`);
    } else {
      expect(view.container.textContent).not.toContain('with applied filter');
    }
  } finally {
    view.unmount();
    client.clear();
  }
}

async function txPageCsvRetainsOutgoingFilterOnSecondPage() {
  await assertDateRangeExportFromSecondPage('from');
}

async function txPageCsvRetainsIncomingFilterOnSecondPage() {
  await assertDateRangeExportFromSecondPage('to');
}

async function txPageCsvDoesNotInventAFilterOnSecondPage() {
  await assertDateRangeExportFromSecondPage();
}

test(txPageCsvRetainsOutgoingFilterOnSecondPage.name, txPageCsvRetainsOutgoingFilterOnSecondPage);
test(txPageCsvRetainsIncomingFilterOnSecondPage.name, txPageCsvRetainsIncomingFilterOnSecondPage);
test(txPageCsvDoesNotInventAFilterOnSecondPage.name, txPageCsvDoesNotInventAFilterOnSecondPage);
