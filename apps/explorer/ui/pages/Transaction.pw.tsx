import React from 'react';

import { base } from 'mocks/txs/tx';
import { test, expect } from 'playwright/lib';

import Transaction from './Transaction';

test('entity overview +@mobile +@dark-mode', async({ render, mockApiResponse, mockTextAd }) => {
  await mockTextAd();
  await mockApiResponse('general:tx', base, { pathParams: { hash: base.hash } });
  await mockApiResponse('general:tx_token_transfers', { items: [], next_page_params: null }, { pathParams: { hash: base.hash } });
  await mockApiResponse('general:tx_state_changes', {
    items: [],
    next_page_params: { items_count: 0, state_changes: null },
  }, { pathParams: { hash: base.hash } });
  const component = await render(<Transaction/>, { hooksConfig: { router: { pathname: '/tx/[hash]', query: { hash: base.hash } } } });
  await expect(component.getByRole('heading', { level: 1 })).toHaveText(base.hash);
  await expect(component).toHaveScreenshot();
});
