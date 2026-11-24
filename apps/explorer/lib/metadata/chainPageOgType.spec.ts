import { expect, test } from 'vitest';

import getPageOgType from './getPageOgType';

test('chainDirectoryHasRootPageOgType', function chainDirectoryHasRootPageOgType() {
  expect(getPageOgType('/chains')).toBe('Root page');
});

test('chainDetailHasRegularPageOgType', function chainDetailHasRegularPageOgType() {
  expect(getPageOgType('/chains/[id]')).toBe('Regular page');
});
