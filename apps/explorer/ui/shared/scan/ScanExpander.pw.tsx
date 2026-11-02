import { test, expect } from '@playwright/experimental-ct-react';
import React from 'react';

import { Provider } from 'toolkit/chakra/provider';

import ScanExpander from './ScanExpander';

test('supports Tab, Enter, Space and repeated pointer activation without submitting a form', async({ mount, page }) => {
  const component = await mount(
    <Provider>
      <form data-submitted="false">
        <button type="button">Before details</button>
        <ScanExpander>
          <span>Gas usage and fee per gas</span>
        </ScanExpander>
        <button type="button">After details</button>
      </form>
    </Provider>,
  );

  await component.evaluate((form) => {
    form.addEventListener('submit', (event) => {
      event.preventDefault();
      form.setAttribute('data-submitted', 'true');
    });
  });

  const toggle = component.locator('[data-toggle]');
  const details = component.getByText('Gas usage and fee per gas');

  await expect(component.getByRole('button', { name: '+ Click to show more' })).toHaveAttribute('type', 'button');
  await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  await expect(details).toHaveCount(0);

  await component.getByRole('button', { name: 'Before details' }).focus();
  await page.keyboard.press('Tab');
  await expect(toggle).toBeFocused();

  await page.keyboard.press('Enter');
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  await expect(toggle).toHaveText('- Click to show less');
  await expect(details).toBeVisible();
  await expect(toggle).toBeFocused();

  await page.keyboard.press('Space');
  await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  await expect(toggle).toHaveText('+ Click to show more');
  await expect(details).toHaveCount(0);
  await expect(toggle).toBeFocused();

  await toggle.click();
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  await expect(details).toBeVisible();
  await toggle.click();
  await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  await expect(details).toHaveCount(0);

  await page.keyboard.press('Tab');
  await expect(component.getByRole('button', { name: 'After details' })).toBeFocused();
  await page.keyboard.press('Shift+Tab');
  await expect(toggle).toBeFocused();
  await page.keyboard.press('Shift+Tab');
  await expect(component.getByRole('button', { name: 'Before details' })).toBeFocused();
  await expect(component).toHaveAttribute('data-submitted', 'false');
});

test('keeps initially open details and custom prompts keyboard-operable', async({ mount, page }) => {
  const component = await mount(
    <Provider>
      <ScanExpander defaultOpen label="Advanced" showText="Expand details" hideText="Collapse details">
        <span>Already visible</span>
      </ScanExpander>
    </Provider>,
  );

  const toggle = component.getByRole('button', { name: 'Collapse details' });
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  await expect(component.getByText('Advanced:')).toBeVisible();
  await expect(component.getByText('Already visible')).toBeVisible();

  await toggle.focus();
  await page.keyboard.press('Space');
  await expect(component.getByRole('button', { name: 'Expand details' })).toBeFocused();
  await expect(component.locator('[data-toggle]')).toHaveAttribute('aria-expanded', 'false');
  await expect(component.getByText('Already visible')).toHaveCount(0);

  await page.keyboard.press('Enter');
  await expect(component.getByRole('button', { name: 'Collapse details' })).toBeFocused();
  await expect(component.locator('[data-toggle]')).toHaveAttribute('aria-expanded', 'true');
  await expect(component.getByText('Already visible')).toBeVisible();
});
