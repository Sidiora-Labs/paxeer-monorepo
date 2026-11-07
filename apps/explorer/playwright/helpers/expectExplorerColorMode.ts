import type { Page } from '@playwright/test';

import { expect } from 'playwright/lib';
import { LIGHT, DARK } from 'toolkit/theme/foundations/palette';

export async function expectExplorerColorMode(page: Page) {
  const mode = await page.evaluate(() => window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
  await expect(page.locator('html')).toHaveClass(new RegExp(`\\b${ mode }\\b`));
  const canvas = mode === 'dark' ? DARK.canvas : LIGHT.canvas;
  const rgb = [ 1, 3, 5 ].map((offset) => parseInt(canvas.slice(offset, offset + 2), 16));
  await expect(page.locator('body')).toHaveCSS('background-color', `rgb(${ rgb.join(', ') })`);
  await expect(page.locator('aside, [data-label="sidebar"], [data-label="navigation-vertical"]')).toHaveCount(0);
}
