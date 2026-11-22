import assert from 'node:assert/strict';
import { chromium } from 'playwright';
import { origin } from './environment.mjs';

async function measureCreationLayout(page) {
  await page.goto(`${origin}/create`, { waitUntil: 'domcontentloaded' });
  await page.locator('[data-shell-ready]').waitFor();
  await page.locator('#create-name').waitFor();
  await page.getByText("Custom rules can't load right now, so only Standard is available.").waitFor();
  await page.evaluate(() => document.fonts.ready);

  const layout = await page.evaluate(() => {
    const heading = document.querySelector('h1');
    const firstHeading = document.querySelector('#create-details-title');
    const link = document.querySelector('a[href="/create/chain"]');
    const fee = [...document.querySelectorAll('dt')]
      .find(element => element.textContent === 'Creation fee').nextElementSibling;
    function measure(element) {
      const box = element.getBoundingClientRect();
      const style = getComputedStyle(element);
      return {
        top: box.top,
        height: box.height,
        fontSize: style.fontSize,
        lineHeight: style.lineHeight,
        fontWeight: style.fontWeight,
      };
    }
    return {
      heading: measure(heading),
      row: measure(heading.parentElement),
      link: {
        ...measure(link),
        text: link.textContent,
        href: link.getAttribute('href'),
      },
      firstHeading: measure(firstHeading),
      firstHeadingOffset: firstHeading.getBoundingClientRect().top - heading.getBoundingClientRect().top,
      fee: measure(fee),
      scrollbarGutter: getComputedStyle(document.documentElement).scrollbarGutter,
    };
  });

  await page.keyboard.press('Tab');
  await page.locator('a[href="/create/chain"]').focus();
  layout.focus = await page.locator('a[href="/create/chain"]').evaluate(element => {
    const style = getComputedStyle(element);
    return {
      visible: element.matches(':focus-visible'),
      outlineStyle: style.outlineStyle,
      outlineWidth: parseFloat(style.outlineWidth),
    };
  });

  await page.goto(`${origin}/create/chain`, { waitUntil: 'domcontentloaded' });
  await page.locator('#chain-name').waitFor();
  await page.evaluate(() => document.fonts.ready);
  layout.chainHeading = await page.locator('h1').evaluate(element => {
    const style = getComputedStyle(element);
    return {
      height: element.getBoundingClientRect().height,
      fontSize: style.fontSize,
      lineHeight: style.lineHeight,
      fontWeight: style.fontWeight,
    };
  });
  return layout;
}

const browser = await chromium.launch();
try {
  for (const [width, height] of [[1440, 900], [834, 900], [390, 844]]) {
    const context = await browser.newContext({
      viewport: { width, height },
      colorScheme: 'dark',
      serviceWorkers: 'block',
    });
    try {
      const page = await context.newPage();
      const layout = await measureCreationLayout(page);
      assert.equal(layout.row.height, width > 834 ? 24 : 44);
      assert.equal(layout.link.height, width > 834 ? 24 : 44);
      assert.equal(layout.link.text, 'Launch a chain and token');
      assert.equal(layout.link.href, '/create/chain');
      assert.equal(layout.heading.height, 24);
      assert.equal(layout.firstHeading.height, 20);
      assert.equal(layout.firstHeadingOffset, width > 834 ? 40 : 50);
      assert.equal(layout.fee.fontSize, '16px');
      assert.equal(layout.fee.lineHeight, '24px');
      assert.equal(layout.fee.fontWeight, '700');
      assert.equal(layout.scrollbarGutter, 'stable');
      assert.equal(layout.focus.visible, true);
      assert.equal(layout.focus.outlineStyle, 'solid');
      assert.equal(layout.focus.outlineWidth, 2);
      assert.equal(layout.chainHeading.height, 24);
      assert.equal(layout.chainHeading.fontSize, '16px');
      assert.equal(layout.chainHeading.lineHeight, '24px');
      assert.equal(layout.chainHeading.fontWeight, '600');
      console.log(JSON.stringify({ width, height, ...layout }));
    } finally {
      await context.close();
    }
  }
} finally {
  await browser.close();
}
