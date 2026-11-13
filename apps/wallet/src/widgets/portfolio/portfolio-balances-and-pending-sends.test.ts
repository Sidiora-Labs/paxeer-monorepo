// @vitest-environment jsdom
import { parseWalletReadRequest, walletReadResponseHeaders } from '@/lib/wallet-read-contract';
import { createServer, type Server, type ServerResponse } from 'node:http';
import { once } from 'node:events';
import { act, createElement, useLayoutEffect, useRef } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { ethers } from 'ethers';
import { initBlockscout, type TokenBalance } from '@/lib/data';
import { fetchPortfolio } from '@/lib/api';
import { queryKeys, usePortfolioQuery } from '@/lib/queries';
import { announceRpcChanged, PAXEER_CONFIG } from '@/lib/constants';
import { RECONCILE_DELAY_MS } from '@/lib/optimistic';
import { VIRTUAL_CHAIN_ID_BASE, virtualNetwork } from '@/lib/networks';
import {
  custodyChoiceRepository,
  preferencesRepository,
  portfolioFiltersRepository,
} from '@/platform/storage/repositories';
import { WalletProvider, useWallet, type WalletContextValue } from '@/wallet/WalletProvider';
import { TestInjectedProvider, announceInjected } from '@/wallet/test/injected';
import type { WalletConfig } from '@/wallet/config';
import { useSendForm, type SubmittedTransfer } from '@/widgets/send/useSendForm';
import { useSendableTokens } from '@/widgets/send/useSendableTokens';
import { HoldingsGrid, type HoldingsGridHolding } from './HoldingsGrid';
import { PortfolioWidget } from './PortfolioWidget';
import { useApplyPendingSend } from './useApplyPendingSend';
import { pendingSendKey, scopedPendingSendsRepository } from './pendingSends';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
const ROOT_CHAIN = PAXEER_CONFIG.chainId;
const VIRTUAL_CHAIN = VIRTUAL_CHAIN_ID_BASE + 42;
const RECIPIENT = new ethers.Wallet(ethers.id('portfolio repair recipient')).address;
const OTHER = new ethers.Wallet(ethers.id('portfolio repair other account')).address;
const TOKEN_ADDRESSES = Array.from(
  { length: 5 },
  (_, i) => new ethers.Wallet(ethers.id(`portfolio token ${i}`)).address
);
const codec = new ethers.Interface([
  'function transfer(address to, uint256 amount) returns (bool)',
]);
type SendState = {
  form: ReturnType<typeof useSendForm>;
  tokens: ReturnType<typeof useSendableTokens>;
};
type Portfolio = Awaited<ReturnType<typeof fetchPortfolio>>;
let server: Server;
let serverUrl: string;
let peer: TestInjectedProvider;
let target: EventTarget;
let stopAnnouncement: () => void;
let client: QueryClient;
let root: Root | null;
let container: HTMLDivElement;
let latestSend: SendState | null;
let latestWallet: WalletContextValue | null;
let latestPortfolio: Portfolio | undefined;
let blockUntil: { current: number };
let config: WalletConfig;
let holdings: TokenBalance[];
let balances: Map<string, string>;
let requests: string[];
let failReads: boolean;

function tokenBalance(index: number, decimals = '0', value = '7', rate = '2'): TokenBalance {
  return {
    value,
    token_id: null,
    token_instance: null,
    token: {
      address_hash: TOKEN_ADDRESSES[index],
      name: `Portfolio token ${index}`,
      symbol: `UNIT${index}`,
      decimals,
      type: 'ERC-20',
      holders_count: '1',
      exchange_rate: rate,
      volume_24h: null,
      total_supply: value,
      icon_url: null,
      circulating_market_cap: null,
    },
  };
}
function respond(response: ServerResponse, body: unknown) {
  response.setHeader('content-type', 'application/json');
  response.end(JSON.stringify(body));
}
function queryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity, gcTime: Infinity } },
  });
}
beforeEach(async () => {
  localStorage.clear();
  sessionStorage.clear();
  peer = new TestInjectedProvider(ROOT_CHAIN);
  peer.authorised = true;
  target = new EventTarget();
  stopAnnouncement = announceInjected(target, peer).stop;
  custodyChoiceRepository.write('injected');
  holdings = TOKEN_ADDRESSES.map((_, i) => tokenBalance(i));
  balances = new Map([
    [peer.wallet.address.toLowerCase(), ethers.parseEther('12').toString()],
    [OTHER.toLowerCase(), ethers.parseEther('20').toString()],
  ]);
  requests = [];
  failReads = false;
  latestSend = null;
  latestWallet = null;
  latestPortfolio = undefined;
  root = null;
  server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', 'http://localhost');
    const read = parseWalletReadRequest(url);
    for (const [key, value] of Object.entries(walletReadResponseHeaders(read)))
      response.setHeader(key, value);
    const path = url.pathname;
    requests.push(path);
    if (failReads) {
      response.statusCode = 503;
      respond(response, { error: 'recorded temporary indexer failure' });
      return;
    }
    const match = path.match(/^\/api\/v2\/addresses\/(0x[0-9a-fA-F]{40})(.*)$/);
    if (!match) {
      response.statusCode = 404;
      respond(response, { error: `Unexpected route ${path}` });
      return;
    }
    switch (match[2]) {
      case '':
        respond(response, {
          hash: match[1],
          coin_balance: balances.get(match[1].toLowerCase()) ?? '0',
          exchange_rate: '1',
        });
        return;
      case '/token-balances':
        respond(response, holdings);
        return;
      case '/counters':
        respond(response, {
          transactions_count: '0',
          token_transfers_count: '0',
          gas_usage_count: '0',
          validations_count: '0',
        });
        return;
      default:
        response.statusCode = 404;
        respond(response, { error: `Unexpected suffix ${match[2]}` });
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Portfolio fixture did not bind.');
  serverUrl = `http://127.0.0.1:${address.port}`;
  initBlockscout({ baseUrl: serverUrl });
  config = {
    chainId: ROOT_CHAIN,
    gatewayUrl: serverUrl,
    rpcUrl: serverUrl,
    identityUrl: serverUrl,
    identityKey: 'unused-by-injected-wallet',
    authRedirectUrl: null,
  };
  client = queryClient();
  container = document.createElement('div');
  document.body.appendChild(container);
});
afterEach(async () => {
  await stop();
  client.clear();
  stopAnnouncement();
  container.remove();
  await new Promise<void>((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
    server.closeAllConnections();
  });
});
function SendProbe() {
  const wallet = useWallet();
  const form = useSendForm();
  const tokens = useSendableTokens(wallet.address ?? undefined);
  useLayoutEffect(() => {
    latestSend = { form, tokens };
    latestWallet = wallet;
  });
  return null;
}
function ApplyProbe({ address, loading = false }: { address: string; loading?: boolean }) {
  const portfolio = usePortfolioQuery(address);
  const guard = useRef(0);
  useApplyPendingSend({
    address,
    loading: loading || portfolio.isPending,
    optimisticBlockUntilRef: guard,
  });
  useLayoutEffect(() => {
    latestPortfolio = portfolio.data;
    blockUntil = guard;
  });
  return null;
}
function current() {
  if (!latestSend) throw new Error('Send producer is not mounted.');
  return latestSend;
}
async function until(check: () => boolean, timeout = 4000) {
  await expect
    .poll(
      async () => {
        await act(async () => {
          await new Promise((resolve) => setTimeout(resolve, 10));
        });
        return check();
      },
      { timeout, interval: 20 }
    )
    .toBe(true);
}
async function mount(children: React.ReactElement) {
  await act(async () => {
    root ??= createRoot(container);
    root.render(
      createElement(QueryClientProvider, {
        client,
        children: createElement(WalletProvider, {
          config,
          identity: null,
          eventTarget: target,
          children,
        }),
      })
    );
  });
}
async function stop() {
  await act(async () => {
    root?.unmount();
  });
  root = null;
  latestSend = null;
  latestWallet = null;
  latestPortfolio = undefined;
}
async function readySend() {
  await mount(createElement(SendProbe));
  await until(() => latestWallet?.status === 'ready' && !!latestSend && !latestSend.tokens.loading);
}
async function submit(
  tokenAddress: string | undefined,
  amount: string
): Promise<SubmittedTransfer> {
  const token = current().tokens.tokens.find((item) => item.address === tokenAddress);
  if (!token) throw new Error('Production token list has no requested token.');
  await act(async () => {
    current().form.setTo(RECIPIENT);
    current().form.setAmount(amount);
  });
  const count = peer.sent.length;
  await act(async () => {
    await current().form.submit(token);
  });
  expect(current().form.error).toBe('');
  expect(peer.sent.length).toBe(count + 1);
  const transfer = current().form.transfer;
  if (!transfer) throw new Error('Real send producer did not submit.');
  expect(transfer.hash).toBe(ethers.Transaction.from(peer.sent[count]).hash);
  return transfer;
}
async function selectNetwork(chainId: number) {
  await act(async () => {
    preferencesRepository.update((value) => ({
      ...value,
      activeChainId: chainId === ROOT_CHAIN ? null : chainId,
      networks:
        chainId === ROOT_CHAIN
          ? value.networks
          : [
              {
                ...virtualNetwork(chainId, 'Portfolio tenant'),
                rpcUrl: 'https://rpc.fixture.invalid/chain/42',
              },
            ],
    }));
    announceRpcChanged();
  });
  peer.chainId = chainId;
  config = { ...config, chainId };
  await readySend();
  await until(() => latestWallet?.chainId === chainId);
}
function cached(address = peer.wallet.address) {
  const value = client.getQueryData<Portfolio>(queryKeys.portfolio(address));
  if (!value) throw new Error('Production portfolio cache is empty.');
  return value;
}
function click(text: string) {
  const button = Array.from(container.querySelectorAll('button')).find(
    (item) => item.textContent?.trim() === text
  );
  if (!button) throw new Error(`Missing real control: ${text}`);
  button.click();
}

async function portfolio_zero_decimals_all_layouts_and_unknown_precision() {
  const portfolio = await fetchPortfolio(peer.wallet.address);
  const props = {
    hidden: false,
    loading: false,
    nativeBalance: '12',
    nativeValueUsd: 12,
    allHoldingsCount: 5,
  };
  await mount(createElement(HoldingsGrid, { ...props, holdings: portfolio.token_holdings }));
  const balancesShown = Array.from(container.querySelectorAll('p')).filter(
    (item) => item.textContent === '7'
  );
  expect(balancesShown).toHaveLength(5);
  expect(container.textContent).not.toContain('0.0000');
  for (const decimals of [null, undefined, -1, 37, 1.5, NaN, Infinity]) {
    const unknown: HoldingsGridHolding[] = portfolio.token_holdings.map((holding) => ({
      ...holding,
      decimals,
    }));
    await mount(createElement(HoldingsGrid, { ...props, holdings: unknown }));
    expect(
      Array.from(container.querySelectorAll('p')).filter(
        (item) => item.textContent === 'Precision unavailable'
      )
    ).toHaveLength(5);
  }
  expect(requests.some((path) => path.endsWith('/token-balances'))).toBe(true);
}

async function portfolio_filters_apply_to_cards_grid_and_small_portfolio_after_restart() {
  holdings[1] = tokenBalance(1, '0', '1', '0.1');
  client.setQueryData(queryKeys.paxPrice('PAX'), { latest: 0 });
  await mount(createElement(PortfolioWidget, { onNavigate: () => {} }));
  await until(() => !!container.querySelector('[aria-label="Token grid"]'));
  await act(async () => {
    click('Filter');
  });
  await act(async () => {
    click('UNIT0');
    click('Hide dust (<$1)');
  });
  for (const label of ['Token cards', 'Token grid']) {
    const view = container.querySelector(`[aria-label="${label}"]`);
    expect(view?.textContent).not.toContain('Portfolio token 0');
    expect(view?.textContent).not.toContain('Portfolio token 1');
    expect(view?.textContent).toContain('Portfolio token 2');
  }
  expect(portfolioFiltersRepository.read()).toMatchObject({
    hideDust: true,
    hiddenTokens: [TOKEN_ADDRESSES[0].toLowerCase()],
  });
  await stop();
  client.clear();
  client = queryClient();
  holdings = holdings.slice(0, 2);
  client.setQueryData(queryKeys.paxPrice('PAX'), { latest: 0 });
  await mount(createElement(PortfolioWidget, { onNavigate: () => {} }));
  await until(() => !!container.querySelector('[aria-label="Token cards"]'));
  expect(container.querySelector('[aria-label="Token cards"]')?.textContent).not.toContain(
    'Portfolio token'
  );
  await act(async () => {
    click('Filter');
  });
  await act(async () => {
    click('UNIT0');
    click('Hide dust (<$1)');
  });
  expect(container.querySelector('[aria-label="Token cards"]')?.textContent).toContain(
    'Portfolio token 0'
  );
  expect(container.querySelector('[aria-label="Token cards"]')?.textContent).toContain(
    'Portfolio token 1'
  );
  expect(container.querySelector('[aria-label="Token grid"]')).toBeNull();
}

async function pending_send_account_chain_hash_survive_restart_without_cross_account_delta() {
  await readySend();
  const sender = peer.wallet.address;
  const first = await submit(TOKEN_ADDRESSES[0], '1');
  const second = await submit(undefined, '2');
  await selectNetwork(VIRTUAL_CHAIN);
  const virtual = await submit(undefined, '3');
  expect(Object.keys(scopedPendingSendsRepository.read())).toHaveLength(3);
  const virtualKey = `${VIRTUAL_CHAIN}:${sender.toLowerCase()}:${virtual.hash.toLowerCase()}`;
  expect(scopedPendingSendsRepository.read()[virtualKey].event.chainId).toBe(
    Number(ethers.Transaction.from(peer.sent[2]).chainId)
  );
  await stop();
  await mount(createElement(ApplyProbe, { address: OTHER }));
  await until(() => !!latestPortfolio);
  expect(cached(OTHER).native_balance.balance_raw).toBe(ethers.parseEther('20').toString());
  expect(Object.keys(scopedPendingSendsRepository.read())).toHaveLength(3);
  await mount(createElement(ApplyProbe, { address: sender }));
  await until(
    () => !!latestPortfolio && latestPortfolio.address.toLowerCase() === sender.toLowerCase()
  );
  expect(cached().native_balance.balance_raw).toBe(ethers.parseEther('12').toString());
  expect(cached().token_holdings[0].balance_raw).toBe('7');
  await stop();
  await act(async () => {
    preferencesRepository.update((value) => ({ ...value, activeChainId: null }));
    announceRpcChanged();
  });
  await mount(createElement(ApplyProbe, { address: sender }));
  await until(() => cached().token_holdings[0].balance_raw === '6');
  expect(cached().native_balance.balance_raw).toBe(ethers.parseEther('10').toString());
  expect(scopedPendingSendsRepository.read()[virtualKey].reconcileAt).toBeNull();
  const records = scopedPendingSendsRepository.read();
  expect(
    Object.values(records)
      .map(({ event }) => event.txHash)
      .sort()
  ).toEqual([first.hash, second.hash, virtual.hash].sort());
  await stop();
  await mount(createElement(ApplyProbe, { address: sender }));
  expect(cached().token_holdings[0].balance_raw).toBe('6');
  expect(cached().native_balance.balance_raw).toBe(ethers.parseEther('10').toString());
}

async function pending_zero_decimals_real_signed_send_updates_raw_units_and_recovers() {
  await readySend();
  const transfer = await submit(TOKEN_ADDRESSES[0], '1');
  expect(codec.decodeFunctionData('transfer', ethers.Transaction.from(peer.sent[0]).data)[1]).toBe(
    1n
  );
  const records = scopedPendingSendsRepository.read();
  const recovery = Object.values(records)[0];
  expect(recovery.event).toMatchObject({
    sender: peer.wallet.address,
    chainId: ROOT_CHAIN,
    txHash: transfer.hash,
    decimals: 0,
    amount: '1',
  });
  expect(records[pendingSendKey(recovery.event)]).toEqual(recovery);
  await stop();
  await mount(createElement(ApplyProbe, { address: peer.wallet.address }));
  await until(() => cached().token_holdings[0].balance_raw === '6');
  expect(cached().token_holdings[0].balance).toBe('6');
  await stop();
  await mount(createElement(ApplyProbe, { address: peer.wallet.address }));
  expect(cached().token_holdings[0].balance_raw).toBe('6');
  // A new application cache is freshly fetched from the real indexer; a durable
  // applied entry must not subtract a second time after this colder restart.
  await stop();
  client.clear();
  client = queryClient();
  holdings[0] = tokenBalance(0, '0', '6');
  await mount(createElement(ApplyProbe, { address: peer.wallet.address }));
  await until(() => !!latestPortfolio);
  expect(cached().token_holdings[0].balance_raw).toBe('6');
  expect(Object.keys(scopedPendingSendsRepository.read())).toHaveLength(1);
}

async function pending_unknown_or_mismatched_precision_never_converts() {
  for (const decimals of [NaN, -1, 37, 1.5, 18]) {
    client.setQueryData(
      queryKeys.portfolio(peer.wallet.address),
      await fetchPortfolio(peer.wallet.address)
    );
    await readySend();
    await submit(TOKEN_ADDRESSES[0], '1');
    const portfolio = await fetchPortfolio(peer.wallet.address);
    // Exercise the actual cache boundary with unavailable or conflicting metadata.
    portfolio.token_holdings[0] = { ...portfolio.token_holdings[0], decimals };
    await stop();
    client.setQueryData(queryKeys.portfolio(peer.wallet.address), portfolio);
    await mount(createElement(ApplyProbe, { address: peer.wallet.address }));
    expect(cached().token_holdings[0].balance_raw).toBe('7');
    expect(
      Object.values(scopedPendingSendsRepository.read()).every(
        (entry) => entry.reconcileAt !== null
      )
    ).toBe(true);
    await stop();
  }
}

async function pending_reconcile_survives_loading_account_change_and_reopen() {
  await readySend();
  const sender = peer.wallet.address;
  await submit(TOKEN_ADDRESSES[0], '1');
  await stop();
  await mount(createElement(ApplyProbe, { address: sender }));
  await until(() => cached().token_holdings[0].balance_raw === '6');
  const due = Object.values(scopedPendingSendsRepository.read())[0].reconcileAt!;
  expect(blockUntil.current).toBe(due + 5_000);
  await mount(createElement(ApplyProbe, { address: sender, loading: true }));
  await mount(createElement(ApplyProbe, { address: sender, loading: false }));
  expect(cached().token_holdings[0].balance_raw).toBe('6');
  await mount(createElement(ApplyProbe, { address: OTHER }));
  await until(() => latestPortfolio?.address.toLowerCase() === OTHER.toLowerCase());
  expect(blockUntil.current).toBe(0);
  expect(Object.values(scopedPendingSendsRepository.read())[0].reconcileAt).toBe(due);
  await stop();
  await mount(createElement(ApplyProbe, { address: sender }));
  holdings[0] = tokenBalance(0, '0', '5');
  const before = requests.length;
  await until(
    () => Object.keys(scopedPendingSendsRepository.read()).length === 0,
    RECONCILE_DELAY_MS + 5000
  );
  expect(cached().token_holdings[0].balance_raw).toBe('5');
  expect(cached(OTHER).native_balance.balance_raw).toBe(ethers.parseEther('20').toString());
  expect(blockUntil.current).toBe(0);
  expect(requests.slice(before).every((path) => path.includes(sender))).toBe(true);
  expect(requests.slice(before).filter((path) => path.endsWith('/token-balances'))).toHaveLength(2);
}

async function pending_failed_reconcile_remains_durable_and_retries() {
  await readySend();
  await submit(undefined, '1');
  await stop();
  await mount(createElement(ApplyProbe, { address: peer.wallet.address }));
  await until(() => cached().native_balance.balance_raw === ethers.parseEther('11').toString());
  const due = Object.values(scopedPendingSendsRepository.read())[0].reconcileAt!;
  failReads = true;
  await until(
    () => Object.values(scopedPendingSendsRepository.read())[0]?.reconcileAt! > due,
    RECONCILE_DELAY_MS + 5000
  );
  expect(cached().native_balance.balance_raw).toBe(ethers.parseEther('11').toString());
  expect(Object.keys(scopedPendingSendsRepository.read())).toHaveLength(1);
  failReads = false;
  balances.set(peer.wallet.address.toLowerCase(), ethers.parseEther('10').toString());
  await stop();
  await mount(createElement(ApplyProbe, { address: peer.wallet.address }));
  await until(
    () => Object.keys(scopedPendingSendsRepository.read()).length === 0,
    RECONCILE_DELAY_MS + 5000
  );
  expect(cached().native_balance.balance_raw).toBe(ethers.parseEther('10').toString());
}

test(
  portfolio_zero_decimals_all_layouts_and_unknown_precision.name,
  portfolio_zero_decimals_all_layouts_and_unknown_precision
);
test(
  portfolio_filters_apply_to_cards_grid_and_small_portfolio_after_restart.name,
  portfolio_filters_apply_to_cards_grid_and_small_portfolio_after_restart
);
test(
  pending_send_account_chain_hash_survive_restart_without_cross_account_delta.name,
  pending_send_account_chain_hash_survive_restart_without_cross_account_delta
);
test(
  pending_zero_decimals_real_signed_send_updates_raw_units_and_recovers.name,
  pending_zero_decimals_real_signed_send_updates_raw_units_and_recovers
);
test(
  pending_unknown_or_mismatched_precision_never_converts.name,
  pending_unknown_or_mismatched_precision_never_converts
);
test(
  pending_reconcile_survives_loading_account_change_and_reopen.name,
  pending_reconcile_survives_loading_account_change_and_reopen,
  75000
);
test(
  pending_failed_reconcile_remains_durable_and_retries.name,
  pending_failed_reconcile_remains_durable_and_retries,
  140000
);
