// @vitest-environment jsdom

import { act } from '@testing-library/react';
import React from 'react';

import type { PaxeerXTxSignature } from 'types/api/paxeerXSignature';

import { base } from 'mocks/txs/tx';
import { render, routerState } from 'ui/shared/layout/testWrapper';
import { afterAll, beforeEach, describe, expect, it, test, vi } from 'vitest';

const { answer, requests, server, originalEnvs } = await vi.hoisted(async() => {
  const { generateKeyPairSync, sign } = await import('node:crypto');
  const { createServer } = await import('node:http');
  const receipt = Buffer.from('transaction details sequencer receipt');
  const { privateKey, publicKey } = generateKeyPairSync('ed25519');
  const answer: PaxeerXTxSignature = {
    rung: 'receipted',
    receipt: `0x${ receipt.toString('hex') }`,
    receipt_key: {
      key_id: `0x${ '56'.repeat(32) }`,
      public_key: `0x${ publicKey.export({ format: 'der', type: 'spki' }).subarray(-32).toString('hex') }`,
    },
    signature: `0x${ sign(null, new Uint8Array(receipt), privateKey).toString('hex') }`,
  };
  const requests: Array<string | undefined> = [];
  const server = createServer((request, response) => {
    requests.push(request.url);
    response.setHeader('Content-Type', 'application/json');
    if (request.url === `/api/v2/transactions/${ base.hash }/signature`) {
      response.writeHead(200);
      response.end(JSON.stringify(answer));
    } else {
      response.writeHead(404);
      response.end(JSON.stringify({ message: 'Not found' }));
    }
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('The signature server must listen on a TCP port');
  }
  const originalEnvs = window.__envs;
  window.__envs = {
    ...originalEnvs,
    NEXT_PUBLIC_PAXEER_X_ENABLED: 'true',
    NEXT_PUBLIC_API_PROTOCOL: 'http',
    NEXT_PUBLIC_API_HOST: '127.0.0.1',
    NEXT_PUBLIC_API_PORT: String(address.port),
    NEXT_PUBLIC_API_BASE_PATH: '/',
    NEXT_PUBLIC_USE_NEXT_JS_PROXY: 'false',
    NEXT_PUBLIC_APP_ENV: 'testing',
  };
  return { answer, requests, server, originalEnvs };
});

vi.mock('next/router', async() => (await import('ui/shared/layout/testWrapper')).nextRouterModule());

import TxInfo from './TxInfo';

const labels = (container: HTMLElement, scope: string) =>
  Array.from(container.querySelectorAll(`${ scope } [data-detailed-info-label]`)).map((item) => item.textContent);

afterAll(async() => {
  window.__envs = originalEnvs;
  fetchMock.enableMocks();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
});

async function tx_info_shows_sequencer_signature_row() {
  const { container, findByText } = render(<TxInfo data={ base } isLoading={ false }/>);

  expect(await findByText('Sequencer signature')).toBeDefined();
  expect(await findByText(`Key id: ${ answer.receipt_key.key_id }`)).toBeDefined();
  const signature = await findByText(answer.signature);
  expect(signature.parentElement?.querySelector('[data-rung="receipted"]')?.textContent).toBe('Receipted');
  expect(labels(container, '[data-tx-info] > [data-detailed-info]').slice(0, 4))
    .toEqual([ 'Transaction hash', 'Status and method', 'Sequencer signature', 'Block' ]);
  expect(requests.filter((path) => path?.endsWith('/signature')))
    .toEqual([ `/api/v2/transactions/${ base.hash }/signature` ]);
}

describe('TxInfo', () => {
  beforeEach(() => {
    routerState.pathname = '/tx/[hash]';
    routerState.query = { hash: base.hash };
    fetchMock.disableMocks();
    requests.length = 0;
  });

  test(tx_info_shows_sequencer_signature_row.name, tx_info_shows_sequencer_signature_row);

  it('puts the detail card before the more details expander', () => {
    const { container } = render(<TxInfo data={ base } isLoading={ false }/>);

    const blocks = Array.from(container.querySelectorAll('[data-tx-info] > [data-detailed-info], [data-tx-info] > [data-scan-expander]'));

    expect(blocks).toHaveLength(2);
    expect(blocks[0].hasAttribute('data-detailed-info')).toBe(true);
    expect(blocks[1].hasAttribute('data-scan-expander')).toBe(true);
  });

  it('orders the overview rows from the hash down to the gas price', () => {
    const { container } = render(<TxInfo data={ base } isLoading={ false }/>);

    const expected = [ 'Transaction hash', 'Status and method', 'Block', 'Timestamp', 'From', 'To', 'Value', 'Transaction fee', 'Gas price' ];

    expect(labels(container, '[data-tx-info] > [data-detailed-info]').filter((label) => expected.includes(label ?? ''))).toEqual(expected);
  });

  it('carries the hash with its copy control and the method beside the status', () => {
    const { container } = render(<TxInfo data={ base } isLoading={ false }/>);

    const rows = container.querySelectorAll('[data-tx-info] > [data-detailed-info] [data-scan-value]');

    expect(rows[0].querySelector('[aria-label="copy"]')).not.toBeNull();
    expect(rows[1].textContent).toContain(base.method);
  });

  it('shows the confirmations as a chip on the block row', () => {
    const { container } = render(<TxInfo data={ base } isLoading={ false }/>);

    expect(container.querySelector('[data-block-confirmations]')?.textContent)
      .toBe(`${ base.confirmations } Block confirmations`);
  });

  it('offers the timestamp together with its zone control', () => {
    const { container } = render(<TxInfo data={ base } isLoading={ false }/>);

    const timestampRow = Array.from(container.querySelectorAll('[data-tx-info] > [data-detailed-info] [data-scan-value]'))[3];

    expect(timestampRow.querySelector('button')).not.toBeNull();
  });

  it('marks a verified recipient contract', () => {
    const verified = { ...base, to: { ...base.to!, is_contract: true, is_verified: true } };
    const { container } = render(<TxInfo data={ verified } isLoading={ false }/>);

    expect(container.querySelector('[data-verified-mark]')).not.toBeNull();
  });

  it('leaves the mark off an unverified recipient contract', () => {
    const unverified = { ...base, to: { ...base.to!, is_contract: true, is_verified: false } };
    const { container } = render(<TxInfo data={ unverified } isLoading={ false }/>);

    expect(container.querySelector('[data-verified-mark]')).toBeNull();
  });

  it('keeps the gas usage, nonce and raw input out of the overview until the expander is opened', () => {
    const { container } = render(<TxInfo data={ base } isLoading={ false }/>);

    const expander = container.querySelector('[data-scan-expander]') as HTMLElement;

    expect(expander.getAttribute('data-open')).toBe('false');
    expect(expander.querySelector('[data-content]')).toBeNull();

    act(() => {
      (expander.querySelector('[data-toggle]') as HTMLElement).click();
    });

    expect(expander.getAttribute('data-open')).toBe('true');
    expect(labels(container, '[data-scan-expander]')).toEqual(expect.arrayContaining([ 'Gas usage & limit by txn', 'Other', 'Raw input' ]));
  });
});
