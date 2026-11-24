// @vitest-environment jsdom

import { ChakraProvider } from '@chakra-ui/react';
import { render, within } from '@testing-library/react';
import React from 'react';

import theme from 'toolkit/theme/theme';
import { expect, test } from 'vitest';

import { Button } from './button';
import { Link } from './link';

test('button navigation has one native link', function buttonNavigationHasOneNativeLink() {
  const linkRef = React.createRef<HTMLAnchorElement>();
  const { container, unmount } = render(
    <ChakraProvider value={ theme }>
      <Button asChild display="inline-flex" size="md" variant="outline">
        <Link ref={ linkRef } href="/" variant="plain">Back to home</Link>
      </Button>
    </ChakraProvider>,
  );
  try {
    const link = within(container).getByRole('link', { name: 'Back to home' });
    expect(link.tagName).toBe('A');
    expect(link.getAttribute('href')).toBe('/');
    expect(link.tabIndex).toBe(0);
    expect(linkRef.current).toBe(link);
    expect(link.classList.contains('chakra-button')).toBe(true);
    expect(container.querySelectorAll('a[href], button, [tabindex]')).toHaveLength(1);
    expect(link.querySelector('a, button, input, [tabindex]')).toBeNull();
  } finally {
    unmount();
  }
});
