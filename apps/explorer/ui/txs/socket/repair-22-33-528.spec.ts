// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, waitFor } from '@testing-library/react';
import App from 'next/app';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import Router from 'next/dist/shared/lib/router/router';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import type { AddressFromToFilter, AddressTransactionsResponse } from 'types/api/address';
import type { Transaction } from 'types/api/transaction';

import { getResourceKey } from 'lib/api/useApiQuery';
import { SocketProvider, useSocket } from 'lib/socket/context';
import { base } from 'mocks/txs/tx';
import useAddressTxsQuery from 'ui/address/useAddressTxsQuery';
import { expect, test } from 'vitest';
import { createTestSocket } from 'vitest/utils/socketServer';

import useTxsSocketTypeAddress from './useTxsSocketTypeAddress';

const ADDRESS = base.from.hash;

function sortedTransaction(id: string, value: string, fee: string, pending = false): Transaction {
  return {
    ...base,
    hash: `0x${ id.repeat(64) }`,
    from: { ...base.from },
    to: { ...base.from },
    value,
    fee: { type: pending ? 'maximum' : 'actual', value: fee },
    status: pending ? null : 'ok',
    result: pending ? 'pending' : 'success',
    block_number: pending ? null : base.block_number,
    timestamp: pending ? null : base.timestamp,
  };
}

async function addressSortedCacheReceivesChannelUpdates() {
  const initialNextData = window.__NEXT_DATA__;
  const middlewareMatchers = Object.getOwnPropertyDescriptor(window, '__DEV_MIDDLEWARE_MATCHERS');
  window.__NEXT_DATA__ = { props: {}, page: '/address/[hash]', query: {}, buildId: 'sorted-address-cache' };
  Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', { configurable: true, value: [] });

  try {
    for (const field of [ 'value', 'fee' ] as const) {
      for (const order of [ 'asc', 'desc' ] as const) {
        for (const filter of [ undefined, 'from', 'to' ] as const) {
          const socket = await createTestSocket();
          const queryClient = new QueryClient({ defaultOptions: { queries: {
            retry: false,
            refetchOnMount: false,
            refetchOnReconnect: false,
            refetchOnWindowFocus: false,
          } } });
          const topic = `addresses:${ ADDRESS.toLowerCase() }`;
          const activeKey = getResourceKey('general:address_txs', {
            pathParams: { hash: ADDRESS },
            queryParams: { filter, sort: field, order },
          });
          const original = sortedTransaction('1', '20', '20');
          const nextPageParams = { block_number: 100, index: 1, items_count: 1 };
          queryClient.setQueryData<AddressTransactionsResponse>(activeKey, { items: [ original ], next_page_params: nextPageParams });

          const inactiveKeys = [
            getResourceKey('general:address_txs', { pathParams: { hash: ADDRESS }, queryParams: { filter } }),
            getResourceKey('general:address_txs', {
              pathParams: { hash: ADDRESS }, queryParams: { filter, sort: field, order: order === 'asc' ? 'desc' : 'asc' },
            }),
            getResourceKey('general:address_txs', {
              pathParams: { hash: ADDRESS }, queryParams: { filter, sort: field, order }, chainId: '125',
            }),
          ];
          const inactiveData: AddressTransactionsResponse = { items: [ sortedTransaction('9', '999', '999') ], next_page_params: null };
          inactiveKeys.forEach(key => queryClient.setQueryData<AddressTransactionsResponse>(key, structuredClone(inactiveData)));

          let socketContext: ReturnType<typeof useSocket>;
          let currentQuery: ReturnType<typeof useAddressTxsQuery> | undefined;
          let unmount: (() => void) | undefined;

          function AddressProbe() {
            currentQuery = useAddressTxsQuery({ addressHash: ADDRESS, enabled: true });
            useTxsSocketTypeAddress({ isLoading: currentQuery.query.isPlaceholderData });
            socketContext = useSocket();
            return React.createElement('output', {}, JSON.stringify(currentQuery.query.data?.items));
          }

          const router = new Router('/address/[hash]', {
            hash: ADDRESS, page: '1', sort: field, order, ...(filter ? { filter } : {}),
          }, `/address/${ ADDRESS }`, {
            initialProps: { pageProps: {} },
            pageLoader: new PageLoader('sorted-address-cache', ''),
            App,
            Component: AddressProbe,
            wrapApp: (WrappedApp) => (props: React.ComponentProps<typeof App>) => React.createElement(WrappedApp, props),
            subscription: async(data, CurrentApp) => {
              renderToStaticMarkup(React.createElement(CurrentApp, { ...data.props, Component: data.Component, router }));
            },
            isFallback: false,
          });

          try {
            await router._initialMatchesMiddlewarePromise;
            const mounted = render(React.createElement(QueryClientProvider, { client: queryClient },
              React.createElement(RouterContext.Provider, { value: router },
                React.createElement(SocketProvider, { url: socket.url, children: React.createElement(AddressProbe) }))));
            unmount = mounted.unmount;

            await waitFor(() => {
              expect(socketContext?.channelRegistry.current[topic]?.channel.state).toBe('joined');
              expect(currentQuery?.query.isPlaceholderData).toBe(false);
              expect(currentQuery?.query.data?.items).toEqual([ original ]);
            });

            const lowValue = sortedTransaction('2', '3', '100');
            const updated = sortedTransaction('1', '50', '50');
            const pending = sortedTransaction('3', '100', '3', true);
            const rejected = { ...sortedTransaction('4', '1', '1'), from: base.to!, to: base.to };
            expect(rejected.from.hash.toLowerCase()).not.toBe(ADDRESS.toLowerCase());

            await act(async() => {
              socket.send(topic, 'transaction', { transactions: [ lowValue, updated, ...(filter ? [ rejected ] : []) ] });
            });
            const ascendingFirst = field === 'value' ? [ lowValue, updated ] : [ updated, lowValue ];
            const firstExpected = order === 'asc' ? ascendingFirst : ascendingFirst.slice().reverse();
            await waitFor(() => {
              expect(JSON.parse(mounted.container.querySelector('output')!.textContent!)).toEqual(firstExpected);
            });

            await act(async() => {
              socket.send(topic, 'pending_transaction', { transactions: [ pending, updated, lowValue ] });
            });
            const ascending = field === 'value' ? [ lowValue, updated, pending ] : [ pending, updated, lowValue ];
            const expected = order === 'asc' ? ascending : ascending.slice().reverse();
            await waitFor(() => {
              expect(JSON.parse(mounted.container.querySelector('output')!.textContent!)).toEqual(expected);
              expect(queryClient.getQueryData(activeKey)).toEqual({ items: expected, next_page_params: nextPageParams });
            });
            for (const key of inactiveKeys) {
              expect(queryClient.getQueryData(key)).toEqual(inactiveData);
            }
            expect(currentQuery?.sort).toBe(`${ field }-${ order }`);
            expect(currentQuery?.filterValue).toBe(filter as AddressFromToFilter);
          } finally {
            unmount?.();
            queryClient.clear();
            await socket.close();
          }
        }
      }
    }
  } finally {
    window.__NEXT_DATA__ = initialNextData;
    if (middlewareMatchers) {
      Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', middlewareMatchers);
    } else {
      Reflect.deleteProperty(window, '__DEV_MIDDLEWARE_MATCHERS');
    }
  }
}

test(addressSortedCacheReceivesChannelUpdates, { timeout: 30_000 });
