import React, { act } from 'react';
import { afterAll, afterEach, expect, test } from 'vitest';
import { cleanup, close, connectWallet, expectHeading, mount } from '../heading.test-support';
import { SdkClient } from '@kindle/sdk';
import { MarketClientContext } from '@/core/market/client';
import { serveRecorded, v2Reads, type Replay } from '@/core/test-fixtures/market-production/v2-reads';

const { default: SwapModule } = await import('./index');
const previousMetadata = process.env.NEXT_PUBLIC_METADATA_API_URL;
const replays: Replay[] = [];
afterEach(async () => {
  await cleanup();
  for (const replay of replays.splice(0)) await replay.close();
  if (previousMetadata === undefined) delete process.env.NEXT_PUBLIC_METADATA_API_URL;
  else process.env.NEXT_PUBLIC_METADATA_API_URL = previousMetadata;
});
afterAll(close);

function expectSwapHeading() {
  expectHeading(document.body, 'Swap');
  expect(document.querySelector('h1')?.className).toBe('shrink-0 text-md font-bold text-text');
}

function swap_without_an_account_and_while_loading_has_one_heading() {
  const view = mount(<SwapModule />, '/swap');
  expect(view.container.textContent).toContain('Loading tokens...');
  expect(view.container.textContent).toContain('Connect wallet');
  expectSwapHeading();
  const amount = view.container.querySelector('input[aria-label="You pay amount"]');
  expect(amount?.getAttribute('inputmode')).toBe('decimal');
  expect(amount?.classList.contains('tabular-nums')).toBe(true);
  const connect = [...view.container.querySelectorAll('button')].find(button => button.textContent === 'Connect wallet');
  expect(connect?.classList.contains('bg-surface-inverse')).toBe(true);
  expect(view.container.querySelector('button[aria-label="Switch tokens"]')).not.toBeNull();
}

async function swap_connected_and_empty_search_keeps_one_heading() {
  await connectWallet();
  const replay = await serveRecorded([v2Reads.responses.poolListTrending, v2Reads.responses.statsBatch]);
  replays.push(replay);
  const { origin } = replay;
  process.env.NEXT_PUBLIC_METADATA_API_URL = origin;
  const streamOrigin = origin.replace('http:', 'ws:');
  const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin, approvedStreamOrigins: [streamOrigin], production: false });
  const view = mount(<MarketClientContext.Provider value={client}><SwapModule /></MarketClientContext.Provider>, '/swap');
  expectSwapHeading();
  const select = [...view.container.querySelectorAll('button')].find(node => node.textContent?.includes('Select coin'));
  expect(select).toBeDefined();
  act(() => select!.click());
  expectSwapHeading();
  const input = document.querySelector<HTMLInputElement>('#swap-coin-search')!;
  act(() => {
    Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')!.set!.call(input, 'no matching coin');
    input.dispatchEvent(new window.Event('input', { bubbles: true }));
  });
  await expect.poll(() => document.body.textContent, { timeout: 15000 }).toContain('No coins match "no matching coin"');
  expect([...document.querySelectorAll('h3')].map(node => node.textContent)).toContain('No coins match "no matching coin"');
  expectSwapHeading();
  expect(document.querySelector('#swap-selector-title')?.tagName).toBe('H2');
  expect(replay.requests).toContain('GET /v2/pools?sort=trending&limit=100');
  expect(replay.requests).toContain('POST /v2/pools/stats:batch');
}

async function swap_token_error_keeps_one_heading() {
  const origin = 'http://127.0.0.1:0';
  const streamOrigin = 'ws://127.0.0.1:0';
  const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin, approvedStreamOrigins: [streamOrigin], production: false });
  const view = mount(<MarketClientContext.Provider value={client}><SwapModule /></MarketClientContext.Provider>, '/swap');
  const select = [...view.container.querySelectorAll('button')].find(node => node.textContent?.includes('Select coin'));
  expect(select).toBeDefined();
  act(() => select!.click());
  await expect.poll(() => document.body.textContent, { timeout: 15000 }).toContain("Coins can't load right now");
  expectSwapHeading();
  expect(document.querySelector('[role="alert"] h3')?.textContent).toBe("Coins can't load right now");
}

test(swap_without_an_account_and_while_loading_has_one_heading.name, swap_without_an_account_and_while_loading_has_one_heading);
test(swap_connected_and_empty_search_keeps_one_heading.name, swap_connected_and_empty_search_keeps_one_heading, 20000);
test(swap_token_error_keeps_one_heading.name, swap_token_error_keeps_one_heading, 20000);
