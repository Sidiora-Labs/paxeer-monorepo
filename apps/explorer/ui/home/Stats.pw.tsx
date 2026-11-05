import type { Locator } from '@playwright/test';
import React from 'react';

import * as statsMock from 'mocks/stats/index';
import { test, expect } from 'playwright/lib';

import Stats from './Stats';

async function assertWidgets(component: Locator, hasGas = true) {
  await expect(component.locator('[data-stat]')).toHaveCount(5);
  await expect(component.locator('[data-stat] h2')).toHaveText([ 'Total blocks', 'Average block time', 'Total transactions', 'Wallet addresses', 'Gas price' ]);
  await expect(component.locator('[data-stat="total_blocks"] [data-value]')).toHaveText(Number(statsMock.base.total_blocks).toLocaleString());
  await expect(component.locator('[data-stat="total_txs"] [data-value]')).toHaveText(Number(statsMock.base.total_transactions).toLocaleString());
  await expect(component.locator('[data-stat="wallet_addresses"] [data-value]')).toHaveText(Number(statsMock.base.total_addresses).toLocaleString());
  await expect(component.locator('[data-stat="average_block_time"] [data-value]')).toHaveText('6.2s');
  await expect(component.locator('[data-stat="gas_tracker"] [data-value]')).toHaveText(hasGas ? '23.75 Gwei' : '—');
}

test.beforeEach(async({ mockEnvs, mockApiResponse }) => {
  await mockEnvs([ [ 'NEXT_PUBLIC_STATS_API_HOST', '' ] ]);
  await mockApiResponse('general:stats_charts_txs', { chart_data: [] });
});

test.describe('all items', () => {
  let component: Locator;

  test.beforeEach(async({ render, mockApiResponse, mockEnvs }) => {
    await mockEnvs([ [ 'NEXT_PUBLIC_HOMEPAGE_STATS', '["total_blocks","average_block_time","total_txs","wallet_addresses","gas_tracker","btc_locked"]' ] ]);
    await mockApiResponse('general:stats', statsMock.withBtcLocked);
    component = await render(<Stats/>);
  });

  test('+@mobile +@dark-mode', async() => {
    await assertWidgets(component);
    await expect(component.locator('[data-stat="btc_locked"]')).toHaveCount(0);
    await expect(component).toHaveScreenshot();
  });
});

test('no gas info +@dark-mode', async({ render, mockApiResponse }) => {
  await mockApiResponse('general:stats', statsMock.withoutGasInfo);
  const component = await render(<Stats/>);
  await assertWidgets(component, false);
  await expect(component).toHaveScreenshot();
});

test('five tiles with the legacy four-item setting +@mobile +@dark-mode', async({ render, mockApiResponse, mockEnvs }) => {
  await mockEnvs([ [ 'NEXT_PUBLIC_HOMEPAGE_STATS', '["total_txs","gas_tracker","wallet_addresses","total_blocks"]' ] ]);
  await mockApiResponse('general:stats', statsMock.base);
  const component = await render(<Stats/>);
  await assertWidgets(component);
  await expect(component).toHaveScreenshot();
});

test('five tiles with the legacy three-item setting +@mobile +@dark-mode', async({ render, mockApiResponse, mockEnvs }) => {
  await mockEnvs([ [ 'NEXT_PUBLIC_HOMEPAGE_STATS', '["total_txs","wallet_addresses","total_blocks"]' ] ]);
  await mockApiResponse('general:stats', statsMock.base);
  const component = await render(<Stats/>);
  await assertWidgets(component);
  await expect(component).toHaveScreenshot();
});
