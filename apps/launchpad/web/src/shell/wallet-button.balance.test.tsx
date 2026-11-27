// @vitest-environment jsdom
import React, { act } from 'react';
import { spawn, type ChildProcess } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { WagmiProvider } from 'wagmi';
import { connect, disconnect } from 'wagmi/actions';
import {
  createPublicClient, createTestClient, defineChain, encodeAbiParameters,
  erc20Abi, http, keccak256, toHex, type Address,
} from 'viem';
import { PathnameContext } from 'next/dist/shared/lib/hooks-client-context.shared-runtime';
import { WalletRuntimeProvider } from '@/core/wallet-sdk/runtime';
import { USDL_ADDRESS } from '@/core/network/contracts';
import { resolveAnvilBinary } from '@/core/test-anvil';
import { clearAccountSession } from '@/widgets/profile/account-transport';
import { clearNavigationSession, installNavigationSession, makeNavigationConfig, NAVIGATION_ACCOUNT } from './navigation-session.fixture';
import WalletButton from './wallet-button';

const chain = defineChain({
  id: 125,
  name: 'Header balance',
  nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
  rpcUrls: { default: { http: [] } },
});
const previousRpc = process.env.NEXT_PUBLIC_RPC_URL;
const originalFetch = globalThis.fetch;
const originalVisibility = Object.getOwnPropertyDescriptor(document, 'visibilityState');
const requests: string[] = [];
const calls: Array<{ to: string; data: string; block: string }> = [];
let node: ChildProcess | undefined;
let relay: Server;
let rpcUrl: string;
let failing = false;
let held = false;
const release: Array<() => void> = [];
const mounted: Array<{ root: Root; queries: QueryClient }> = [];
const balanceSlot = keccak256(encodeAbiParameters(
  [{ type: 'address' }, { type: 'uint256' }], [NAVIGATION_ACCOUNT, 0n],
));

async function settle(check: () => boolean, timeout = 15_000) {
  const deadline = Date.now() + timeout;
  while (!check()) {
    if (Date.now() >= deadline) throw new Error('Header balance did not settle');
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 20)); });
  }
}

function visibility(value: 'visible' | 'hidden') {
  Object.defineProperty(document, 'visibilityState', { configurable: true, value });
  document.dispatchEvent(new Event('visibilitychange'));
}

const client = () => createPublicClient({ chain, transport: http(rpcUrl, { retryCount: 0 }) });
const controls = () => createTestClient({ mode: 'anvil', chain, transport: http(rpcUrl) });

async function setBalance(value: bigint) {
  await controls().setStorageAt({ address: USDL_ADDRESS, index: balanceSlot, value: toHex(value, { size: 32 }) });
  expect(await client().readContract({ address: USDL_ADDRESS, abi: erc20Abi, functionName: 'balanceOf', args: [NAVIGATION_ACCOUNT] })).toBe(value);
}

beforeAll(async () => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const reservation = createServer();
  await new Promise<void>(resolve => reservation.listen(0, '127.0.0.1', resolve));
  const port = (reservation.address() as AddressInfo).port;
  await new Promise<void>(resolve => reservation.close(() => resolve()));
  rpcUrl = `http://127.0.0.1:${port}`;
  node = spawn(resolveAnvilBinary(), ['--host', '127.0.0.1', '--port', String(port), '--chain-id', '125', '--silent'], { stdio: 'ignore' });
  let spawnError: Error | undefined;
  node.once('error', error => { spawnError = error; });
  const deadline = Date.now() + 10_000;
  for (;;) {
    if (spawnError || node.exitCode !== null) throw spawnError ?? new Error('Header balance chain exited');
    try { if (await client().getChainId() === 125) break; } catch (error) {
      if (Date.now() >= deadline) throw error;
    }
    await new Promise(resolve => setTimeout(resolve, 25));
  }
  const fixture = JSON.parse(readFileSync(process.env.L25_QUALIFICATION_CONFIG_FILE!, 'utf8')) as {
    network: { publicRPC: string; addresses: { USDL: Address } };
  };
  const source = createPublicClient({ transport: http(fixture.network.publicRPC) });
  const runtime = await source.getCode({ address: fixture.network.addresses.USDL });
  expect(runtime).toBeDefined();
  expect(runtime).not.toBe('0x');
  await controls().setCode({ address: USDL_ADDRESS, bytecode: runtime! });

  relay = createServer((request, response) => {
    const chunks: Buffer[] = [];
    request.on('data', chunk => chunks.push(Buffer.from(chunk)));
    request.on('end', () => {
      const body = Buffer.concat(chunks).toString();
      const rpc = JSON.parse(body) as { method: string; params: [{ to: string; data: string }, string] };
      if (rpc.method === 'eth_call') calls.push({ ...rpc.params[0], block: rpc.params[1] });
      const forward = () => {
        if (failing) { response.destroy(); return; }
        void originalFetch(rpcUrl, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body })
          .then(async upstream => { response.writeHead(upstream.status, { 'Content-Type': 'application/json' }); response.end(await upstream.text()); })
          .catch(() => response.destroy());
      };
      if (held) release.push(forward);
      else forward();
    });
  });
  await new Promise<void>(resolve => relay.listen(0, '127.0.0.1', resolve));
  process.env.NEXT_PUBLIC_RPC_URL = `http://127.0.0.1:${(relay.address() as AddressInfo).port}`;
  globalThis.fetch = (input, init) => {
    requests.push(input instanceof Request ? input.url : String(input));
    return originalFetch(input, init);
  };
}, 30_000);

afterEach(async () => {
  for (const { root, queries } of mounted.splice(0)) {
    await act(async () => root.unmount());
    queries.clear();
  }
  held = false;
  failing = false;
  for (const forward of release.splice(0)) forward();
  clearNavigationSession();
  document.body.innerHTML = '';
  visibility('visible');
  requests.length = 0;
  calls.length = 0;
});

afterAll(async () => {
  globalThis.fetch = originalFetch;
  if (previousRpc === undefined) delete process.env.NEXT_PUBLIC_RPC_URL;
  else process.env.NEXT_PUBLIC_RPC_URL = previousRpc;
  if (originalVisibility) Object.defineProperty(document, 'visibilityState', originalVisibility);
  else Reflect.deleteProperty(document, 'visibilityState');
  if (relay) {
    relay.closeAllConnections();
    await new Promise<void>(resolve => relay.close(() => resolve()));
  }
  if (node && node.exitCode === null) {
    node.kill('SIGTERM');
    await new Promise<void>(resolve => node!.once('exit', () => resolve()));
  }
});

async function renderHeader(signedIn = true) {
  await setBalance(36_357_687n);
  const config = makeNavigationConfig();
  const queries = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  mounted.push({ root, queries });
  await act(async () => root.render(
    <WagmiProvider config={config}>
      <QueryClientProvider client={queries}>
        <PathnameContext.Provider value="/">
          <WalletRuntimeProvider><WalletButton /></WalletRuntimeProvider>
        </PathnameContext.Provider>
      </QueryClientProvider>
    </WagmiProvider>,
  ));
  await act(async () => { await connect(config, { connector: config.connectors[0] }); });
  if (signedIn) await act(async () => installNavigationSession());
  const pill = () => container.querySelector<HTMLButtonElement>('button[data-numeric]');
  return { config, container, pill };
}

async function pillShowsTheUsdlBalanceOfTheWallet() {
  const { pill } = await renderHeader();
  await settle(() => pill()?.textContent === '36.36 USDL');
  expect(pill()?.textContent).not.toBe('2.43 USDL');
  expect(requests.filter(url => /\/portfolio(?:[/?]|$)/.test(url))).toEqual([]);
  expect(calls).toHaveLength(1);
  expect(calls[0]).toEqual({
    to: USDL_ADDRESS.toLowerCase(),
    data: `0x70a08231${NAVIGATION_ACCOUNT.slice(2).padStart(64, '0')}`,
    block: 'latest',
  });
}

async function pillSaysUsdlBalanceInTitleAndLabel() {
  const { pill } = await renderHeader();
  await settle(() => pill()?.textContent === '36.36 USDL');
  expect(pill()?.title).toBe('USDL balance 36.36 USDL');
  expect(pill()?.getAttribute('aria-label')).toBe('USDL balance 36.36 USDL, open account menu');
}

async function pillShowsADashWhenTheReadFails() {
  failing = true;
  const { pill } = await renderHeader();
  expect(pill()).not.toBeNull();
  expect(pill()!.querySelector('[aria-busy="true"]')).not.toBeNull();
  await settle(() => pill()?.textContent === '—');
  expect(pill()?.title).toBe("Can't load your balance right now");
  failing = false;
  await setBalance(0n);
  await act(async () => visibility('visible'));
  await settle(() => pill()?.textContent === '0.00 USDL');
  expect(pill()?.title).toBe('USDL balance 0.00 USDL');
  expect(pill()?.getAttribute('aria-label')).toBe('USDL balance 0.00 USDL, open account menu');
  failing = true;
  await act(async () => visibility('visible'));
  await settle(() => pill()?.textContent === '—');
  expect(pill()?.title).toBe("Can't load your balance right now");
}

async function balanceBelongsOnlyToTheCurrentSession() {
  const { config, pill, container } = await renderHeader(false);
  expect(pill()).toBeNull();
  expect(calls).toEqual([]);
  held = true;
  await act(async () => installNavigationSession());
  await settle(() => calls.length === 1);
  await act(async () => clearAccountSession(false));
  expect(pill()).toBeNull();
  expect(container.textContent).toContain('Sign in');
  held = false;
  for (const forward of release.splice(0)) forward();
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 50)); });
  expect(pill()).toBeNull();
  await act(async () => installNavigationSession());
  await settle(() => pill()?.textContent === '36.36 USDL');
  held = true;
  await setBalance(40_000_000n);
  const before = calls.length;
  await act(async () => installNavigationSession());
  expect(pill()?.textContent).not.toContain('36.36 USDL');
  await settle(() => calls.length > before);
  held = false;
  for (const forward of release.splice(0)) forward();
  await settle(() => pill()?.textContent === '40.00 USDL');
  await act(async () => { await disconnect(config); });
  expect(pill()).toBeNull();
  expect(container.textContent).not.toContain('40.00 USDL');
}

async function balancePollsOnlyWhileVisible() {
  const { pill } = await renderHeader();
  await settle(() => pill()?.textContent === '36.36 USDL');
  await setBalance(37_000_000n);
  await settle(() => pill()?.textContent === '37.00 USDL', 17_000);
  expect(calls).toHaveLength(2);
  await act(async () => visibility('hidden'));
  await setBalance(38_000_000n);
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 15_100)); });
  expect(calls).toHaveLength(2);
  expect(pill()?.textContent).toBe('37.00 USDL');
  await act(async () => visibility('visible'));
  await settle(() => pill()?.textContent === '38.00 USDL');
  expect(calls).toHaveLength(3);
}

test(pillShowsTheUsdlBalanceOfTheWallet.name, pillShowsTheUsdlBalanceOfTheWallet);
test(pillSaysUsdlBalanceInTitleAndLabel.name, pillSaysUsdlBalanceInTitleAndLabel);
test(pillShowsADashWhenTheReadFails.name, pillShowsADashWhenTheReadFails, 30_000);
test(balanceBelongsOnlyToTheCurrentSession.name, balanceBelongsOnlyToTheCurrentSession);
test(balancePollsOnlyWhileVisible.name, balancePollsOnlyWhileVisible, 40_000);
