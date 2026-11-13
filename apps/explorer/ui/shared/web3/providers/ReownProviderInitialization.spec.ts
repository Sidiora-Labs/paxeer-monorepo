// @vitest-environment jsdom

import { mainnet } from '@reown/appkit/networks';
import { useAppKit } from '@reown/appkit/react';
import { cleanup, fireEvent, render } from '@testing-library/react';
import React from 'react';
import { useAccount } from 'wagmi';

import { expect, test } from 'vitest';

import ReownProvider, { ReownInitialization } from './ReownProvider';

function WalletConnection() {
  const { open } = useAppKit();
  const handleConnect = () => open();
  return React.createElement('button', { onClick: handleConnect }, 'Connect wallet');
}

function walletInitializationExceptionIsRetainedAndRetryable() {
  const options: ConstructorParameters<typeof ReownInitialization>[0] = {
    networks: [ mainnet ],
    projectId: 'private-project-id-fixture',
    metadata: {
      name: 'Explorer',
      description: 'Explorer',
      url: 'https://localhost/?token=private-token-fixture',
      icons: [],
    },
  };
  // A sparse network list reaches the real SDK's network lookup and throws synchronously,
  // before its asynchronous clients or transports can start.
  options.networks.length = 2;
  const initialization = new ReownInitialization(options);
  expect(initialization.state.status).toBe('failed');
  if (initialization.state.status !== 'failed') {
    throw new Error('The real AppKit constructor must reject the malformed networks');
  }
  const firstError = initialization.state.error;
  expect(firstError).toBeInstanceOf(TypeError);
  if (!(firstError instanceof TypeError)) {
    throw new Error('Expected the SDK network lookup to retain its TypeError');
  }
  expect(firstError.stack).toContain('extendDefaultCaipNetwork');

  try {
    const view = render(React.createElement(ReownProvider, { initialization, children: React.createElement(WalletConnection) }));
    expect(view.getByRole('alert').textContent).toBe('Wallet connections are unavailable. Please retry.Retry wallet connection');
    expect(view.queryByRole('button', { name: 'Connect wallet' })).toBeNull();
    expect(initialization.state.error).toBe(firstError);
    expect(view.container.innerHTML).not.toContain(options.projectId);
    expect(view.container.innerHTML).not.toContain('private-token-fixture');
    expect(view.container.innerHTML).not.toContain(firstError.message);

    fireEvent.click(view.getByRole('button', { name: 'Retry wallet connection' }));
    expect(initialization.state.status).toBe('failed');
    expect(initialization.state.error).toBeInstanceOf(TypeError);
    expect(initialization.state.error).not.toBe(firstError);
    expect(view.getByRole('alert').textContent).toContain('Wallet connections are unavailable.');
    expect(view.queryByRole('button', { name: 'Connect wallet' })).toBeNull();
    expect(view.container.innerHTML).not.toContain(options.projectId);
    expect(view.container.innerHTML).not.toContain('private-token-fixture');

    const retryError = initialization.state.error;
    view.unmount();
    const reopenedView = render(React.createElement(ReownProvider, { initialization, children: React.createElement(WalletConnection) }));
    expect(initialization.state.error).toBe(retryError);
    expect(reopenedView.getByRole('alert').textContent).toContain('Wallet connections are unavailable.');
    expect(reopenedView.queryByRole('button', { name: 'Connect wallet' })).toBeNull();
    expect(reopenedView.container.innerHTML).not.toContain(options.projectId);
    expect(reopenedView.container.innerHTML).not.toContain('private-token-fixture');
  } finally {
    cleanup();
  }
}

function ReadOnlyAccount() {
  const account = useAccount();
  return React.createElement('p', null, account.status);
}

function disabledWalletConfigurationPreservesReadOnlyProvider() {
  const initialization = new ReownInitialization();
  expect(initialization.state).toEqual({ status: 'disabled' });
  try {
    const view = render(React.createElement(ReownProvider, { initialization, children: React.createElement(ReadOnlyAccount) }));
    expect(view.getByText('disconnected')).toBeTruthy();
    expect(view.queryByRole('alert')).toBeNull();
  } finally {
    cleanup();
  }
}

test(walletInitializationExceptionIsRetainedAndRetryable);
test(disabledWalletConfigurationPreservesReadOnlyProvider);
