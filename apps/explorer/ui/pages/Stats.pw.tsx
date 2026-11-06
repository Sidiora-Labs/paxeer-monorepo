import React from 'react';

import { base as block } from 'mocks/blocks/block';
import { base as stats } from 'mocks/stats';
import { averageGasPrice } from 'mocks/stats/line';
import { base as lines } from 'mocks/stats/lines';
import { test, expect } from 'playwright/lib';

import Stats from './Stats';

test('network stats +@dark-mode +@mobile', async({ render, mockApiResponse, mockTextAd, page }) => {
  await mockTextAd();
  await mockApiResponse('general:stats', stats);
  await mockApiResponse('general:homepage_blocks', [ block ]);
  await mockApiResponse('stats:counters', { counters: [] });
  await mockApiResponse('stats:lines', { sections: lines.sections.filter((section) => section.id === 'gas').map((section) => ({
    ...section, charts: section.charts.filter((chart) => chart.id === 'averageGasPrice'),
  })) });
  const chartUrl = await mockApiResponse('stats:line', averageGasPrice, { pathParams: { id: 'averageGasPrice' }, queryParams: { from: '**' } });
  const component = await render(<Stats/>);
  await page.waitForResponse(chartUrl);
  await page.waitForFunction(() => document.querySelector('path[data-name="chart-small"]')?.getAttribute('opacity') === '1');
  await expect(component).toHaveScreenshot();
});
