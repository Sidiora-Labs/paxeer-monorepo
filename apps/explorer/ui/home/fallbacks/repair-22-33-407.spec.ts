// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, screen, waitFor } from '@testing-library/react';
import PageLoader from 'next/dist/client/page-loader';
import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import Router from 'next/dist/shared/lib/router/router';
import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { join } from 'node:path';
import { gunzipSync } from 'node:zlib';
import React from 'react';
import type { RpcBlock, RpcTransactionReceipt } from 'viem';

import { afterAll, afterEach, beforeEach, expect, test } from 'vitest';

interface Transcript {
  blocks: Array<RpcBlock<'latest', true>>;
  emptyBlocks: Array<RpcBlock<'latest', true>>;
  revertedBlock: RpcBlock<'latest', true>;
  receipts: Array<RpcTransactionReceipt>;
}

interface RpcRequest {
  id: number;
  method: string;
  params: Array<string | boolean>;
}

const transcript = JSON.parse(gunzipSync(new Uint8Array(readFileSync(join(__dirname, 'repair-22-33-407.rpc.json.gz')))).toString()) as Transcript;
const receipts = new Map(transcript.receipts.map((receipt) => [ receipt.transactionHash, receipt ]));
let currentBlock = transcript.blocks[0];
let blockRequests = 0;
const receiptRequests = new Set<string>();
const unexpectedRequests: Array<string> = [];
let holdReceipts = false;
let pendingReceipts: Array<() => void> = [];

function releaseReceipts() {
  holdReceipts = false;
  pendingReceipts.forEach((release) => release());
  pendingReceipts = [];
}

const server = createServer(async(request, response) => {
  request.setEncoding('utf8');
  let body = '';
  for await (const chunk of request) {
    body += chunk;
  }
  const rpc = JSON.parse(body) as RpcRequest;
  let result: RpcBlock<'latest', true> | RpcTransactionReceipt | undefined;
  if (rpc.method === 'eth_getBlockByNumber' && rpc.params[0] === 'latest' && rpc.params[1] === true) {
    blockRequests += 1;
    result = currentBlock;
  } else if (rpc.method === 'eth_getTransactionReceipt') {
    receiptRequests.add(String(rpc.params[0]));
    result = receipts.get(String(rpc.params[0]) as `0x${ string }`);
    if (holdReceipts) {
      await new Promise<void>((resolve) => pendingReceipts.push(resolve));
    }
  }
  response.setHeader('content-type', 'application/json');
  if (!result) {
    unexpectedRequests.push(`${ rpc.method }:${ JSON.stringify(rpc.params) }`);
    response.end(JSON.stringify({ jsonrpc: '2.0', id: rpc.id, error: { code: -32602, message: 'Request absent from recording' } }));
    return;
  }
  response.end(JSON.stringify({ jsonrpc: '2.0', id: rpc.id, result }));
});

fetchMock.disableMocks();
await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
const address = server.address();
if (!address || typeof address === 'string') {
  throw new Error('The RPC recording server did not bind a TCP port');
}
window.__envs = {
  ...window.__envs,
  NEXT_PUBLIC_NETWORK_RPC_URL: `http://127.0.0.1:${ address.port }`,
  NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
  NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
};
window.__NEXT_DATA__ = { buildId: 'repair-22-33-407', page: '/', query: {}, props: {} };
window.__DEV_MIDDLEWARE_MATCHERS = [];
const renderPage: ConstructorParameters<typeof Router>[3]['App'] = ({ Component }) => React.createElement(Component);
const router = new Router('/', {}, '/', {
  initialProps: {},
  pageLoader: new PageLoader('repair-22-33-407', ''),
  App: renderPage,
  Component: Probe,
  wrapApp: (App) => App,
  subscription: async() => {
    throw new Error('These feed tests must not navigate');
  },
  isFallback: false,
});

const { render } = await import('ui/shared/layout/testWrapper');
const { HomeRpcDataContextProvider, useHomeRpcDataContext } = await import('./rpcDataContext');
const { 'default': LatestBlocksDegraded } = await import('./LatestBlocksDegraded');
const { 'default': LatestTxsDegraded } = await import('./LatestTxsDegraded');
type Snapshot = Omit<ReturnType<typeof useHomeRpcDataContext>, 'enable'>;
let queryClient: QueryClient;

function Probe() {
  const context = useHomeRpcDataContext();
  return React.createElement(React.Fragment, null,
    React.createElement('output', { 'data-testid': 'rpc-state' }, JSON.stringify(context)),
    React.createElement('button', {
      onClick: () => {
        context.enable(true, 'latest-txs');
        context.enable(true, 'latest-txs');
      },
    }, 'Repeat transaction subscription'),
  );
}

function tree(blocks: boolean, txs: boolean) {
  return React.createElement(QueryClientProvider, { client: queryClient },
    React.createElement(RouterContext.Provider, { value: router },
      React.createElement(HomeRpcDataContextProvider, null,
        blocks && React.createElement(LatestBlocksDegraded, { maxNum: 5 }),
        txs && React.createElement(LatestTxsDegraded, { maxNum: 5 }),
        React.createElement(Probe),
      ),
    ),
  );
}

function state(): Snapshot {
  return JSON.parse(screen.getByTestId('rpc-state').textContent || 'null') as Snapshot;
}

async function observeBlock(block: Transcript['blocks'][number]) {
  currentBlock = block;
  await waitFor(() => {
    expect(state().blocks[0]?.hash).toBe(block.hash);
    expect(state().isLoading).toBe(false);
    expect(state().isError).toBe(false);
  }, { timeout: 7_000 });
}

beforeEach(() => {
  currentBlock = transcript.blocks[0];
  blockRequests = 0;
  receiptRequests.clear();
  unexpectedRequests.length = 0;
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

afterEach(() => {
  cleanup();
  releaseReceipts();
  queryClient.clear();
  expect(unexpectedRequests).toEqual([]);
});

afterAll(async() => {
  window.removeEventListener('popstate', router.onPopState);
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  fetchMock.enableMocks();
});

async function rpcFeedStopsUpdating() {
  const view = render(tree(false, true));
  let observed = 0;
  for (const block of transcript.blocks) {
    await observeBlock(block);
    observed += block.transactions.length;
    expect(state().totalTxs).toBe(observed);
    const hashes = block.transactions.slice(0, 5).map((tx) => tx.hash);
    expect(state().txs.map((tx) => tx.hash)).toEqual(hashes);
    expect(Array.from(view.container.querySelectorAll('[data-latest-tx]'), (row) => row.getAttribute('data-latest-tx')))
      .toEqual([ ...hashes, ...hashes ]);
  }
  expect(observed).toBeGreaterThan(5);

  view.rerender(tree(false, false));
  expect(state().isEnabled).toBe(false);
  view.rerender(tree(false, true));
  await waitFor(() => expect(state().isLoading).toBe(false), { timeout: 7_000 });
  expect(state().totalTxs).toBe(observed);
  expect(new Set(state().txs.map((tx) => tx.hash)).size).toBe(5);
}

async function rpcEmptyBlockLoading() {
  currentBlock = transcript.emptyBlocks[0];
  const view = render(tree(true, true));
  for (const block of transcript.emptyBlocks) {
    await observeBlock(block);
    expect(state().totalTxs).toBe(0);
    expect(state().txs).toEqual([]);
    const row = view.container.querySelector(`[data-latest-block="${ Number(BigInt(block.number)) }"]`);
    expect(row).not.toBeNull();
    expect(row?.querySelector('[data-loading]')).toBeNull();
    expect(row?.textContent).toContain('0 txns');
    expect(screen.getByText('No latest transactions found.')).toBeTruthy();
  }
  expect(receiptRequests.size).toBe(0);
  expect(state().blocks.map((block) => block.hash)).toEqual(transcript.emptyBlocks.map((block) => block.hash).reverse());
}

async function rpcRevertedStatus() {
  currentBlock = transcript.revertedBlock;
  holdReceipts = true;
  const view = render(tree(true, true));
  await observeBlock(transcript.revertedBlock);
  expect(state().txs.every((tx) => tx.status === null)).toBe(true);
  expect(view.container.querySelector('[data-latest-block] [data-loading]')).toBeNull();
  await waitFor(() => expect(receiptRequests.size).toBe(5));
  releaseReceipts();

  const failedReceipt = transcript.receipts.find((receipt) => receipt.blockHash === currentBlock.hash && receipt.status === '0x0');
  expect(failedReceipt).toBeDefined();
  await waitFor(() => {
    expect(state().txs.find((tx) => tx.hash === failedReceipt?.transactionHash)?.status).toBe('error');
    const rows = view.container.querySelectorAll(`[data-latest-tx="${ failedReceipt?.transactionHash }"]`);
    expect(rows).toHaveLength(2);
    rows.forEach((row) => expect(row.textContent).toContain('Failed'));
  });
  expect(state().txs.filter((tx) => tx.hash !== failedReceipt?.transactionHash).every((tx) => tx.status === 'ok')).toBe(true);
}

async function rpcSubscriberLifecycle() {
  const view = render(tree(true, true));
  await observeBlock(transcript.blocks[0]);
  expect(state().subscriptions).toEqual([ 'latest-blocks', 'latest-txs' ]);
  expect(blockRequests).toBe(1);
  fireEvent.click(screen.getByText('Repeat transaction subscription'));
  expect(state().subscriptions).toEqual([ 'latest-blocks', 'latest-txs' ]);
  expect(blockRequests).toBe(1);

  view.rerender(tree(false, true));
  expect(state().subscriptions).toEqual([ 'latest-txs' ]);
  expect(state().isEnabled).toBe(true);
  expect(state().isLoading).toBe(false);
  await observeBlock(transcript.blocks[1]);
  expect(blockRequests).toBe(2);

  view.rerender(tree(false, false));
  expect(state().subscriptions).toEqual([]);
  expect(state().isEnabled).toBe(false);
  currentBlock = transcript.blocks[2];
  const stoppedRequests = blockRequests;
  await new Promise((resolve) => setTimeout(resolve, 5_200));
  expect(blockRequests).toBe(stoppedRequests);
  expect(state().blocks[0].hash).toBe(transcript.blocks[1].hash);

  view.rerender(tree(true, true));
  await observeBlock(transcript.blocks[2]);
  expect(state().isEnabled).toBe(true);
  expect(blockRequests).toBe(stoppedRequests + 1);
  expect(state().totalTxs).toBe(transcript.blocks.reduce((total, block) => total + block.transactions.length, 0));

  view.unmount();
  currentBlock = transcript.emptyBlocks[0];
  const reopened = render(tree(true, false));
  await observeBlock(transcript.emptyBlocks[0]);
  expect(state().totalTxs).toBe(0);
  expect(reopened.container.querySelector(`[data-latest-block="${ Number(BigInt(currentBlock.number)) }"]`)).not.toBeNull();
  expect(blockRequests).toBe(stoppedRequests + 2);
}

test(rpcFeedStopsUpdating, { timeout: 30_000 });
test(rpcEmptyBlockLoading, { timeout: 15_000 });
test(rpcRevertedStatus, { timeout: 15_000 });
test(rpcSubscriberLifecycle, { timeout: 30_000 });
