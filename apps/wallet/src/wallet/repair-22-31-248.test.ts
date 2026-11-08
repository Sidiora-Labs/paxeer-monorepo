// @vitest-environment jsdom
import { act, createElement, useLayoutEffect } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterAll, afterEach, beforeEach, expect, test, vi } from 'vitest';
import { ethers } from 'ethers';
import { ChainDisconnectedError, type Hex } from '@paxeer/wallet';
import { getActiveRpcUrl, PAXEER_CONFIG } from '@/lib/constants';
import { virtualNetwork } from '@/lib/networks';
import { custodyChoiceRepository, preferencesRepository } from '@/platform/storage/repositories';
import { IdentitySession } from './identity';
import {
  WalletNetworkProvider,
  WalletNotReadyError,
  WalletProvider,
  useWallet,
  type WalletContextValue,
} from './WalletProvider';
import {
  EMAIL,
  EMAIL_CODE,
  fixtureBody,
  loadConstructions,
  startGateway,
  type TestGateway,
} from './test/gateway';

vi.hoisted(() => {
  vi.stubEnv('NEXT_PUBLIC_PAXEER_RPC_URL', 'https://rpc.example');
});

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let gateway: TestGateway;
let container: HTMLDivElement;
let root: Root | null = null;
let current: WalletContextValue | null = null;
let sessions: IdentitySession[] = [];

function WalletNetworkProbe() {
  const value = useWallet();
  useLayoutEffect(() => {
    current = value;
  });
  return null;
}

function context(): WalletContextValue {
  if (!current) throw new Error('the wallet context did not render');
  return current;
}

function networkProvider(): WalletNetworkProvider {
  const provider = context().wallet?.provider;
  if (!(provider instanceof WalletNetworkProvider)) throw new Error('no embedded network provider');
  return provider;
}

function newIdentity(): IdentitySession {
  const identity = IdentitySession.create(gateway.config, { detectSessionInUrl: false });
  sessions.push(identity);
  return identity;
}

async function mount(identity: IdentitySession): Promise<void> {
  const target = new EventTarget();
  await act(async () => {
    root = createRoot(container);
    root.render(
      createElement(WalletProvider, {
        config: gateway.config,
        identity,
        eventTarget: target,
        children: createElement(WalletNetworkProbe),
      })
    );
  });
}

async function unmount(): Promise<void> {
  await act(async () => root?.unmount());
  root = null;
  current = null;
}

async function untilStatus(status: WalletContextValue['status']): Promise<void> {
  for (let attempt = 0; attempt < 200; attempt += 1) {
    if (context().status === status) return;
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 10));
    });
  }
  throw new Error(`expected ${status}; got ${context().status}: ${context().error}`);
}

async function signIn(): Promise<IdentitySession> {
  const identity = newIdentity();
  await mount(identity);
  await untilStatus('signed-out');
  await act(async () => {
    await context().verifyEmailCode(EMAIL, EMAIL_CODE);
  });
  expect(context().status).toBe('ready');
  return identity;
}

async function addNetwork(chainId: number): Promise<void> {
  await act(async () => {
    await networkProvider().request({
      method: 'wallet_addEthereumChain',
      params: [{ chainId: ethers.toQuantity(chainId), chainName: `Network ${chainId}` }],
    });
  });
}

async function switchNetwork(chainId: number): Promise<void> {
  await act(async () => {
    await networkProvider().request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId: ethers.toQuantity(chainId) }],
    });
  });
}

beforeEach(async () => {
  localStorage.clear();
  sessions = [];
  gateway = await startGateway();
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(async () => {
  await unmount();
  for (const identity of sessions) await identity.client.auth.stopAutoRefresh();
  container.remove();
  await gateway.close();
  localStorage.clear();
});

afterAll(() => vi.unstubAllEnvs());

async function virtual_network_disconnect_switch_retains_identity_and_validates_sends() {
  await signIn();
  const original = context();
  const provider = networkProvider();
  const selected = virtualNetwork(125000001, 'Network 125000001');
  const recipient = loadConstructions().transaction.to as Hex;
  expect(original.chainId).toBe(PAXEER_CONFIG.chainId);
  expect(original.identity?.email).toBe(EMAIL);

  await addNetwork(selected.chainId);
  await switchNetwork(selected.chainId);
  expect(context().status).toBe('ready');
  expect(context().chainId).toBe(selected.chainId);
  expect(context().wallet).toBe(original.wallet);
  expect(context().identity).toBe(original.identity);
  expect(context().address).toBe(original.address);
  expect(provider.isConnected()).toBe(true);
  expect(getActiveRpcUrl()).toBe(selected.rpcUrl);
  expect(preferencesRepository.read().activeChainId).toBe(selected.chainId);
  expect(context().writeEpoch).toBeGreaterThan(original.writeEpoch);
  expect(original.isWriteCurrent(original.writeEpoch, original.address!, original.chainId!)).toBe(
    false
  );
  expect(context().isWriteCurrent(context().writeEpoch, original.address!, selected.chainId)).toBe(
    true
  );

  const requestsBeforeRefusal = gateway.requests.length;
  await expect(
    context().sendTransaction({ to: recipient, value: 1n, chainId: PAXEER_CONFIG.chainId })
  ).rejects.toBeInstanceOf(ChainDisconnectedError);
  expect(gateway.requests).toHaveLength(requestsBeforeRefusal);

  // The recorded gateway answers for the root chain. Both virtual send APIs must
  // carry the selected chain over HTTP and refuse that mismatched answer.
  await act(async () => {
    await expect(context().send({ to: recipient, value: '0.001' })).rejects.toThrow(
      'send response chain differs from the requested chain'
    );
    await expect(context().sendTransaction({ to: recipient, value: 2n })).rejects.toThrow(
      'send response chain differs from the requested chain'
    );
  });
  expect(
    gateway.requests
      .filter((request) => request.path === '/v1/wallet/send')
      .map((request) => request.body)
  ).toEqual([
    // Transfers normalize recipients to checksum casing; raw transactions preserve the input.
    {
      tx: {
        from: original.address,
        chainId: selected.chainId,
        to: ethers.getAddress(recipient),
        value: '1000000000000000',
      },
    },
    { tx: { from: original.address, chainId: selected.chainId, to: recipient, value: '2' } },
  ]);
  await act(async () => context().refresh());
  expect(context().status).toBe('ready');
  expect(context().wallet).toBe(original.wallet);
  expect(context().identity).toBe(original.identity);

  await addNetwork(125000002);
  const firstEpoch = context().writeEpoch;
  await switchNetwork(125000002);
  expect(context().chainId).toBe(125000002);
  expect(context().isWriteCurrent(firstEpoch, original.address!, selected.chainId)).toBe(false);
  expect(context().wallet).toBe(original.wallet);
  await switchNetwork(PAXEER_CONFIG.chainId);
  expect(context().status).toBe('ready');
  expect(context().chainId).toBe(PAXEER_CONFIG.chainId);
  expect(context().wallet).toBe(original.wallet);
  await act(async () => {
    await expect(context().send({ to: recipient, value: '0.001' })).resolves.toBe(
      fixtureBody('default', 'POST', '/v1/wallet/send').tx_hash
    );
  });
  expect(gateway.requests.filter((request) => request.path === '/v1/wallet/send')).toHaveLength(3);
  expect(
    gateway.requests.filter((request) => request.path === '/v1/wallet/send').at(-1)?.body
  ).toEqual({
    tx: {
      from: original.address,
      chainId: PAXEER_CONFIG.chainId,
      to: ethers.getAddress(recipient),
      value: '1000000000000000',
    },
  });
}

async function virtual_network_disconnect_reload_and_reconnect_restore_selection() {
  const identity = await signIn();
  await addNetwork(125000001);
  await switchNetwork(125000001);
  const original = context();
  const selected = preferencesRepository.read();
  const rpcUrl = getActiveRpcUrl();
  expect(custodyChoiceRepository.read()).toBe('embedded');
  await unmount();
  await identity.client.auth.stopAutoRefresh();

  const restoredIdentity = newIdentity();
  await mount(restoredIdentity);
  await untilStatus('ready');
  expect(context().wallet).not.toBe(original.wallet);
  expect(context().chainId).toBe(125000001);
  expect(context().identity).toEqual(original.identity);
  expect(context().address).toBe(original.address);
  expect(preferencesRepository.read()).toEqual(selected);
  expect(getActiveRpcUrl()).toBe(rpcUrl);
  expect(await networkProvider().request({ method: 'eth_chainId' })).toBe(
    ethers.toQuantity(125000001)
  );
  await act(async () => context().refresh());

  const restored = context().wallet;
  await act(async () => {
    await expect(context().connectEmbedded()).resolves.toBe(original.address);
  });
  expect(context().wallet).not.toBe(restored);
  expect(context().status).toBe('ready');
  expect(context().chainId).toBe(125000001);
  expect(context().identity).toEqual(original.identity);
  expect(getActiveRpcUrl()).toBe(rpcUrl);
  const requestCount = gateway.requests.length;
  await expect(context().sendTransaction({ chainId: 125000002 })).rejects.toBeInstanceOf(
    ChainDisconnectedError
  );
  expect(gateway.requests).toHaveLength(requestCount);
  await act(async () => context().refresh());
  expect(context().status).toBe('ready');
  expect(gateway.requests.filter((request) => request.path === '/auth/v1/verify')).toHaveLength(1);
}

async function virtual_network_disconnect_refuses_unsaved_switches_and_stale_selection() {
  await signIn();
  const original = context();
  const provider = networkProvider();
  await expect(
    provider.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId: ethers.toQuantity(125000001) }],
    })
  ).rejects.toBeInstanceOf(ChainDisconnectedError);
  await expect(
    provider.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId: '0x1' }],
    })
  ).rejects.toBeInstanceOf(ChainDisconnectedError);
  expect(context().wallet).toBe(original.wallet);
  expect(context().status).toBe('ready');
  expect(context().chainId).toBe(original.chainId);

  await addNetwork(125000001);
  preferencesRepository.update((preferences) => ({ ...preferences, activeChainId: 125000001 }));
  const requests = gateway.requests.length;
  await expect(context().sendTransaction({ value: 1n })).rejects.toBeInstanceOf(
    WalletNotReadyError
  );
  await expect(
    context().send({ to: loadConstructions().transaction.to, value: '1' })
  ).rejects.toBeInstanceOf(WalletNotReadyError);
  await expect(context().refresh()).rejects.toBeInstanceOf(WalletNotReadyError);
  expect(gateway.requests).toHaveLength(requests);
  expect(context().isWriteCurrent(original.writeEpoch, original.address!, original.chainId!)).toBe(
    false
  );
  await switchNetwork(125000001);
  expect(context().chainId).toBe(125000001);
  expect(context().wallet).toBe(original.wallet);
  await act(async () => context().refresh());
  expect(context().status).toBe('ready');
}

test(
  virtual_network_disconnect_switch_retains_identity_and_validates_sends.name,
  virtual_network_disconnect_switch_retains_identity_and_validates_sends
);
test(
  virtual_network_disconnect_reload_and_reconnect_restore_selection.name,
  virtual_network_disconnect_reload_and_reconnect_restore_selection
);
test(
  virtual_network_disconnect_refuses_unsaved_switches_and_stale_selection.name,
  virtual_network_disconnect_refuses_unsaved_switches_and_stale_selection
);
