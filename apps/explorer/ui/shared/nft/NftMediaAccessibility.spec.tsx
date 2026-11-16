// @vitest-environment jsdom

import React from 'react';

import { base as tokenFixture } from 'mocks/tokens/tokenInstance';
import { render } from 'ui/shared/layout/testWrapper';
import { expect, test } from 'vitest';
import { fireEvent, waitFor } from 'vitest/lib';

import NftMedia from './NftMedia';

test('nftPreviewKeepsFocusThroughDismissal', async function nftPreviewKeepsFocusThroughDismissal() {
  const data = { ...tokenFixture, animation_url: null, image_url: null, metadata: null, thumbnails: null };
  const view = render(<NftMedia data={ data } withFullscreen/>);
  const trigger = view.getByRole('button', { name: 'Open NFT media preview' });
  expect(trigger.tagName).toBe('BUTTON');
  expect(trigger.getAttribute('aria-haspopup')).toBe('dialog');
  trigger.focus();
  expect(document.activeElement).toBe(trigger);
  fireEvent.click(trigger);
  await waitFor(() => expect(view.getByRole('dialog', { name: 'NFT media preview' })).toBeDefined());
  fireEvent.click(view.getByRole('button', { name: /close/i }));
  await waitFor(() => expect(view.queryByRole('dialog')).toBeNull());
  await waitFor(() => expect(document.activeElement).toBe(trigger));
  view.rerender(<NftMedia data={ data }/>);
  expect(view.queryByRole('button', { name: 'Open NFT media preview' })).toBeNull();
});
