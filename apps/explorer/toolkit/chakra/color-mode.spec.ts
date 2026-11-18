import { createHash } from 'node:crypto';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import { app } from 'nextjs/csp/policies/app';

import { expect, test } from 'vitest';

import { ColorModeProvider } from './color-mode';

test('colorModeInitializationScriptsHaveCspHashes', () => {
  const sources = app()['script-src'];
  for (const defaultTheme of [ 'system', 'light', 'dark' ]) {
    const markup = renderToStaticMarkup(createElement(ColorModeProvider, { defaultTheme }));
    const script = /<script[^>]*>([\s\S]*?)<\/script>/.exec(markup)?.[1];
    expect(script).toBeDefined();
    const digest = createHash('sha256').update(script!).digest('base64');
    expect(sources).toContain(`'sha256-${ digest }'`);
  }
  expect(sources).not.toContain('\'unsafe-inline\'');
  expect(sources).not.toContain('*');
});
