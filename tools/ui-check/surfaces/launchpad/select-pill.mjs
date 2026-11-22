import assert from 'node:assert/strict';
import { PNG } from 'pngjs';

export async function measureSelectPill(select, screenshot) {
  await select.scrollIntoViewIfNeeded();
  const geometry = await select.evaluate(element => {
    const pill = element.parentElement;
    const label = pill.querySelector('label');
    const box = pill.getBoundingClientRect();
    const clip = {
      x: Math.floor(box.x), y: Math.floor(box.y),
      width: Math.ceil(box.right) - Math.floor(box.x),
      height: Math.ceil(box.bottom) - Math.floor(box.y),
    };
    function textRegion(node, text) {
      const rect = node.getBoundingClientRect();
      const style = getComputedStyle(node);
      const canvas = document.createElement('canvas');
      const context = canvas.getContext('2d');
      context.font = `${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
      context.letterSpacing = style.letterSpacing;
      const left = rect.left + parseFloat(style.borderLeftWidth) + parseFloat(style.paddingLeft);
      return {
        left: left - clip.x,
        right: left - clip.x + context.measureText(text).width,
        color: style.color,
      };
    }
    return {
      clip,
      pillHeight: box.height,
      selectHeight: element.getBoundingClientRect().height,
      label: label ? textRegion(label, label.textContent) : null,
      value: textRegion(element, element.selectedOptions[0].textContent),
    };
  });
  const png = PNG.sync.read(await select.page().screenshot({ path: screenshot, clip: geometry.clip }));
  function inkMiddle(region) {
    const color = [...region.color.matchAll(/[\d.]+/g)].slice(0, 3).map(match => Number(match[0]));
    assert.equal(color.length, 3, 'Text uses an RGB color');
    const rows = [];
    for (let y = 0; y < png.height; y += 1) {
      for (let x = Math.ceil(region.left); x < Math.floor(region.right); x += 1) {
        const index = (y * png.width + x) * 4;
        if (color.every((channel, i) => Math.abs(png.data[index + i] - channel) <= 32)) {
          rows.push(y);
          break;
        }
      }
    }
    assert.ok(rows.length > 0, 'The clip contains text ink');
    return (rows[0] + rows.at(-1) + 1) / 2;
  }
  const valueMiddle = inkMiddle(geometry.value);
  const labelMiddle = geometry.label ? inkMiddle(geometry.label) : null;
  return {
    pillHeight: geometry.pillHeight,
    selectHeight: geometry.selectHeight,
    labelMiddle,
    valueMiddle,
    difference: labelMiddle === null ? null : Math.abs(labelMiddle - valueMiddle),
  };
}

export async function assertSelectPillSize(select, screenshot, width, size = 'sm') {
  const measurement = await measureSelectPill(select, screenshot);
  if (width <= 834) {
    assert.ok(measurement.selectHeight >= 44, 'The select keeps its touch height');
  } else if (size === 'sm') {
    assert.equal(measurement.pillHeight, 28, 'The small pill keeps its desktop height');
  }
  return measurement;
}

export async function assertRewardsSortAlignment(select, screenshot, width) {
  assert.equal(await select.getAttribute('name'), 'Sort by');
  const measurement = await assertSelectPillSize(select, screenshot, width);
  assert.notEqual(measurement.labelMiddle, null, 'The sort label is visible');
  assert.ok(measurement.difference <= 1, 'Label and value ink centers differ by at most one pixel');
  return measurement;
}
