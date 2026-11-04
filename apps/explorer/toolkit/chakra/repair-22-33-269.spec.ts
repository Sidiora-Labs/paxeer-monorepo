// @vitest-environment jsdom

import { ChakraProvider, createListCollection } from '@chakra-ui/react';
import type { ListCollection } from '@chakra-ui/react';
import { act, cleanup, fireEvent, render, waitFor } from '@testing-library/react';
import React from 'react';

import { afterEach, expect, test } from 'vitest';

import theme from '../theme/theme';
import { Image } from './image';
import type { SelectAsyncProps, SelectOption } from './select';
import { SelectAsync } from './select';

const options: Array<SelectOption> = [
  { value: 'alpha', label: 'Alpha' },
  { value: 'beta', label: 'Beta' },
  { value: 'gamma', label: 'Gamma' },
];

interface ReadRequest {
  input: string;
  value: Array<string>;
  resolve: (collection: ListCollection<SelectOption>) => void;
  reject: (reason: Error) => void;
}

class OptionStore {
  readonly requests: Array<ReadRequest> = [];

  readonly load: SelectAsyncProps['loadOptions'] = (input, value) => new Promise((resolve, reject) => {
    this.requests.push({ input, value: [ ...value ], resolve, reject });
  });

  release(index: number) {
    const request = this.requests[index];
    const items = options.filter((item) => (
      request.value.includes(item.value) ||
      (request.value.length === 0 && item.label.toLowerCase().includes(request.input.toLowerCase()))
    ));
    request.resolve(createListCollection({ items }));
  }

  fail(index: number) {
    this.requests[index].reject(new Error('Option store read failed'));
  }
}

function withTheme(child: React.ReactNode) {
  return React.createElement(ChakraProvider, { value: theme, children: child });
}

function asyncSelect(store: OptionStore, props: Partial<SelectAsyncProps> = {}) {
  return withTheme(React.createElement(SelectAsync, {
    placeholder: 'Choose an option',
    loadOptions: store.load,
    portalled: false,
    ...props,
  }));
}

function nativeSelect(container: HTMLElement) {
  const select = container.querySelector('select');
  if (!select) throw new Error('The production SelectRoot did not render its native form control');
  return select;
}

function selectRoot(container: HTMLElement) {
  const root = container.querySelector('[data-scope="select"][data-part="root"]');
  if (!root) throw new Error('The production SelectRoot did not render');
  return root;
}

afterEach(cleanup);

async function explorerImageRetry() {
  const loaded: Array<string | null> = [];
  const failed: Array<string | null> = [];
  const image = (src: string, srcSet?: string) => withTheme(React.createElement(Image, {
    src,
    srcSet,
    alt: 'Asset',
    fallback: React.createElement('span', { role: 'status' }, 'Image unavailable'),
    onLoad: (event) => loaded.push(event.currentTarget.getAttribute('src')),
    onError: (event) => failed.push(event.currentTarget.getAttribute('src')),
  }));
  const view = render(image('/broken.png'));
  const initial = view.getByAltText('Asset');
  expect(getComputedStyle(initial).display).toBe('none');
  expect(view.container.querySelector('[data-loading]')).not.toBeNull();
  fireEvent.error(initial);
  expect(view.getByRole('status').textContent).toBe('Image unavailable');
  expect(failed).toEqual([ '/broken.png' ]);

  view.rerender(image('/replacement.png'));
  const replacement = view.getByAltText('Asset');
  expect(view.queryByRole('status')).toBeNull();
  expect(replacement.getAttribute('src')).toBe('/replacement.png');
  expect(getComputedStyle(replacement).display).toBe('none');
  expect(view.container.querySelector('[data-loading]')).not.toBeNull();
  fireEvent.load(replacement);
  expect(getComputedStyle(replacement).display).toBe('block');
  expect(view.container.querySelector('[data-loading]')).toBeNull();
  expect(loaded).toEqual([ '/replacement.png' ]);

  view.rerender(image('/next.png'));
  const next = view.getByAltText('Asset');
  expect(getComputedStyle(next).display).toBe('none');
  fireEvent.error(next);
  view.rerender(image('/next.png', '/responsive.png 2x'));
  const responsive = view.getByAltText('Asset');
  expect(responsive.getAttribute('srcset')).toBe('/responsive.png 2x');
  fireEvent.load(responsive);
  expect(getComputedStyle(responsive).display).toBe('block');
  expect(view.queryByRole('status')).toBeNull();
}

async function asyncSelectControlledValue() {
  const store = new OptionStore();
  const changes: Array<Array<string>> = [];
  const onValueChange: SelectAsyncProps['onValueChange'] = ({ value }) => changes.push(value);
  const view = render(asyncSelect(store, { value: [ 'alpha' ], onValueChange }));
  expect(store.requests.map(({ value }) => value)).toEqual([ [ 'alpha' ] ]);
  expect(selectRoot(view.container).getAttribute('aria-busy')).toBe('true');
  await act(async() => store.release(0));
  expect(nativeSelect(view.container).value).toBe('alpha');
  expect(view.getByRole('combobox').textContent).toContain('Alpha');
  expect(selectRoot(view.container).getAttribute('aria-busy')).toBe('false');

  view.rerender(asyncSelect(store, { value: [ 'beta' ], onValueChange }));
  expect(store.requests[1].value).toEqual([ 'beta' ]);
  expect(nativeSelect(view.container).querySelector('option[value="alpha"]')).not.toBeNull();
  await act(async() => store.release(1));
  expect(nativeSelect(view.container).value).toBe('beta');
  expect(view.getByRole('combobox').textContent).toContain('Beta');
  expect(view.getByRole('combobox').textContent).not.toContain('Alpha');
  expect(changes).toEqual([]);

  view.rerender(asyncSelect(store, { value: [], onValueChange }));
  expect(store.requests[2].value).toEqual([]);
  await act(async() => store.release(2));
  expect(nativeSelect(view.container).value).toBe('');
  expect(view.getByRole('combobox').textContent).toContain('Choose an option');
  act(() => view.getByRole('combobox').focus());
  fireEvent.keyDown(view.getByRole('combobox'), { key: 'End' });
  await waitFor(() => expect(changes).toEqual([ [ 'gamma' ] ]));
  expect(store.requests).toHaveLength(3);
  await waitFor(() => expect(nativeSelect(view.container).value).toBe(''));
  expect(view.getByRole('combobox').textContent).toContain('Choose an option');
  expect(view.getByRole('combobox').getAttribute('aria-expanded')).toBe('false');
}

async function asyncSelectUncontrolledValue() {
  const store = new OptionStore();
  const changes: Array<Array<string>> = [];
  const view = render(asyncSelect(store, {
    defaultValue: [ 'alpha' ],
    onValueChange: ({ value }) => changes.push(value),
  }));
  expect(store.requests[0].value).toEqual([ 'alpha' ]);
  await act(async() => store.release(0));
  expect(nativeSelect(view.container).value).toBe('alpha');
  fireEvent.change(nativeSelect(view.container), { target: { value: '' } });
  await waitFor(() => expect(store.requests).toHaveLength(2));
  expect(store.requests[1].value).toEqual([]);
  await act(async() => store.release(1));
  fireEvent.change(nativeSelect(view.container), { target: { value: 'beta' } });
  await waitFor(() => expect(store.requests).toHaveLength(3));
  expect(store.requests[2].value).toEqual([ 'beta' ]);
  await act(async() => store.release(2));
  expect(nativeSelect(view.container).value).toBe('beta');
  expect(view.getByRole('combobox').textContent).toContain('Beta');
  expect(changes).toEqual([ [], [ 'beta' ] ]);
}

async function asyncSelectRace() {
  const store = new OptionStore();
  const view = render(asyncSelect(store, { value: [ 'alpha' ] }));
  view.rerender(asyncSelect(store, { value: [ 'beta' ] }));
  expect(store.requests.map(({ value }) => value)).toEqual([ [ 'alpha' ], [ 'beta' ] ]);
  await act(async() => store.release(1));
  expect(view.getByRole('combobox').textContent).toContain('Beta');
  await act(async() => store.release(0));
  expect(nativeSelect(view.container).querySelector('option[value="beta"]')).not.toBeNull();
  expect(nativeSelect(view.container).querySelector('option[value="alpha"]')).toBeNull();
  expect(view.getByRole('combobox').textContent).toContain('Beta');

  view.rerender(asyncSelect(store, { value: [ 'alpha' ] }));
  view.rerender(asyncSelect(store, { value: [ 'gamma' ] }));
  await act(async() => store.fail(2));
  expect(view.queryByRole('alert')).toBeNull();
  expect(selectRoot(view.container).getAttribute('aria-busy')).toBe('true');
  expect(nativeSelect(view.container).querySelector('option[value="beta"]')).not.toBeNull();
  await act(async() => store.release(3));
  expect(view.getByRole('combobox').textContent).toContain('Gamma');
  expect(selectRoot(view.container).getAttribute('aria-busy')).toBe('false');
}

async function asyncSelectRejectionRecovery() {
  const store = new OptionStore();
  const view = render(asyncSelect(store, { value: [ 'alpha' ] }));
  await act(async() => store.release(0));
  view.rerender(asyncSelect(store, { value: [ 'beta' ] }));
  await act(async() => store.fail(1));
  expect(view.getByRole('alert').textContent).toContain('Could not load options');
  expect(selectRoot(view.container).getAttribute('aria-busy')).toBe('false');
  expect(nativeSelect(view.container).querySelector('option[value="alpha"]')).not.toBeNull();

  view.rerender(asyncSelect(store, { value: [ 'gamma' ] }));
  expect(view.queryByRole('alert')).toBeNull();
  expect(selectRoot(view.container).getAttribute('aria-busy')).toBe('true');
  await act(async() => store.release(2));
  expect(view.getByRole('combobox').textContent).toContain('Gamma');
  expect(view.queryByRole('alert')).toBeNull();
  expect(selectRoot(view.container).getAttribute('aria-busy')).toBe('false');
}

async function asyncSelectUnmount() {
  const store = new OptionStore();
  let commits = 0;
  const view = render(React.createElement(React.Profiler, {
    id: 'select',
    onRender: () => {
      commits += 1;
    },
  }, asyncSelect(store, { value: [ 'alpha' ] })));
  view.rerender(React.createElement(React.Profiler, {
    id: 'select',
    onRender: () => {
      commits += 1;
    },
  }, asyncSelect(store, { value: [ 'beta' ] })));
  expect(store.requests).toHaveLength(2);
  view.unmount();
  const unmountedCommits = commits;
  await act(async() => {
    store.release(0);
    store.fail(1);
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  expect(view.container.childElementCount).toBe(0);
  expect(commits).toBe(unmountedCommits);
  const reopened = render(asyncSelect(store, { value: [ 'gamma' ] }));
  expect(store.requests[2].value).toEqual([ 'gamma' ]);
  await act(async() => store.release(2));
  expect(reopened.getByRole('combobox').textContent).toContain('Gamma');
  expect(reopened.queryByRole('alert')).toBeNull();
}

test('explorerImageRetry', explorerImageRetry);
test('asyncSelectControlledValue', asyncSelectControlledValue);
test('asyncSelectUncontrolledValue', asyncSelectUncontrolledValue);
test('asyncSelectRace', asyncSelectRace);
test('asyncSelectRejectionRecovery', asyncSelectRejectionRecovery);
test('asyncSelectUnmount', asyncSelectUnmount);
