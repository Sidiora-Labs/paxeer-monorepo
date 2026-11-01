// @vitest-environment jsdom

import React from 'react';

import * as paxeerXMock from 'mocks/paxeerX/unifiedAccount';
import { describe, expect, it } from 'vitest';
import { screen, within } from 'vitest/lib';

import IdentityList from './IdentityList';
import { render } from './testWrapper';

function identity_list_renders_evm_pax_did() {
  const identities = {
    ...paxeerXMock.unifiedAccount.identities,
    kernel_account: paxeerXMock.kernelAccount,
  };
  const { container } = render(<IdentityList identities={ identities }/>);

  const rows = Array.from(container.querySelectorAll<HTMLElement>('[data-identity]'));

  expect(rows.map((row) => row.dataset.identity)).toEqual([ 'evm', 'pax', 'did' ]);

  const expectedRows = [
    { label: 'EVM address', value: paxeerXMock.evmAddress },
    { label: 'Paxeer address', value: paxeerXMock.paxAddress },
    { label: 'LayerX DID', value: paxeerXMock.did },
  ];

  expectedRows.forEach(({ label, value }, index) => {
    const row = within(rows[index]);

    expect(row.getByText(label)).toBeTruthy();
    expect(row.getByText(value)).toBeTruthy();
    expect(row.getAllByRole('button', { name: 'copy' })).toHaveLength(1);
  });

  expect(screen.queryByText('Kernel account')).toBeNull();
  expect(screen.queryByText(paxeerXMock.kernelAccount)).toBeNull();
  expect(container.querySelector('[data-identity="kernel_account"]')).toBeNull();
}

describe('IdentityList', () => {
  it(identity_list_renders_evm_pax_did.name, identity_list_renders_evm_pax_did);

  it('gives every identity a copy button', () => {
    render(<IdentityList identities={ paxeerXMock.unifiedAccount.identities }/>);

    expect(screen.getAllByLabelText('copy')).toHaveLength(3);
  });

  it('hides identities the account does not have', () => {
    const { container } = render(<IdentityList identities={ paxeerXMock.unifiedAccountEvmOnly.identities }/>);

    expect(screen.getByText('EVM address')).toBeTruthy();
    expect(screen.queryByText('Paxeer address')).toBeNull();
    expect(screen.queryByText('LayerX DID')).toBeNull();
    expect(screen.queryByText('Kernel account')).toBeNull();
    expect(container.querySelectorAll('[data-identity]')).toHaveLength(1);
  });

  it('links the EVM address to its address page', () => {
    const { container } = render(<IdentityList identities={ paxeerXMock.unifiedAccount.identities }/>);

    const link = container.querySelector(`a[href="/address/${ paxeerXMock.evmAddress }"]`);

    expect(link).toBeTruthy();
  });
  it('heads the scan table card with the identity count and its note', () => {
    const { container } = render(<IdentityList identities={ paxeerXMock.unifiedAccount.identities }/>);

    const card = container.querySelector('[data-scan-table-card]');

    expect(card?.querySelector('[data-title]')?.textContent).toBe('A total of 3 identities found');
    expect(card?.querySelector('[data-note]')?.textContent).toBe('Every spelling of this account the node answers for');
  });

  it('renders the rows in the shared table and closes the card with a show-rows footer', () => {
    const { container } = render(<IdentityList identities={ paxeerXMock.unifiedAccount.identities }/>);

    expect(container.querySelector('[data-scan-table-card] [data-label="paxeer-x-identities"]')).toBeTruthy();
    expect(container.querySelector('[data-scan-table-card] [data-footer] [data-scan-show-rows]')).toBeTruthy();
  });

});
