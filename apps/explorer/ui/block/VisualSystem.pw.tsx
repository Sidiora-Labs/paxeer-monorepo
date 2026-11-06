import React from 'react';

import { base as block } from 'mocks/blocks/block';
import { expectExplorerColorMode } from 'playwright/helpers/expectExplorerColorMode';
import { test, expect } from 'playwright/lib';
import Block from 'ui/pages/Block';
import Layout from 'ui/shared/layout/Layout';

test('block visual system +@dark-mode', async({ render, mockApiResponse, page }) => {
  const height = String(block.height);
  await mockApiResponse('general:block', block, { pathParams: { height_or_hash: height } });
  const component = await render(<Layout><Block/></Layout>, {
    hooksConfig: { router: { pathname: '/block/[height_or_hash]', query: { height_or_hash: height, tab: 'index' }, isReady: true } },
  });
  await expect(component.locator('[data-section-group="BLOCK INFO"]')).toContainText(block.hash);
  await expectExplorerColorMode(page);
  await expect(component).toHaveScreenshot();
});
