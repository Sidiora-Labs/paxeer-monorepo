import React from 'react';

import { expectExplorerColorMode } from 'playwright/helpers/expectExplorerColorMode';
import { test, expect } from 'playwright/lib';

import HeaderDesktop from './HeaderDesktop';

test('header visual system +@dark-mode', async({ render, page }) => {
  const component = await render(<HeaderDesktop/>);
  await expect(component.locator('[data-label="desktop-header"]')).toBeVisible();
  await expect(component.getByLabel(/Switch to (light|dark) theme/)).toBeVisible();
  await expectExplorerColorMode(page);
  await expect(component).toHaveScreenshot();
});
