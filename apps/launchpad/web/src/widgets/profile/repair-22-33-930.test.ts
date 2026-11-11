import { createHash } from 'node:crypto';
import { createServer, type IncomingMessage } from 'node:http';
import type { AddressInfo } from 'node:net';
import { createRequire } from 'node:module';
import React, { act, createElement as h, useLayoutEffect } from 'react';
import type { Root } from 'react-dom/client';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';
import { privateKeyToAccount } from 'viem/accounts';
import { verifyMessage } from 'viem';
import type { ConnectedExtendedProfileResponse } from '@kindle/sdk-v2';
import type { AccountAsset, ExtendedProfile, ExtendedProfileBody } from './account-transport';
import type { SeriesPoint } from './portfolio-transport';
import { operationStatement, type OperationEnvelope } from '@/core/operation-envelope';

interface BrowserDom { window: Window & typeof globalThis }
type JsdomModule = { JSDOM: new (html: string, options: { url: string; pretendToBeVisual: boolean }) => BrowserDom };
const { JSDOM } = createRequire(import.meta.url)('jsdom') as JsdomModule;
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { url: 'http://localhost:3000', pretendToBeVisual: true });
const browserGlobals = globalThis as unknown as Record<string, unknown>;
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globalThis || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  browserGlobals[key] = typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
}
browserGlobals.window = dom.window;
browserGlobals.document = dom.window.document;
browserGlobals.IS_REACT_ACT_ENVIRONMENT = true;

const owner = privateKeyToAccount('0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318');
const wallet = owner.address.toLowerCase();
const token = 'profile-repair-access';
const assets = new Map<string, AccountAsset>();
const receivedFiles = new Map<string, Buffer>();
const patches: ExtendedProfileBody[] = [];
let pendingSaveAcknowledgment: Promise<void> | null = null;
let profile: ExtendedProfile;
const originalAvatar = '11111111-1111-4111-8111-111111111111';
const originalBanner = '22222222-2222-4222-8222-222222222222';
function resetProfile(withImages: boolean) {
  profile = { wallet, displayName: 'Original', bio: '', website: null, socials: {}, creatorPools: null, revision: '1', avatarAssetId: withImages ? originalAvatar : null, bannerAssetId: withImages ? originalBanner : null };
  patches.length = 0;
  assets.clear();
  receivedFiles.clear();
}
function profileResponse(): ConnectedExtendedProfileResponse {
  return { data: { profile }, generatedAt: new Date().toISOString(), nextCursor: null, quality: 'complete', requestId: 'profile-repair', revision: profile.revision, watermark: null };
}
async function authorize(request: IncomingMessage, body: string) {
  const encoded = request.headers['x-kindle-operation'];
  const signature = request.headers['x-kindle-operation-signature'];
  if (request.headers.authorization !== `Bearer ${token}` || typeof encoded !== 'string' || typeof signature !== 'string') return false;
  const envelope = JSON.parse(Buffer.from(encoded, 'base64url').toString()) as OperationEnvelope;
  if (envelope.actor !== wallet || envelope.action !== 'profile.update' || envelope.method !== request.method || envelope.path !== request.url || envelope.bodyHash !== createHash('sha256').update(body).digest('hex')) return false;
  return verifyMessage({ address: owner.address, message: operationStatement(envelope), signature: signature as `0x${string}` });
}
const server = createServer(async (request, response) => {
  const reply = (status: number, body: unknown) => {
    response.writeHead(status, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify(body));
  };
  try {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const bytes = Buffer.concat(chunks);
    const path = request.url ?? '';
    if (path === `/v3/profiles/${wallet}` && request.method === 'GET') return reply(200, profileResponse());
    if (path === `/v3/profiles/${wallet}` && request.method === 'PATCH') {
      const body = bytes.toString();
      if (!await authorize(request, body)) return reply(403, { code: 'SCOPE_DENIED' });
      const patch = JSON.parse(body) as ExtendedProfileBody;
      if (patch.expectedRevision !== profile.revision || request.headers['if-match'] !== `"${profile.revision}"`) return reply(412, { code: 'PRECONDITION_FAILED' });
      for (const kind of ['avatarAssetId', 'bannerAssetId'] as const) {
        const id = patch[kind];
        if (id && id !== originalAvatar && id !== originalBanner && assets.get(id)?.state !== 'approved') return reply(400, { code: 'ASSET_NOT_APPROVED' });
      }
      patches.push(patch);
      const { expectedRevision, socials, ...fields } = patch;
      profile = {
        ...profile, ...fields,
        socials: socials ? Object.fromEntries(Object.entries(socials).filter((entry): entry is [string, string] => typeof entry[1] === 'string')) : profile.socials,
        revision: String(BigInt(expectedRevision) + 1n),
      };
      const acknowledgment = profileResponse();
      if (pendingSaveAcknowledgment) await pendingSaveAcknowledgment;
      return reply(200, acknowledgment);
    }
    const match = new RegExp(`^/v2/accounts/${wallet}/assets/([0-9a-f-]+)$`).exec(path);
    if (match && request.headers.authorization === `Bearer ${token}`) {
      const id = match[1];
      if (request.method === 'PUT') {
        receivedFiles.set(id, bytes);
        const asset: AccountAsset = { assetId: id, ownerWallet: wallet, sha256: createHash('sha256').update(bytes).digest('hex'), mediaType: request.headers['content-type'] ?? '', bytes: bytes.length, state: 'quarantined', publicUrl: null };
        assets.set(id, asset);
        return reply(200, asset);
      }
      const asset = assets.get(id);
      if (request.method === 'GET' && asset) {
        const approved: AccountAsset = { ...asset, state: 'approved', publicUrl: `${origin}/images/${id}.png` };
        assets.set(id, approved);
        return reply(200, approved);
      }
    }
    reply(404, { code: 'NOT_FOUND' });
  } catch (error) {
    reply(500, { code: 'FIXTURE_ERROR', message: String(error) });
  }
});
const origin = await new Promise<string>(resolve => server.listen(0, '127.0.0.1', () => resolve(`http://127.0.0.1:${(server.address() as AddressInfo).port}`)));
const previousGateway = process.env.NEXT_PUBLIC_GATEWAY_URL;
process.env.NEXT_PUBLIC_GATEWAY_URL = origin;

const { createRoot, hydrateRoot } = await import('react-dom/client');
const { renderToString } = await import('react-dom/server');
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { WagmiProvider, createConfig, http } = await import('wagmi');
const { injected } = await import('wagmi/connectors');
const { connect, disconnect } = await import('wagmi/actions');
const { defineChain } = await import('viem');
const { BalanceVisibilityProvider, BalanceVisibilityToggle, MaskedAmount, useBalanceVisibility } = await import('./balance-visibility');
const { PortfolioChart } = await import('./portfolio-chart');
const { default: ProfileEditModal } = await import('./profile-edit-modal');
const { accountEpoch, clearAccountSession, editableExtendedProfile, installAccountSession } = await import('./account-transport');

class BrowserWallet {
  private readonly listeners = new Map<string, Set<(...args: unknown[]) => void>>();
  async request({ method }: { method: string }): Promise<unknown> {
    if (method === 'eth_chainId') return '0x7d';
    if (method === 'eth_accounts' || method === 'eth_requestAccounts') return [wallet];
    if (method === 'wallet_requestPermissions') return [{ parentCapability: 'eth_accounts' }];
    throw Object.assign(new Error(`Unsupported method ${method}`), { code: 4200 });
  }
  on(event: string, listener: (...args: unknown[]) => void) { (this.listeners.get(event) ?? this.listeners.set(event, new Set()).get(event)!).add(listener); return this; }
  removeListener(event: string, listener: (...args: unknown[]) => void) { this.listeners.get(event)?.delete(listener); return this; }
}
(window as unknown as { ethereum: BrowserWallet }).ethereum = new BrowserWallet();
const chain = defineChain({ id: 125, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: [origin] } } });
const config = createConfig({ chains: [chain], connectors: [injected()], transports: { 125: http(origin) }, ssr: false, multiInjectedProviderDiscovery: false });
const mounted = new Set<{ root: Root; container: HTMLDivElement; dispose?: () => void }>();
function mount(node: React.ReactElement, container = document.createElement('div')) {
  document.body.appendChild(container);
  const view = { root: createRoot(container), container };
  mounted.add(view);
  act(() => view.root.render(node));
  return view;
}
function unmount(view: { root: Root; container: HTMLDivElement; dispose?: () => void }) {
  act(() => view.root.unmount());
  view.dispose?.();
  view.container.remove();
  mounted.delete(view);
}
async function waitFor(check: () => boolean) {
  const deadline = Date.now() + 4000;
  while (!check()) {
    if (Date.now() > deadline) throw new Error(`Condition not met: ${document.body.textContent}`);
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
  }
}
function button(scope: ParentNode, label: string) {
  const found = Array.from(scope.querySelectorAll('button')).find(element => element.textContent === label || element.getAttribute('aria-label') === label);
  expect(found, label).toBeDefined();
  return found!;
}
function exposedText(scope: HTMLElement) {
  const copy = scope.cloneNode(true) as HTMLElement;
  copy.querySelectorAll('[aria-hidden="true"], .invisible, [hidden]').forEach(element => element.remove());
  return copy.textContent ?? '';
}
const series: SeriesPoint[] = Array.from({ length: 5 }, (_, index) => ({ time: new Date(Date.UTC(2026, 0, index + 1)).toISOString(), valueRaw: BigInt(index + 1) * 1_000_000n, complete: index !== 4 }));

beforeAll(async () => { await connect(config, { connector: config.connectors[0]! }); });
afterEach(() => {
  for (const view of mounted) unmount(view);
  clearAccountSession(false);
  window.localStorage.removeItem('sidiora_balances_hidden');
});
afterAll(async () => {
  await disconnect(config);
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  if (previousGateway === undefined) delete process.env.NEXT_PUBLIC_GATEWAY_URL;
  else process.env.NEXT_PUBLIC_GATEWAY_URL = previousGateway;
});

async function balance_mask_first_render() {
  const first = mount(h(BalanceVisibilityProvider, null, h(BalanceVisibilityToggle), h(MaskedAmount, null, '9,876.54 USDL')));
  expect(first.container.querySelector('[data-masked]')).toBeNull();
  act(() => button(first.container, 'Hide balances').click());
  expect(window.localStorage.getItem('sidiora_balances_hidden')).toBe('1');
  unmount(first);

  const frames: { hidden: boolean; text: string; chart: string | null }[] = [];
  let container: HTMLDivElement;
  function ObserveHydration() {
    const { hidden } = useBalanceVisibility();
    useLayoutEffect(() => { frames.push({ hidden, text: exposedText(container), chart: container.querySelector('[role="img"]')?.getAttribute('aria-label') ?? null }); });
    return null;
  }
  const page = h(BalanceVisibilityProvider, null,
    h(BalanceVisibilityToggle), h(MaskedAmount, null, '9,876.54 USDL'),
    h(PortfolioChart, { points: series, tone: 'gain' }), h(ObserveHydration));
  for (let reopen = 0; reopen < 2; reopen++) {
    container = document.createElement('div');
    container.innerHTML = renderToString(page);
    document.body.appendChild(container);
    expect(container.querySelector('[data-masked="true"]')).not.toBeNull();
    expect(exposedText(container)).not.toContain('9,876.54');
    expect(container.querySelector('[role="img"]')?.getAttribute('aria-label')).toBe('Portfolio chart hidden while balances are hidden');
    const hydrationErrors: unknown[] = [];
    let root!: Root;
    await act(async () => { root = hydrateRoot(container, page, { onRecoverableError: error => hydrationErrors.push(error) }); });
    const view = { root, container };
    mounted.add(view);
    expect(hydrationErrors).toEqual([]);
    expect(frames.length).toBeGreaterThan(0);
    for (const frame of frames) {
      expect(frame.hidden).toBe(true);
      expect(frame.text).not.toContain('9,876.54');
      expect(frame.chart).toBe('Portfolio chart hidden while balances are hidden');
    }
    expect(button(container, 'Show balances').getAttribute('aria-pressed')).toBe('true');
    unmount(view);
  }
  const shown = mount(h(BalanceVisibilityProvider, null, h(BalanceVisibilityToggle), h(MaskedAmount, null, '9,876.54 USDL')));
  act(() => button(shown.container, 'Show balances').click());
  expect(exposedText(shown.container)).toContain('9,876.54 USDL');
  expect(window.localStorage.getItem('sidiora_balances_hidden')).toBeNull();
}

function portfolio_hover_range() {
  const view = mount(h(PortfolioChart, { points: series, tone: 'gain' }));
  function selectLast() {
    const svg = view.container.querySelector('svg')!;
    act(() => svg.dispatchEvent(new dom.window.MouseEvent('pointermove', { bubbles: true, clientX: 600 })));
    expect(view.container.querySelector('[role="status"]')).not.toBeNull();
    expect(svg.getAttribute('aria-describedby')).toBe(view.container.querySelector('[role="status"]')?.id);
  }
  selectLast();
  expect(view.container.querySelector('[role="status"]')?.textContent).toContain('5.00 USDL');
  expect(view.container.querySelector('[role="status"]')?.textContent).toContain('some prices missing');
  const shorter = series.slice(0, 2);
  act(() => view.root.render(h(PortfolioChart, { points: shorter, tone: 'gain' })));
  expect(view.container.querySelector('[role="status"]')).toBeNull();
  expect(view.container.querySelector('svg')?.hasAttribute('aria-describedby')).toBe(false);
  selectLast();
  expect(view.container.querySelector('[role="status"]')?.textContent).toContain('2.00 USDL');
  for (const points of [[], series.slice(0, 1), [], shorter]) {
    act(() => view.root.render(h(PortfolioChart, { points, tone: 'flat' })));
    expect(view.container.querySelector('[role="status"]')).toBeNull();
    expect(view.container.querySelector('svg')?.hasAttribute('aria-describedby')).toBe(false);
  }
  expect(view.container.querySelector('svg')?.getAttribute('aria-label')).toContain('2.00 USDL');
  selectLast();
  const replacement = shorter.map(point => ({ ...point, valueRaw: point.valueRaw + 10_000_000n }));
  act(() => view.root.render(h(PortfolioChart, { points: replacement, tone: 'gain' })));
  expect(view.container.querySelector('[role="status"]')).toBeNull();
  selectLast();
  expect(view.container.querySelector('[role="status"]')?.textContent).toContain('12.00 USDL');
}

async function imageRemoval(kind: 'avatarAssetId' | 'bannerAssetId', withImages: boolean) {
  resetProfile(withImages);
  installAccountSession({ accessExpiresAt: new Date(Date.now() + 3_600_000).toISOString(), deviceLabel: 'Repair browser', refreshExpiresAt: new Date(Date.now() + 86_400_000).toISOString(), revokedAt: null, scopes: ['account'], sessionId: 'profile-repair-session', wallet }, token, accountEpoch(), { signMessage: (_wallet, message) => owner.signMessage({ message }) });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  let saved = 0;
  const modal = (open: boolean) => h(WagmiProvider, { config }, h(QueryClientProvider, { client }, h(ProfileEditModal, { open, onClose: () => undefined, onSaved: () => { saved++; } })));
  const view = { ...mount(modal(true)), dispose: () => client.clear() };
  // Replace the mount registration so cleanup also owns the query client.
  for (const registered of mounted) if (registered.root === view.root) mounted.delete(registered);
  mounted.add(view);
  await waitFor(() => document.querySelector<HTMLInputElement>('#profile-edit-displayName')?.value === 'Original');
  const input = document.querySelector<HTMLInputElement>(`#profile-edit-${kind}`)!;
  const picker = input.parentElement!;
  const baseline = profile[kind];
  const other = kind === 'avatarAssetId' ? 'bannerAssetId' : 'avatarAssetId';
  const otherBaseline = profile[other];
  const imageBytes = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aWQAAAABJRU5ErkJggg==', 'base64');
  // A browser File is the input value; Node's native File sends the same bytes through fetch.
  const file = new File([imageBytes], 'profile.png', { type: 'image/png' });
  Object.defineProperty(input, 'files', { configurable: true, value: [file] });
  act(() => input.dispatchEvent(new dom.window.Event('change', { bubbles: true })));
  await waitFor(() => picker.textContent?.includes('Under review') ?? false);
  expect(button(document, 'Save changes').disabled).toBe(true);
  act(() => button(picker, 'Refresh').click());
  await waitFor(() => picker.textContent?.includes('Approved') ?? false);
  const uploadedId = Array.from(assets.keys())[0];
  expect(receivedFiles.get(uploadedId)).toEqual(imageBytes);
  const preview = picker.querySelector('img');
  expect(preview).not.toBeNull();
  expect(preview?.getAttribute('src')).toBe(assets.get(uploadedId)?.publicUrl);
  act(() => preview!.dispatchEvent(new dom.window.Event('load')));
  expect(button(document, 'Save changes').disabled).toBe(false);
  act(() => button(picker, 'Remove').click());
  expect(picker.querySelector('img')).toBeNull();
  expect(picker.textContent).toContain(baseline ? 'Current image' : 'No image');
  expect(button(document, 'Save changes').disabled).toBe(true);
  const nameInput = document.querySelector<HTMLInputElement>('#profile-edit-displayName')!;
  act(() => {
    Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')!.set!.call(nameInput, 'After removal');
    nameInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  expect(button(document, 'Save changes').disabled).toBe(false);
  act(() => button(document, 'Save changes').click());
  await waitFor(() => saved === 1);
  expect(patches).toHaveLength(1);
  expect(patches[0][kind]).toBe(baseline);
  expect(patches[0][other]).toBe(otherBaseline);
  expect(patches[0][kind]).not.toBe(uploadedId);
  const recovered = await editableExtendedProfile(wallet);
  expect(recovered[kind]).toBe(baseline);
  expect(recovered[other]).toBe(otherBaseline);
  expect(recovered.displayName).toBe('After removal');
  act(() => view.root.render(modal(false)));
  expect(document.querySelector('[role="dialog"]')).toBeNull();
  act(() => view.root.render(modal(true)));
  await waitFor(() => document.querySelector<HTMLInputElement>('#profile-edit-displayName')?.value === 'After removal');
  const reopened = document.querySelector<HTMLInputElement>(`#profile-edit-${kind}`)!.parentElement!;
  expect(reopened.textContent).toContain(baseline ? 'Current image' : 'No image');
  expect(reopened.textContent).not.toContain('Approved');
  expect(reopened.querySelector('img')).toBeNull();
  unmount(view);
}
async function profile_image_remove_draft_avatar() { await imageRemoval('avatarAssetId', true); }
async function profile_image_remove_draft_banner() { await imageRemoval('bannerAssetId', true); }
async function profile_image_remove_draft_empty_avatar() { await imageRemoval('avatarAssetId', false); }
async function profile_image_remove_draft_empty_banner() { await imageRemoval('bannerAssetId', false); }

async function imageRemovalDuringSave(kind: 'avatarAssetId' | 'bannerAssetId') {
  resetProfile(true);
  installAccountSession({ accessExpiresAt: new Date(Date.now() + 3_600_000).toISOString(), deviceLabel: 'Repair browser', refreshExpiresAt: new Date(Date.now() + 86_400_000).toISOString(), revokedAt: null, scopes: ['account'], sessionId: 'profile-repair-session', wallet }, token, accountEpoch(), { signMessage: (_wallet, message) => owner.signMessage({ message }) });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  let saved = 0;
  const view = { ...mount(h(WagmiProvider, { config }, h(QueryClientProvider, { client }, h(ProfileEditModal, { open: true, onClose: () => undefined, onSaved: () => { saved++; } })))), dispose: () => client.clear() };
  for (const registered of mounted) if (registered.root === view.root) mounted.delete(registered);
  mounted.add(view);
  await waitFor(() => document.querySelector<HTMLInputElement>('#profile-edit-displayName')?.value === 'Original');
  const input = document.querySelector<HTMLInputElement>(`#profile-edit-${kind}`)!;
  const picker = input.parentElement!;
  const other = kind === 'avatarAssetId' ? 'bannerAssetId' : 'avatarAssetId';
  const otherBaseline = profile[other];
  const initialRevision = profile.revision;
  const imageBytes = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aWQAAAABJRU5ErkJggg==', 'base64');
  const file = new File([imageBytes], 'profile.png', { type: 'image/png' });
  Object.defineProperty(input, 'files', { configurable: true, value: [file] });
  act(() => input.dispatchEvent(new dom.window.Event('change', { bubbles: true })));
  await waitFor(() => picker.textContent?.includes('Under review') ?? false);
  expect(button(document, 'Save changes').disabled).toBe(true);
  act(() => button(picker, 'Refresh').click());
  await waitFor(() => picker.textContent?.includes('Approved') ?? false);
  const uploadedId = Array.from(assets.keys())[0];
  const publicUrl = assets.get(uploadedId)?.publicUrl;
  expect(receivedFiles.get(uploadedId)).toEqual(imageBytes);
  expect(picker.querySelector('img')?.getAttribute('src')).toBe(publicUrl);
  expect(button(picker, 'Remove').disabled).toBe(false);
  expect(button(document, 'Save changes').disabled).toBe(false);

  let acknowledge!: () => void;
  pendingSaveAcknowledgment = new Promise<void>(resolve => { acknowledge = resolve; });
  try {
    act(() => button(document, 'Save changes').click());
    await waitFor(() => patches.length === 1);
    expect(patches[0].expectedRevision).toBe(initialRevision);
    expect(patches[0][kind]).toBe(uploadedId);
    expect(profile[kind]).toBe(uploadedId);
    expect(profile.revision).toBe(String(BigInt(initialRevision) + 1n));
    expect(saved).toBe(0);
    expect(document.body.textContent).not.toContain('Profile saved');
    expect(button(document, 'Save changes').disabled).toBe(true);
    expect(input.disabled).toBe(true);
    expect(button(picker, 'Choose image').disabled).toBe(true);
    expect(button(picker, 'Refresh').disabled).toBe(true);
    const remove = button(picker, 'Remove');
    expect(remove.disabled).toBe(true);
    act(() => remove.click());
    expect(picker.textContent).toContain('Approved · profile.png');
    expect(picker.querySelector('img')?.getAttribute('src')).toBe(publicUrl);
    expect(saved).toBe(0);
    expect(patches).toHaveLength(1);
  } finally {
    pendingSaveAcknowledgment = null;
    acknowledge();
  }

  await waitFor(() => document.body.textContent?.includes('Profile saved') ?? false);
  expect(saved).toBe(1);
  expect(picker.textContent).toContain('Current image');
  expect(picker.textContent).not.toContain('profile.png');
  expect(picker.querySelector('img')).toBeNull();
  expect(Array.from(picker.querySelectorAll('button')).map(element => element.textContent)).toEqual(['Choose image']);
  expect(input.disabled).toBe(false);
  expect(button(picker, 'Choose image').disabled).toBe(false);
  expect(button(document, 'Save changes').disabled).toBe(true);
  const recovered = await editableExtendedProfile(wallet);
  expect(recovered[kind]).toBe(uploadedId);
  expect(recovered[other]).toBe(otherBaseline);
  const acknowledgedRevision = recovered.revision;
  expect(acknowledgedRevision).toBe(String(BigInt(initialRevision) + 1n));

  const nameInput = document.querySelector<HTMLInputElement>('#profile-edit-displayName')!;
  expect(nameInput.disabled).toBe(false);
  act(() => {
    Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')!.set!.call(nameInput, 'After acknowledgment');
    nameInput.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  expect(button(document, 'Save changes').disabled).toBe(false);
  act(() => button(document, 'Save changes').click());
  await waitFor(() => saved === 2);
  expect(document.body.textContent).toContain('Profile saved');
  expect(patches).toHaveLength(2);
  expect(patches[1].expectedRevision).toBe(acknowledgedRevision);
  expect(patches[1].displayName).toBe('After acknowledgment');
  expect(patches[1][kind]).toBe(uploadedId);
  expect(patches[1][other]).toBe(otherBaseline);
  const subsequent = await editableExtendedProfile(wallet);
  expect(subsequent.revision).toBe(String(BigInt(acknowledgedRevision) + 1n));
  expect(subsequent.displayName).toBe('After acknowledgment');
  expect(subsequent[kind]).toBe(uploadedId);
  expect(subsequent[other]).toBe(otherBaseline);
  expect(button(document, 'Save changes').disabled).toBe(true);
  unmount(view);
}
async function profile_image_remove_during_save_avatar() { await imageRemovalDuringSave('avatarAssetId'); }
async function profile_image_remove_during_save_banner() { await imageRemovalDuringSave('bannerAssetId'); }

test(balance_mask_first_render.name, balance_mask_first_render);
test(portfolio_hover_range.name, portfolio_hover_range);
test(profile_image_remove_draft_avatar.name, profile_image_remove_draft_avatar);
test(profile_image_remove_draft_banner.name, profile_image_remove_draft_banner);
test(profile_image_remove_draft_empty_avatar.name, profile_image_remove_draft_empty_avatar);
test(profile_image_remove_draft_empty_banner.name, profile_image_remove_draft_empty_banner);
test(profile_image_remove_during_save_avatar.name, profile_image_remove_during_save_avatar);
test(profile_image_remove_during_save_banner.name, profile_image_remove_during_save_banner);
