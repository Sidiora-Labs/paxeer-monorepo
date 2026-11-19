// @vitest-environment jsdom

import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import { spacing } from 'toolkit/theme/foundations/spacing';
import { marketStyleRules } from 'ui/home/marketData.test-utils';
import { render, Wrapper } from 'ui/shared/layout/testWrapper';
import { describe, it, expect, vi } from 'vitest';
import { fireEvent } from 'vitest/lib';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED: 'false',
    NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: '',
  };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import SearchBarInput from './SearchBarInput';

describe('search bar input', () => {
  it('hands every typed term to its owner', function handsTypedTermsToOwner() {
    const onChange = vi.fn();
    const { container } = render(<SearchBarInput onChange={ onChange } value=""/>);

    fireEvent.change(container.querySelector('input') as HTMLInputElement, { target: { value: '0xb64a' } });

    expect(onChange).toHaveBeenCalledWith('0xb64a');
  });

  it('submits the form it sits in', function submitsSearchForm() {
    const onSubmit = vi.fn((event: React.FormEvent<HTMLFormElement>) => event.preventDefault());
    const { container } = render(<SearchBarInput onSubmit={ onSubmit } value="0xb64a"/>);

    fireEvent.submit(container.querySelector('form') as HTMLFormElement);

    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it('draws the hero and the header field on the thin product border', function drawsProductBorder() {
    const { container: hero } = render(<SearchBarInput isHeroBanner value=""/>);
    const { container: header } = render(<SearchBarInput value=""/>);

    [ hero, header ].forEach((container) => {
      const input = container.querySelector('input') as HTMLInputElement;

      expect(input.placeholder).toBe(container === hero ? 'Search or ask: a transaction, address, block or token' : 'Search or ask…');
      expect(window.getComputedStyle(input).borderWidth).toBe('var(--chakra-border-widths-1)');
      expect(input.name).toBe('q');
      expect(input.type).toBe('search');
      expect(window.getComputedStyle(input).borderWidth).not.toBe('0px');
    });
  });
});

it('places text after the search mark before measurement in header and hero fields', function searchTextClearsTheMarkBeforeMeasurement() {
  for (const isHeroBanner of [ false, true ]) {
    const container = document.createElement('div');
    container.innerHTML = renderToStaticMarkup(<Wrapper><SearchBarInput isHeroBanner={ isHeroBanner } value="PAX"/></Wrapper>);
    const input = container.querySelector('input')!;
    const { container: mounted } = render(<SearchBarInput isHeroBanner={ isHeroBanner } value="PAX"/>);
    const styles = marketStyleRules(input);

    expect(input.className).toBe(mounted.querySelector('input')?.className);

    expect(input.value).toBe('PAX');
    expect(input.placeholder).toBe(isHeroBanner ? 'Search or ask: a transaction, address, block or token' : 'Search or ask…');
    expect(styles).toContain('padding-inline-start:var(--chakra-spacing-10)');
    expect(styles).toContain('padding-inline-end:calc(var(--chakra-spacing-16)+var(--chakra-spacing-12))');
    const startPadding = parseFloat(spacing['10'].value);
    const markWidthWithMargins = parseFloat(spacing['5'].value) + 2 * parseFloat(spacing['2'].value);
    expect(startPadding).toBeGreaterThan(markWidthWithMargins);
  }
});
