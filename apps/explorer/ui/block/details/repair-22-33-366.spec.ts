// @vitest-environment jsdom

import React from 'react';

import type { Block } from 'types/api/block';

import { withBlobTxs } from 'mocks/blocks/block';
import * as DetailedInfo from 'ui/shared/DetailedInfo/DetailedInfo';
import { render } from 'ui/shared/layout/testWrapper';
import { expect, test } from 'vitest';

import BlockDetailsBlobInfo from './BlockDetailsBlobInfo';
import './BlockDetailsBlobInfo.spec';

async function renderBlockResponse(block: Block) {
  const response = new Response(JSON.stringify(block), {
    headers: { 'content-type': 'application/json' },
  });
  const data = await response.json() as Block;
  const { container } = render(React.createElement(
    DetailedInfo.Container,
    null,
    React.createElement(BlockDetailsBlobInfo, { data }),
  ));

  return Array.from(container.querySelectorAll('[data-scan-value]')).map((node) => node.textContent);
}

test('blobGasUnitsPreserveBackendCounter', async function blobGasUnitsPreserveBackendCounter() {
  const values = await renderBlockResponse(withBlobTxs);

  expect(withBlobTxs.excess_blob_gas).toBe('79429632');
  expect(values).toHaveLength(4);
  expect(values[0]).toContain('21.518435987');
  expect(values[0]).toContain('Gwei');
  expect(values[1]).toBe('393,216');
  expect(values[2]).toContain('0.008461393325064192');
  expect(values[2]).toContain('100%');
  expect(values[3]).toBe('79,429,632 gas');
});

test('blobGasUnitsPreserveZeroCounter', async function blobGasUnitsPreserveZeroCounter() {
  const values = await renderBlockResponse({ ...withBlobTxs, excess_blob_gas: '0' });

  expect(values).toHaveLength(4);
  expect(values[3]).toBe('0 gas');
});

test('blobGasUnitsPreserveIntegerPrecision', async function blobGasUnitsPreserveIntegerPrecision() {
  const values = await renderBlockResponse({ ...withBlobTxs, excess_blob_gas: '9007199254740993' });

  expect(values).toHaveLength(4);
  expect(values[3]).toBe('9,007,199,254,740,993 gas');
});
