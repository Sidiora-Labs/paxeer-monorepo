// @vitest-environment jsdom

import { ChakraProvider } from '@chakra-ui/react';
import { render, within } from '@testing-library/react';
import React from 'react';

import theme from 'toolkit/theme/theme';
import { expect, test } from 'vitest';

import NotFound from './NotFound';

function notFoundShowsOneHomeAction() {
  const { container, unmount } = render(
    <ChakraProvider value={ theme }>
      <NotFound/>
    </ChakraProvider>,
  );
  try {
    expect(within(container).getByRole('heading', { level: 1, name: 'Page not found' })).toBeTruthy();
    expect(within(container).getByText(
      'This page is no longer explorable! If you are lost, use the search bar to find what you are looking for.',
    )).toBeTruthy();
    const link = within(container).getByRole('link', { name: 'Back to home' });
    expect(link.tagName).toBe('A');
    expect(link.getAttribute('href')).toBe('/');
    expect(link.tabIndex).toBe(0);
    expect(container.querySelectorAll('a[href], button, [tabindex]')).toHaveLength(1);
    expect(link.querySelector('a, button, [tabindex]')).toBeNull();
    expect(container.querySelector('svg use')?.getAttribute('href')).toMatch(/#error-pages\/404$/);
  } finally {
    unmount();
  }
}

test(notFoundShowsOneHomeAction);
