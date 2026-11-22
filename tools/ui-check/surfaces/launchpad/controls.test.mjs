import assert from 'node:assert/strict';
import { after, test } from 'node:test';
import { chromium } from 'playwright';
import { origin } from './environment.mjs';

const browser = await chromium.launch({ headless: true });
after(() => browser.close());

async function openView(width, path, height = 900) {
  const context = await browser.newContext({
    viewport: { width, height },
    ignoreHTTPSErrors: true,
  });
  const page = await context.newPage();
  await page.goto(new URL(path, origin).href, { waitUntil: 'networkidle', timeout: 180000 });
  await page.locator('[data-shell-ready]').waitFor();
  await page.evaluate(() => document.fonts.ready);
  return { context, page };
}

async function tickerDesktopAndTouchSizes() {
  for (const width of [1440, 834, 390]) {
    const { context, page } = await openView(width, '/?render=empty', width === 390 ? 844 : 900);
    try {
      const ticker = page.getByRole('region', { name: 'Market ticker' });
      if (width === 390) {
        assert.equal(await ticker.isVisible(), false);
        continue;
      }
      await ticker.locator('[data-ticker-empty]').waitFor();
      const sizes = await ticker.evaluate((section) => {
        const group = section.querySelector('[role="radiogroup"]');
        const feed = section.querySelector('[data-ticker-empty]');
        const link = feed.querySelector('a');
        const rect = (el) => {
          const box = el.getBoundingClientRect();
          return { width: box.width, height: box.height, middle: box.y + box.height / 2 };
        };
        return {
          section: rect(section), group: rect(group), feed: rect(feed), link: rect(link),
          padding: getComputedStyle(group).padding,
          buttons: [...group.querySelectorAll('button')].map(rect),
        };
      });
      if (width > 834) {
        assert.equal(sizes.section.height, 32);
        assert.equal(sizes.group.width, 132);
        assert.equal(sizes.group.height, 28);
        assert.equal(sizes.padding, '2px');
        assert.equal(sizes.link.height, 16);
        assert.ok(Math.abs(sizes.feed.middle - sizes.group.middle) <= 1);
        assert.ok(Math.abs(sizes.link.middle - sizes.group.middle) <= 1);
        assert.ok(sizes.buttons.every((box) => box.height === 24 && box.width === 24));
      } else {
        assert.equal(sizes.section.height, 44);
        assert.equal(sizes.group.width, 228);
        assert.equal(sizes.group.height, 44);
        assert.equal(sizes.padding, '0px');
        assert.equal(sizes.link.height, 44);
        assert.ok(sizes.buttons.every((box) => box.height >= 44 && box.width >= 44));
      }
      console.log(JSON.stringify({ view: 'ticker', width, sizes }));
    } finally {
      await context.close();
    }
  }
}

async function tickerUnavailableKeepsItsRow() {
  for (const width of [834, 1440]) {
    const { context, page } = await openView(width, '/?render=error');
    try {
      const ticker = page.getByRole('region', { name: 'Market ticker' });
      await ticker.locator(':scope > div:last-child:empty').waitFor();
      assert.equal(await ticker.getByRole('radio').count(), 5);
      assert.equal(await ticker.getByRole('radio', { name: 'Top earners' }).getAttribute('aria-checked'), 'true');
      assert.equal((await ticker.boundingBox()).height, width === 834 ? 44 : 32);
    } finally {
      await context.close();
    }
  }
}

async function cookieNoticeSizes() {
  for (const width of [1440, 834, 390]) {
    const { context, page } = await openView(width, '/?render=first-visit', width === 390 ? 844 : 900);
    try {
      await page.getByRole('button', { name: 'I understand and agree' }).click();
      const banner = page.locator('[data-cookie-banner]');
      await banner.waitFor();
      const sizes = await banner.evaluate((el) => ({
        lineHeight: getComputedStyle(el.querySelector('p')).lineHeight,
        buttons: [...el.querySelectorAll('button')].map((button) => {
          const box = button.getBoundingClientRect();
          return { text: button.textContent.trim(), width: box.width, height: box.height };
        }),
      }));
      assert.equal(sizes.lineHeight, '18px');
      assert.deepEqual(sizes.buttons.map((button) => button.height), width > 834 ? [28, 28] : [44, 44]);
      await banner.getByRole('button', { name: 'Decline' }).click();
      assert.equal(await banner.count(), 0);
      console.log(JSON.stringify({ view: 'cookie notice', width, sizes }));
    } finally {
      await context.close();
    }
  }
}

async function selectPillSizesAndChevron() {
  const profile = '/profile/0x4444444444444444444444444444444444444444';
  for (const [width, path] of [[1440, '/'], [1024, '/'], [1440, '/holder-rewards'], [390, '/holder-rewards'], [1440, profile], [390, profile]]) {
    const { context, page } = await openView(width, path, width === 390 ? 844 : 900);
    try {
      if (path === profile) {
        await page.getByRole('tab', { name: 'Activity', exact: true }).click();
        await page.getByRole('combobox', { name: 'Filter by action' }).waitFor();
      }
      const selects = page.locator('select.sl-native-select:visible');
      assert.ok(await selects.count() > 0);
      if (path === profile) assert.equal(await page.getByRole('combobox', { name: 'Filter by action' }).count(), 1);
      for (const select of await selects.all()) {
        const sizes = await select.evaluate((el) => {
          const pill = el.parentElement;
          const box = pill.getBoundingClientRect();
          const picker = getComputedStyle(el, '::picker-icon');
          const label = pill.querySelector('label')?.getBoundingClientRect();
          const value = el.getBoundingClientRect();
          return {
            name: el.name, width: box.width, height: box.height, selectHeight: value.height,
            content: picker.content, mask: picker.maskImage, markerDisplay: picker.display,
            compact: el.classList.contains('appearance-none'),
            labelMiddle: label ? label.y + label.height / 2 : null,
            valueMiddle: value.y + value.height / 2,
            svgCount: pill.querySelectorAll('svg').length,
          };
        });
        assert.equal(sizes.content, '\"\"');
        assert.ok(sizes.mask.includes('data:image/svg+xml'));
        assert.equal(sizes.svgCount, 0);
        assert.equal(sizes.markerDisplay, sizes.compact ? 'none' : 'block');
        assert.equal(sizes.height, width <= 834 ? 46 : sizes.name === 'Filter by action' ? 38 : 28);
        if (sizes.labelMiddle !== null) assert.ok(Math.abs(sizes.labelMiddle - sizes.valueMiddle) <= 1);
        if (path === '/' && sizes.name === 'Window') assert.equal(sizes.width, 69);
        console.log(JSON.stringify({ view: path === profile ? 'profile activity' : path, width, sizes }));
      }
    } finally {
      await context.close();
    }
  }
}

test(tickerDesktopAndTouchSizes);
test(tickerUnavailableKeepsItsRow);
test(cookieNoticeSizes);
test(selectPillSizesAndChevron);
