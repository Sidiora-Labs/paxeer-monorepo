// @vitest-environment jsdom

import { createServer, type RequestListener, type Server } from 'node:http';
import { NextRequest } from 'next/server';
import { verifyMessage, type Hex } from 'viem';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { backendConfigurationProblems, walletParentOrigins } from './backend-origins';
import { installPostMessageEip1193Provider } from './post-message-eip1193';
import { getPaxeerRpcUrls, RpcConfigurationError } from './rpc';
import { redactedErrorPayload } from './report-error';
import {
  WalletParentFixture,
  walletMessageTarget,
  walletParentSigner,
} from './test-fixtures/post-message-parent';

// Register the unchanged private-fixture refusals and the production replay cases
// in the task's single focused gate.
import './runtime-authority-production.test';

declare const jsdom: { reconfigure(options: { url: string }): void };
const browserWindow = window;
const frames: HTMLIFrameElement[] = [];
const peers: WalletParentFixture[] = [];
const providers: NonNullable<ReturnType<typeof installPostMessageEip1193Provider>>[] = [];
const servers: Server[] = [];
const RPC_VARIABLES = [
  'NEXT_PUBLIC_RPC_URL',
  'NEXT_PUBLIC_RPC_URL_FALLBACK',
  'NEXT_PUBLIC_PAXEER_RPC_URL',
] as const;

beforeEach(() => {
  vi.stubEnv('NODE_ENV', 'production');
  for (const variable of RPC_VARIABLES) vi.stubEnv(variable, undefined);
  for (const variable of [
    'NEXT_PUBLIC_DATA_API_URL',
    'NEXT_PUBLIC_GATEWAY_URL',
    'NEXT_PUBLIC_METADATA_API_URL',
    'NEXT_PUBLIC_PNL_API_URL',
  ])
    vi.stubEnv(variable, 'https://backend.example');
  for (const variable of [
    'NEXT_PUBLIC_STATS_API_URL',
    'NEXT_PUBLIC_LIVESTREAM_API_URL',
    'NEXT_PUBLIC_STREAM_ORIGIN',
    'NEXT_PUBLIC_PRIVATE_API_ORIGIN',
    'NEXT_PUBLIC_PRIVATE_STREAM_ORIGIN',
    'NEXT_PUBLIC_SUPABASE_URL',
    'NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY',
    'NEXT_PUBLIC_WALLETCONNECT_PROJECT_ID',
    'NEXT_PUBLIC_WALLET_IFRAME_ORIGIN',
    'NEXT_PUBLIC_SENTRY_DSN',
  ])
    vi.stubEnv(variable, undefined);
  jsdom.reconfigure({ url: 'https://wallet.example/' });
});

afterEach(async () => {
  for (const provider of providers.splice(0)) provider.destroy();
  for (const peer of peers.splice(0)) peer.close();
  for (const frame of frames.splice(0)) frame.remove();
  for (const server of servers.splice(0)) {
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve())),
    );
  }
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

function frameAt(url: string): Window {
  const frame = browserWindow.document.createElement('iframe');
  frame.src = url;
  browserWindow.document.body.append(frame);
  frames.push(frame);
  if (!frame.contentWindow) throw new Error('Frame window unavailable');
  return frame.contentWindow;
}

function embeddedWallet(allowedParentOrigins?: string[], handshake = true) {
  const child = frameAt('https://launchpad.example/embed');
  const peer = new WalletParentFixture(child.parent, child, handshake);
  peers.push(peer);
  vi.stubGlobal('window', child);
  const provider = installPostMessageEip1193Provider(
    allowedParentOrigins ? { allowedParentOrigins } : undefined,
  );
  if (!provider) throw new Error('Wallet provider unavailable');
  providers.push(provider);
  return { provider, peer, child };
}

async function walletParentConfigurationRejectsInvalidOriginsAndFeedsCspAndBridge() {
  for (const invalid of [
    'http://wallet.example',
    'https://user:secret@wallet.example',
    'https://wallet.example/path',
    'https://wallet.example/a/..',
    'https://wallet.example?code=secret',
    'https://wallet.example#token',
    'file:///wallet',
    'data:text/plain,wallet',
    'broken',
    'https://wallet.example,,https://other.example',
  ]) {
    vi.stubEnv('NEXT_PUBLIC_WALLET_IFRAME_ORIGIN', invalid);
    expect(() => walletParentOrigins(), invalid).toThrow('NEXT_PUBLIC_WALLET_IFRAME_ORIGIN');
    expect(backendConfigurationProblems().map((problem) => problem.variable)).toContain(
      'NEXT_PUBLIC_WALLET_IFRAME_ORIGIN',
    );
  }
  vi.stubEnv(
    'NEXT_PUBLIC_WALLET_IFRAME_ORIGIN',
    ' https://other.example/, https://wallet.example, https://wallet.example/ ',
  );
  expect(walletParentOrigins()).toEqual(['https://other.example', 'https://wallet.example']);
  const { middleware } = await import('../middleware');
  const policy = middleware(new NextRequest('https://launchpad.example/')).headers.get(
    'Content-Security-Policy',
  )!;
  for (const directive of ['frame-ancestors', 'frame-src']) {
    const values = policy
      .split(';')
      .map((value) => value.trim())
      .find((value) => value.startsWith(`${directive} `))!
      .split(/\s+/);
    expect(values.filter((value) => value.startsWith('https://'))).toEqual(walletParentOrigins());
  }
  const { provider, peer } = embeddedWallet();
  await peer.ready;
  expect(await provider.request({ method: 'eth_accounts' })).toEqual([walletParentSigner.address]);
  expect(backendConfigurationProblems()).toEqual([]);
  vi.stubEnv('NODE_ENV', 'development');
  vi.stubEnv('NEXT_PUBLIC_WALLET_IFRAME_ORIGIN', 'http://localhost:3050');
  expect(walletParentOrigins()).toEqual(['http://localhost:3050']);
}

async function postmessageDestroySettlesSigningExactlyOnceAndCanReopen() {
  const { provider, peer } = embeddedWallet(['https://wallet.example']);
  await peer.ready;
  peer.holdSigning = true;
  let settled = 0;
  const signing = provider
    .request({
      method: 'personal_sign',
      params: ['Approve this action', walletParentSigner.address],
    })
    .then(
      (value) => {
        settled++;
        return value;
      },
      (error) => {
        settled++;
        return error as Error & { code: number };
      },
    );
  await expect.poll(() => peer.requests.length).toBe(1);
  provider.destroy();
  provider.destroy();
  expect(await signing).toMatchObject({
    code: 4900,
    message: 'postMessage provider is disconnected',
  });
  await peer.answer(peer.requests[0]);
  await expect(provider.request({ method: 'eth_accounts' })).rejects.toMatchObject({ code: 4900 });
  expect(settled).toBe(1);
  expect(peer.requests).toHaveLength(1);
  peer.holdSigning = false;
  const reopened = installPostMessageEip1193Provider({
    allowedParentOrigins: ['https://wallet.example'],
  });
  if (!reopened) throw new Error('Reopened provider unavailable');
  providers.push(reopened);
  expect(reopened).not.toBe(provider);
  await expect
    .poll(async () => {
      try {
        return await reopened.request({ method: 'eth_accounts' });
      } catch {
        return undefined;
      }
    })
    .toEqual([walletParentSigner.address]);
  expect(settled).toBe(1);
}

async function postmessageSenderRejectsSiblingResponsesAndMalformedEvents() {
  const { provider, peer } = embeddedWallet(['https://wallet.example', 'https://other.example']);
  await peer.ready;
  peer.holdSigning = true;
  const sibling = frameAt('https://wallet.example/sibling');
  const events: unknown[] = [];
  provider.on('accountsChanged', (accounts) => events.push(accounts));
  let settled = false;
  const message = 'Sign through the admitted parent';
  const signing = provider
    .request({ method: 'personal_sign', params: [message, walletParentSigner.address] })
    .then((value) => {
      settled = true;
      return value;
    });
  await expect.poll(() => peer.requests.length).toBe(1);
  const id = peer.requests[0].id;
  const forged = { target: walletMessageTarget, type: 'response', id, result: 'forged' };
  peer.send(forged, sibling);
  peer.accountsChanged(sibling);
  peer.send(forged, peer.parent, 'https://other.example');
  peer.accountsChanged(peer.parent, 'https://other.example');
  for (const data of [
    { ...forged, id: [id] },
    { ...forged, error: { code: 4001, message: 'Rejected' } },
    {
      target: walletMessageTarget,
      type: 'response',
      id,
      error: { code: '4001', message: 'Rejected' },
    },
    {
      target: walletMessageTarget,
      type: 'event',
      event: 'accountsChanged',
      params: ['invalid-account'],
    },
    {
      target: walletMessageTarget,
      type: 'event',
      event: 'accountsChanged',
      params: walletParentSigner.address,
    },
    {
      target: walletMessageTarget,
      type: 'event',
      event: 'chainChanged',
      params: { chainId: '0x7d' },
    },
  ])
    peer.send(data);
  await Promise.resolve();
  expect(settled).toBe(false);
  expect(events).toEqual([]);
  peer.accountsChanged();
  expect(events).toEqual([[walletParentSigner.address]]);
  await peer.answer(peer.requests[0]);
  const signature = await signing;
  expect(
    await verifyMessage({
      address: walletParentSigner.address,
      message,
      signature: signature as Hex,
    }),
  ).toBe(true);
  peer.send({ target: walletMessageTarget, type: 'response', id, result: 'duplicate' });
  expect(await signing).toBe(signature);
}

async function postmessageOutboundOriginPinsAdmittedParent() {
  const admitted = ['https://other.example', 'https://wallet.example'];
  const { provider, peer } = embeddedWallet(admitted, false);
  await peer.ready;
  await expect(
    provider.request({ method: 'personal_sign', params: ['secret approval'] }),
  ).rejects.toMatchObject({ code: 4900 });
  expect(peer.requests).toEqual([]);
  const sibling = frameAt('https://wallet.example/opener');
  peer.send({ target: walletMessageTarget, type: 'ready' }, sibling);
  await expect(provider.request({ method: 'eth_accounts' })).rejects.toMatchObject({ code: 4900 });
  peer.send({ target: walletMessageTarget, type: 'ready' });
  peer.send({ target: walletMessageTarget, type: 'ready' }, peer.parent, admitted[0]);
  expect(await provider.request({ method: 'eth_accounts' })).toEqual([walletParentSigner.address]);
  expect(() =>
    installPostMessageEip1193Provider({
      allowedParentOrigins: admitted,
      targetOrigin: admitted[0],
    }),
  ).toThrow('conflicts');
  // Navigation preserves the parent Window identity. The pinned targetOrigin
  // must still prevent disclosure, even to another entry in the allowlist.
  jsdom.reconfigure({ url: `${admitted[0]}/` });
  const afterNavigation = provider.request({
    method: 'personal_sign',
    params: ['private after navigation'],
  });
  const disconnected = expect(afterNavigation).rejects.toMatchObject({ code: 4900 });
  await new Promise((resolve) => setTimeout(resolve, 0));
  expect(peer.requests).toHaveLength(1);
  provider.destroy();
  await disconnected;
  expect(() =>
    installPostMessageEip1193Provider({ allowedParentOrigins: admitted, targetOrigin: '*' }),
  ).toThrow();
  expect(() =>
    installPostMessageEip1193Provider({
      allowedParentOrigins: admitted,
      targetOrigin: 'https://untrusted.example',
    }),
  ).toThrow('not an admitted parent');

  // CSP permits self framing, but this parent is absent from the wallet list.
  jsdom.reconfigure({ url: 'https://launchpad.example/' });
  const outside = embeddedWallet(admitted);
  await expect(
    outside.provider.request({ method: 'personal_sign', params: ['must stay private'] }),
  ).rejects.toMatchObject({ code: 4900 });
  await new Promise((resolve) => setTimeout(resolve, 0));
  expect(outside.peer.requests).toEqual([]);
}

async function loopback(handler: RequestListener): Promise<string> {
  const server = createServer(handler);
  servers.push(server);
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Loopback server unavailable');
  return `http://127.0.0.1:${address.port}`;
}

async function telemetryRedactionProtectsAuthCallbackTransports() {
  const secret = 'oauthCredentialThatMustNeverLeaveThePage';
  jsdom.reconfigure({
    url: `https://launchpad.example/auth/callback?code=${secret}#access_token=${secret}`,
  });
  const raw = new Error(
    `Bearer ${secret} failed at https://launchpad.example/auth/callback?code=${secret}`,
  );
  raw.stack = `Error: ${secret}\n at callback?refresh_token=${secret}:1:1`;
  const context = {
    area: 'auth-callback',
    code: 'unauthorized',
    action: secret,
    accessToken: secret,
    nested: { secret },
    correlationId: '12345678-1234-4234-9234-123456789abc',
  };
  const payload = redactedErrorPayload(raw, context);
  const serialized = JSON.stringify(payload);
  expect(serialized).not.toContain(secret);
  expect(payload.url).toBe('https://launchpad.example/auth/callback');
  expect(payload.message).toBe('Error: unauthorized');
  expect(payload).not.toHaveProperty('stack');
  expect(payload.context).toEqual({
    area: 'auth-callback',
    code: 'unauthorized',
    correlationId: context.correlationId,
    deploymentVersion: expect.stringMatching(/^[0-9a-f]{64}$/),
  });
  expect(
    redactedErrorPayload(secret, { area: secret, correlationId: secret }).context,
  ).toMatchObject({
    area: 'widget',
    code: 'render_failed',
    correlationId: expect.stringMatching(/^[0-9a-f-]{36}$/),
  });

  const received: string[] = [];
  const origin = await loopback(async (request, response) => {
    let body = '';
    for await (const chunk of request) body += chunk;
    received.push(body);
    response.writeHead(200, { 'Content-Type': 'application/json' }).end('{}');
  });
  // The exact production beacon codec crosses a real HTTP transport into the sink.
  expect((await fetch(`${origin}/api/errors`, { method: 'POST', body: serialized })).ok).toBe(true);
  expect(JSON.parse(received[0])).toEqual(payload);

  vi.stubEnv('NEXT_PUBLIC_SENTRY_DSN', `${origin.replace('://', '://public@')}/1`);
  vi.resetModules();
  const sentryModule = await import('@sentry/nextjs');
  const Sentry = (
    'default' in sentryModule ? sentryModule.default : sentryModule
  ) as typeof sentryModule;
  Sentry.init({
    dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
    defaultIntegrations: false,
    sendClientReports: false,
    sampleRate: 1,
  });
  try {
    Sentry.setUser({ id: secret });
    Sentry.addBreadcrumb({ message: secret });
    Sentry.addEventProcessor((event) => ({
      ...event,
      request: { url: `https://launchpad.example/?code=${secret}` },
      extra: { secret },
    }));
    const { reportError } = await import('./report-error');
    reportError(raw, context);
    await expect
      .poll(async () => {
        await Sentry.flush(1000);
        return received.length;
      })
      .toBe(2);
    const envelope = received[1];
    expect(envelope).not.toContain(secret);
    expect(envelope).toContain('Error: unauthorized');
    expect(envelope).toContain(context.correlationId);
  } finally {
    await Sentry.close();
    vi.resetModules();
  }
}

async function rpcOverridePolicySelectsValidatedTransport() {
  for (const variable of RPC_VARIABLES) {
    for (const invalid of [
      'http://api1.mainnet-beta.paxeer.network',
      'https://node.hyperpaxeer.com',
      'https://node.hyperpaxeer.com.',
      'https://public-mainnet.rpcpaxeer.online',
      'https://rpc.example/rpc',
      'https://rpc.example/evm',
      'https://rpc.example/rpc/..',
      'https://operator:secret@rpc.example',
      'https://rpc.example?key=secret',
      'https://rpc.example#secret',
      'wss://rpc.example',
      '',
      'malformed',
    ]) {
      vi.stubEnv(variable, invalid);
      expect(() => getPaxeerRpcUrls(), `${variable}=${invalid}`).toThrow(RpcConfigurationError);
      expect(backendConfigurationProblems().map((problem) => problem.variable)).toContain(variable);
    }
    vi.stubEnv(variable, undefined);
  }
  vi.stubEnv('NEXT_PUBLIC_RPC_URL', 'https://api1.mainnet-beta.paxeer.network/');
  vi.stubEnv('NEXT_PUBLIC_RPC_URL_FALLBACK', 'https://api2.mainnet-beta.paxeer.network');
  vi.stubEnv('NEXT_PUBLIC_PAXEER_RPC_URL', 'https://api1.mainnet-beta.paxeer.network');
  expect(getPaxeerRpcUrls()).toEqual([
    'https://api1.mainnet-beta.paxeer.network',
    'https://api2.mainnet-beta.paxeer.network',
  ]);
  const requests: string[] = [];
  const origin = await loopback(async (request, response) => {
    let body = '';
    for await (const chunk of request) body += chunk;
    const rpc = JSON.parse(body) as { id: number; method: string };
    requests.push(rpc.method);
    response
      .writeHead(200, { 'Content-Type': 'application/json' })
      .end(JSON.stringify({ jsonrpc: '2.0', id: rpc.id, result: '0x7d' }));
  });
  vi.stubEnv('NODE_ENV', 'test');
  vi.stubEnv('NEXT_PUBLIC_RPC_URL', origin);
  vi.stubEnv('NEXT_PUBLIC_RPC_URL_FALLBACK', undefined);
  vi.stubEnv('NEXT_PUBLIC_PAXEER_RPC_URL', undefined);
  vi.resetModules();
  const { getPublicClient, paxeerWalletChain } = await import('./wallet-sdk/client');
  const { paxeer } = await import('./wagmi-config');
  expect(paxeerWalletChain.rpcUrls.default.http).toEqual([origin]);
  expect(paxeer.rpcUrls.default.http).toEqual([origin]);
  expect(await getPublicClient().getChainId()).toBe(125);
  expect(requests).toEqual(['eth_chainId']);
  vi.resetModules();
}

test(
  walletParentConfigurationRejectsInvalidOriginsAndFeedsCspAndBridge.name,
  walletParentConfigurationRejectsInvalidOriginsAndFeedsCspAndBridge,
);
test(
  postmessageDestroySettlesSigningExactlyOnceAndCanReopen.name,
  postmessageDestroySettlesSigningExactlyOnceAndCanReopen,
);
test(
  postmessageSenderRejectsSiblingResponsesAndMalformedEvents.name,
  postmessageSenderRejectsSiblingResponsesAndMalformedEvents,
);
test(postmessageOutboundOriginPinsAdmittedParent.name, postmessageOutboundOriginPinsAdmittedParent);
test(
  telemetryRedactionProtectsAuthCallbackTransports.name,
  telemetryRedactionProtectsAuthCallbackTransports,
);
test(rpcOverridePolicySelectsValidatedTransport.name, rpcOverridePolicySelectsValidatedTransport);
