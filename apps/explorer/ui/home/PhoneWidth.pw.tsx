import { Box } from '@chakra-ui/react';
import type { Locator } from '@playwright/test';
import React from 'react';

import * as blockMock from 'mocks/blocks/block';
import * as dailyTxsMock from 'mocks/stats/daily_txs';
import * as statsMock from 'mocks/stats/index';
import type { TestFnArgs } from 'playwright/lib';
import { expect, test } from 'playwright/lib';
import Home from 'ui/pages/Home';
import NetworkStats from 'ui/pages/Stats';
import StatsWidget from 'ui/shared/stats/StatsWidget';

async function expectOneLine(element: Locator): Promise<void> {
  const layout = await element.evaluate((node) => {
    const range = document.createRange();
    range.selectNodeContents(node);
    const lines = new Set(Array.from(range.getClientRects()).filter(rect => rect.width > 0).map(rect => Math.round(rect.top)));
    const bounds = node.getBoundingClientRect();
    const parent = node.parentElement!.getBoundingClientRect();
    return { lines: lines.size, fits: bounds.right <= parent.right + 1, noOverflow: node.scrollWidth <= node.clientWidth + 1 };
  });
  expect(layout).toEqual({ lines: 1, fits: true, noOverflow: true });
}

test.setTimeout(60_000);

for (const width of [ 320, 390, 430 ]) {
  for (const colorScheme of [ 'light', 'dark' ] as const) {
    async function phoneStatisticsKeepValuesSearchAndColumnHeadsWhole({ page, render, mockApiResponse, mockEnvs }: TestFnArgs): Promise<void> {
      await page.setViewportSize({ width, height: 900 });
      await page.emulateMedia({ colorScheme });
      await mockEnvs([
        [ 'NEXT_PUBLIC_STATS_API_HOST', '' ],
        [ 'NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED', 'false' ],
        [ 'NEXT_PUBLIC_MARKETPLACE_CONFIG_URL', '' ],
      ]);
      await mockApiResponse('general:stats', statsMock.base);
      await mockApiResponse('general:homepage_blocks', [ blockMock.base, blockMock.base2 ]);
      await mockApiResponse('general:stats_charts_txs', dailyTxsMock.base);
      await mockApiResponse('general:stats_charts_market', { available_supply: '0', chart_data: [] });
      await mockApiResponse('general:stats_charts_secondary_coin_market', { available_supply: '0', chart_data: [] });
      await mockApiResponse('general:homepage_txs', []);
      await mockApiResponse('general:tokens', { items: [], next_page_params: null });

      const home = await render(<Box p={ 4 }><Home/></Box>, { hooksConfig: { router: { pathname: '/', query: {}, isReady: true } } });
      await expect(home.locator('[data-stat="total_blocks"] [data-value]')).toHaveText('30,215,608');
      await page.evaluate(() => document.fonts.ready);
      for (const value of await home.locator('[data-stat] [data-value] > span').all()) {
        await expectOneLine(value);
      }
      const search = home.locator('[data-label="hero-search"] button[aria-label="Search"]:visible');
      await expect(search).toHaveText('Search or ask…');
      await expectOneLine(search.locator('span').last());
      await expect(search).toHaveCSS('white-space', 'nowrap');
      await page.screenshot({ path: test.info().outputPath('home.png'), fullPage: true });
      await home.unmount();

      const stats = await render(<Box p={ 4 }><NetworkStats/></Box>, { hooksConfig: { router: { pathname: '/stats', query: {}, isReady: true } } });
      const region = stats.locator('[role="region"][aria-label="Recent blocks"]');
      await expect(region.locator('tbody tr')).toHaveCount(2);
      await expect(region.locator('th')).toHaveText([ 'Block', 'Age', 'Txn', 'Gas used' ]);
      for (const header of await region.locator('th').all()) {
        await expectOneLine(header);
      }
      expect(await region.evaluate(node => node.scrollWidth <= node.clientWidth)).toBe(true);
      await region.focus();
      await expect(region).toBeFocused();
      await expect(region.locator('th').first()).toHaveCSS('position', 'sticky');
      for (const value of await stats.locator('[data-network-stats-hero] [data-value]').all()) {
        await expectOneLine(value);
      }
      await page.screenshot({ path: test.info().outputPath('stats.png'), fullPage: true });
      await stats.unmount();

      const tile = await render(<Box w="160px"><StatsWidget label="Total transactions" value="1,234,567,890"/></Box>);
      const value = tile.locator('[data-value] > span');
      await expectOneLine(value);
      const fontSize = await value.evaluate(node => parseFloat(getComputedStyle(node).fontSize));
      expect(fontSize).toBeGreaterThanOrEqual(14);
      expect(fontSize).toBeLessThanOrEqual(24);
      await tile.unmount();
      const longTile = await render(<Box w="160px"><StatsWidget label="Total transactions" value="123,456,789,012,345,678,901,234,567,890"/></Box>);
      const longValue = longTile.locator('[data-value] > span');
      await expect(longValue).toHaveCSS('font-size', '14px');
      expect(await longValue.evaluate(node => node.getBoundingClientRect().height)).toBeGreaterThan(28);
      expect(await longValue.evaluate(node => node.scrollWidth <= node.clientWidth)).toBe(true);
    }

    test(`phone statistics keep values search and column heads whole at ${ width } in ${ colorScheme }`,
      phoneStatisticsKeepValuesSearchAndColumnHeadsWhole);
  }
}
