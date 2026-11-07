import React from 'react';

import { indexingStatus } from 'mocks/stats/index';
import { test, expect } from 'playwright/lib';
import * as pwConfig from 'playwright/utils/config';

import Layout from './Layout';

test('base view +@mobile', async({ render, mockEnvs, mockApiResponse }) => {
  await mockEnvs([
    [
      'NEXT_PUBLIC_MAINTENANCE_ALERT_MESSAGE',
      'We are currently lacking pictures of <i>ducks</i>. Please <a href="mailto:duck@blockscout.com">send</a> us one.',
    ],
  ]);
  await mockApiResponse('general:homepage_indexing_status', indexingStatus);
  const component = await render(<Layout>Page Content</Layout>);
  await expect(component.locator('main')).toHaveText('Page Content');
  await expect(component.locator('footer')).toBeVisible();
  await expect(component.getByText(/10% Blocks Indexed/)).toBeVisible();
  await component.evaluate(async() => {
    await document.fonts.ready;
  });
  await expect(component).toHaveScreenshot();
});

test.describe('xxl screen', () => {
  test.use({ viewport: pwConfig.viewport.xxl });

  test('vertical navigation', async({ render, mockEnvs, mockApiResponse }) => {
    await mockEnvs([
      [ 'NEXT_PUBLIC_NAVIGATION_LAYOUT', 'vertical' ],
    ]);
    await mockApiResponse('general:homepage_indexing_status', indexingStatus);
    const component = await render(<Layout>Page Content</Layout>);
    await expect(component.locator('main')).toHaveText('Page Content');
    await expect(component.locator('footer')).toBeVisible();
    await expect(component.getByText(/10% Blocks Indexed/)).toBeVisible();
    await component.evaluate(async() => {
      await document.fonts.ready;
    });
    await expect(component).toHaveScreenshot();
  });

  test('horizontal navigation', async({ render, mockEnvs, mockApiResponse }) => {
    await mockEnvs([
      [ 'NEXT_PUBLIC_NAVIGATION_LAYOUT', 'horizontal' ],
    ]);
    await mockApiResponse('general:homepage_indexing_status', indexingStatus);
    const component = await render(<Layout>Page Content</Layout>);
    await expect(component.locator('main')).toHaveText('Page Content');
    await expect(component.locator('footer')).toBeVisible();
    await expect(component.getByText(/10% Blocks Indexed/)).toBeVisible();
    await component.evaluate(async() => {
      await document.fonts.ready;
    });
    await expect(component).toHaveScreenshot();
  });
});
