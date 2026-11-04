// @vitest-environment jsdom
import { createServer, type Server, type ServerResponse } from 'node:http';
import { once } from 'node:events';
import { act, createElement, useLayoutEffect } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { ethers } from 'ethers';
import { announceRpcChanged, PAXEER_CONFIG } from '@/lib/constants';
import { initBlockscout, type TokenBalance } from '@/lib/data';
import { fetchPortfolio } from '@/lib/api';
import { queryKeys } from '@/lib/queries';
import { consumePendingSend, subtractFromRawBalance } from '@/lib/optimistic';
import { rootNetwork, virtualNetwork, VIRTUAL_CHAIN_ID_BASE } from '@/lib/networks';
import {
  custodyChoiceRepository,
  preferencesRepository,
  submittedTransfersRepository,
} from '@/platform/storage/repositories';
import { WalletProvider, useWallet, type WalletContextValue } from '@/wallet/WalletProvider';
import { TestInjectedProvider, INJECTED_KEY, announceInjected } from '@/wallet/test/injected';
import type { WalletConfig } from '@/wallet/config';
import { useSendWidgetState } from './SendWidget';
import { readTokenPrecision } from './useSendableTokens';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const ROOT = PAXEER_CONFIG.chainId;
const VIRTUAL = VIRTUAL_CHAIN_ID_BASE + 42;
const RECIPIENT = new ethers.Wallet(ethers.id('send repair recipient')).address;
const OTHER_RECIPIENT = new ethers.Wallet(ethers.id('send repair other recipient')).address;
const WHOLE = new ethers.Wallet(ethers.id('zero decimal token')).address;
const SIX = new ethers.Wallet(ethers.id('six decimal token')).address;
const UNKNOWN = new ethers.Wallet(ethers.id('unknown precision token')).address;
const transferCodec = new ethers.Interface([
  'function transfer(address to, uint256 amount) returns (bool)',
]);

// This committed signing fixture uses the existing EIP-1193 peer's real ethers
// signing and event transport, with a second key for account-change coverage.
class AccountSigningPeer extends TestInjectedProvider {
  override wallet = new ethers.Wallet(INJECTED_KEY);

  changeAccount() {
    this.wallet = new ethers.Wallet(ethers.id('send repair second account'));
    this.emit('accountsChanged', [this.wallet.address]);
  }
}

type SendState = ReturnType<typeof useSendWidgetState>;
let server: Server;
let serverUrl: string;
let peer: AccountSigningPeer;
let target: EventTarget;
let stopAnnouncement: () => void;
let client: QueryClient;
let root: Root | null;
let container: HTMLDivElement;
let latest: SendState | null;
let walletValue: WalletContextValue | null;
let selectedChain: number;
let config: WalletConfig;
let cold: boolean;
let held: Array<() => void>;
let balances: Map<string, string>;
let holdings: TokenBalance[];
let requests: string[];

function tokenBalance(address: string, decimals: string, value: string): TokenBalance {
  return {
    value,
    token_id: null,
    token_instance: null,
    token: {
      address_hash: address,
      name: `Fixture ${decimals}`,
      symbol: decimals === '0' ? 'WHOLE' : 'SIX',
      decimals,
      type: 'ERC-20',
      holders_count: '1',
      exchange_rate: null,
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

beforeEach(async () => {
  localStorage.clear();
  sessionStorage.clear();
  peer = new AccountSigningPeer(ROOT);
  peer.authorised = true;
  target = new EventTarget();
  stopAnnouncement = announceInjected(target, peer).stop;
  custodyChoiceRepository.write('injected');
  balances = new Map([[peer.wallet.address.toLowerCase(), ethers.parseEther('12').toString()]]);
  holdings = [tokenBalance(WHOLE, '0', '7'), tokenBalance(SIX, '6', '9000000')];
  requests = [];
  cold = false;
  held = [];
  latest = null;
  walletValue = null;
  root = null;
  selectedChain = ROOT;
  server = createServer((request, response) => {
    const path = new URL(request.url ?? '/', 'http://localhost').pathname;
    requests.push(path);
    const match = path.match(/^\/api\/v2\/addresses\/(0x[0-9a-fA-F]{40})(.*)$/);
    if (!match) {
      response.statusCode = 404;
      respond(response, { error: `Unexpected fixture route: ${path}` });
      return;
    }
    const send = () => {
      if (response.destroyed) return;
      switch (match[2]) {
        case '':
          respond(response, {
            hash: match[1],
            coin_balance: balances.get(match[1].toLowerCase()) ?? '0',
            exchange_rate: null,
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
          respond(response, { error: `Unexpected fixture suffix: ${match[2]}` });
      }
    };
    if (cold) held.push(send);
    else send();
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string')
    throw new Error('The portfolio fixture did not bind a port.');
  serverUrl = `http://127.0.0.1:${address.port}`;
  initBlockscout({ baseUrl: serverUrl });
  config = {
    chainId: ROOT,
    gatewayUrl: serverUrl,
    rpcUrl: serverUrl,
    identityUrl: serverUrl,
    identityKey: 'unused-by-injected-wallet',
    authRedirectUrl: null,
  };
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity, gcTime: Infinity } },
  });
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(async () => {
  cold = false;
  for (const send of held.splice(0)) send();
  await stop();
  client.clear();
  stopAnnouncement();
  container.remove();
  await new Promise<void>((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
    server.closeAllConnections();
  });
});

function Probe({ preselect }: { preselect?: string }) {
  const state = useSendWidgetState(preselect);
  const wallet = useWallet();
  useLayoutEffect(() => {
    latest = state;
    walletValue = wallet;
  });
  return null;
}

function current(): SendState {
  if (!latest) throw new Error('The real send hook did not mount.');
  return latest;
}

function wallet(): WalletContextValue {
  if (!walletValue) throw new Error('The real wallet provider did not mount.');
  return walletValue;
}

async function until(check: () => boolean) {
  await expect
    .poll(
      async () => {
        await act(async () => {
          await new Promise((resolve) => setTimeout(resolve, 5));
        });
        return check();
      },
      { timeout: 3000, interval: 10 }
    )
    .toBe(true);
}

async function render(preselect?: string) {
  await act(async () => {
    root ??= createRoot(container);
    const children = createElement(Probe, { preselect });
    const provider = createElement(WalletProvider, {
      config,
      identity: null,
      eventTarget: target,
      children,
    });
    root.render(createElement(QueryClientProvider, { client, children: provider }));
  });
  await until(() => wallet().status === 'ready');
}

async function ready(preselect?: string) {
  await render(preselect);
  await until(() => !current().tokensLoading && current().selectedToken !== null);
}

async function stop() {
  await act(async () => {
    root?.unmount();
  });
  root = null;
  latest = null;
  walletValue = null;
}

async function selectNetwork(chainId: number, walletToo = true) {
  selectedChain = chainId;
  await act(async () => {
    preferencesRepository.update((value) => ({
      ...value,
      activeChainId: chainId === ROOT ? null : chainId,
      networks:
        chainId === ROOT
          ? value.networks
          : [
              {
                ...virtualNetwork(chainId, 'Fixture tenant'),
                rpcUrl: `https://rpc.fixture.invalid/chain/${chainId - VIRTUAL_CHAIN_ID_BASE}`,
              },
            ],
    }));
    announceRpcChanged();
  });
  if (walletToo) {
    peer.chainId = chainId;
    config = { ...config, chainId };
    await render();
    await until(() => wallet().chainId === chainId && current().selectedToken !== null);
  }
}

async function scan(data: string) {
  await act(async () => {
    current().handleScan(data);
  });
}

async function sendNative(amount: string) {
  await scan(`ethereum:${RECIPIENT}@${selectedChain}?value=${ethers.parseEther(amount)}`);
  expect(current().scanError).toBe('');
  expect(current().selectedToken?.address).toBeUndefined();
  await act(async () => {
    await current().submit();
  });
  expect(current().form.error).toBe('');
  expect(current().form.transfer).not.toBeNull();
  return current().form.transfer!;
}

async function send_token_snapshot_stale_cold_load_refresh_and_account_change() {
  cold = true;
  await render();
  await until(() => held.length === 3);
  expect(current().tokensLoading).toBe(true);
  expect(current().selectedToken).toBeNull();
  await act(async () => {
    cold = false;
    for (const send of held.splice(0)) send();
  });
  await until(() => current().selectedToken?.balanceRaw === ethers.parseEther('12').toString());
  await act(async () => {
    current().form.applyPercentage(100, current().selectedToken);
  });
  expect(current().form.amount).toBe('12');
  balances.set(peer.wallet.address.toLowerCase(), ethers.parseEther('4').toString());
  await act(async () => {
    await client.refetchQueries({ queryKey: queryKeys.portfolio(peer.wallet.address) });
  });
  await until(() => current().selectedToken?.balanceRaw === ethers.parseEther('4').toString());
  await act(async () => {
    current().form.applyPercentage(25, current().selectedToken);
    current().form.setTo(RECIPIENT);
  });
  expect(current().form.amount).toBe('1');
  let valid = false;
  await act(async () => {
    valid = current().validateBeforeConfirm();
  });
  expect(valid).toBe(true);
  await act(async () => {
    current().form.setAmount('5');
  });
  await act(async () => {
    valid = current().validateBeforeConfirm();
  });
  expect(valid).toBe(false);
  expect(current().form.error).toContain('Insufficient balance');
  await act(async () => {
    current().setSelectedToken(current().tokens.find((token) => token.address === SIX)!);
    peer.changeAccount();
  });
  expect(current().selectedToken).toBeNull();
  expect(current().form.amount).toBe('');
  balances.set(peer.wallet.address.toLowerCase(), ethers.parseEther('8').toString());
  await act(async () => {
    await wallet().connectInjected(wallet().injected[0].info.uuid);
  });
  await until(() => current().selectedToken?.balanceRaw === ethers.parseEther('8').toString());
  expect(current().selectedToken?.address).toBeUndefined();
  expect(requests).toContain(`/api/v2/addresses/${peer.wallet.address}`);
}

async function send_qr_network_matches_wallet_and_selected_network_and_invalidates() {
  await ready();
  await selectNetwork(VIRTUAL);
  await scan(`ethereum:${RECIPIENT}@${VIRTUAL}?value=1000000000000000000`);
  expect(current().scanError).toBe('');
  expect(current().form.to).toBe(RECIPIENT);
  expect(current().form.amount).toBe('1.0');
  const stale = current();
  await scan(`ethereum:${OTHER_RECIPIENT}@${ROOT}?value=2000000000000000000`);
  expect(current().scanError).toContain('Switch networks explicitly');
  expect(current().form.to).toBe(RECIPIENT);
  expect(current().form.amount).toBe('1.0');
  await selectNetwork(ROOT, false);
  expect(wallet().chainId).toBe(VIRTUAL);
  expect(current().form.to).toBe('');
  expect(current().form.amount).toBe('');
  await act(async () => {
    await stale.submit();
  });
  expect(peer.sent).toHaveLength(0);
  await scan(`ethereum:${OTHER_RECIPIENT}@${ROOT}?value=1000000000000000000`);
  expect(current().scanError).toContain('same chain');
  expect(current().form.to).toBe('');
  await selectNetwork(ROOT);
  await scan(`ethereum:${OTHER_RECIPIENT}@${ROOT}?value=1000000000000000000`);
  expect(current().scanError).toBe('');
  expect(current().form.to).toBe(OTHER_RECIPIENT);
}

async function send_native_qr_asset_binds_native_and_contract_and_rejects_atomically() {
  await ready(SIX);
  expect(current().selectedToken?.address).toBe(SIX);
  await scan(`ethereum:${RECIPIENT}@${ROOT}?value=2000000000000000000`);
  expect(current().selectedToken?.address).toBeUndefined();
  expect(current().form.amount).toBe('2.0');
  await act(async () => {
    await current().submit();
  });
  const native = ethers.Transaction.from(peer.sent[0]);
  expect(native.to).toBe(RECIPIENT);
  expect(native.value).toBe(ethers.parseEther('2'));
  expect(native.data).toBe('0x');
  await act(async () => {
    current().form.clearTransfer();
  });
  await scan(`ethereum:${SIX}@${ROOT}/transfer?address=${OTHER_RECIPIENT}&uint256=1234567`);
  expect(current().selectedToken?.address).toBe(SIX);
  expect(current().form.amount).toBe('1.234567');
  const expected = {
    token: current().selectedToken,
    to: current().form.to,
    amount: current().form.amount,
  };
  for (const request of [
    `ethereum:${UNKNOWN}@${ROOT}/transfer?address=${RECIPIENT}&uint256=1`,
    `ethereum:${WHOLE}@${ROOT}/transfer?address=${RECIPIENT}&uint256=1.5`,
    `ethereum:${RECIPIENT}@${ROOT}?value=1.5`,
    `ethereum:${RECIPIENT}@${ROOT}?value=${ethers.MaxUint256 + 1n}`,
    `ethereum:${RECIPIENT}@${ROOT}?amount=1.0000000000000000001`,
    `ethereum:${RECIPIENT}@${ROOT}?value=1&amount=2`,
    `ethereum:${RECIPIENT}@${ROOT}?value=1&value=2`,
    `ethereum:${RECIPIENT}@${ROOT}?gas=bad&value=1`,
    `ethereum:invalid@${ROOT}?value=1`,
  ]) {
    await scan(request);
    expect(current().scanError, request).not.toBe('');
    expect(current().selectedToken).toBe(expected.token);
    expect(current().form.to).toBe(expected.to);
    expect(current().form.amount).toBe(expected.amount);
  }
  await act(async () => {
    await current().submit();
  });
  const token = ethers.Transaction.from(peer.sent[1]);
  expect(token.to).toBe(SIX);
  expect(token.value).toBe(0n);
  const decoded = transferCodec.decodeFunctionData('transfer', token.data);
  expect(decoded[0]).toBe(OTHER_RECIPIENT);
  expect(decoded[1]).toBe(1234567n);
}

async function send_recovery_root_chain_reopens_and_isolates_chain_and_sender() {
  await ready();
  const sender = peer.wallet.address;
  const rootTransfer = await sendNative('1');
  expect(rootTransfer.chainId).toBe(ROOT);
  await selectNetwork(VIRTUAL);
  expect(current().form.transfer).toBeNull();
  const virtualTransfer = await sendNative('2');
  expect(virtualTransfer.chainId).toBe(VIRTUAL);
  const signed = ethers.Transaction.from(peer.sent[1]);
  expect(virtualTransfer.hash).toBe(signed.hash);
  expect(virtualTransfer.chainId).toBe(Number(signed.chainId));
  const records = submittedTransfersRepository.read();
  expect(records[`${ROOT}:${sender.toLowerCase()}`]).toEqual(rootTransfer);
  expect(records[`${VIRTUAL}:${sender.toLowerCase()}`]).toEqual(virtualTransfer);
  await stop();
  await ready();
  expect(current().form.transfer).toEqual(virtualTransfer);
  expect(current().form.txHash).toBe(virtualTransfer.hash);
  expect(current().form.amount).toBe('2.0');
  expect(`${current().activeNetwork.explorerUrl}/tx/${current().form.txHash}`).toBe(
    `${virtualNetwork(VIRTUAL, 'Fixture tenant').explorerUrl}/tx/${signed.hash}`
  );
  await selectNetwork(ROOT);
  expect(current().form.transfer).toEqual(rootTransfer);
  expect(`${current().activeNetwork.explorerUrl}/tx/${current().form.txHash}`).toBe(
    `${rootNetwork().explorerUrl}/tx/${rootTransfer.hash}`
  );
  await act(async () => {
    current().form.clearTransfer();
  });
  expect(submittedTransfersRepository.read()[`${ROOT}:${sender.toLowerCase()}`]).toBeUndefined();
  expect(submittedTransfersRepository.read()[`${VIRTUAL}:${sender.toLowerCase()}`]).toEqual(
    virtualTransfer
  );
  await selectNetwork(VIRTUAL);
  expect(current().form.transfer).toEqual(virtualTransfer);
  await act(async () => {
    peer.changeAccount();
  });
  expect(current().form.transfer).toBeNull();
  await act(async () => {
    await wallet().connectInjected(wallet().injected[0].info.uuid);
  });
  await until(() => current().selectedToken !== null);
  expect(current().form.transfer).toBeNull();
  expect(submittedTransfersRepository.read()[`${VIRTUAL}:${sender.toLowerCase()}`]).toEqual(
    virtualTransfer
  );
}

async function send_zero_decimals_real_portfolio_qr_signing_optimistic_and_recovery() {
  await ready();
  const whole = current().tokens.find((token) => token.address === WHOLE)!;
  expect(whole.decimals).toBe(0);
  expect(whole.balance).toBe('7');
  expect(whole.balanceRaw).toBe('7');
  await act(async () => {
    current().setSelectedToken(whole);
    current().form.applyPercentage(100, whole);
  });
  expect(current().form.amount).toBe('7');
  await scan(`ethereum:${WHOLE}@${ROOT}/transfer?address=${RECIPIENT}&uint256=1`);
  expect(current().form.amount).toBe('1');
  await act(async () => {
    await current().submit();
  });
  expect(current().form.error).toBe('');
  const signed = ethers.Transaction.from(peer.sent[0]);
  expect(signed.to).toBe(WHOLE);
  expect(signed.value).toBe(0n);
  expect(transferCodec.decodeFunctionData('transfer', signed.data)[1]).toBe(1n);
  expect(current().form.transfer?.intent?.amountRaw).toBe('1');
  const pending = consumePendingSend()!;
  expect(pending).toMatchObject({
    tokenAddress: WHOLE,
    amount: '1',
    decimals: 0,
    txHash: signed.hash,
  });
  expect(subtractFromRawBalance(whole.balanceRaw, pending.amount, pending.decimals)).toBe('6');
  const identity = current().form.transfer;
  await stop();
  await ready();
  expect(current().form.transfer).toEqual(identity);
  expect(current().form.transfer?.decimals).toBe(0);
  expect(current().form.transfer?.intent?.amountRaw).toBe('1');
  expect(requests.some((path) => path.endsWith('/token-balances'))).toBe(true);
}

async function send_unknown_precision_never_converts_or_funds() {
  holdings.push(tokenBalance(UNKNOWN, 'not-known', '1000000000'));
  await ready();
  expect(current().tokens.some((token) => token.address === UNKNOWN)).toBe(false);
  for (const value of [null, undefined, -1, 37, 1.5, NaN, Infinity, '0', '18']) {
    expect(readTokenPrecision(value)).toBeNull();
  }
  expect(readTokenPrecision(0)).toBe(0);
  expect(readTokenPrecision(36)).toBe(36);
  const portfolio = await fetchPortfolio(peer.wallet.address);
  // The cache is also a real input boundary (including older persisted metadata).
  const missing = JSON.parse(JSON.stringify(portfolio)) as Record<string, unknown>;
  const tokenHoldings = missing.token_holdings as Array<Record<string, unknown>>;
  tokenHoldings[0].decimals = null;
  delete tokenHoldings[1].decimals;
  await act(async () => {
    client.setQueryData(queryKeys.portfolio(peer.wallet.address), missing);
  });
  await until(() => current().tokens.length === 1);
  await scan(`ethereum:${WHOLE}@${ROOT}/transfer?address=${RECIPIENT}&uint256=1`);
  expect(current().scanError).toContain('precision');
  expect(current().form.to).toBe('');
  expect(current().form.amount).toBe('');
  expect(peer.sent).toHaveLength(0);
}

test(
  send_token_snapshot_stale_cold_load_refresh_and_account_change.name,
  send_token_snapshot_stale_cold_load_refresh_and_account_change
);
test(
  send_qr_network_matches_wallet_and_selected_network_and_invalidates.name,
  send_qr_network_matches_wallet_and_selected_network_and_invalidates
);
test(
  send_native_qr_asset_binds_native_and_contract_and_rejects_atomically.name,
  send_native_qr_asset_binds_native_and_contract_and_rejects_atomically
);
test(
  send_recovery_root_chain_reopens_and_isolates_chain_and_sender.name,
  send_recovery_root_chain_reopens_and_isolates_chain_and_sender
);
test(
  send_zero_decimals_real_portfolio_qr_signing_optimistic_and_recovery.name,
  send_zero_decimals_real_portfolio_qr_signing_optimistic_and_recovery
);
test(
  send_unknown_precision_never_converts_or_funds.name,
  send_unknown_precision_never_converts_or_funds
);
