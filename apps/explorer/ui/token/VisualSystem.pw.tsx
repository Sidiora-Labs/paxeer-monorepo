import React from 'react';

import { token as contract } from 'mocks/address/address';
import { tokenInfo, tokenCounters } from 'mocks/tokens/tokenInfo';
import { expectExplorerColorMode } from 'playwright/helpers/expectExplorerColorMode';
import { test, expect } from 'playwright/lib';
import Token from 'ui/pages/Token';
import Layout from 'ui/shared/layout/Layout';

test('token visual system +@dark-mode', async({ render, mockApiResponse, page }) => {
  const hash = tokenInfo.address_hash;
  const pathParams = { hash };
  await mockApiResponse('general:token', tokenInfo, { pathParams });
  await mockApiResponse('general:address', contract, { pathParams });
  await mockApiResponse('general:token_counters', tokenCounters, { pathParams });
  const component = await render(<Layout><Token/></Layout>, {
    hooksConfig: { router: { pathname: '/token/[hash]', query: { hash, tab: 'markets' }, isReady: true } },
  });
  await expect(component.locator('[data-token-markets]')).toBeVisible();
  await expect(component.locator('[data-entity-kind-label]')).toHaveText('TOKEN');
  await expectExplorerColorMode(page);
  await expect(component).toHaveScreenshot();
});
