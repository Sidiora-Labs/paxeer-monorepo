import type { TestInfo } from '@playwright/test';
import React from 'react';

import * as txMock from 'mocks/txs/tx';
import type { TestFnArgs } from 'playwright/lib';
import { test, expect } from 'playwright/lib';
import MainColumn from 'ui/shared/layout/components/MainColumn';

import TxsTable from './TxsTable';

const ordinary = { ...txMock.base, from: { ...txMock.base.from, ens_domain_name: null } };
const failed = { ...txMock.withDecodedRevertReason, hash: txMock.base2.hash };
const pending = { ...txMock.pending, hash: txMock.base3.hash };
const noop = () => undefined;

async function transactionCounterpartiesFit({ render, page }: TestFnArgs, testInfo: TestInfo) {
  const component = await render(
    <MainColumn>
      <TxsTable
        txs={ [ txMock.priceUpdate, ordinary, failed, pending ] }
        sort="default"
        onSortToggle={ noop }
        top={ 0 }
        showBlockInfo
        stickyHeader={ false }
      />
    </MainColumn>,
  );
  await page.evaluate(() => document.fonts.ready);

  const table = component.locator('table');
  const region = component.getByRole('region', { name: 'Transactions' });
  const rows = table.locator('tbody tr');
  const sender = rows.nth(0).locator('td').nth(5).locator('[data-entity-content]');
  const addresses = rows.nth(1).locator('td').nth(5).locator('[data-entity-content]');

  await expect(sender).toHaveText('0x7c...9f01');
  await expect(addresses).toHaveText([ '0x04...84B9', '0xd7...5859' ]);
  await expect(rows.nth(0).getByRole('link', { name: 'Price feed', exact: true })).toBeVisible();
  expect(await region.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);

  const shortenedContent = [
    'tbody tr:nth-child(-n+2) [data-entity-kind=address] [data-entity-content]',
    '[data-entity-kind=tx] [data-entity-content]',
  ].join(', ');
  for (const content of await table.locator(shortenedContent).all()) {
    expect(await content.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
  }

  for (const button of await table.getByRole('button', { name: 'copy', exact: true }).all()) {
    const box = await button.boundingBox();
    expect(box?.width).toBeGreaterThanOrEqual(44);
    expect(box?.height).toBeGreaterThanOrEqual(44);
  }

  await expect(rows.nth(2).locator('[data-status]')).toHaveAttribute('data-status', 'error');
  await expect(rows.nth(3).locator('[data-status]')).toHaveAttribute('data-status', 'pending');
  await expect(rows.nth(1).locator('td').nth(6)).toContainText('42');
  await expect(rows.nth(1).locator('td').nth(7)).toContainText('0.00714317');
  expect(await table.locator('thead th').nth(2).evaluate((element) => element.getBoundingClientRect().width)).toBe(224);
  expect(await table.locator('thead th').nth(5).evaluate((element) => element.getBoundingClientRect().width))
    .toBe(await table.evaluate((element) => element.getBoundingClientRect().width) - 904);

  for (const index of [ 6, 7 ]) {
    expect(await rows.nth(1).locator('td').nth(index).evaluate((element) => {
      const style = getComputedStyle(element);
      const context = document.createElement('canvas').getContext('2d')!;
      context.font = `${ style.fontWeight } ${ style.fontSize } ${ style.fontFamily }`;
      return element.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight) >= context.measureText('123,456.7891').width;
    })).toBe(true);
  }

  await testInfo.attach('transactions', { body: await page.screenshot(), contentType: 'image/png' });
}

for (const width of [ 1280, 1440 ]) {
  test.describe(`viewport ${ width }px`, () => {
    test.use({ viewport: { width, height: 900 } });
    test('shows shortened counterparties and the price feed without horizontal scrolling', transactionCounterpartiesFit);
  });
}
