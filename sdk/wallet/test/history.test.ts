import { readFileSync } from 'node:fs';
import { expect, test } from 'vitest';
import { EndpointClient, decodeChainHistoryPage, decodeHistoryPage } from '../src/endpoint.js';
import { JsonRpcTransportError } from '../src/rpc.js';
import type { HistoryPage } from '../src/types.js';

type Exchange = {
  request: { method: string; params: [string, string | null, number, null] };
  response: { result: HistoryPage };
};

async function chain_history_keeps_paxeer_rows_and_drops_the_side_label(): Promise<void> {
  const exchanges = JSON.parse(
    readFileSync(new URL('./fixtures/endpoint/available.json', import.meta.url), 'utf8'),
  ) as Exchange[];
  const history = exchanges.filter((exchange) => exchange.request.method === 'px_getUnifiedHistory');
  expect(history).toHaveLength(3);

  let cursor: string | null = null;
  for (const [index, exchange] of history.entries()) {
    const [address, requestedCursor, limit] = exchange.request.params;
    expect(requestedCursor).toBe(cursor);
    const url = `data:application/json,${encodeURIComponent(JSON.stringify({
      jsonrpc: '2.0',
      id: 1,
      ...exchange.response,
    }))}`;
    const client = new EndpointClient({ url });
    const page = await client.getChainHistory(address, cursor, { limit });
    const id = ['5', '3', '1'][index];
    expect(page).toEqual({
      account: address,
      items: [{
        id,
        height_or_seq: `100${id}`,
        kind: 'erc20_transfer',
        direction: 'in',
        account: address,
        counterparty: `0x${'55'.repeat(20)}`,
        asset: `evm:0x${'33'.repeat(20)}`,
        amount: `${id}000`,
        tx_id: `0x${`0${id}`.repeat(32)}`,
        ordinal: '0',
        final: index !== 0,
        decoded: { method: 'transfer' },
        asset_metadata: null,
      }],
      next_cursor: ['4', '2', null][index],
    });
    expect(page).not.toHaveProperty('accounts');
    for (const item of page.items) {
      expect(item).not.toHaveProperty('chain');
      expect(item).not.toHaveProperty('side');
    }
    expect(decodeChainHistoryPage(exchange.response.result)).toEqual(page);
    expect(decodeHistoryPage(exchange.response.result).items).toHaveLength(index === 2 ? 1 : 2);
    cursor = page.next_cursor;
  }
  expect(cursor).toBeNull();

  const first = history[0].response.result;
  const paxeer = first.items[0];
  const layerx = first.items[1];
  const layerxOnly = { ...first, items: [layerx] };
  expect(decodeChainHistoryPage(layerxOnly)).toEqual({
    account: first.account.evm_address,
    items: [],
    next_cursor: '4',
  });
  expect(decodeChainHistoryPage({ ...first, items: [], next_cursor: null })).toEqual({
    account: first.account.evm_address,
    items: [],
    next_cursor: null,
  });
  const mismatchedSides = decodeChainHistoryPage({
    ...first,
    items: [{ ...paxeer, side: 'layerx' }, { ...layerx, side: 'paxeer' }],
  });
  expect(mismatchedSides.items.map((item) => item.id)).toEqual(['5']);
  expect(mismatchedSides.items[0]).not.toHaveProperty('chain');
  expect(mismatchedSides.items[0]).not.toHaveProperty('side');
  expect(first.items[0]).toHaveProperty('chain', 'paxeer');
  expect(first.items[0]).toHaveProperty('side', 'paxeer');

  for (const invalid of [
    { ...first, next_cursor: 'bad cursor' },
    { ...first, items: [{ ...paxeer, amount: 'invalid' }] },
    { ...first, items: [{ ...paxeer, chain: 'unknown' }] },
    { ...first, account: { ...first.account, evm_address: null } },
  ]) {
    expect(() => decodeChainHistoryPage(invalid)).toThrow(JsonRpcTransportError);
  }

  const client = new EndpointClient();
  for (const limit of [0, 101, 1.5, NaN, Infinity]) {
    await expect(client.getChainHistory(history[0].request.params[0], null, { limit })).rejects.toThrow(RangeError);
  }
  await expect(client.getChainHistory(history[0].request.params[0], 'bad cursor')).rejects.toThrow(RangeError);
}

test(chain_history_keeps_paxeer_rows_and_drops_the_side_label, chain_history_keeps_paxeer_rows_and_drops_the_side_label);
