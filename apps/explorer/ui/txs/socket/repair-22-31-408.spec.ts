// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, waitFor } from '@testing-library/react';
import App from 'next/app';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import Router from 'next/dist/shared/lib/router/router';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { getAddress } from 'viem';

import type { AddressFromToFilter, AddressTransactionsResponse } from 'types/api/address';
import type { AddressParam } from 'types/api/addressParams';
import type { Transaction } from 'types/api/transaction';

import { getResourceKey } from 'lib/api/useApiQuery';
import { SocketProvider, useSocket } from 'lib/socket/context';
import { expect, test } from 'vitest';
import { createTestSocket } from 'vitest/utils/socketServer';

import useTxsSocketTypeAddress from './useTxsSocketTypeAddress';

const TARGET = getAddress('0x5aeda56215b167893e80b4fe645ba6d5bab767de');
const OTHER = getAddress('0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359');

function addressParam(hash: string): AddressParam {
  return {
    hash,
    implementations: null,
    name: null,
    is_contract: false,
    is_verified: null,
    ens_domain_name: null,
    private_tags: [],
    public_tags: [],
    watchlist_names: [],
  };
}

function transaction(hash: string, from: string, to: string | null, pending: boolean): Transaction {
  return {
    hash,
    from: addressParam(from),
    to: to === null ? null : addressParam(to),
    created_contract: null,
    result: pending ? 'pending' : 'success',
    confirmations: pending ? 0 : 1,
    status: pending ? null : 'ok',
    block_number: pending ? null : 1,
    timestamp: pending ? null : '2026-01-01T00:00:00.000000Z',
    confirmation_duration: null,
    value: '1',
    fee: { type: pending ? 'maximum' : 'actual', value: '21000' },
    gas_price: '1',
    type: 2,
    gas_used: pending ? null : '21000',
    gas_limit: '21000',
    max_fee_per_gas: '1',
    max_priority_fee_per_gas: '0',
    priority_fee: pending ? null : '0',
    base_fee_per_gas: pending ? null : '1',
    transaction_burnt_fee: pending ? null : '21000',
    nonce: 0,
    position: pending ? null : 0,
    revert_reason: null,
    raw_input: '0x',
    decoded_input: null,
    token_transfers: [],
    token_transfers_overflow: false,
    exchange_rate: null,
    historic_exchange_rate: null,
    method: null,
    transaction_types: [ 'coin_transfer' ],
    transaction_tag: null,
    actions: [],
    has_error_in_internal_transactions: false,
  };
}

async function addressCaseUpdatesReachFilteredCache() {
  expect(TARGET).not.toBe(TARGET.toLowerCase());
  const initialNextData = window.__NEXT_DATA__;
  const middlewareMatchers = Object.getOwnPropertyDescriptor(window, '__DEV_MIDDLEWARE_MATCHERS');
  window.__NEXT_DATA__ = { props: {}, page: '/address/[hash]', query: {}, buildId: 'address-case' };
  Object.defineProperty(window, '__DEV_MIDDLEWARE_MATCHERS', { configurable: true, value: [] });

  try {
    for (const urlAddress of [ TARGET.toLowerCase(), TARGET ]) {
      for (const payloadAddress of [ TARGET, TARGET.toLowerCase() ]) {
        for (const event of [ 'transaction', 'pending_transaction' ] as const) {
          const socket = await createTestSocket();
          const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
          const topic = `addresses:${ TARGET.toLowerCase() }`;
          const filters: Array<AddressFromToFilter> = [ undefined, 'from', 'to' ];
          const cacheKey = (filter: AddressFromToFilter) => getResourceKey('general:address_txs', {
            pathParams: { hash: urlAddress },
            queryParams: filter ? { filter } : undefined,
          });
          const routers: Array<Router> = filters.map((filter) => new Router('/address/[hash]', {
            hash: urlAddress,
            ...(filter ? { filter } : {}),
          }, `/address/${ urlAddress }`, {
            initialProps: { pageProps: {} },
            pageLoader: new PageLoader('address-case', ''),
            App,
            Component: AddressProbe,
            wrapApp: (WrappedApp) => (props: React.ComponentProps<typeof App>) => React.createElement(WrappedApp, props),
            subscription: async(data, CurrentApp) => {
              renderToStaticMarkup(React.createElement(CurrentApp, { ...data.props, Component: data.Component, router: routers[0] }));
            },
            isFallback: false,
          }));
          let unmount: (() => void) | undefined;
          let socketContext: ReturnType<typeof useSocket>;

          function AddressProbe() {
            useTxsSocketTypeAddress({});
            socketContext = useSocket();
            return null;
          }

          try {
            await Promise.all(routers.map((router) => router._initialMatchesMiddlewarePromise));
            for (const filter of filters) {
              queryClient.setQueryData<AddressTransactionsResponse>(cacheKey(filter), { items: [], next_page_params: null });
            }
            const mounted = render(React.createElement(QueryClientProvider, { client: queryClient },
              React.createElement(SocketProvider, {
                url: socket.url,
                children: routers.map((router, index) => React.createElement(RouterContext.Provider, { value: router, key: index },
                  React.createElement(AddressProbe))),
              })));
            unmount = mounted.unmount;

            await waitFor(() => {
              const entry = socketContext?.channelRegistry.current[topic];
              expect(entry?.channel.state).toBe('joined');
              expect(entry?.subscribers).toBe(3);
            });

            const pending = event === 'pending_transaction';
            const outgoing = transaction(`0x${ '1'.repeat(64) }`, payloadAddress, OTHER, pending);
            const incoming = transaction(`0x${ '2'.repeat(64) }`, OTHER, payloadAddress, pending);
            const selfTransfer = transaction(`0x${ '3'.repeat(64) }`, payloadAddress, payloadAddress, pending);
            const creation = transaction(`0x${ '4'.repeat(64) }`, payloadAddress, null, pending);
            const expected = [
              [ outgoing, incoming, selfTransfer, creation ],
              [ outgoing, selfTransfer, creation ],
              [ incoming, selfTransfer ],
            ];

            await act(async() => {
              socket.send(topic, event, { transactions: [ outgoing, incoming, selfTransfer, creation ] });
            });
            await waitFor(() => {
              for (const [ index, filter ] of filters.entries()) {
                const items = queryClient.getQueryData<AddressTransactionsResponse>(cacheKey(filter))?.items;
                expect(items?.map((item) => item.hash).sort()).toEqual(expected[index].map((item) => item.hash).sort());
              }
            });

            await act(async() => {
              socket.send(topic, event, { transactions: [ outgoing, incoming, selfTransfer, creation ].map((item) => ({ ...item, value: '2' })) });
            });
            await waitFor(() => {
              for (const [ index, filter ] of filters.entries()) {
                const items = queryClient.getQueryData<AddressTransactionsResponse>(cacheKey(filter))?.items;
                expect(items).toHaveLength(expected[index].length);
                expect(items?.every((item) => item.value === '2')).toBe(true);
              }
            });
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

test(addressCaseUpdatesReachFilteredCache, { timeout: 30_000 });
