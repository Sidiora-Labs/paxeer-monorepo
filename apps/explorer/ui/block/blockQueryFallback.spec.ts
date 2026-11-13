// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import type { AppProps } from 'next/dist/shared/lib/router/router';
import { Router } from 'next/router';
import { once } from 'node:events';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';
import { boolean, number, object, parse, string, tuple } from 'valibot';
import { numberToHex } from 'viem';

import type { ClusterChainConfig } from 'types/multichain';

import config from 'configs/app';
import { MultichainContext } from 'lib/contexts/multichain';
import * as blockFixture from 'mocks/blocks/block';
import { GET_BLOCK, GET_TRANSACTION } from 'stubs/RPC';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';

import useBlockQuery from './useBlockQuery';
import useBlockTxsQuery from './useBlockTxsQuery';

const heightOrHash = String(GET_BLOCK.number);
const blockHashes = [ `0x${ 'a1'.repeat(32) }`, `0x${ 'b2'.repeat(32) }` ] as const;
const txHashes = [ `0x${ 'c3'.repeat(32) }`, `0x${ 'd4'.repeat(32) }` ] as const;

// The committed RPC fixtures are replayed over HTTP, including their wire quantities,
// so viem's real transport and decoders feed the production hooks and selectors.
function blockTranscript(chainIndex: number, latest: boolean, includeTransactions: boolean) {
  const hash = blockHashes[chainIndex];
  const block = {
    ...GET_BLOCK,
    hash,
    number: latest ? GET_BLOCK.number + BigInt(chainIndex + 1) : GET_BLOCK.number,
    transactions: includeTransactions ? [ {
      ...GET_TRANSACTION,
      hash: txHashes[chainIndex],
      blockHash: hash,
      blockNumber: GET_BLOCK.number,
      value: BigInt(chainIndex + 101),
      type: GET_TRANSACTION.typeHex,
    } ] : [ txHashes[chainIndex] ],
  };

  return JSON.parse(JSON.stringify(block, (_key, value) =>
    typeof value === 'bigint' || typeof value === 'number' ? numberToHex(value) : value));
}

async function openReplay(blockApiAvailable = false) {
  const generalApi = config.apis.general;
  if (!generalApi) {
    throw new Error('The committed test configuration must configure the general API');
  }
  const requests: Array<{ chainIndex: number; method: string; params: [ string, boolean ] }> = [];
  const apiRequests: Array<string> = [];
  const server = createServer(async(request, response) => {
    response.setHeader('Content-Type', 'application/json');
    const path = request.url || '';
    const chainIndex = path.startsWith('/b/') ? 1 : 0;
    if (!path.endsWith('/rpc')) {
      apiRequests.push(path);
      if (blockApiAvailable && path.endsWith(`/blocks/${ heightOrHash }`)) {
        response.end(JSON.stringify({ ...blockFixture.base, hash: blockHashes[chainIndex], height: Number(GET_BLOCK.number) }));
        return;
      }
      response.writeHead(501);
      response.end(JSON.stringify({ message: 'Indexer unavailable' }));
      return;
    }

    const chunks = [];
    for await (const chunk of request) {
      chunks.push(chunk);
    }
    const rpc = parse(object({ id: number(), method: string(), params: tuple([ string(), boolean() ]) }),
      JSON.parse(Buffer.concat(chunks).toString()));
    requests.push({ chainIndex, method: rpc.method, params: rpc.params });
    response.end(JSON.stringify({
      jsonrpc: '2.0',
      id: rpc.id,
      result: blockTranscript(chainIndex, rpc.params[0] === 'latest', rpc.params[1]),
    }));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const endpoint = `http://127.0.0.1:${ (server.address() as AddressInfo).port }`;
  const chains: Array<ClusterChainConfig> = [ 'a', 'b' ].map((slug, index) => ({
    id: String(125 + index),
    name: `Chain ${ slug }`,
    slug,
    app_config: {
      ...config,
      chain: { ...config.chain, id: String(125 + index), rpcUrls: [ `${ endpoint }/${ slug }/rpc` ] },
      apis: {
        ...config.apis,
        general: { ...generalApi, endpoint, basePath: `/${ slug }/api` },
      },
    },
  }));

  return {
    chains,
    requests,
    apiRequests,
    close: () => new Promise<void>((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
      server.closeAllConnections();
    }),
  };
}

function createHarness(initialChain: ClusterChainConfig) {
  let chain = initialChain;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  window.__NEXT_DATA__ = { buildId: 'repair', props: {}, page: '/block/[height_or_hash]', query: {} };
  window.__DEV_MIDDLEWARE_MATCHERS = [];
  window.__MIDDLEWARE_MATCHERS = [];
  window.__SSG_MANIFEST = new Set();
  const router = new Router('/block/[height_or_hash]', { height_or_hash: heightOrHash, chain_id: chain.id }, `/block/${ heightOrHash }`, {
    initialProps: {},
    pageLoader: new PageLoader('repair', ''),
    App: ReplayApp,
    Component: ReplayPage,
    wrapApp: App => App,
    subscription: async() => {},
    isFallback: false,
  });
  const wrapper = ({ children }: { children: React.ReactNode }) => React.createElement(
    RouterContext.Provider,
    { value: router },
    React.createElement(QueryClientProvider, { client },
      React.createElement(MultichainContext.Provider, { value: { chain } }, children)),
  );

  return {
    client,
    wrapper,
    selectChain: (nextChain: ClusterChainConfig) => {
      chain = nextChain;
    },
    dispose: () => {
      window.removeEventListener('popstate', router.onPopState);
      client.clear();
    },
  };
}

function ReplayApp({ Component }: AppProps) {
  return React.createElement(Component);
}

function ReplayPage() {
  return null;
}

beforeAll(() => {
  fetchMock.disableMocks();
});
afterEach(cleanup);
afterAll(() => {
  fetchMock.enableMocks();
});

async function rpcCacheDomainKeepsSelectedChainDuringApiFailure() {
  const replay = await openReplay();
  const harness = createHarness(replay.chains[0]);
  const hook = renderHook(() => useBlockQuery({ heightOrHash }), { wrapper: harness.wrapper });

  try {
    await waitFor(() => {
      expect(hook.result.current.isDegradedData).toBe(true);
      expect(hook.result.current.data?.hash).toBe(blockHashes[0]);
    });

    act(() => {
      harness.selectChain(replay.chains[1]);
      hook.rerender();
    });
    expect(hook.result.current.data?.hash).not.toBe(blockHashes[0]);
    await waitFor(() => {
      expect(hook.result.current.isDegradedData).toBe(true);
      expect(hook.result.current.data?.hash).toBe(blockHashes[1]);
      expect(hook.result.current.isFutureBlock).toBe(false);
    });

    expect(replay.apiRequests).toEqual(expect.arrayContaining([
      `/a/api/api/v2/blocks/${ heightOrHash }`, `/b/api/api/v2/blocks/${ heightOrHash }`,
    ]));
    for (const chainIndex of [ 0, 1 ]) {
      expect(replay.requests).toContainEqual({ chainIndex, method: 'eth_getBlockByNumber', params: [ 'latest', false ] });
      expect(replay.requests).toContainEqual({
        chainIndex, method: 'eth_getBlockByNumber', params: [ numberToHex(GET_BLOCK.number), false ],
      });
      expect(harness.client.getQueryData([ 'RPC', 'block', replay.chains[chainIndex].id, 'latest' ]))
        .toMatchObject({ number: GET_BLOCK.number + BigInt(chainIndex + 1), hash: blockHashes[chainIndex] });
    }

    act(() => {
      harness.selectChain(replay.chains[0]);
      hook.rerender();
    });
    await waitFor(() => expect(hook.result.current.data?.hash).toBe(blockHashes[0]));
    expect(replay.requests).toHaveLength(4);
  } finally {
    hook.unmount();
    harness.dispose();
    await replay.close();
  }
}

async function rpcTxsDomainKeepsSelectedChainDuringApiFailure() {
  const replay = await openReplay(true);
  const harness = createHarness(replay.chains[0]);
  const hook = renderHook(() => {
    const block = useBlockQuery({ heightOrHash });
    const transactions = useBlockTxsQuery({ heightOrHash, blockQuery: block, tab: 'txs' });
    return { block, transactions };
  }, { wrapper: harness.wrapper });

  try {
    await waitFor(() => {
      expect(hook.result.current.block.isDegradedData).toBe(false);
      expect(hook.result.current.transactions.isDegradedData).toBe(true);
      expect(hook.result.current.transactions.data?.items[0].hash).toBe(txHashes[0]);
    });
    act(() => {
      harness.selectChain(replay.chains[1]);
      hook.rerender();
    });
    expect(hook.result.current.transactions.data?.items[0].hash).not.toBe(txHashes[0]);
    await waitFor(() => {
      expect(hook.result.current.block.data?.hash).toBe(blockHashes[1]);
      expect(hook.result.current.block.isDegradedData).toBe(false);
      expect(hook.result.current.transactions.isDegradedData).toBe(true);
      expect(hook.result.current.transactions.data?.items).toHaveLength(1);
      expect(hook.result.current.transactions.data?.items[0]).toMatchObject({
        hash: txHashes[1], block_number: Number(GET_BLOCK.number), value: '102', gas_limit: GET_TRANSACTION.gas.toString(),
      });
    });
    for (const chainIndex of [ 0, 1 ]) {
      expect(replay.requests).toContainEqual({
        chainIndex, method: 'eth_getBlockByNumber', params: [ numberToHex(GET_BLOCK.number), true ],
      });
    }
    expect(replay.apiRequests).toEqual(expect.arrayContaining([
      `/a/api/api/v2/blocks/${ heightOrHash }/transactions`, `/b/api/api/v2/blocks/${ heightOrHash }/transactions`,
    ]));
    act(() => {
      harness.selectChain(replay.chains[0]);
      hook.rerender();
    });
    await waitFor(() => expect(hook.result.current.transactions.data?.items[0].hash).toBe(txHashes[0]));
    expect(replay.requests.filter(({ params }) => params[1])).toHaveLength(2);
  } finally {
    hook.unmount();
    harness.dispose();
    await replay.close();
  }
}

test('rpcCacheDomainKeepsSelectedChainDuringApiFailure', rpcCacheDomainKeepsSelectedChainDuringApiFailure);
test('rpcTxsDomainKeepsSelectedChainDuringApiFailure', rpcTxsDomainKeepsSelectedChainDuringApiFailure);
