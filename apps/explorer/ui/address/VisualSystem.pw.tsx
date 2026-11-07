import React from 'react';

import * as address from 'mocks/address/address';
import { forContract } from 'mocks/address/counters';
import { base as counters } from 'mocks/address/tabCounters';
import { expectExplorerColorMode } from 'playwright/helpers/expectExplorerColorMode';
import { test, expect } from 'playwright/lib';
import Address from 'ui/pages/Address';
import Layout from 'ui/shared/layout/Layout';

test('address visual system +@dark-mode', async({ render, mockApiResponse, page }) => {
  const pathParams = { hash: address.hash };
  await mockApiResponse('general:address', address.contract, { pathParams });
  await mockApiResponse('general:address_counters', forContract, { pathParams });
  await mockApiResponse('general:address_tabs_counters', counters, { pathParams });
  await mockApiResponse('general:address_token_balances', [], { pathParams });
  const component = await render(<Layout><Address/></Layout>, {
    hooksConfig: { router: { pathname: '/address/[hash]', query: { hash: address.hash }, isReady: true } },
  });
  await expect(component.locator('[data-section-group="ADDRESS INFO"]')).toBeVisible();
  await expect(component.locator('[data-entity-kind-label]')).toHaveText('CONTRACT');
  await expectExplorerColorMode(page);
  await expect(component).toHaveScreenshot();
});
