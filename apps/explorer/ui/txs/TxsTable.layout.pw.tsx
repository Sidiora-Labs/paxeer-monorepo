import React from 'react';

import * as txMock from 'mocks/txs/tx';
import { test, expect } from 'playwright/lib';

import TxsTable from './TxsTable';

for (const width of [ 1280, 1440 ]) {
  test.describe(`transactions at ${ width } pixels`, () => {
    test.use({ viewport: { width, height: 800 } });

    test('shows price update labels and counterparties in full', async function priceUpdateLabelsFit({ render, page }, testInfo) {
      const component = await render(
        <TxsTable
          txs={ [
            { ...txMock.priceUpdate, price_update: { ...txMock.priceUpdate.price_update, observation_count: 18 } },
            { ...txMock.priceUpdate, hash: txMock.base2.hash, price_update: { ...txMock.priceUpdate.price_update, observation_count: 999 } },
            txMock.pending,
          ] }
          sort="default"
          top={ 0 }
          showBlockInfo
          stickyHeader={ false }
        />,
      );

      await page.evaluate(async() => {
        await document.fonts.ready;
      });
      await component.screenshot({ path: testInfo.outputPath('transactions.png') });

      for (const count of [ 18, 999 ]) {
        const chip = component.locator(`[data-scan-method="Price update · ${ count } markets"]`);
        const label = chip.locator(':scope > span');

        await expect(chip).toBeVisible();
        expect(await label.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
        await expect(chip.locator('..').locator('..')).toHaveAttribute('title', `Price update · ${ count } markets`);
      }

      const region = page.getByRole('region', { name: 'Transactions' });
      expect(await region.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);

      const counterparty = component.locator('tbody tr').last().locator('td').nth(5);
      await expect(counterparty.locator('a[href^="/address/"]')).toHaveCount(2);
      expect(await counterparty.evaluate((cell) => Array.from(cell.querySelectorAll('[data-entity-kind="address"]')).every((entity) => {
        const bounds = entity.getBoundingClientRect();
        const cellBounds = cell.getBoundingClientRect();
        return bounds.left >= cellBounds.left && bounds.right <= cellBounds.right;
      }))).toBe(true);
    });
  });
}
