import assert from 'node:assert/strict';

export async function measureHolderRewardsTypography(page) {
  await page.evaluate(() => document.fonts.ready);
  return page.evaluate(() => {
    function measure(element) {
      const style = getComputedStyle(element);
      const box = element.getBoundingClientRect();
      return {
        text: element.textContent,
        fontSize: parseFloat(style.fontSize),
        lineHeight: parseFloat(style.lineHeight),
        fontWeight: Number(style.fontWeight),
        minHeight: parseFloat(style.minHeight),
        left: box.left,
        right: box.right,
        top: box.top,
        bottom: box.bottom,
        height: box.height,
      };
    }

    return {
      width: window.innerWidth,
      heading: measure(document.querySelector('#main-content h1')),
      tiles: [...document.querySelectorAll('section[aria-labelledby="holder-rewards-stats-title"] dl > div')].map(tile => {
        const total = tile.querySelector('dd[data-numeric]');
        return {
          box: measure(tile),
          label: measure(tile.querySelector('dt')),
          total: measure(total),
          loading: total.getAttribute('aria-busy') === 'true',
          content: [...total.children].map(measure),
          captions: [...tile.querySelectorAll('dd:not([data-numeric])')].map(measure),
        };
      }),
    };
  });
}

export function assertHolderRewardsTypography(metrics) {
  const small = metrics.width < 640;
  const totalHeight = small ? 39.984375 : 48;
  assert.equal(metrics.heading.fontWeight, 500);
  assert.equal(metrics.heading.fontSize, small ? 28 : 40);
  assert.equal(metrics.heading.lineHeight, small ? 34.0001 : 46);
  assert.equal(metrics.tiles.length, 3);
  for (const tile of metrics.tiles) {
    assert.equal(tile.total.fontSize, small ? 36 : 44);
    assert.equal(tile.total.lineHeight, small ? 40 : 48);
    assert.equal(tile.total.fontWeight, 700);
    assert.equal(tile.box.minHeight, small ? 94 : 126);
    assert.ok(tile.box.height >= tile.box.minHeight);
    assert.ok(tile.total.height >= totalHeight);
    for (const content of tile.content) {
      assert.ok(content.left >= tile.total.left);
      assert.ok(content.right <= tile.total.right);
      assert.ok(content.top >= tile.total.top);
      assert.ok(content.bottom <= tile.total.bottom);
    }
    if (tile.loading) {
      assert.equal(tile.total.height, totalHeight);
      assert.equal(tile.content.length, 1);
      assert.equal(tile.content[0].height, totalHeight);
    }
    const unit = tile.content.find(content => content.text === 'USDL');
    if (unit) {
      assert.equal(unit.fontSize, 16);
      assert.equal(unit.lineHeight, 24);
      assert.equal(unit.fontWeight, 600);
      assert.equal(tile.total.height, totalHeight);
    }
  }
}
