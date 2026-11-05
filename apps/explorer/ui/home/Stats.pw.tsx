import type { Locator } from '@playwright/test';
import React from 'react';

import * as blockMock from 'mocks/blocks/block';
import * as statsMock from 'mocks/stats/index';
import { test, expect } from 'playwright/lib';

import Stats from './Stats';

async function assertWidgets(component: Locator, ids: Array<string>) {
  await expect(component.locator('[data-highlight]')).toHaveCount(ids.length);
  for (const id of ids) {
    await expect(component.locator(`[data-highlight="${ id }"] [data-label]`)).toBeVisible();
  }
  if (ids.includes('total_txs')) {
    await expect(component.locator('[data-highlight="total_txs"] [data-value]')).toHaveText(
      Number(statsMock.base.total_transactions).toLocaleString(undefined, { maximumFractionDigits: 2, notation: 'compact' }),
    );
  }
  if (ids.includes('total_blocks')) {
    await expect(component.locator('[data-highlight="total_blocks"] [data-value]')).toHaveText(blockMock.base.height.toLocaleString());
  }
  if (ids.includes('wallet_addresses')) {
    await expect(component.locator('[data-highlight="wallet_addresses"] [data-value]')).toHaveText(Number(statsMock.base.total_addresses).toLocaleString());
  }
  if (ids.includes('average_block_time')) {
    await expect(component.locator('[data-highlight="average_block_time"] [data-value]')).toHaveText('6.2s');
  }
  if (ids.includes('gas_tracker')) {
    await expect(component.locator('[data-highlight="gas_tracker"] [data-value]')).toContainText('23.8');
  } else {
    await expect(component.locator('[data-highlight="gas_tracker"]')).toHaveCount(0);
  }
  if (ids.includes('btc_locked')) {
    await expect(component.locator('[data-highlight="btc_locked"] [data-value]')).toHaveText('3,337.49');
  } else {
    await expect(component.locator('[data-highlight="btc_locked"]')).toHaveCount(0);
  }
}

test.describe('all items', () => {
  let component: Locator;

  test.beforeEach(async({ render, mockApiResponse, mockEnvs }) => {
    await mockEnvs([
      [ 'NEXT_PUBLIC_HOMEPAGE_STATS', '["total_blocks","average_block_time","total_txs","wallet_addresses","gas_tracker","btc_locked"]' ],
      [ 'NEXT_PUBLIC_STATS_API_HOST', '' ],
      [ 'NEXT_PUBLIC_HOMEPAGE_CHARTS', '[]' ],
    ]);
    await mockApiResponse('general:stats', statsMock.withBtcLocked);
    await mockApiResponse('general:homepage_blocks', [ blockMock.base ]);
    component = await render(<Stats/>);
  });

  test('+@mobile +@dark-mode', async() => {
    await assertWidgets(component, [ 'total_blocks', 'average_block_time', 'total_txs', 'wallet_addresses', 'gas_tracker', 'btc_locked' ]);
    await expect(component).toHaveScreenshot();
  });
});

test('no gas info', async({ render, mockApiResponse, mockEnvs }) => {
  await mockEnvs([
    [ 'NEXT_PUBLIC_HOMEPAGE_STATS', '["total_blocks","average_block_time","total_txs","wallet_addresses","gas_tracker"]' ],
    [ 'NEXT_PUBLIC_STATS_API_HOST', '' ],
    [ 'NEXT_PUBLIC_HOMEPAGE_CHARTS', '[]' ],
  ]);
  await mockApiResponse('general:stats', statsMock.withoutGasInfo);
  await mockApiResponse('general:homepage_blocks', [ blockMock.base ]);
  const component = await render(<Stats/>);

  await assertWidgets(component, [ 'total_blocks', 'average_block_time', 'total_txs', 'wallet_addresses' ]);
  await expect(component).toHaveScreenshot();
});

test('4 items default view +@mobile -@default', async({ render, mockApiResponse, mockEnvs }) => {
  await mockEnvs([
    [ 'NEXT_PUBLIC_HOMEPAGE_STATS', '["total_txs","gas_tracker","wallet_addresses","total_blocks"]' ],
    [ 'NEXT_PUBLIC_STATS_API_HOST', '' ],
    [ 'NEXT_PUBLIC_HOMEPAGE_CHARTS', '[]' ],
  ]);
  await mockApiResponse('general:stats', statsMock.base);
  await mockApiResponse('general:homepage_blocks', [ blockMock.base ]);
  const component = await render(<Stats/>);
  await assertWidgets(component, [ 'total_txs', 'gas_tracker', 'wallet_addresses', 'total_blocks' ]);
  await expect(component).toHaveScreenshot();
});

test('3 items default view +@mobile -@default', async({ render, mockApiResponse, mockEnvs }) => {
  await mockEnvs([
    [ 'NEXT_PUBLIC_HOMEPAGE_STATS', '["total_txs","wallet_addresses","total_blocks"]' ],
    [ 'NEXT_PUBLIC_STATS_API_HOST', '' ],
    [ 'NEXT_PUBLIC_HOMEPAGE_CHARTS', '[]' ],
  ]);
  await mockApiResponse('general:stats', statsMock.base);
  await mockApiResponse('general:homepage_blocks', [ blockMock.base ]);
  const component = await render(<Stats/>);
  await assertWidgets(component, [ 'total_txs', 'wallet_addresses', 'total_blocks' ]);
  await expect(component).toHaveScreenshot();
});
