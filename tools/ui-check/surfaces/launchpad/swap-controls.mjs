import assert from 'node:assert/strict';
import { writeFile } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';
import { chromium } from 'playwright';

export async function measureSwapControls(page) {
  return page.evaluate(() => {
    function box(element) {
      const bounds = element.getBoundingClientRect();
      const style = getComputedStyle(element);
      return {
        x: bounds.x, y: bounds.y, width: bounds.width, height: bounds.height,
        fontSize: style.fontSize, lineHeight: style.lineHeight,
      };
    }
    return ['You pay', 'You receive'].map(label => {
      const input = document.querySelector(`input[aria-label="${label} amount"]`);
      assertInput(input);
      const row = input.parentElement.parentElement;
      return {
        label,
        button: box(row.querySelector('button')),
        input: box(input),
        field: box(row.parentElement),
      };
    });
    function assertInput(input) {
      if (!input) throw new Error('Swap amount field is visible');
    }
  });
}

export async function checkSwapControls(page, address) {
  const measurements = [];
  for (const [width, height] of [[1440, 900], [834, 900], [390, 844]]) {
    await page.setViewportSize({ width, height });
    for (const selected of [null, 'You pay', 'You receive']) {
      await page.goto(new URL('/swap', address).href);
      await page.locator('[data-shell-ready]').waitFor();
      await page.getByText('Loading tokens...', { exact: true }).waitFor({ state: 'hidden' });
      await page.evaluate(() => document.fonts.ready);
      if (selected !== null) {
        await page.getByRole('textbox', { name: `${selected} amount`, exact: true })
          .locator('xpath=../..').getByRole('button').click();
        await page.getByRole('dialog').locator('button.w-full').first().click();
      }
      const fields = await measureSwapControls(page);
      for (const field of fields) {
        assert.equal(field.button.height, width > 834 ? 32 : 44, `${field.label} coin height at ${width}`);
        if (width <= 834) {
          assert.ok(field.button.width >= 44, `${field.label} coin width at ${width}`);
          assert.ok(field.input.height >= 44, `${field.label} amount height at ${width}`);
        }
      }
      measurements.push({ width, height, selected, fields });
    }
  }
  return measurements;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    const measurements = await checkSwapControls(page, process.argv[2]);
    await writeFile(process.argv[3], `${JSON.stringify(measurements, null, 2)}\n`);
    console.log('Swap coin controls measure 32px on desktop and 44px for touch.');
  } finally {
    await browser.close();
  }
}
