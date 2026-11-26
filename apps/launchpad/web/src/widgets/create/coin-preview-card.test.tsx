import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { chromium } from 'playwright';
import { expect, test } from 'vitest';
import { CoinPreviewCard } from './coin-preview-card';
import { CreateFormFields, DESCRIPTION_MAX_CHARS } from './create-form-fields';
import type { CreationDraftV2 } from './creation-storage';

async function coinPreviewDescriptionWrapsFullyAtDoubleTextSize() {
  const descriptions = [
    'A coin for science.',
    'A community coin for science and discovery. '.repeat(40),
    'S'.repeat(DESCRIPTION_MAX_CHARS),
  ];
  const draft: CreationDraftV2 = {
    draftId: 'preview', version: 2, creator: '0x1234567890abcdef1234567890abcdef12345678', chainId: 125,
    name: 'Science Inu', symbol: 'SCINU', description: descriptions[0], tags: [], socialLinks: [],
    uploadRefs: [], firstBuyRaw: '0', opticalSelection: 'standard', feeStrategy: 0, updatedAt: Date.now(),
  };
  const previews = descriptions.map(description => renderToStaticMarkup(
    <div data-create-text>
      <aside className="min-w-0 shrink-0 lg:w-56 2xl:w-60">
        <CoinPreviewCard
          name="Science Inu"
          symbol="SCINU"
          description={description}
          image={null}
          strategy={3}
          socialLinks={['https://example.org']}
        />
      </aside>
      <CreateFormFields
        draft={{ ...draft, description }} errors={{}} locked={false}
        onEdit={() => {}} registeredOpticals={[]} registry={{ state: 'ready' }} opticalReason={() => null}
      />
    </div>,
  ));
  const require = createRequire(import.meta.url);
  const nodeEntry = createRequire(require.resolve('@tailwindcss/postcss')).resolve('@tailwindcss/node');
  const { compile } = await import(pathToFileURL(nodeEntry).href) as {
    compile(css: string, options: { base: string; onDependency(path: string): void }):
      Promise<{ build(candidates: string[]): string }>;
  };
  const appDirectory = join(process.cwd(), 'src/app');
  const compiler = await compile(readFileSync(join(appDirectory, 'globals.css'), 'utf8'), {
    base: appDirectory,
    onDependency() {},
  });
  const candidates = [...previews.join('').matchAll(/class="([^"]+)"/g)]
    .flatMap(match => match[1].replaceAll('&#x27;', "'").replaceAll('&amp;', '&').split(/\s+/));
  const css = compiler.build(candidates) + readFileSync(join(process.cwd(), 'src/widgets/create/create-text.css'), 'utf8');
  const browser = await chromium.launch();
  try {
    const page = await browser.newPage();
    for (const width of [390, 1440]) {
      await page.setViewportSize({ width, height: 900 });
      for (const scheme of ['light', 'dark'] as const) {
        for (const [index, preview] of previews.entries()) {
          await page.setContent(`<!doctype html><html data-color-mode="${scheme}"><head><style>${css}</style></head><body>${preview}</body></html>`);
          const heights: number[] = [];
          for (const scale of [1, 2]) {
            if (scale === 2) {
              await page.evaluate(() => {
                const sizes = [...document.querySelectorAll<HTMLElement>('body, body *')]
                  .map(element => [element, getComputedStyle(element).fontSize] as const);
                for (const [element, size] of sizes) {
                  element.style.setProperty('font-size', `${parseFloat(size) * 2}px`, 'important');
                }
              });
            }
            const layout = await page.evaluate(() => {
              const description = document.querySelector<HTMLElement>('article .line-clamp-2')!;
              const card = description.closest('article')!;
              const text = document.createRange();
              text.selectNodeContents(description);
              const bounds = description.getBoundingClientRect();
              const cardBounds = card.getBoundingClientRect();
              const following = description.nextElementSibling!.getBoundingClientRect();
              const inputStyle = getComputedStyle(document.querySelector('textarea')!);
              return {
                text: description.textContent,
                fontSize: parseFloat(getComputedStyle(description).fontSize),
                lineHeight: parseFloat(getComputedStyle(description).lineHeight),
                inputFontSize: parseFloat(inputStyle.fontSize),
                inputLineHeight: parseFloat(inputStyle.lineHeight),
                height: cardBounds.height,
                visibleHeight: description.clientHeight,
                contentHeight: description.scrollHeight,
                visibleWidth: description.clientWidth,
                contentWidth: description.scrollWidth,
                textInside: [...text.getClientRects()].every(rect =>
                  rect.top >= bounds.top && rect.bottom <= bounds.bottom &&
                  rect.left >= bounds.left && rect.right <= bounds.right),
                descriptionInside: bounds.top >= cardBounds.top && bounds.bottom <= cardBounds.bottom,
                followingBelow: following.top >= bounds.bottom,
                pageWidth: document.documentElement.getBoundingClientRect().width,
                contentPageWidth: document.documentElement.scrollWidth,
              };
            });
            expect(layout.text).toBe(descriptions[index].trim());
            expect(layout.fontSize).toBe(13 * scale);
            expect(layout.lineHeight).toBe(18 * scale);
            expect(layout.inputFontSize).toBe(14 * scale);
            expect(layout.inputLineHeight).toBe(20 * scale);
            expect(layout.contentHeight).toBe(layout.visibleHeight);
            expect(layout.contentWidth).toBe(layout.visibleWidth);
            expect(layout.textInside).toBe(true);
            expect(layout.descriptionInside).toBe(true);
            expect(layout.followingBelow).toBe(true);
            expect(layout.contentPageWidth).toBe(layout.pageWidth);
            heights.push(layout.height);
          }
          expect(heights[1]).toBeGreaterThan(heights[0]);
        }
      }
    }
  } finally {
    await browser.close();
  }
}

test(coinPreviewDescriptionWrapsFullyAtDoubleTextSize.name, coinPreviewDescriptionWrapsFullyAtDoubleTextSize, 60_000);
