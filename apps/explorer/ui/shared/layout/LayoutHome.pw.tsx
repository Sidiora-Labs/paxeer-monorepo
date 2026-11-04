import React from 'react';

import { indexingStatus } from 'mocks/stats/index';
import { test, expect } from 'playwright/lib';

import LayoutHome from './LayoutHome';

test('base view +@mobile', async({ render, mockApiResponse }) => {
  await mockApiResponse('general:homepage_indexing_status', indexingStatus);
  const component = await render(<LayoutHome>Page Content</LayoutHome>);
  await expect(component.locator('[data-label="hero-band"]')).toBeVisible();
  await expect(component.getByText('Page Content', { exact: true })).toBeVisible();
  await expect(component.locator('footer')).toBeVisible();
  await expect(component.getByText(/10% Blocks Indexed/)).toBeVisible();
  await component.evaluate(async() => {
    await document.fonts.ready;
  });
  await expect(component).toHaveScreenshot();
});
