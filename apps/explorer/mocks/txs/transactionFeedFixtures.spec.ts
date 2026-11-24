// @vitest-environment jsdom

import { RouterContext } from 'next/dist/shared/lib/router-context.shared-runtime';
import { createServer } from 'node:http';
import React from 'react';
import * as v from 'valibot';
import { createPublicClient, http } from 'viem';
import { formatTransaction } from 'viem/utils';

import type { FeedMarketsResponse, PriceUpdateItem } from 'types/api/paxeerXFeed';
import type { Transaction } from 'types/api/transaction';

import formatTxData from 'lib/web3/rpc/formatTxData';
import { rpcBlockWithTxsInfo } from 'mocks/blocks/block';
import { markets } from 'mocks/paxeerX/feed';
import { base as formattedEvmFixture, priceUpdate, priceUpdateItem, rpcRevertedTxReceipt, rpcTxReceipt } from 'mocks/txs/tx';
import { TableBody, TableRoot } from 'toolkit/chakra/table';
import applyPriceUpdates from 'ui/paxeerX/feed/applyPriceUpdates';
import { nextRouterModule, render } from 'ui/shared/layout/testWrapper';
import type { TransactionFeedItem } from 'ui/txs/TxPriceUpdate';
import { isPriceUpdate } from 'ui/txs/TxPriceUpdate';
import TxsContent from 'ui/txs/TxsContent';
import TxsListItem from 'ui/txs/TxsListItem';
import TxsTableItem from 'ui/txs/TxsTableItem';
import { expect, expectTypeOf, test } from 'vitest';

async function rpcReceiptStatusDecodesForHomeFallback() {
  const requests: Array<unknown> = [];
  const receipts = [ rpcTxReceipt, rpcRevertedTxReceipt ];
  const server = createServer((request, response) => {
    let body = '';
    request.setEncoding('utf8');
    request.on('data', (chunk: string) => {
      body += chunk;
    });
    request.on('end', () => {
      const message = v.safeParse(v.object({
        jsonrpc: v.literal('2.0'),
        id: v.number(),
        method: v.literal('eth_getTransactionReceipt'),
        params: v.tuple([ v.union(receipts.map((receipt) => v.literal(receipt.transactionHash))) ]),
      }), JSON.parse(body));
      if (!message.success) {
        response.writeHead(400);
        response.end();
        return;
      }
      const receipt = receipts.find((receipt) => receipt.transactionHash === message.output.params[0]);
      requests.push(message.output);
      response.writeHead(200, { 'Content-Type': 'application/json' });
      response.end(JSON.stringify({ jsonrpc: '2.0', id: message.output.id, result: receipt }));
    });
  });

  fetchMock.disableMocks();
  try {
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    const address = server.address();
    if (!address || typeof address === 'string') {
      throw new Error('The receipt server must listen on a loopback TCP port');
    }
    const client = createPublicClient({ transport: http(`http://127.0.0.1:${ address.port }`, { retryCount: 0 }) });
    const receipt = await client.getTransactionReceipt({ hash: rpcTxReceipt.transactionHash });

    expect(requests).toEqual([ {
      jsonrpc: '2.0',
      id: expect.any(Number),
      method: 'eth_getTransactionReceipt',
      params: [ rpcTxReceipt.transactionHash ],
    } ]);
    expect(rpcTxReceipt.status).toBe('0x1');
    expect(receipt.status).toBe('success');
    expect(receipt.blockNumber).toBe(BigInt(rpcTxReceipt.blockNumber));
    expect(receipt.gasUsed).toBe(BigInt(rpcTxReceipt.gasUsed));

    const transaction = rpcBlockWithTxsInfo.transactions[0];
    if (typeof transaction === 'string') {
      throw new Error('The Home fallback fixture must include full transactions');
    }
    const decodedTransaction = formatTransaction(transaction);
    if (decodedTransaction.blockHash === null || decodedTransaction.blockNumber === null || decodedTransaction.transactionIndex === null) {
      throw new Error('The Home fallback fixture must contain a mined transaction');
    }
    const formatted = formatTxData({
      ...decodedTransaction,
      blockHash: decodedTransaction.blockHash,
      blockNumber: decodedTransaction.blockNumber,
      transactionIndex: decodedTransaction.transactionIndex,
    }, receipt, BigInt(1), null);
    expect(formatted?.status).toBe('ok');
    expect(formatted?.gas_used).toBe(receipt.gasUsed.toString());
    expect(formatted?.fee.value).toBe((receipt.gasUsed * receipt.effectiveGasPrice).toString());

    const revertedReceipt = await client.getTransactionReceipt({ hash: rpcRevertedTxReceipt.transactionHash });
    expect(rpcRevertedTxReceipt.status).toBe('0x0');
    expect(revertedReceipt.status).toBe('reverted');
    expect(revertedReceipt.transactionHash).toBe(rpcRevertedTxReceipt.transactionHash);
    const reverted = formatTxData({
      ...decodedTransaction,
      hash: rpcRevertedTxReceipt.transactionHash,
      blockHash: decodedTransaction.blockHash,
      blockNumber: decodedTransaction.blockNumber,
      transactionIndex: decodedTransaction.transactionIndex,
    }, revertedReceipt, BigInt(1), null);
    expect(reverted?.status).toBe('error');
    expect(reverted?.gas_used).toBe(revertedReceipt.gasUsed.toString());
    expect(reverted?.fee.value).toBe((revertedReceipt.gasUsed * revertedReceipt.effectiveGasPrice).toString());
    expect(requests).toHaveLength(2);
  } finally {
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    fetchMock.enableMocks();
  }
}

function priceFixtureKeepsItsNativeTypeThroughThePriceConsumer() {
  expectTypeOf(priceUpdate).toEqualTypeOf<PriceUpdateItem>();
  expectTypeOf(priceUpdate).not.toMatchTypeOf<Transaction>();
  expect(priceUpdate).toBe(priceUpdateItem);
  expect(priceUpdate.type).toBe(127);
  expect(priceUpdate.transaction_types).toEqual([ 'price_update' ]);
  for (const field of [ 'raw_input', 'nonce', 'created_contract', 'gas_price', 'exchange_rate' ]) {
    expect(priceUpdate).not.toHaveProperty(field);
  }

  const initial: FeedMarketsResponse = {
    ...markets,
    chain_head: priceUpdate.block_number - 1,
    items: markets.items.map((market) => ({ ...market, latest: null })),
  };
  const updated = applyPriceUpdates(initial, [ priceUpdate ]);
  const observation = priceUpdate.price_update.observations.find((item) => item.market_id === 0);
  expect(observation).toBeDefined();
  expect(updated.items[0].latest).toEqual({
    index: priceUpdate.index,
    index_price: observation?.index_price,
    mark_price: observation?.mark_price,
    source_timestamp_ms: observation?.source_timestamp_ms,
    sequence: observation?.sequence,
    update_hash: priceUpdate.hash,
    block_number: priceUpdate.block_number,
    timestamp: priceUpdate.timestamp,
  });
  expect(updated.chain_head).toBe(priceUpdate.block_number);
  expect(updated.items[1]).toBe(initial.items[1]);
  expect(applyPriceUpdates(updated, [ priceUpdate ])).toBe(updated);
}

function nativePriceFixtureRendersBesideEvmTransactions() {
  const items: Array<TransactionFeedItem> = [ priceUpdate ];
  const router = nextRouterModule().useRouter();
  const renderConsumer = (element: React.ReactElement) => render(
    React.createElement(RouterContext.Provider, { value: router }, element),
  );

  expect(isPriceUpdate(items[0])).toBe(true);
  const list = renderConsumer(React.createElement(TxsListItem, { tx: items[0], showBlockInfo: true }));
  expect(list.container.querySelector('a[href^="/feed/update/"]')?.getAttribute('href')).toBe(`/feed/update/${ priceUpdate.hash }`);
  expect(list.container.querySelector('[data-scan-method]')?.getAttribute('data-scan-method')).toBe('Price update · 2 markets');
  expect(list.container.querySelector('[data-price-update-to] a[href="/feed"]')?.textContent).toBe('Price feed');
  expect(list.container.querySelector('[data-scan-preview]')).toBeNull();
  expect(list.container.textContent).toContain('Value-');
  expect(list.container.textContent).toContain('Fee-');
  list.unmount();

  const table = renderConsumer(React.createElement(TableRoot, { variant: 'scan' },
    React.createElement(TableBody, null, React.createElement(TxsTableItem, { tx: items[0], showBlockInfo: true })),
  ));
  const cells = table.container.querySelectorAll('td');
  expect(cells).toHaveLength(8);
  expect(cells[0].querySelector('[data-scan-preview]')).toBeNull();
  expect(cells[1].querySelector('a')?.getAttribute('href')).toBe(`/feed/update/${ priceUpdate.hash }`);
  expect(cells[6].textContent).toBe('-');
  expect(cells[7].textContent).toBe('-');
  table.unmount();

  const content = renderConsumer(React.createElement(TxsContent, {
    items: [ ...items, formattedEvmFixture ],
    pagination: {
      page: 1,
      onNextPageClick: () => undefined,
      onPrevPageClick: () => undefined,
      resetPage: () => undefined,
      hasPages: false,
      hasNextPage: false,
      canGoBackwards: false,
      isLoading: false,
      isVisible: false,
    },
    sort: 'default',
    isPlaceholderData: false,
    isError: false,
    stickyHeader: false,
    isInsideTableCard: true,
  }));
  const rows = content.container.querySelectorAll('tbody tr');
  expect(rows).toHaveLength(2);
  expect(rows[0].querySelector('a[href^="/feed/update/"]')?.getAttribute('href')).toBe(`/feed/update/${ priceUpdate.hash }`);
  expect(rows[1].querySelector('a[href^="/tx/"]')?.getAttribute('href')).toBe(`/tx/${ formattedEvmFixture.hash }`);
  expect(isPriceUpdate(formattedEvmFixture)).toBe(false);
  content.unmount();
}

test(rpcReceiptStatusDecodesForHomeFallback);
test(priceFixtureKeepsItsNativeTypeThroughThePriceConsumer);
test(nativePriceFixtureRendersBesideEvmTransactions);
