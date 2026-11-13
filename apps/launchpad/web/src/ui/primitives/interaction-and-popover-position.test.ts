// @vitest-environment jsdom
import { act, createElement, useRef, useState, type ReactElement } from 'react';
import { createPortal } from 'react-dom';
import { createRoot } from 'react-dom/client';
import { afterEach, beforeAll, expect, test } from 'vitest';
import { Avatar } from './avatar';
import { Button } from './button';
import { SegmentedControl, type SegmentOption } from './controls';
import { DisclosureRow } from './states';
import { useMounted, usePopoverPosition } from './use-popover';

beforeAll(() => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

const cleanups: Array<() => void> = [];

afterEach(() => {
  for (const cleanup of cleanups.splice(0).reverse()) cleanup();
});

function mount(node: ReactElement) {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  let mounted = true;
  function stop() {
    if (!mounted) return;
    act(() => root.unmount());
    container.remove();
    mounted = false;
  }
  cleanups.push(stop);
  act(() => root.render(node));
  return {
    container,
    render(next: ReactElement) { act(() => root.render(next)); },
    stop,
  };
}

function required<T extends Element>(container: ParentNode, selector: string): T {
  const element = container.querySelector<T>(selector);
  expect(element, selector).not.toBeNull();
  if (!element) throw new Error(`Missing ${selector}`);
  return element;
}

function avatar_recovery() {
  const failedUrl = 'data:image/png;base64,broken';
  const successfulUrl = 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/%3E';
  const view = mount(createElement(Avatar, { src: failedUrl, name: 'Ada Lovelace' }));
  act(() => required<HTMLImageElement>(view.container, 'img').dispatchEvent(new Event('error')));
  expect(view.container.querySelector('img')).toBeNull();
  expect(view.container.textContent).toBe('AL');

  view.render(createElement(Avatar, { src: successfulUrl, name: 'Ada Lovelace' }));
  const recovered = required<HTMLImageElement>(view.container, 'img');
  expect(recovered.getAttribute('src')).toBe(successfulUrl);
  act(() => recovered.dispatchEvent(new Event('load')));
  expect(view.container.querySelector('img')).toBe(recovered);
  expect(recovered.alt).toBe('Ada Lovelace');

  view.stop();
  const reopened = mount(createElement(Avatar, { src: successfulUrl, name: 'Ada Lovelace' }));
  expect(required<HTMLImageElement>(reopened.container, 'img').getAttribute('src')).toBe(successfulUrl);
}

function pointerActivate(element: HTMLElement) {
  act(() => {
    element.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true }));
    element.dispatchEvent(new MouseEvent('pointerup', { bubbles: true }));
    element.click();
  });
}

function keyboardActivate(element: HTMLElement, key: 'Enter' | ' ') {
  act(() => {
    element.focus();
    element.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
    element.dispatchEvent(new KeyboardEvent('keyup', { key, bubbles: true, cancelable: true }));
    // jsdom does not synthesize the browser's activation click from key events.
    element.click();
  });
}

function navigationConsumer() {
  const destinations: string[] = [];
  function consume(event: MouseEvent) {
    const anchor = (event.target as Element).closest('a[href]');
    if (!anchor) return;
    event.preventDefault();
    destinations.push(anchor.getAttribute('href')!);
  }
  document.addEventListener('click', consume, true);
  cleanups.push(() => document.removeEventListener('click', consume, true));
  return destinations;
}

function disabled_button_link() {
  const destinations = navigationConsumer();
  let activations = 0;
  const props = { href: '/explore', children: 'Explore', onClick: () => { activations += 1; } };
  const view = mount(createElement(Button, { ...props, disabled: true }));
  const disabled = required<HTMLButtonElement>(view.container, 'button:disabled');
  expect(disabled.type).toBe('button');
  expect(view.container.querySelector('[href]')).toBeNull();
  pointerActivate(disabled);
  keyboardActivate(disabled, 'Enter');
  keyboardActivate(disabled, ' ');
  expect(destinations).toEqual([]);
  expect(activations).toBe(0);
  expect(document.activeElement).not.toBe(disabled);

  view.render(createElement(Button, { ...props, disabled: false }));
  const enabled = required<HTMLAnchorElement>(view.container, 'a[href="/explore"]');
  pointerActivate(enabled);
  keyboardActivate(enabled, 'Enter');
  expect(destinations).toEqual(['/explore', '/explore']);

  view.render(createElement(Button, { ...props, disabled: true }));
  pointerActivate(required<HTMLButtonElement>(view.container, 'button:disabled'));
  expect(destinations).toHaveLength(2);
}

function disabled_disclosure_link() {
  const destinations = navigationConsumer();
  let toggles = 0;
  const props = { id: 'rules', href: '/docs/terms', label: 'Rules', onToggle: () => { toggles += 1; } };
  const view = mount(createElement(DisclosureRow, { ...props, disabled: true }));
  const disabled = required<HTMLButtonElement>(view.container, 'button:disabled');
  expect(disabled.type).toBe('button');
  expect(disabled.textContent).toBe('Rules');
  expect(view.container.querySelector('[href]')).toBeNull();
  pointerActivate(disabled);
  keyboardActivate(disabled, 'Enter');
  keyboardActivate(disabled, ' ');
  expect(destinations).toEqual([]);
  expect(toggles).toBe(0);
  expect(document.activeElement).not.toBe(disabled);

  view.render(createElement(DisclosureRow, { ...props, disabled: false }));
  const enabled = required<HTMLAnchorElement>(view.container, 'a[href="/docs/terms"]');
  pointerActivate(enabled);
  keyboardActivate(enabled, 'Enter');
  expect(destinations).toEqual(['/docs/terms', '/docs/terms']);
  expect(toggles).toBe(0);
}

function SegmentFixture({ options }: { options: SegmentOption<string>[] }) {
  const [value, setValue] = useState(options.find(option => !option.disabled)!.value);
  return createElement(SegmentedControl<string>, { options, value, onChange: setValue, label: 'Window' });
}

function segmented_end() {
  const cases: SegmentOption<string>[][] = [
    [{ value: 'one', label: 'One' }],
    [{ value: 'one', label: 'One' }, { value: 'two', label: 'Two' }],
    [{ value: 'one', label: 'One' }, { value: 'two', label: 'Two' }, { value: 'three', label: 'Three' }],
    [{ value: 'one', label: 'One' }, { value: 'two', label: 'Two', disabled: true }],
    [{ value: 'one', label: 'One' }, { value: 'two', label: 'Two' }, { value: 'three', label: 'Three', disabled: true }],
    [{ value: 'one', label: 'One', disabled: true }, { value: 'two', label: 'Two' }, { value: 'three', label: 'Three', disabled: true }, { value: 'four', label: 'Four' }, { value: 'five', label: 'Five', disabled: true }],
  ];
  for (const options of cases) {
    const view = mount(createElement(SegmentFixture, { options }));
    const enabled = Array.from(view.container.querySelectorAll<HTMLButtonElement>('button:not(:disabled)'));
    act(() => {
      enabled[0].focus();
      enabled[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'End', bubbles: true, cancelable: true }));
    });
    const last = enabled[enabled.length - 1];
    expect(document.activeElement).toBe(last);
    expect(last.getAttribute('aria-checked')).toBe('true');
    expect(last.tabIndex).toBe(0);
    expect(view.container.querySelectorAll('[aria-checked="true"]')).toHaveLength(1);
    view.stop();
  }
}

// Committed geometry inputs for the DOM peer: jsdom has no layout engine.
// The production hook, refs, portal, effects and consuming styles remain real.
class GeometryAnchor extends HTMLElement {
  getBoundingClientRect() {
    return new DOMRect(window.innerWidth - 36, window.innerHeight - 36, 32, 24);
  }
}

class GeometryLayer extends HTMLElement {
  get offsetWidth() { return Number(this.getAttribute('data-width')); }
  get offsetHeight() { return Number(this.getAttribute('data-height')); }
}

customElements.define('repair-popover-anchor', GeometryAnchor);
customElements.define('repair-popover-layer', GeometryLayer);

function PopoverFixture({ open, width = 240, height = 120, align = 'start' }: {
  open: boolean;
  width?: number;
  height?: number;
  align?: 'start' | 'end' | 'center';
}) {
  const anchorRef = useRef<HTMLElement | null>(null);
  const layerRef = useRef<HTMLElement | null>(null);
  const mounted = useMounted();
  const position = usePopoverPosition(anchorRef, layerRef, open, align);
  return createElement('div', null,
    createElement('repair-popover-anchor', { ref: anchorRef }),
    mounted && open && position ? createPortal(createElement('repair-popover-layer', {
      ref: layerRef,
      'data-width': width,
      'data-height': height,
      style: { position: 'fixed', top: position.top, left: position.left, minWidth: position.minWidth },
    }), document.body) : null,
  );
}

function expectPlacement(width: number, height: number) {
  const layer = required<HTMLElement>(document, 'repair-popover-layer');
  expect(layer.parentElement).toBe(document.body);
  expect(layer.style.left).toBe(`${window.innerWidth - width - 8}px`);
  expect(layer.style.top).toBe(`${window.innerHeight - 36 - 4 - height}px`);
  expect(layer.style.minWidth).toBe('32px');
  expect(parseFloat(layer.style.left) + width).toBeLessThanOrEqual(window.innerWidth - 8);
  expect(parseFloat(layer.style.top) + height).toBeLessThan(window.innerHeight - 36);
  return layer;
}

function initial_popover_position() {
  for (const align of ['start', 'end', 'center'] as const) {
    const view = mount(createElement(PopoverFixture, { open: false, align }));
    expect(document.querySelector('repair-popover-layer')).toBeNull();
    view.render(createElement(PopoverFixture, { open: true, align }));
    expectPlacement(240, 120);

    view.render(createElement(PopoverFixture, { open: false, align }));
    expect(document.querySelector('repair-popover-layer')).toBeNull();
    view.render(createElement(PopoverFixture, { open: true, width: 320, height: 180, align }));
    expectPlacement(320, 180);
    view.stop();
    expect(document.querySelector('repair-popover-layer')).toBeNull();
  }
}

function popover_layer_size_changes() {
  const view = mount(createElement(PopoverFixture, { open: true }));
  const initialLayer = expectPlacement(240, 120);
  view.render(createElement(PopoverFixture, { open: true, width: 320, height: 180 }));
  expect(expectPlacement(320, 180)).toBe(initialLayer);
  view.render(createElement(PopoverFixture, { open: true, width: 160, height: 80 }));
  expect(expectPlacement(160, 80)).toBe(initialLayer);
}

test(avatar_recovery.name, avatar_recovery);
test(disabled_button_link.name, disabled_button_link);
test(segmented_end.name, segmented_end);
test(disabled_disclosure_link.name, disabled_disclosure_link);
test(initial_popover_position.name, initial_popover_position);
test(popover_layer_size_changes.name, popover_layer_size_changes);
