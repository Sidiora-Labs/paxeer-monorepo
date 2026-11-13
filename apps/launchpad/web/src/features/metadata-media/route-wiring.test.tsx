import type { IncomingHttpHeaders } from 'node:http';
import { createServer } from 'node:https';
import { getCACertificates, setDefaultCACertificates } from 'node:tls';
import type { AddressInfo } from 'node:net';
import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import { createHash, randomUUID } from 'node:crypto';
import { EventEmitter } from 'node:events';
import React, { act, type ReactNode } from 'react';
import { expect, test } from 'vitest';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import { hexToBytes, recoverMessageAddress, type EIP1193Provider, type Hex } from 'viem';
import type { MetadataV2Data } from '@/core/clients/metadata';
import type { FrozenAsset } from './frozen-adapter';
import {
  metadataBrowserLabels, recordMetadataRequest, recordMetadataResponse,
  captureCommittedImage, captureViewerImage,
} from './measure.browser.mjs';

interface BrowserDom { window: Window & typeof globalThis & { close(): void }; reconfigure(options: { url: string }): void }
const { JSDOM } = createRequire(import.meta.url)('jsdom') as {
  JSDOM: new (html: string, options: { url: string; pretendToBeVisual: boolean }) => BrowserDom;
};
type NetworkRow = Parameters<typeof captureCommittedImage>[0][number];
interface RequestRow { method: string; url: string; headers: IncomingHttpHeaders; body: string }

async function metadata_routes_connect_editor_and_viewer() {
  const owner = privateKeyToAccount(generatePrivateKey());
  const wire = JSON.parse(readFileSync(`${__dirname}/../../core/test-fixtures/market-production/core-api-wire.json`, 'utf8'));
  const detail = wire.v2Reads.responses.poolDetailReal;
  const poolAddress: string = detail.body.data.pool.poolAddress;
  const tokenAddress: string = detail.body.data.token.address;
  expect(tokenAddress).not.toBe(poolAddress);
  const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a1foAAAAASUVORK5CYII=', 'base64');
  const digest = createHash('sha256').update(png).digest('hex');
  const variants = Object.fromEntries(['256', '512', '1024'].map(size => [size, `https://images.example/approved/${digest}/${size}/${'b'.repeat(64)}.png`]));
  let metadata: MetadataV2Data = {
    token: { ...detail.body.data.token, logoAssetId: null, metadataRevision: '7' },
    description: '', displayName: null, displaySymbol: null, revision: '7', tags: [], socialLinks: [],
  };
  let asset: FrozenAsset | null = null;
  let origin = '', chainId = 125, outage = false, uploadCount = 0;
  let releaseCancelled: (() => void) | undefined;
  let loginMessage = '', loggedIn = false;
  const network: NetworkRow[] = [], requests: RequestRow[] = [], failures: unknown[] = [], signatures: string[] = [];
  const envelope = (data: unknown) => ({ data, generatedAt: new Date().toISOString(), nextCursor: null, quality: 'complete', requestId: randomUUID(), revision: null, watermark: null });
  const sessionId = randomUUID(), challengeId = randomUUID(), accessToken = randomUUID();
  const session = () => ({ sessionId, wallet: owner.address.toLowerCase(), deviceLabel: 'Web browser', scopes: ['account'], accessExpiresAt: new Date(Date.now() + 900000).toISOString(), refreshExpiresAt: new Date(Date.now() + 86400000).toISOString(), revokedAt: null });
  const certificateDirectory = `${__dirname}/../../core/test-fixtures/authority-production`;
  const key = readFileSync(`${certificateDirectory}/loopback-key.pem.txt`);
  const cert = readFileSync(`${certificateDirectory}/loopback-cert.pem.txt`);
  const trustedCertificates = getCACertificates();
  setDefaultCACertificates([...trustedCertificates, cert.toString()]);
  const server = createServer({ key, cert }, async (request, response) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const bytes = Buffer.concat(chunks);
      const row = { method: request.method ?? 'GET', url: origin + request.url, headers: request.headers, body: bytes.toString('utf8') };
      requests.push(row);
      recordMetadataRequest(network, row);
      const path = new URL(row.url).pathname;
      const send = (status: number, body: unknown) => {
        recordMetadataResponse(network, { ...row, status, body, failure: undefined });
        response.writeHead(status, { 'Content-Type': 'application/json', 'X-Schema-Version': '2' }).end(JSON.stringify(body));
      };
      if (path === '/') {
        const rpc = JSON.parse(row.body);
        let result: unknown;
        if (rpc.method === 'eth_chainId') result = `0x${chainId.toString(16)}`;
        else if (rpc.method === 'eth_accounts' || rpc.method === 'eth_requestAccounts') result = [owner.address];
        else if (rpc.method === 'eth_getCode' && rpc.params[0].toLowerCase() === owner.address.toLowerCase()) result = '0x';
        else if (rpc.method === 'personal_sign') {
          expect(rpc.params[1].toLowerCase()).toBe(owner.address.toLowerCase());
          const message = new TextDecoder().decode(hexToBytes(rpc.params[0]));
          signatures.push(message);
          result = await owner.signMessage({ message });
        } else {
          send(200, { jsonrpc: '2.0', id: rpc.id, error: { code: -32601, message: 'Method is not available on this signing service' } });
          return;
        }
        send(200, { jsonrpc: '2.0', id: rpc.id, result });
        return;
      }
      if (path === '/v2/auth/challenges' && row.method === 'POST') {
        const input = JSON.parse(row.body);
        expect(input.wallet).toBe(owner.address.toLowerCase());
        expect(input.origin).toBe(origin);
        loginMessage = `Authorize ${input.wallet} on ${origin} with nonce ${randomUUID()}`;
        send(201, envelope({ challenge: { challengeId, wallet: input.wallet, nonce: 'ab'.repeat(32), domain: new URL(origin).host, uri: origin, chainId: '125', purpose: 'login', issuedAt: new Date().toISOString(), expiresAt: new Date(Date.now() + 300000).toISOString(), message: loginMessage } }));
        return;
      }
      if (path === '/v2/auth/sessions' && row.method === 'POST') {
        const input = JSON.parse(row.body);
        expect(input.challengeId).toBe(challengeId);
        expect(input.proof.kind).toBe('eoa_eip191');
        expect((await recoverMessageAddress({ message: loginMessage, signature: input.proof.signature })).toLowerCase()).toBe(owner.address.toLowerCase());
        loggedIn = true;
        send(200, envelope({ session: session(), accessToken }));
        return;
      }
      if (path.startsWith(`/v2/tokens/${tokenAddress}/assets/`) && row.method === 'PUT') {
        expect(loggedIn).toBe(true);
        expect(row.headers.authorization).toBe(`Bearer ${accessToken}`);
        expect(row.headers['content-type']).toBe('image/png');
        expect(bytes).toEqual(png);
        const operation = path.split('/').at(-1)!;
        expect(row.headers['idempotency-key']).toBe(operation);
        if (asset) expect(asset.assetId).toBe(operation);
        asset = { assetId: operation, bytes: bytes.length, mediaType: 'image/png', ownerWallet: owner.address.toLowerCase(), sha256: '0x' + digest, state: 'approved', publicUrl: variants['512'] };
        uploadCount++;
        if (uploadCount === 1) { request.socket.destroy(); return; }
        if (uploadCount === 2) { releaseCancelled = () => send(200, envelope({ asset })); return; }
        send(200, envelope({ asset }));
        return;
      }
      if (path === `/v2/tokens/${tokenAddress}/metadata`) {
        if (outage) { send(503, { code: 'DEPENDENCY_UNAVAILABLE', retryable: false }); return; }
        if (row.method === 'PATCH') {
          const patch = JSON.parse(row.body);
          expect(patch.expectedRevision).toBe(metadata.revision);
          expect(row.headers.authorization).toBe(`Bearer ${accessToken}`);
          expect(row.headers['x-action-wallet']).toBe(owner.address.toLowerCase());
          expect(row.headers['x-action-audience']).toBe(origin);
          const statement = ['KindleLaunch metadata action v2', origin, '125', owner.address.toLowerCase(), row.headers['idempotency-key'], 'PATCH', path, createHash('sha256').update(bytes).digest('hex'), row.headers['x-action-nonce'], row.headers['x-action-expires']].join('\n');
          expect((await recoverMessageAddress({ message: statement, signature: row.headers['x-action-signature'] as Hex })).toLowerCase()).toBe(owner.address.toLowerCase());
          expect(signatures.at(-1)).toBe(statement);
          expect(Number(row.headers['x-action-expires'])).toBeGreaterThan(Date.now() / 1000);
          const revision = (BigInt(metadata.revision) + 1n).toString();
          if (patch.logoAssetId) expect(patch.logoAssetId).toBe(asset?.assetId);
          metadata = { ...metadata, ...patch, revision, token: { ...metadata.token, metadataRevision: revision, logoAssetId: patch.logoAssetId ?? metadata.token.logoAssetId } };
          // Patch-only fields belong to the request, not MetadataGetData.
          metadata = { token: metadata.token, displayName: metadata.displayName, displaySymbol: metadata.displaySymbol, description: metadata.description, tags: metadata.tags, socialLinks: metadata.socialLinks, revision };
        }
        send(200, envelope(metadata));
        return;
      }
      if (path === `/v2/tokens/${tokenAddress}/media`) {
        if (outage) { send(503, { code: 'DEPENDENCY_UNAVAILABLE', retryable: false }); return; }
        send(200, envelope({ metadata: { chainId: 125, poolAddress, tokenAddress, revision: metadata.revision, updatedAt: null, source: 'chain', moderationStatus: 'allowed', logoAssetId: metadata.token.logoAssetId, display: { name: null, symbol: null, description: metadata.description, tags: [], socials: [] } }, image: asset ? { assetId: asset.assetId, sha256: asset.sha256, mime: 'image/png', width: 1, height: 1, variants } : null }));
        return;
      }
      if (path === `/v2/pools/${poolAddress}`) { send(detail.status, detail.body); return; }
      if (path === `/v2/pools/${poolAddress}/stats`) { send(wire.stats.status, wire.stats.body); return; }
      // Other panels see a real unavailable service, without replacing their components or hooks.
      send(503, { code: 'DEPENDENCY_UNAVAILABLE', message: 'Service unavailable', retryable: false });
    } catch (error) {
      failures.push(error);
      response.writeHead(500).end(JSON.stringify({ code: 'CONTRACT_VIOLATION' }));
    }
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  origin = `https://127.0.0.1:${(server.address() as AddressInfo).port}`;
  const dom = new JSDOM('<!doctype html><html lang="en"><body><main id="root"></main></body></html>', { url: origin, pretendToBeVisual: true });
  const savedGlobals = new Map<string, PropertyDescriptor | undefined>();
  const install = (key: string, value: unknown) => { if (!savedGlobals.has(key)) savedGlobals.set(key, Object.getOwnPropertyDescriptor(globalThis, key)); Object.defineProperty(globalThis, key, { value, configurable: true, writable: true }); };
  for (const key of Object.getOwnPropertyNames(dom.window)) {
    if (key in globalThis || key.startsWith('_')) continue;
    const value = (dom.window as unknown as Record<string, unknown>)[key];
    install(key, typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value);
  }
  install('window', dom.window); install('document', dom.window.document); install('IS_REACT_ACT_ENVIRONMENT', true);
  install('CustomEvent', dom.window.CustomEvent);
  const environment = ['DATA_API_URL', 'GATEWAY_URL', 'METADATA_API_URL', 'PNL_API_URL', 'EXPLORER_API', 'PAXEER_RPC_URL', 'STREAM_ORIGIN'].map(name => `NEXT_PUBLIC_${name}`);
  const savedEnvironment = new Map(environment.map(name => [name, process.env[name]]));
  for (const name of environment) process.env[name] = name.endsWith('STREAM_ORIGIN') ? origin.replace('https:', 'wss:') : origin;
  const { createRoot } = await import('react-dom/client');
  const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
  const { WagmiProvider, createConfig, http } = await import('wagmi');
  const { connect, disconnect, signMessage } = await import('wagmi/actions');
  const { injected } = await import('wagmi/connectors');
  const { PathParamsContext } = await import('next/dist/shared/lib/hooks-client-context.shared-runtime');
  const { WalletRuntimeProvider } = await import('@/core/wallet-sdk/runtime');
  const { MarketClientProvider } = await import('@/page-modules/token/market-session-context');
  const { SdkClient } = await import('@kindle/sdk');
  const { default: TokenMetadataPage } = await import('@/app/tokens/[tokenAddress]/metadata/page');
  const { default: CoinPage } = await import('@/app/coin/[address]/page');
  const auth = await import('@/widgets/profile/account-transport');
  dom.reconfigure({ url: `${origin}/tokens/${tokenAddress}/metadata` });
  const events = new EventEmitter();
  const provider = {
    on: events.on.bind(events), removeListener: events.removeListener.bind(events),
    async request(args: { method: string; params?: unknown }) {
      const result = await fetch(origin, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ jsonrpc: '2.0', id: 1, ...args }) });
      const body = await result.json();
      if (body.error) throw Object.assign(new Error(body.error.message), { code: body.error.code });
      return body.result;
    },
  } as EIP1193Provider;
  const chain = { id: 125, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: [origin] } } } as const;
  const config = createConfig({ chains: [chain], connectors: [injected({ target: { id: 'loopback-signer', name: 'Loopback signer', provider } })], transports: { 125: http(origin, { retryCount: 0 }) }, multiInjectedProviderDiscovery: false });
  const queries = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const stream = origin.replace('https:', 'wss:');
  const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: stream, approvedStreamOrigins: [stream], production: false });
  const root = createRoot(document.getElementById('root')!);
  const render = async (route: ReactNode, token = tokenAddress) => act(async () => root.render(
    <WagmiProvider config={config} reconnectOnMount={false}><QueryClientProvider client={queries}><WalletRuntimeProvider><MarketClientProvider client={client}><PathParamsContext.Provider value={{ tokenAddress: token }}>{route}</PathParamsContext.Provider></MarketClientProvider></WalletRuntimeProvider></QueryClientProvider></WagmiProvider>,
  ));
  const waitFor = async (check: () => boolean, message: string) => {
    const deadline = Date.now() + 5000;
    while (!check() && Date.now() < deadline) await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
    expect(check(), `${message}: ${document.body.textContent?.slice(-1500)}; service failures: ${failures.map(String).join(', ')}`).toBe(true);
  };
  const region = (name: string) => document.querySelector<HTMLElement>(`section[aria-label="${name}"]`);
  const button = (element: ParentNode, name: string) => Array.from(element.querySelectorAll('button')).find(value => value.textContent === name)!;
  try {
    await render(<TokenMetadataPage />);
    await waitFor(() => document.body.textContent!.includes('Connect the creator wallet'), 'disconnected route');
    expect(region(metadataBrowserLabels.editor)).toBeNull();
    expect(uploadCount).toBe(0);
    await act(async () => { await connect(config, { connector: config.connectors[0]! }); });
    await waitFor(() => document.body.textContent!.includes('Sign in with the creator wallet'), 'connection alone does not authorize image upload');
    expect(region(metadataBrowserLabels.editor)).toBeNull();
    const challenge = await auth.loginChallenge(owner.address, new AbortController().signal);
    const signature = await signMessage(config, { message: challenge.message });
    const login = await auth.loginProof(challenge.challengeId, owner.address.toLowerCase(), signature, 'eoa_eip191', new AbortController().signal);
    await act(async () => auth.installAccountSession(login.data.session, login.data.accessToken, auth.accountEpoch()));
    await waitFor(() => region(metadataBrowserLabels.editor) !== null, 'authenticated route editor');
    const editor = region(metadataBrowserLabels.editor)!;
    const label = Array.from(editor.querySelectorAll('label')).find(value => value.textContent === metadataBrowserLabels.chooser)!;
    const chooser = document.getElementById(label.htmlFor) as HTMLInputElement;
    expect(chooser.type).toBe('file');
    const alert = document.getElementById(chooser.getAttribute('aria-describedby')!)!;
    expect(alert.getAttribute('role')).toBe('alert');
    const upload = button(editor, metadataBrowserLabels.upload), cancel = button(editor, 'Cancel');
    expect(upload.disabled).toBe(true);
    Object.defineProperty(chooser, 'files', { value: [new File([new Uint8Array(png)], 'coin.png', { type: 'image/png' })] });
    await act(async () => { chooser.dispatchEvent(new dom.window.Event('change', { bubbles: true })); });
    chooser.focus(); expect(document.activeElement).toBe(chooser);
    upload.focus(); expect(document.activeElement).toBe(upload);
    expect(upload.disabled).toBe(false);
    await act(async () => upload.click());
    await waitFor(() => alert.textContent === metadataBrowserLabels.uploadUnavailable, 'network failure remains actionable');
    expect(chooser.getAttribute('aria-invalid')).toBe('true');
    expect(editor.textContent).not.toContain(metadataBrowserLabels.updated);
    await act(async () => upload.click());
    await waitFor(() => uploadCount === 2, 'retry reaches real HTTP service');
    const puts = requests.filter(row => row.method === 'PUT');
    expect(puts[0].headers['idempotency-key']).toBe(puts[1].headers['idempotency-key']);
    cancel.focus(); expect(document.activeElement).toBe(cancel);
    expect(cancel.disabled).toBe(false);
    await act(async () => cancel.click());
    expect(alert.textContent).toBe(metadataBrowserLabels.cancelled);
    await act(async () => { releaseCancelled!(); await new Promise(resolve => setTimeout(resolve, 10)); });
    expect(editor.textContent).not.toContain(metadataBrowserLabels.updated);
    expect(requests.filter(row => row.method === 'PATCH')).toHaveLength(0);
    await act(async () => upload.click());
    await waitFor(() => metadata.revision === '8', 'signed metadata commit');
    await waitFor(() => editor.querySelector('[role="status"]')?.textContent === metadataBrowserLabels.updated, 'route confirms approved image');
    expect(editor.textContent).not.toContain(metadataBrowserLabels.pending);
    const operation = asset!.assetId;
    const approval = captureCommittedImage(network, tokenAddress, operation, digest);
    expect(approval.approved_revision).toBe('8');
    expect(approval.metadata_operation_id).not.toBe(operation);
    expect(uploadCount).toBe(3);
    expect(signatures).toHaveLength(2);
    expect(region(metadataBrowserLabels.viewer)?.querySelector('textarea')).not.toBeNull();

    chainId = 1;
    await act(async () => { events.emit('chainChanged', '0x1'); });
    await waitFor(() => document.body.textContent!.includes('Switch your wallet to Paxeer'), 'wrong chain route');
    expect(region(metadataBrowserLabels.editor)).toBeNull();
    expect(uploadCount).toBe(3);
    chainId = 125;
    await act(async () => { events.emit('chainChanged', '0x7d'); });
    await waitFor(() => document.body.textContent!.includes('Sign in with the creator wallet'), 'chain change invalidates session');
    expect(region(metadataBrowserLabels.editor)).toBeNull();

    dom.reconfigure({ url: `${origin}/coin/${poolAddress}` });
    const coin = await CoinPage({ params: Promise.resolve({ address: poolAddress }) });
    await render(coin);
    await waitFor(() => !!region(metadataBrowserLabels.viewer)?.querySelector(`img[alt="${metadataBrowserLabels.image}"]`), 'coin route consumes committed image');
    expect(document.querySelectorAll(`section[aria-label="${metadataBrowserLabels.viewer}"]`)).toHaveLength(1);
    const viewer = region(metadataBrowserLabels.viewer)!;
    const image = viewer.querySelector('img')!;
    expect(image.getAttribute('width')).toBe('1'); expect(image.getAttribute('height')).toBe('1');
    expect(image.getAttribute('srcset')).toBe(`${variants['256']} 256w, ${variants['512']} 512w, ${variants['1024']} 1024w`);
    expect(image.getAttribute('sizes')).toBe('(max-width: 512px) 100vw, 512px');
    expect(viewer.textContent).toContain(tokenAddress);
    expect(captureViewerImage(network, tokenAddress, approval, digest, image.src)).toEqual({ revision: '8', asset_id: operation });
    expect(requests.some(row => row.url.includes(`/tokens/${poolAddress}/`))).toBe(false);
    await waitFor(() => !!viewer.querySelector('[aria-label="Trading"] button') && !!viewer.querySelector('[data-mobile-trade-cta] button'), 'real desktop and mobile controls load');
    for (const width of [390, 1440]) {
      Object.defineProperty(dom.window, 'innerWidth', { value: width, configurable: true });
      await act(async () => dom.window.dispatchEvent(new dom.window.Event('resize')));
      const trading = viewer.querySelector('[aria-label="Trading"]')!;
      expect(trading.querySelector('.lg\\:block')).not.toBeNull();
      const mobile = trading.querySelector('[data-mobile-trade-cta]')!;
      expect(mobile.className).toContain('lg:hidden');
      expect(button(mobile, 'Sign in to trade').disabled).toBe(false);
    }
    outage = true;
    await act(async () => { await queries.invalidateQueries({ queryKey: ['metadata-v2', 125, tokenAddress] }); await queries.invalidateQueries({ queryKey: ['metadata-media-v2', 125, tokenAddress] }); });
    await waitFor(() => viewer.textContent!.includes(metadataBrowserLabels.outage), 'route metadata outage');
    expect(viewer.textContent).toContain(tokenAddress);
    expect(viewer.querySelector('img')).toBeNull();
    expect(document.querySelectorAll(`section[aria-label="${metadataBrowserLabels.viewer}"]`)).toHaveLength(1);
    const trading = viewer.querySelector('[aria-label="Trading"]')!;
    const sell = button(trading, 'Sell');
    expect(sell.disabled).toBe(false);
    await act(async () => sell.click());
    expect(trading.querySelector('[aria-label="Portion of holdings to sell"]')).not.toBeNull();
    const mobile = trading.querySelector('[data-mobile-trade-cta]')!;
    expect(button(mobile, 'Sign in to trade').disabled).toBe(false);
    expect(network.some(row => row.event === 'response' && row.status === 503 && row.url.endsWith('/metadata'))).toBe(true);
    expect(failures).toEqual([]);
  } finally {
    await act(async () => root.unmount());
    queries.clear(); auth.clearAccountSession(false);
    await disconnect(config);
    for (const [key, value] of savedGlobals) { if (value) Object.defineProperty(globalThis, key, value); else Reflect.deleteProperty(globalThis, key); }
    for (const [key, value] of savedEnvironment) { if (value === undefined) delete process.env[key]; else process.env[key] = value; }
    dom.window.close();
    server.closeAllConnections();
    await new Promise<void>(resolve => server.close(() => resolve()));
    setDefaultCACertificates(trustedCertificates);
  }
}

test(metadata_routes_connect_editor_and_viewer, { timeout: 120000 }, metadata_routes_connect_editor_and_viewer);
