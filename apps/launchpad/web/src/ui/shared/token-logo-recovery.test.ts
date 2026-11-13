// @vitest-environment jsdom
import { act, createElement } from 'react';
import { createRoot } from 'react-dom/client';
import { expect, test } from 'vitest';
import { metadataApiUrl } from '@/core/sdk-config';
import TokenLogo from './token-logo';
import './token-image.test';

function clearedTokenLogo() {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  const addressA = '0x1111111111111111111111111111111111111111';
  const addressB = '0x2222222222222222222222222222222222222222';
  const render = (tokenAddress: string) => act(() => root.render(createElement(TokenLogo, {
    tokenAddress,
    symbol: 'TOKEN',
    size: 40,
    className: 'test-logo',
  })));
  const image = () => {
    const element = container.querySelector('img');
    if (!element) throw new Error('TokenLogo did not render an image');
    return element;
  };

  try {
    render(addressA);
    expect(image().getAttribute('src')).toBe(metadataApiUrl(`/logo/${addressA}.png`));
    render('');
    expect(image().getAttribute('src')).toBe('/shadcn.png');
    render(addressB);
    expect(image().getAttribute('src')).toBe(metadataApiUrl(`/logo/${addressB}.png`));
    expect(image().alt).toBe('TOKEN');
    expect(image().width).toBe(40);
    expect(image().height).toBe(40);
    expect(image().classList.contains('test-logo')).toBe(true);

    act(() => { image().dispatchEvent(new Event('error')); });
    expect(image().getAttribute('src')).toBe('/shadcn.png');
    render(addressB);
    expect(image().getAttribute('src')).toBe('/shadcn.png');
    render('');
    expect(image().getAttribute('src')).toBe('/shadcn.png');
    render(addressB);
    expect(image().getAttribute('src')).toBe(metadataApiUrl(`/logo/${addressB}.png`));
    render(addressA);
    expect(image().getAttribute('src')).toBe(metadataApiUrl(`/logo/${addressA}.png`));
  } finally {
    act(() => root.unmount());
    container.remove();
  }
}

test(clearedTokenLogo.name, clearedTokenLogo);
