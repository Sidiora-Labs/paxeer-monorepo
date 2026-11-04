import React from 'react';

import { test, expect } from 'playwright/lib';

import PromotionalBanners from './PromotionalBanners';
import { IMAGE_URL_1, IMAGE_URL_2, IMAGE_URL_3, HIGHLIGHTS_CONFIG_URL, HIGHLIGHTS_CONFIG } from './repair-22-33-408.fixtures';

test('three banners +@dark-mode', async({ render, page, mockEnvs, mockAssetResponse }) => {
  await mockEnvs([
    [ 'NEXT_PUBLIC_HOMEPAGE_HIGHLIGHTS_CONFIG', HIGHLIGHTS_CONFIG_URL ],
  ]);
  await page.route('**/explorer/assets/configs/homepage_highlights_config.json', (route) => route.fulfill({ json: HIGHLIGHTS_CONFIG }));
  await mockAssetResponse(IMAGE_URL_1, './playwright/mocks/image_s.jpg');
  await mockAssetResponse(IMAGE_URL_2, './playwright/mocks/image_md.jpg');
  await mockAssetResponse(IMAGE_URL_3, './playwright/mocks/image_long.jpg');

  const component = await render(<PromotionalBanners/>);

  await expect(component.locator('[data-promotional-banner]')).toHaveCount(3);
  for (const banner of HIGHLIGHTS_CONFIG) {
    await expect(component.getByText(banner.title, { exact: true })).toBeVisible();
    await expect(component.getByText(banner.description, { exact: true })).toBeVisible();
  }
  await expect(component.getByRole('link', { name: /Duck Deep into Transactions/ })).toHaveAttribute('href', 'https://example.com');
  await expect(component.getByRole('link', { name: /Geese Token Swap/ })).toHaveAttribute('href', '/essential-dapps/swap');
  const isDark = await page.locator('html').evaluate((element) => element.classList.contains('dark'));
  await expect(component.getByRole('img', { name: 'Duck Deep into Transactions' })).toHaveAttribute('src', IMAGE_URL_1);
  await expect(component.getByRole('img', { name: 'Geese Token Swap' })).toHaveAttribute('src', isDark ? IMAGE_URL_3 : IMAGE_URL_2);
  await expect(component).toHaveScreenshot();
});

test('two banners', async({ render, page, mockEnvs, mockAssetResponse }) => {
  await mockEnvs([
    [ 'NEXT_PUBLIC_HOMEPAGE_HIGHLIGHTS_CONFIG', HIGHLIGHTS_CONFIG_URL ],
  ]);
  await page.route('**/explorer/assets/configs/homepage_highlights_config.json', (route) => route.fulfill({ json: HIGHLIGHTS_CONFIG.slice(0, 2) }));
  await mockAssetResponse(IMAGE_URL_1, './playwright/mocks/image_s.jpg');
  await mockAssetResponse(IMAGE_URL_2, './playwright/mocks/image_md.jpg');
  await mockAssetResponse(IMAGE_URL_3, './playwright/mocks/image_long.jpg');

  const component = await render(<PromotionalBanners/>);

  await expect(component.locator('[data-promotional-banner]')).toHaveCount(2);
  for (const banner of HIGHLIGHTS_CONFIG.slice(0, 2)) {
    await expect(component.getByText(banner.title, { exact: true })).toBeVisible();
    await expect(component.getByText(banner.description, { exact: true })).toBeVisible();
  }
  await expect(component.getByText('Duckling Smart Contracts')).toHaveCount(0);
  await expect(component.getByRole('link', { name: /Duck Deep into Transactions/ })).toHaveAttribute('href', 'https://example.com');
  await expect(component.getByRole('link', { name: /Geese Token Swap/ })).toHaveAttribute('href', '/essential-dapps/swap');
  await expect(component.getByRole('img', { name: 'Duck Deep into Transactions' })).toHaveAttribute('src', IMAGE_URL_1);
  await expect(component.getByRole('img', { name: 'Geese Token Swap' })).toHaveAttribute('src', IMAGE_URL_2);
  await expect(component).toHaveScreenshot();
});
