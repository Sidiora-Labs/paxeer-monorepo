// @vitest-environment jsdom

import { fireEvent, within } from '@testing-library/react';
import { Blob as NodeBlob, resolveObjectURL } from 'node:buffer';
import { URL as NodeURL } from 'node:url';
import React from 'react';

import { render } from 'ui/paxeerX/account/testWrapper';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';

import BlobData from './BlobData';
import imageBlobWithZeroesBytes from './image_with_zeroes.blob';

const textData = '0x0048656c6c6f20776f726c6400';
const textBytes = Buffer.from('0048656c6c6f20776f726c6400', 'hex');
const textHash = `0x${ 'ab'.repeat(32) }`;
const imageHash = `0x${ 'cd'.repeat(32) }`;
const downloads: Array<{ blob: NodeBlob | undefined; filename: string; url: string }> = [];
const originalBlob = Object.getOwnPropertyDescriptor(globalThis, 'Blob');
const originalCreateObjectURL = Object.getOwnPropertyDescriptor(URL, 'createObjectURL');
const originalRevokeObjectURL = Object.getOwnPropertyDescriptor(URL, 'revokeObjectURL');
const nativeAnchorClick = HTMLAnchorElement.prototype.click;

beforeAll(() => {
  // Use the real platform Blob and object-URL store, which jsdom does not supply.
  Object.defineProperty(globalThis, 'Blob', { configurable: true, writable: true, value: NodeBlob });
  Object.defineProperty(URL, 'createObjectURL', { configurable: true, writable: true, value: NodeURL.createObjectURL });
  Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, writable: true, value: NodeURL.revokeObjectURL });

  // Record the real anchor's download event before downloadBlob revokes its URL.
  HTMLAnchorElement.prototype.click = function(this: HTMLAnchorElement) {
    this.addEventListener('click', (event) => {
      event.preventDefault();
      downloads.push({ blob: resolveObjectURL(this.href), filename: this.download, url: this.href });
    }, { once: true });
    nativeAnchorClick.call(this);
  };
});

afterEach(() => {
  downloads.length = 0;
});

afterAll(() => {
  HTMLAnchorElement.prototype.click = nativeAnchorClick;
  if (originalBlob) Object.defineProperty(globalThis, 'Blob', originalBlob);
  if (originalCreateObjectURL) Object.defineProperty(URL, 'createObjectURL', originalCreateObjectURL);
  else Reflect.deleteProperty(URL, 'createObjectURL');
  if (originalRevokeObjectURL) Object.defineProperty(URL, 'revokeObjectURL', originalRevokeObjectURL);
  else Reflect.deleteProperty(URL, 'revokeObjectURL');
});

function blobElement(data: string, hash: string, isLoading = false) {
  return React.createElement(BlobData, { data, hash, isLoading });
}

function expectFormat(container: HTMLElement, expected: string) {
  const select = container.querySelector('select');
  expect(select).toBeInstanceOf(HTMLSelectElement);
  expect(select?.value).toBe(expected);
  expect(Array.from(select!.options, (option) => option.value)).toContain(expected);
  const trigger = container.querySelector('[role="combobox"]');
  expect(trigger).toBeInstanceOf(HTMLButtonElement);
  expect(trigger?.textContent).toBe(expected);
}

function chooseFormat(container: HTMLElement, value: string) {
  const select = container.querySelector('select');
  expect(select).toBeInstanceOf(HTMLSelectElement);
  fireEvent.change(select!, { target: { value } });
  expectFormat(container, value);
}

async function expectDownload(container: HTMLElement, expected: Buffer, hash: string, mime: string) {
  const count = downloads.length;
  fireEvent.click(within(container).getByRole('button', { name: /^Download$/ }));
  expect(downloads).toHaveLength(count + 1);
  const download = downloads[count];
  expect(download.filename).toBe(`blob_${ hash }`);
  expect(download.blob).toBeInstanceOf(NodeBlob);
  expect(download.blob?.type).toBe(mime);
  expect(Buffer.from(await download.blob!.arrayBuffer())).toEqual(expected);
  expect(resolveObjectURL(download.url)).toBeUndefined();
}

async function blob_navigation_coverage() {
  const view = render(blobElement(imageBlobWithZeroesBytes, imageHash));
  expectFormat(view.container, 'Image');
  expect(view.container.querySelector('img')?.getAttribute('src')).toMatch(/^data:image\/jpeg;base64,/);
  const selector = view.getByRole('combobox');

  view.rerender(blobElement(textData, textHash));
  expect(view.getByRole('combobox')).toBe(selector);
  expectFormat(view.container, 'Raw');
  expect(view.getByText(textData, { exact: true })).toBeDefined();
  expect(view.queryByText('Not an image')).toBeNull();
  expect(view.container.querySelector('option[value="Image"]')).toBeNull();
  await expectDownload(view.container, Buffer.from(textData), textHash, 'application/octet-stream');

  chooseFormat(view.container, 'UTF-8');
  expect(view.container.querySelector('section')?.textContent).toBe(textBytes.toString('utf8'));
  await expectDownload(view.container, textBytes, textHash, 'text/plain');
}

async function blob_format_state_loading_navigation() {
  const view = render(blobElement(imageBlobWithZeroesBytes, imageHash));
  chooseFormat(view.container, 'Raw');
  chooseFormat(view.container, 'Image');

  view.rerender(blobElement(textData, textHash, true));
  expectFormat(view.container, 'Raw');
  expect(view.queryByText('Not an image')).toBeNull();

  view.rerender(blobElement(textData, textHash));
  expectFormat(view.container, 'Raw');
  expect(view.getByText(textData, { exact: true })).toBeDefined();
  await expectDownload(view.container, Buffer.from(textData), textHash, 'application/octet-stream');
}

async function blob_format_state_preserves_explicit_formats() {
  for (const format of [ 'Raw', 'UTF-8', 'Base64' ] as const) {
    const view = render(blobElement(textData, textHash));
    chooseFormat(view.container, format);

    view.rerender(blobElement(imageBlobWithZeroesBytes, imageHash, true));
    expectFormat(view.container, format);
    view.rerender(blobElement(imageBlobWithZeroesBytes, imageHash));
    expectFormat(view.container, format);

    view.rerender(blobElement(textData, textHash, true));
    expectFormat(view.container, format);
    view.rerender(blobElement(textData, textHash));
    expectFormat(view.container, format);

    const expected = {
      Raw: Buffer.from(textData),
      'UTF-8': textBytes,
      Base64: Buffer.from(textBytes.toString('base64')),
    }[format];
    expect(expected).toBeDefined();
    await expectDownload(view.container, expected!, textHash, format === 'UTF-8' ? 'text/plain' : 'application/octet-stream');
    view.unmount();
  }
}

async function blob_format_state_initial_loading_and_image_bytes() {
  const view = render(blobElement('0x', imageHash, true));
  expectFormat(view.container, 'Raw');
  view.rerender(blobElement(imageBlobWithZeroesBytes, imageHash));
  expectFormat(view.container, 'Image');
  const source = view.container.querySelector('img')?.getAttribute('src');
  expect(source).toMatch(/^data:image\/jpeg;base64,/);
  const imageBytes = Buffer.from(source!.split(',')[1], 'base64');
  expect(Array.from(imageBytes.subarray(0, 4))).toEqual([ 255, 216, 255, 224 ]);
  const paddedBytes = Buffer.from(imageBlobWithZeroesBytes.slice(2), 'hex');
  expect(paddedBytes.length).toBeGreaterThan(imageBytes.length);
  expect(imageBytes).toEqual(paddedBytes.filter((_, index) => index % 32 !== 0));
  await expectDownload(view.container, imageBytes, imageHash, 'image/jpeg');
}

test(blob_navigation_coverage);
test(blob_format_state_loading_navigation);
test(blob_format_state_preserves_explicit_formats);
test(blob_format_state_initial_loading_and_image_bytes);
