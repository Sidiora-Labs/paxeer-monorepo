// @vitest-environment jsdom

import React from 'react';

import { Link } from 'toolkit/chakra/link';
import { Provider } from 'toolkit/chakra/provider';
import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { cleanup, render } from 'vitest/lib';

import * as DetailedInfo from './DetailedInfo';

// jsdom implements no CSS media queries, and the Chakra provider reads window.matchMedia on mount
const createMediaQueryList = (query: string): MediaQueryList => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: () => undefined,
  removeListener: () => undefined,
  addEventListener: () => undefined,
  removeEventListener: () => undefined,
  dispatchEvent: () => false,
});

beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', { writable: true, value: createMediaQueryList });
});

// the suite runs without vitest globals, so the testing library cannot register its own teardown
afterEach(cleanup);

const declarationsOf = (element: Element): string => {
  const css = Array.from(document.styleSheets)
    .flatMap((sheet) => Array.from(sheet.cssRules).map((rule) => rule.cssText))
    .join('').replace(/\s+/g, '');

  return Array.from(element.classList)
    .flatMap((name) => Array.from(css.matchAll(new RegExp(`\\.${ name }\\{([^}]*)\\}`, 'g'))).map((match) => match[1]))
    .join(';');
};

describe('DetailedInfo', () => {
  it('marks a value as the one that stacks under its label and wraps on a narrow screen', () => {
    const { container } = render(
      <Provider>
        <DetailedInfo.Container>
          <DetailedInfo.ItemLabel>Timestamp</DetailedInfo.ItemLabel>
          <DetailedInfo.ItemValue>Sep 27 2026 19:16:19 (+02:00 UTC)</DetailedInfo.ItemValue>
        </DetailedInfo.Container>
      </Provider>,
    );

    const value = container.querySelector('[data-detailed-info-value]');

    expect(value?.getAttribute('data-stack-below')).toBe('lg');
    expect(value?.textContent).toBe('Sep 27 2026 19:16:19 (+02:00 UTC)');
  });

  it('marks the grid so a page can style the detail block as a whole', () => {
    const { container } = render(
      <Provider>
        <DetailedInfo.Container>
          <DetailedInfo.ItemLabel>Transaction hash</DetailedInfo.ItemLabel>
          <DetailedInfo.ItemValue>0x01</DetailedInfo.ItemValue>
        </DetailedInfo.Container>
      </Provider>,
    );

    expect(container.querySelector('[data-detailed-info]')).not.toBeNull();
  });

  it('pairs every label with the value that follows it', () => {
    const { container } = render(
      <Provider>
        <DetailedInfo.Container>
          <DetailedInfo.ItemLabel>Transaction hash</DetailedInfo.ItemLabel>
          <DetailedInfo.ItemValue>0x01</DetailedInfo.ItemValue>
          <DetailedInfo.ItemLabel>Status</DetailedInfo.ItemLabel>
          <DetailedInfo.ItemValue>Success</DetailedInfo.ItemValue>
        </DetailedInfo.Container>
      </Provider>,
    );

    const labels = Array.from(container.querySelectorAll('[data-detailed-info-label]'));
    const values = Array.from(container.querySelectorAll('[data-detailed-info-value]'));

    expect(labels.map((label) => label.textContent)).toEqual([ 'Transaction hash', 'Status' ]);
    expect(values.map((value) => value.textContent)).toEqual([ '0x01', 'Success' ]);
  });

  it('explains a label through a hint when one is given', () => {
    const { container } = render(
      <Provider>
        <DetailedInfo.Container>
          <DetailedInfo.ItemLabel hint="The unique identifier of this transaction.">Transaction hash</DetailedInfo.ItemLabel>
          <DetailedInfo.ItemValue>0x01</DetailedInfo.ItemValue>
        </DetailedInfo.Container>
      </Provider>,
    );

    expect(container.querySelector('[data-detailed-info-label] [aria-label="hint"]')).not.toBeNull();
  });

  it('keeps the row height the detail pages share', () => {
    expect(DetailedInfo.ITEM_VALUE_LINE_HEIGHT).toEqual({ base: 'var(--chakra-spacing-7\\.5)', lg: 'var(--chakra-spacing-8)' });
  });

  it.each([ false, true ])('gives the hint only its icon size in the label flow when loading is %s', function hintUsesIconBox(isLoading) {
    const { container } = render(
      <Provider>
        <DetailedInfo.Container>
          <DetailedInfo.ItemLabel hint="Transaction identifier" isLoading={ isLoading }>Transaction hash</DetailedInfo.ItemLabel>
          <DetailedInfo.ItemValue>0x01</DetailedInfo.ItemValue>
        </DetailedInfo.Container>
      </Provider>,
    );

    const hint = container.querySelector('button[aria-label="hint"]') as HTMLButtonElement;
    const box = hint.parentElement as HTMLElement;
    const icon = hint.querySelector('svg') as SVGElement;

    expect(box.tagName).toBe('SPAN');
    expect(declarationsOf(box)).toContain('position:relative');
    expect(declarationsOf(box)).toContain('width:var(--chakra-sizes-5)');
    expect(declarationsOf(box)).toContain('height:var(--chakra-sizes-5)');
    expect(declarationsOf(box)).toContain('flex-shrink:0');
    expect(declarationsOf(icon)).toContain('width:var(--chakra-sizes-5)');
    expect(declarationsOf(icon)).toContain('height:var(--chakra-sizes-5)');
    expect(declarationsOf(hint)).toContain('position:absolute');
    expect(declarationsOf(hint)).toContain('top:50%');
    expect(declarationsOf(hint)).toContain('left:50%');
    expect(declarationsOf(hint)).toContain('transform:translate(-50%,-50%)');
    expect(declarationsOf(box)).toContain('margin-top:calc((1lh-var(--chakra-spacing-5))/2)');
    expect(box.previousElementSibling?.textContent).toBe('Transaction hash');
    expect(hint.hasAttribute('data-loading-skeleton')).toBe(isLoading);
    expect(hint.disabled).toBe(isLoading);
  });

  it('reserves the theme touch target around the hint without covering adjacent controls', function hintReservesTouchTarget() {
    const { container } = render(
      <Provider>
        <DetailedInfo.Container>
          <DetailedInfo.ItemLabel hint="Transaction identifier"><Link href="/tx/0x01">Transaction hash</Link></DetailedInfo.ItemLabel>
          <DetailedInfo.ItemValue><Link href="/tx/0x01">0x01</Link></DetailedInfo.ItemValue>
        </DetailedInfo.Container>
      </Provider>,
    );

    const hint = container.querySelector('button[aria-label="hint"]') as HTMLButtonElement;
    const box = hint.parentElement as HTMLElement;

    expect(window.getComputedStyle(hint).width).toBe('var(--chakra-sizes-5)');
    expect(window.getComputedStyle(hint).height).toBe('var(--chakra-sizes-5)');
    expect(declarationsOf(box)).toContain('margin-inline:calc((var(--chakra-sizes-touch-target)-var(--chakra-spacing-5))/2)');
    expect(container.querySelectorAll('a')).toHaveLength(2);
  });

  it.each([ false, true ])('keeps label and value dimensions with and without a hint when loading is %s', function labelsShareDimensions(isLoading) {
    const { container } = render(
      <Provider>
        <DetailedInfo.Container>
          <DetailedInfo.ItemLabel hint="Transaction identifier" isLoading={ isLoading }>Transaction hash</DetailedInfo.ItemLabel>
          <DetailedInfo.ItemValue>0x01</DetailedInfo.ItemValue>
          <DetailedInfo.ItemLabel isLoading={ isLoading }>Transaction hash</DetailedInfo.ItemLabel>
          <DetailedInfo.ItemValue>0x02</DetailedInfo.ItemValue>
        </DetailedInfo.Container>
      </Provider>,
    );

    const labels = Array.from(container.querySelectorAll('[data-detailed-info-label]'));
    const values = Array.from(container.querySelectorAll('[data-detailed-info-value]'));
    const content = labels.map((label) => label.firstElementChild?.firstElementChild as HTMLElement);
    const minimumHeights = (element: Element) => declarationsOf(element).match(/min-height:[^;]+/g);

    expect(labels).toHaveLength(2);
    expect(values).toHaveLength(2);
    expect(minimumHeights(labels[0])).toEqual([
      'min-height:var(--chakra-spacing-7\\.5)',
      'min-height:var(--chakra-spacing-8)',
    ]);
    expect(minimumHeights(labels[1])).toEqual(minimumHeights(labels[0]));
    expect(values.map(minimumHeights)).toEqual([ minimumHeights(labels[0]), minimumHeights(labels[0]) ]);
    expect(declarationsOf(content[0])).toBe(declarationsOf(content[1]));
    expect(declarationsOf(content[0])).toContain('padding-block:var(--chakra-spacing-1\\.25)');
    expect(declarationsOf(content[0])).toContain('padding-block:var(--chakra-spacing-1)');
    expect(declarationsOf(content[0])).toContain('align-items:flex-start');
    for (const label of labels) {
      expect(label.querySelector('.chakra-skeleton')?.hasAttribute('data-loading')).toBe(isLoading);
    }
  });

  it('spans the divider across both columns', () => {
    const { container } = render(
      <Provider>
        <DetailedInfo.Container>
          <DetailedInfo.ItemLabel>Status</DetailedInfo.ItemLabel>
          <DetailedInfo.ItemValue>Success</DetailedInfo.ItemValue>
          <DetailedInfo.ItemDivider data-divider/>
        </DetailedInfo.Container>
      </Provider>,
    );

    expect(container.querySelector('[data-divider]')).not.toBeNull();
  });
});
