import React from 'react';

import { base as block } from 'mocks/blocks/block';
import { base as dailyTxs } from 'mocks/stats/daily_txs';
import { base as stats } from 'mocks/stats/index';
import { base as statsMain } from 'mocks/stats/main';
import { tokenInfoERC20b } from 'mocks/tokens/tokenInfo';
import { base as transaction } from 'mocks/txs/tx';
import { expectExplorerColorMode } from 'playwright/helpers/expectExplorerColorMode';
import { test, expect } from 'playwright/lib';
import Home from 'ui/pages/Home';
import LayoutHome from 'ui/shared/layout/LayoutHome';

test('home visual system +@dark-mode', async({ render, mockApiResponse, mockAssetResponse, page }) => {
  await mockAssetResponse(stats.coin_image as string, './playwright/mocks/image_s.jpg');
  await mockApiResponse('general:tokens', { items: [ tokenInfoERC20b ], next_page_params: null });
  await mockApiResponse('stats:pages_main', statsMain);
  await mockApiResponse('general:stats', stats);
  await mockApiResponse('general:stats_charts_txs', dailyTxs);
  await mockApiResponse('general:homepage_blocks', [ block ]);
  await mockApiResponse('general:homepage_txs', [ transaction ]);
  const component = await render(<LayoutHome><Home/></LayoutHome>);
  await expect(component.locator('[data-latest-tx]')).toHaveAttribute('data-latest-tx', transaction.hash);
  await expect(component.locator('[data-label="hero-search"] input')).toBeVisible();
  await expectExplorerColorMode(page);
  await expect(component).toHaveScreenshot();
});
