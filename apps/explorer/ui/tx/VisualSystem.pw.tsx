import React from 'react';

import { base as transaction } from 'mocks/txs/tx';
import { expectExplorerColorMode } from 'playwright/helpers/expectExplorerColorMode';
import { test, expect } from 'playwright/lib';
import Transaction from 'ui/pages/Transaction';
import Layout from 'ui/shared/layout/Layout';

test('transaction visual system +@dark-mode', async({ render, mockApiResponse, page }) => {
  const pathParams = { hash: transaction.hash };
  await mockApiResponse('general:tx', transaction, { pathParams });
  await mockApiResponse('general:tx_token_transfers', { items: [], next_page_params: null }, { pathParams });
  await mockApiResponse('general:tx_state_changes', { items: [], next_page_params: { items_count: 0, state_changes: null } }, { pathParams });
  const component = await render(<Layout><Transaction/></Layout>, {
    hooksConfig: { router: { pathname: '/tx/[hash]', query: { hash: transaction.hash }, isReady: true } },
  });
  await expect(component.locator('[data-section-group="SUMMARY"]')).toBeVisible();
  await expect(component.getByRole('heading', { level: 1 })).toHaveText(transaction.hash);
  await expectExplorerColorMode(page);
  await expect(component).toHaveScreenshot();
});
