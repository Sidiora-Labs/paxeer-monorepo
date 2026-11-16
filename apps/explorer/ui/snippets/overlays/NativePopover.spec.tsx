// @vitest-environment jsdom

import React from 'react';

import { Link } from 'toolkit/chakra/link';
import { render } from 'ui/shared/layout/testWrapper';
import { expect, test } from 'vitest';
import { fireEvent, screen, waitFor } from 'vitest/lib';

import NativePopover from './NativePopover';

function PopoverExample() {
  const anchorRef = React.useRef<HTMLButtonElement>(null);
  const [ open, setOpen ] = React.useState(false);
  const toggle = React.useCallback(() => setOpen(value => !value), []);
  const changeOpen = React.useCallback((details: { open: boolean }) => setOpen(details.open), []);
  return (
    <>
      <button ref={ anchorRef } aria-expanded={ open } onClick={ toggle }>Explore</button>
      <NativePopover
        open={ open }
        onOpenChange={ changeOpen }
        anchorRef={ anchorRef }
        anchorName="--example-navigation"
        aria-label="Explore navigation"
      >
        <Link href="/blocks">Blocks</Link>
      </NativePopover>
      <button>Outside</button>
    </>
  );
}

async function popoverEscapeReturnsFocusToItsTrigger() {
  render(<PopoverExample/>);
  const trigger = screen.getByRole('button', { name: 'Explore' });
  trigger.focus();
  fireEvent.click(trigger);
  const link = await screen.findByRole('link', { name: 'Blocks' });
  link.focus();
  fireEvent.keyDown(link, { key: 'Escape' });
  await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Explore navigation' })).toBeNull());
  expect(trigger.getAttribute('aria-expanded')).toBe('false');
  expect(document.activeElement).toBe(trigger);
}

test(popoverEscapeReturnsFocusToItsTrigger, popoverEscapeReturnsFocusToItsTrigger);

async function popoverDismissesWhenThePointerLeavesItsContentAndTrigger() {
  render(<PopoverExample/>);
  const trigger = screen.getByRole('button', { name: 'Explore' });
  fireEvent.click(trigger);
  const panel = await screen.findByRole('dialog', { name: 'Explore navigation' });
  fireEvent.pointerDown(panel);
  fireEvent.pointerDown(trigger);
  expect(trigger.getAttribute('aria-expanded')).toBe('true');
  fireEvent.pointerDown(screen.getByRole('button', { name: 'Outside' }));
  await waitFor(() => expect(trigger.getAttribute('aria-expanded')).toBe('false'));
  expect(screen.queryByRole('dialog', { name: 'Explore navigation' })).toBeNull();
}

test(popoverDismissesWhenThePointerLeavesItsContentAndTrigger, popoverDismissesWhenThePointerLeavesItsContentAndTrigger);
