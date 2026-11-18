import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';
import React, { act, createElement as h } from 'react';
import { createRequire } from 'node:module';
import { EventEmitter } from 'node:events';
import { spawn, spawnSync, type ChildProcess } from 'node:child_process';
import { chmodSync, chownSync, existsSync, mkdtempSync, openSync, closeSync, readFileSync, unlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';
import type { Root } from 'react-dom/client';
import type { Address, EIP1193Parameters, EIP1193Provider, EIP1474Methods, Hex } from 'viem';
import type { ExportJob, TrackedPortfolio } from '@/widgets/profile/account-transport';
import type { OwnerShare, SharePreview } from '@/widgets/pnl/share-transport';
import type { WalletNetworkAuthorityV2 } from '@/core/wallet-sdk/backend';

const { JSDOM } = createRequire(import.meta.url)('jsdom') as typeof import('jsdom');
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://localhost:3000', pretendToBeVisual: true });
const globals = globalThis as unknown as Record<string, unknown>;
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globalThis || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  globals[key] = typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
}
globals.window = dom.window;
globals.document = dom.window.document;
globals.IS_REACT_ACT_ENVIRONMENT = true;

const { createRoot } = await import('react-dom/client');
const { renderToStaticMarkup } = await import('react-dom/server');
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { WagmiProvider, createConfig, http } = await import('wagmi');
const { connect, disconnect, getAccount } = await import('wagmi/actions');
const { injected } = await import('wagmi/connectors');
const { createTestClient, createWalletClient, defineChain } = await import('viem');
const { getPublicClient } = await import('@/core/wallet-sdk/client');
const { WalletRuntimeProvider } = await import('@/core/wallet-sdk/runtime');
const { useEmbeddedConnection } = await import('@/shell/embedded-sign-in');
const { authCallbackState } = await import('@/shell/auth-callback-state');
const { accountEpoch, clearAccountSession, installAccountSession } = await import('@/widgets/profile/account-transport');
const { useProfileData } = await import('@/widgets/profile/use-profile-data');
const { readPublicPortfolio, usePositionPages, useValueHistory } = await import('@/widgets/profile/portfolio-transport');
const { default: PositionsTab, ExportStatus } = await import('@/widgets/profile/positions-tab');
const { BalanceVisibilityProvider } = await import('@/widgets/profile/balance-visibility');
const { PublicShareFigures } = await import('@/widgets/pnl/pnl-card-landing');
const { default: SharePnlModal } = await import('@/widgets/pnl/share-pnl-modal');
const { readPublicShare, PublicShareError } = await import('@/widgets/pnl/share-transport');
const { default: PnlCardPage } = await import('@/app/pnl/[cardId]/page');
const { leaderboardValue } = await import('@/widgets/profile/leaderboard-panel');
const { default: TransferPanel } = await import('@/widgets/funding/transfer-panel');

const rpcUrl = process.env.WALLET_TEST_RPC_URL;
if (!rpcUrl || !['127.0.0.1', 'localhost'].includes(new URL(rpcUrl).hostname) || new URL(rpcUrl).port !== '22500') throw new Error('The owned wallet chain is required');
const rpc = getPublicClient(rpcUrl);
let transport = rpc;
let connectionRequests = 0;
const events = new EventEmitter();
const provider = {
  async request(args: EIP1193Parameters<EIP1474Methods>) {
    if (args.method === 'eth_requestAccounts') connectionRequests++;
    return ({ request: transport.request } as EIP1193Provider).request(args);
  },
  on(event: string, listener: (...args: unknown[]) => void) { events.on(event, listener); return this; },
  removeListener(event: string, listener: (...args: unknown[]) => void) { events.removeListener(event, listener); return this; },
};
(window as unknown as { ethereum: unknown }).ethereum = provider;
const chain = defineChain({ id: 125, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: [rpcUrl] } } });
const config = createConfig({ chains: [chain], connectors: [injected()], transports: { 125: http(rpcUrl, { retryCount: 0 }) }, storage: null, multiInjectedProviderDiscovery: false });

const repository = resolve(dirname(fileURLToPath(import.meta.url)), '../../../../../../');
const output = mkdtempSync(resolve(tmpdir(), 'profile-flow-'));
chmodSync(output, 0o700);
const children: { child: ChildProcess; log: string }[] = [];
const descriptors: number[] = [];
let postgresStarted = false;
let pgBin = '';
let wallet: Address;
let recipient: Address;
let first: TrackedPortfolio;
let otherFirst: TrackedPortfolio;
let preview: SharePreview;
let publicCard: OwnerShare;
let queued: ExportJob;
let complete: ExportJob;
let pnlOrigin = '';
let userOrigin = '';

function command(binary: string, args: string[]) {
  const result = spawnSync(binary, args, { encoding: 'utf8', timeout: 60000 });
  if (result.status !== 0 || result.error) throw new Error(`${binary}: ${result.error?.message ?? result.stderr}`);
  return result.stdout.trim();
}
function postgres(binary: string, args: string[]) {
  return process.getuid?.() === 0 ? command('runuser', ['-u', 'postgres', '--', resolve(pgBin, binary), ...args]) : command(resolve(pgBin, binary), args);
}
function json<T>(name: string): T { return JSON.parse(readFileSync(resolve(output, `${name}.json`), 'utf8')) as T; }
function requests(service: 'pnl' | 'user') { return existsSync(resolve(output, `${service}-requests.json`)) ? json<string[]>(`${service}-requests`) : []; }
const pause = (ms: number) => new Promise<void>(resolve => setTimeout(resolve, ms));
async function waitFor(check: () => boolean, milliseconds = 10000) {
  const deadline = Date.now() + milliseconds;
  while (!check()) {
    if (Date.now() >= deadline) throw new Error(`Profile state did not settle: ${document.body.textContent?.slice(0, 700)}`);
    await act(async () => { await pause(20); });
  }
}
async function ready(name: string, running: { child: ChildProcess; log: string }) {
  const deadline = Date.now() + 180000;
  while (!existsSync(resolve(output, `${name}.json`))) {
    if (running.child.exitCode !== null || Date.now() >= deadline) throw new Error(readFileSync(running.log, 'utf8'));
    await pause(50);
  }
}
function producer(service: string, packagePath: string, source: string, dsn: string) {
  const cwd = resolve(repository, 'services/launchpad', service);
  const overlay = resolve(output, `${service}-overlay.json`);
  writeFileSync(overlay, JSON.stringify({ Replace: { [resolve(cwd, packagePath, 'profile_flow_web_test.go')]: resolve(dirname(fileURLToPath(import.meta.url)), source) } }));
  const log = resolve(output, `${service}.log`);
  const fd = openSync(log, 'w', 0o600);
  descriptors.push(fd);
  const child = spawn('go', ['test', '-overlay', overlay, '-run', '^TestProfileFlowAnswers$', '-count=1', '-timeout=8m', `./${packagePath}`], {
    cwd, env: { ...process.env, GOWORK: 'off', L17_TEST_DSN: dsn, USER_TEST_DATABASE_URL: dsn, PROFILE_FLOW_WALLET: wallet.toLowerCase(), PROFILE_FLOW_OTHER_WALLET: recipient.toLowerCase(), PROFILE_FLOW_OUTPUT: output }, stdio: ['ignore', fd, fd],
  });
  const running = { child, log };
  children.push(running);
  return running;
}
beforeAll(async () => {
  expect(await rpc.getChainId()).toBe(125);
  [wallet, recipient] = await createWalletClient({ transport: http(rpcUrl) }).getAddresses() as [Address, Address];
  pgBin = command('pg_config', ['--bindir']);
  if (process.getuid?.() === 0) chownSync(output, Number(command('id', ['-u', 'postgres'])), Number(command('id', ['-g', 'postgres'])));
  postgres('initdb', ['-D', resolve(output, 'database'), '-U', 'postgres', '--auth-local=trust', '--auth-host=reject', '--no-locale', '--encoding=UTF8']);
  postgres('pg_ctl', ['-D', resolve(output, 'database'), '-l', resolve(output, 'postgres.log'), '-w', 'start', '-o', `-c listen_addresses='' -c unix_socket_directories=${output} -c max_connections=32`]);
  postgresStarted = true;
  const dsn = `postgresql://postgres@/postgres?host=${encodeURIComponent(output)}`;
  const pnl = producer('pnl-tracker', 'internal/production/l17', 'flow-profile-pnl_test.go.txt', dsn);
  const user = producer('user', 'internal/account', 'flow-profile-user_test.go.txt', dsn);
  const leaderboard = producer('api', 'internal/store', 'flow-profile-leaderboard_test.go.txt', dsn);
  await Promise.all([ready('pnl-ready', pnl), ready('user-ready', user), ready('leaderboard', leaderboard)]);
  pnlOrigin = json<{ origin: string }>('pnl-ready').origin;
  userOrigin = json<{ origin: string }>('user-ready').origin;
  process.env.NEXT_PUBLIC_PNL_API_URL = pnlOrigin;
  process.env.NEXT_PUBLIC_GATEWAY_URL = userOrigin;
  first = await readPublicPortfolio(wallet);
  otherFirst = await readPublicPortfolio(recipient);
  preview = json<SharePreview>('preview');
  publicCard = json<OwnerShare>('public');
  queued = json<ExportJob>('queued');
  complete = json<ExportJob>('complete');
}, 240000);

const mounted = new Set<{ root: Root; container: HTMLDivElement; client: InstanceType<typeof QueryClient> }>();
function mount(node: React.ReactNode, authority?: WalletNetworkAuthorityV2) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  const render = (next: React.ReactNode) => act(() => root.render(h(WagmiProvider, { config, reconnectOnMount: false }, h(QueryClientProvider, { client }, authority ? h(WalletRuntimeProvider, { authority, children: next }) : next))));
  const view = { root, container, client, render };
  mounted.add(view);
  render(node);
  return view;
}
afterEach(() => {
  for (const view of mounted) { act(() => view.root.unmount()); view.client.clear(); view.container.remove(); }
  mounted.clear();
  clearAccountSession(false);
});
afterAll(async () => {
  writeFileSync(resolve(output, 'stop'), 'done');
  writeFileSync(resolve(output, 'release-pages'), 'done');
  await disconnect(config);
  try {
    for (const running of children) {
      if (running.child.exitCode === null) await Promise.race([new Promise<void>(resolve => running.child.once('exit', () => resolve())), pause(10000)]);
      if (running.child.exitCode === null) running.child.kill('SIGTERM');
      else expect(running.child.exitCode, readFileSync(running.log, 'utf8')).toBe(0);
    }
  } finally {
    if (postgresStarted) postgres('pg_ctl', ['-D', resolve(output, 'database'), '-w', 'stop', '-m', 'fast']);
    for (const fd of descriptors) closeSync(fd);
    dom.window.close();
  }
}, 30000);
function signIn() {
  installAccountSession({ sessionId: crypto.randomUUID(), wallet, scopes: ['account'], deviceLabel: 'Browser', accessExpiresAt: new Date(Date.now() + 3600000).toISOString(), refreshExpiresAt: new Date(Date.now() + 7200000).toISOString(), revokedAt: null }, crypto.randomUUID(), accountEpoch());
}
function button(container: ParentNode, text: string) {
  const result = Array.from(container.querySelectorAll('button')).find(element => element.textContent === text);
  if (!result) throw new Error(`Control absent: ${text}`);
  return result;
}
function input(container: ParentNode, id: string, value: string) {
  const element = container.querySelector<HTMLInputElement>(`#${id}`)!;
  act(() => {
    Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')!.set!.call(element, value);
    element.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
}

async function embedded_connection_finishes_and_retry_is_explicit() {
  let state: ReturnType<typeof useEmbeddedConnection> | undefined;
  function Connection() { state = useEmbeddedConnection(true, 'signed-in-wallet', config.connectors[0]!.id); return h('output', null, state.connecting ? 'connecting' : state.error ?? 'ready'); }
  transport = getPublicClient('http://127.0.0.1:0');
  const view = mount(h(Connection));
  await waitFor(() => Boolean(state?.error) && !state?.connecting, 20000);
  const attempts = connectionRequests;
  await act(async () => { await pause(200); });
  expect(connectionRequests).toBe(attempts);
  expect(getAccount(config).isConnected).toBe(false);
  transport = rpc;
  act(() => state!.retry());
  await waitFor(() => getAccount(config).isConnected && state?.connecting === false);
  expect(state?.error).toBeNull();
  expect(getAccount(config).address?.toLowerCase()).toBe(wallet.toLowerCase());
  expect(view.container.textContent).toBe('ready');
}

function callback_waits_for_its_wallet_and_surfaces_terminal_errors() {
  const base = { configured: true, authenticated: false, connected: false, authError: null, connectionError: null, timedOut: false };
  expect(authCallbackState(base, 'embedded')).toBe('waiting');
  expect(authCallbackState({ ...base, authenticated: true }, 'embedded')).toBe('waiting');
  for (const state of [{ ...base, configured: false }, { ...base, authError: 'Link expired' }, { ...base, connectionError: 'Connection interrupted' }, { ...base, timedOut: true }]) expect(authCallbackState(state, 'embedded')).toBe('failed');
  const connected = { ...base, authenticated: true, connected: true, connectorId: 'embedded', address: wallet, walletAddress: wallet.toLowerCase() };
  expect(authCallbackState(connected, 'embedded')).toBe('ready');
  expect(authCallbackState({ ...connected, connectorId: 'injected' }, 'embedded')).toBe('failed');
  expect(authCallbackState({ ...connected, address: recipient }, 'embedded')).toBe('failed');
}

async function empty_profile_disables_absent_media() {
  signIn();
  let state: ReturnType<typeof useProfileData> | undefined;
  function Profile() { state = useProfileData(wallet); return h('output', null, state.profileLoading ? 'loading' : state.profileError ?? state.profile?.displayName ?? 'No published profile'); }
  const view = mount(h(Profile));
  await waitFor(() => !state?.profileLoading);
  expect(state?.profile).toBeNull();
  expect(state?.profileError).toBeNull();
  expect(view.container.textContent).toBe('No published profile');
  await act(async () => { state!.refreshProfile(); await pause(100); });
  expect(requests('user').filter(row => row.includes('/assets/'))).toEqual([]);
  const assetQueries = view.client.getQueryCache().getAll().filter(query => query.queryKey.includes('profile-asset'));
  expect(assetQueries.length).toBeGreaterThan(0);
  expect(assetQueries.every(query => query.state.fetchStatus === 'idle' && query.state.error === null)).toBe(true);
}

async function profile_media_errors_are_visible_and_retryable() {
  signIn();
  writeFileSync(resolve(output, 'publish-profile'), 'publish');
  await waitFor(() => existsSync(resolve(output, 'profile-published')));
  let state: ReturnType<typeof useProfileData> | undefined;
  function Profile() { state = useProfileData(wallet); return h('output', null, state.profileError); }
  mount(h(Profile));
  await waitFor(() => Boolean(state?.profileError));
  expect(state?.profile?.displayName).toBe('Paxeer builder');
  expect(state?.profileError).toBe('HTTP_404');
  expect(state?.avatarUrl).toBeNull();
  const previous = requests('user').filter(row => row.includes('/assets/')).length;
  act(() => state!.refreshProfile());
  await waitFor(() => requests('user').filter(row => row.includes('/assets/')).length > previous);
}

async function position_paging_is_private_for_the_owner_and_ends_without_a_session() {
  clearAccountSession(false);
  expect(first.data.positions).toHaveLength(50);
  expect(first.data.next).toBeTruthy();
  let pages: ReturnType<typeof usePositionPages> | undefined;
  let history: ReturnType<typeof useValueHistory> | undefined;
  function Positions() { pages = usePositionPages(first, wallet, true); history = useValueHistory(wallet, '1W', true); return h('output', null, pages.positions.length); }
  const view = mount(h(Positions));
  expect(history?.state).toBe('error');
  const reads = requests('pnl').length;
  act(() => pages!.loadMore());
  await waitFor(() => Boolean(pages?.moreError) && !pages?.loadingMore);
  expect(pages?.moreError).toMatchObject({ status: 401, code: 'AUTH_EXPIRED' });
  expect(requests('pnl')).toHaveLength(reads);
  expect(view.container.textContent).toBe('50');
}

async function visitor_profile_refresh_keeps_media_private() {
  clearAccountSession(false);
  let state: ReturnType<typeof useProfileData> | undefined;
  function Profile() { state = useProfileData(wallet); return h('output', null, state.profileLoading ? 'loading' : state.profileError ?? state.profile?.displayName); }
  const view = mount(h(Profile));
  await waitFor(() => !state?.profileLoading && state?.profile?.displayName === 'Paxeer builder');
  expect(state?.isOwner).toBe(false);
  const assets = requests('user').filter(row => row.includes('/assets/')).length;
  await act(async () => { state!.refreshProfile(); await pause(100); });
  await waitFor(() => view.client.getQueryCache().getAll().every(query => query.state.fetchStatus === 'idle'));
  expect(state?.profileError).toBeNull();
  expect(state?.avatarUrl).toBeNull();
  expect(state?.bannerUrl).toBeNull();
  expect(requests('user').filter(row => row.includes('/assets/'))).toHaveLength(assets);
  expect(view.container.textContent).toBe('Paxeer builder');
}

async function public_pages_accumulate_and_discard_answers_after_a_scope_change() {
  let pages: ReturnType<typeof usePositionPages> | undefined;
  function Positions({ page, address }: { page: TrackedPortfolio | null; address: string }) { pages = usePositionPages(page, address, false); return h('output', null, pages.positions.length); }
  const view = mount(h(Positions, { page: first, address: wallet }));
  act(() => pages!.loadMore());
  await waitFor(() => pages?.positions.length === 51 && !pages.loadingMore);
  expect(pages?.hasMore).toBe(false);
  expect(pages?.moreError).toBeNull();
  expect(new Set(pages?.positions.map(row => row.positionId)).size).toBe(51);
  view.render(h(Positions, { page: null, address: recipient }));
  view.render(h(Positions, { page: first, address: wallet }));
  unlinkSync(resolve(output, 'page-finished'));
  writeFileSync(resolve(output, 'hold-pages'), 'hold');
  const before = requests('pnl').length;
  act(() => { pages!.loadMore(); pages!.loadMore(); });
  await waitFor(() => requests('pnl').length === before + 1);
  expect(pages?.loadingMore).toBe(true);
  view.render(h(Positions, { page: otherFirst, address: recipient }));
  expect(pages?.positions).toHaveLength(50);
  expect(pages?.loadingMore).toBe(false);
  writeFileSync(resolve(output, 'release-pages'), 'release');
  await waitFor(() => existsSync(resolve(output, 'page-finished')));
  await act(async () => { await pause(100); });
  expect(pages?.positions).toHaveLength(50);
  expect(pages?.positions.every(row => row.wallet === recipient.toLowerCase())).toBe(true);
  expect(pages?.moreError).toBeNull();
  expect(view.container.textContent).toBe('50');
}

async function completed_export_exposes_the_download_written_by_the_worker() {
  expect(queued.state).toBe('queued');
  expect(complete.state).toBe('complete');
  let downloaded = '';
  const download = async () => { downloaded = json<{ csv: string }>('pnl-ready').csv; };
  const view = mount(h(ExportStatus, { job: queued, onRefresh: async () => { view.render(h(ExportStatus, { job: complete, onRefresh: async () => undefined, onDownload: download })); }, onDownload: download }));
  expect(view.container.textContent).toContain('Export: queued');
  expect(view.container.textContent).not.toContain('Download CSV');
  act(() => button(view.container, 'Refresh').click());
  expect(view.container.textContent).toContain('Export: complete');
  act(() => button(view.container, 'Download CSV').click());
  expect(downloaded).toContain('event_id,kind,chain_id');
  expect(downloaded.split('\n').filter(Boolean)).toHaveLength(52);
}

async function accounting_loading_and_errors_have_working_retry_controls() {
  signIn();
  const view = mount(h(BalanceVisibilityProvider, null, h(PositionsTab, { walletAddress: wallet, canShare: false, defaultOpen: true })));
  const historyLoading = Array.from(view.container.querySelectorAll('[aria-busy="true"]')).find(element => element.textContent?.includes('Loading PnL history'));
  expect(historyLoading).toBeDefined();
  await waitFor(() => Array.from(view.container.querySelectorAll('button')).some(element => element.textContent === 'Retry positions'), 15000);
  await waitFor(() => Array.from(view.container.querySelectorAll('button')).some(element => element.textContent === 'Retry history'));
  const seen = requests('pnl').length;
  act(() => { button(view.container, 'Retry positions').click(); button(view.container, 'Retry history').click(); });
  await waitFor(() => requests('pnl').length >= seen + 2);
  await waitFor(() => view.client.isFetching() === 0);
  expect(view.container.querySelector('[role="alert"]')).not.toBeNull();
}

async function shared_card_renders_the_exact_return_and_unavailable_value() {
  const share = await readPublicShare(publicCard.id);
  expect(share.values.historicalReturn?.return).toEqual({ numerator: '1', denominator: '4' });
  expect(share.values.realizedPnlRaw).toBeNull();
  const view = mount(h(PublicShareFigures, { share }));
  expect(Array.from(view.container.querySelectorAll('dd')).map(element => element.textContent)).toEqual(['1/4', '—']);
  expect(view.container.textContent).not.toContain('time_weighted');
  expect(view.container.textContent).not.toContain(wallet.toLowerCase());
  const unavailable = json<SharePreview>('unavailable').share;
  expect(unavailable.values.historicalReturn?.return).toBeUndefined();
  view.render(h(PublicShareFigures, { share: unavailable }));
  expect(view.container.querySelector('dd')?.textContent).toBe('Not available yet');
}

async function share_action_errors_keep_the_service_message() {
  const error = await readPublicShare(json<SharePreview>('revoked').id).then(() => { throw new Error('Removed card was published'); }, failure => failure as Error);
  expect(error).toBeInstanceOf(PublicShareError);
  const view = mount(h(SharePnlModal, { snapshotId: first.data.summary.snapshotId, preview, error: error.message, onClose: () => undefined, onSelection: async () => undefined, onPublish: async () => undefined, onRefresh: async () => undefined, onRevoke: async () => undefined }));
  expect(view.container.querySelector('[role="alert"]')?.textContent).toBe('Card withdrawn or expired');
}

async function public_card_failure_has_a_way_back_and_retry_reloads_the_route() {
  const cardId = json<SharePreview>('revoked').id;
  const gone = renderToStaticMarkup(await PnlCardPage({ params: Promise.resolve({ cardId }) }));
  expect(gone).toContain('This card is gone');
  expect(gone).toContain('href="/profile"');
  process.env.NEXT_PUBLIC_PNL_API_URL = 'http://127.0.0.1:0';
  try {
    const unavailable = renderToStaticMarkup(await PnlCardPage({ params: Promise.resolve({ cardId: publicCard.id }) }));
    expect(unavailable).toContain('Try again');
    expect(unavailable).toContain(`href="/pnl/${publicCard.id}"`);
    expect(unavailable).toContain('href="/profile"');
  } finally { process.env.NEXT_PUBLIC_PNL_API_URL = pnlOrigin; }
}

function qualificationAuthority(): WalletNetworkAuthorityV2 {
  const file = process.env.L25_QUALIFICATION_CONFIG_FILE;
  if (!file) throw new Error('The wallet network configuration is required');
  const bytes = readFileSync(file);
  expect(createHash('sha256').update(bytes).digest('hex')).toBe(process.env.L25_QUALIFICATION_CONFIG_SHA256);
  const value = JSON.parse(bytes.toString()) as { network: WalletNetworkAuthorityV2['network']; usdlRuntimeHash: Hex };
  return { network: value.network, usdlRuntimeHash: value.usdlRuntimeHash, committedHead: async signal => { signal.throwIfAborted(); const block = await getPublicClient(value.network.publicRPC).getBlock({ blockTag: 'latest' }); return { number: block.number, hash: block.hash }; } };
}
async function confirmed_withdrawal_opens_a_new_independent_draft() {
  if (!getAccount(config).isConnected) await connect(config, { connector: config.connectors[0]! });
  const view = mount(h(TransferPanel), qualificationAuthority());
  await waitFor(() => Boolean(view.container.querySelector('#transfer-recipient')) && !button(view.container, 'Review').disabled);
  input(view.container, 'transfer-recipient', recipient);
  input(view.container, 'transfer-amount', '0.000000000000000011');
  act(() => button(view.container, 'Review').click());
  await waitFor(() => view.container.textContent?.includes('Confirm withdrawal') ?? false);
  act(() => button(view.container, 'Approve transfer in wallet').click());
  await waitFor(() => view.container.textContent?.includes('Withdrawal confirmed.') ?? false, 20000);
  const hash = view.container.querySelector<HTMLAnchorElement>('[role="status"] a')!.textContent as Hex;
  expect((await rpc.getTransactionReceipt({ hash })).status).toBe('success');
  const nonce = await rpc.getTransactionCount({ address: wallet });
  act(() => button(view.container, 'New withdrawal').click());
  expect(view.container.querySelector<HTMLInputElement>('#transfer-amount')?.value).toBe('');
  expect(view.container.querySelector<HTMLInputElement>('#transfer-recipient')?.disabled).toBe(false);
  expect(view.container.querySelector('[aria-label="Withdrawals"]')?.textContent).toContain(hash);
  input(view.container, 'transfer-amount', '0.000000000000000013');
  act(() => button(view.container, 'Review').click());
  await waitFor(() => view.container.textContent?.includes('Confirm withdrawal') ?? false);
  expect(await rpc.getTransactionCount({ address: wallet })).toBe(nonce);
  expect(button(view.container, 'Approve transfer in wallet').disabled).toBe(false);
}

async function reverted_withdrawal_retains_its_hash_and_opens_a_new_draft() {
  const artifacts = process.env.L25_FIXTURE_ARTIFACTS;
  if (!artifacts) throw new Error('The wallet contracts are required');
  const reject = JSON.parse(readFileSync(resolve(artifacts, 'FundingToken.sol/RejectNative.json'), 'utf8')) as { deployedBytecode: { object: Hex } };
  const view = mount(h(TransferPanel), qualificationAuthority());
  await waitFor(() => Boolean(view.container.querySelector('#transfer-recipient')) && !button(view.container, 'Review').disabled);
  input(view.container, 'transfer-recipient', recipient);
  input(view.container, 'transfer-amount', '0.000000000000000017');
  act(() => button(view.container, 'Review').click());
  await waitFor(() => view.container.textContent?.includes('Confirm withdrawal') ?? false);
  expect(await rpc.getCode({ address: recipient })).toBeUndefined();
  const control = createTestClient({ mode: 'anvil', transport: http(rpcUrl) });
  await control.setCode({ address: recipient, bytecode: reject.deployedBytecode.object });
  try {
    act(() => button(view.container, 'Approve transfer in wallet').click());
    await waitFor(() => view.container.textContent?.includes('Withdrawal failed, no funds moved.') ?? false, 20000);
    const hash = view.container.querySelector<HTMLAnchorElement>('[role="status"] a')!.textContent as Hex;
    expect((await rpc.getTransactionReceipt({ hash })).status).toBe('reverted');
    const nonce = await rpc.getTransactionCount({ address: wallet });
    act(() => button(view.container, 'New withdrawal').click());
    expect(view.container.querySelector<HTMLInputElement>('#transfer-amount')?.value).toBe('');
    expect(button(view.container, 'Review').disabled).toBe(false);
    expect(view.container.querySelector('[aria-label="Withdrawals"]')?.textContent).toContain(hash);
    expect(await rpc.getTransactionCount({ address: wallet })).toBe(nonce);
  } finally { await control.setCode({ address: recipient, bytecode: '0x' }); }
}

function leaderboard_profit_preserves_the_published_sign() {
  const entry = json<import('@/widgets/profile/leaderboard-panel').LeaderboardEntry>('leaderboard');
  expect(entry.metricRaw).toBe('1000000');
  expect(entry.metricSigned).toBe('-1000000');
  expect(leaderboardValue(entry, 'profit')).toBe('-1.00 USDL');
  expect(leaderboardValue(entry, 'volume')).toBe('1.00 USDL');
}

test(leaderboard_profit_preserves_the_published_sign, leaderboard_profit_preserves_the_published_sign);
test(embedded_connection_finishes_and_retry_is_explicit, embedded_connection_finishes_and_retry_is_explicit, 30000);
test(callback_waits_for_its_wallet_and_surfaces_terminal_errors, callback_waits_for_its_wallet_and_surfaces_terminal_errors);
test(empty_profile_disables_absent_media, empty_profile_disables_absent_media);
test(profile_media_errors_are_visible_and_retryable, profile_media_errors_are_visible_and_retryable, 20000);
test(visitor_profile_refresh_keeps_media_private, visitor_profile_refresh_keeps_media_private);
test(position_paging_is_private_for_the_owner_and_ends_without_a_session, position_paging_is_private_for_the_owner_and_ends_without_a_session);
test(public_pages_accumulate_and_discard_answers_after_a_scope_change, public_pages_accumulate_and_discard_answers_after_a_scope_change);
test(completed_export_exposes_the_download_written_by_the_worker, completed_export_exposes_the_download_written_by_the_worker);
test(accounting_loading_and_errors_have_working_retry_controls, accounting_loading_and_errors_have_working_retry_controls, 20000);
test(shared_card_renders_the_exact_return_and_unavailable_value, shared_card_renders_the_exact_return_and_unavailable_value);
test(share_action_errors_keep_the_service_message, share_action_errors_keep_the_service_message);
test(public_card_failure_has_a_way_back_and_retry_reloads_the_route, public_card_failure_has_a_way_back_and_retry_reloads_the_route);
test(confirmed_withdrawal_opens_a_new_independent_draft, confirmed_withdrawal_opens_a_new_independent_draft, 30000);
test(reverted_withdrawal_retains_its_hash_and_opens_a_new_draft, reverted_withdrawal_retains_its_hash_and_opens_a_new_draft, 30000);
