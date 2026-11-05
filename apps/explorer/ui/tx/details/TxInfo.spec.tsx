// @vitest-environment jsdom

import { act } from '@testing-library/react';
import React from 'react';

import { describe, expect, it, test } from 'vitest';

import { answer, transaction as base, render, requests, TxInfo } from './signatureTestFixture';

const labels = (container: HTMLElement, scope: string) =>
  Array.from(container.querySelectorAll(`${ scope } [data-detailed-info-label]`)).map((item) => item.textContent);

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
