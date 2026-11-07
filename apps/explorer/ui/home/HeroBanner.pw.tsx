import React from 'react';

import { test, expect } from 'playwright/lib';
import * as pwConfig from 'playwright/utils/config';

import HeroBanner from './HeroBanner';

test('semantic hero and prompt +@mobile +@dark-mode', async({ render, page }) => {
  const component = await render(<HeroBanner/>);
  const hero = component.locator('[data-label="hero"]');
  const title = component.locator('[data-label="hero-title"]');
  await expect(title).toHaveCSS('font-size', '40px');
  await expect(title).toHaveCSS('line-height', '44px');
  await expect(title).toHaveCSS('font-weight', '600');
  await expect(hero).toHaveCSS('background-image', 'none');
  await expect(component.locator('[data-label="hero-search"] input:visible')).toHaveAttribute(
    'placeholder', 'Search or ask: a transaction, address, block or token',
  );
  await expect(component.locator('[data-label="hero-search"] input:visible')).toHaveCSS('height', '52px');
  await expect(component).toHaveScreenshot({
    mask: [ page.locator(pwConfig.adsBannerSelector) ],
    maskColor: pwConfig.maskColor,
  });
});
