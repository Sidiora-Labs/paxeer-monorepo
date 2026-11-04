import React from 'react';

import { indexingStatus } from 'mocks/stats/index';
import { test, expect } from 'playwright/lib';
import AppError from 'ui/shared/AppError/AppError';

import LayoutError from './LayoutError';

test('base view +@mobile', async({ render, mockApiResponse }) => {
  await mockApiResponse('general:homepage_indexing_status', indexingStatus);
  const component = await render(
    <LayoutError>
      <AppError error={ new Error('An unexpected error has occurred') }/>
    </LayoutError>,
  );
  await expect(component.locator('main')).toBeVisible();
  await expect(component.getByText('Oops! Something went wrong', { exact: true })).toBeVisible();
  await expect(component.locator('footer')).toBeVisible();
  await expect(component.getByText(/10% Blocks Indexed/)).toBeVisible();
  await component.evaluate(async() => {
    await document.fonts.ready;
  });
  await expect(component).toHaveScreenshot();
});
