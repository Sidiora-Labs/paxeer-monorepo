// @vitest-environment jsdom
import { createServer, type Server } from 'node:http';
import { once } from 'node:events';
import { act, createElement, useLayoutEffect } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ethers } from 'ethers';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import { initBlockscout } from '@/lib/data/client';
import { queryKeys } from '@/lib/queries/keys';
import { shortenAddress } from '@/lib/format';
import { WalletProvider, useWallet, type WalletContextValue } from '@/wallet/WalletProvider';
import { announceInjected, INJECTED_UUID, TestInjectedProvider } from '@/wallet/test/injected';
import { TxDetailWidget } from './TxDetailWidget';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const injected = new TestInjectedProvider();
const owner = injected.wallet.address;
const router = '0x1111111111111111111111111111111111111111';
const pool = '0x2222222222222222222222222222222222222222';
const sender = '0x3333333333333333333333333333333333333333';
const hash = ethers.id('transaction detail fixture');
const codec = new ethers.Interface([
  'event Transfer(address indexed from, address indexed to, uint256 value)',
  'function decimals() view returns (uint8)',
]);

// Explorer peer fixture: endpoint values come from the ERC-20 ABI codec.
// The production HTTP client, facade, query hook, wallet provider and widget run unchanged.
function transfer(from: string, to: string, value: bigint, symbol: string, decimals: unknown) {
  const event = codec.encodeEventLog(codec.getEvent('Transfer')!, [from, to, value]);
  const decoded = codec.parseLog(event)!;
  return {
    transaction_hash: hash,
    from: { hash: decoded.args[0] as string },
    to: { hash: decoded.args[1] as string },
    total: { value: decoded.args[2].toString() as string },
    token: { symbol, decimals, icon_url: '/wallet/default_icon.webp' },
  };
}

function transaction() {
  return {
    hash,
    from: { hash: owner },
    to: { hash: router },
    value: '0',
    fee: { value: ethers.parseEther('0.000021').toString() },
    status: 'ok',
    timestamp: '2026-01-01T00:00:00Z',
  };
}

let server: Server;
let transfers: ReturnType<typeof transfer>[];
let tx: ReturnType<typeof transaction>;
let requests: string[];
let client: QueryClient;
let root: Root | null = null;
let container: HTMLDivElement;
let current: WalletContextValue | null = null;
let stopAnnouncement: (() => void) | undefined;

beforeAll(async () => {
  server = createServer((request, response) => {
    const path = request.url ?? '';
    requests.push(path);
    response.setHeader('Content-Type', 'application/json');
    if (path === `/api/v2/transactions/${hash}`) {
      response.end(JSON.stringify(tx));
    } else if (path === `/api/v2/transactions/${hash}/token-transfers`) {
      response.end(JSON.stringify({ items: transfers, next_page_params: null }));
    } else {
      response.statusCode = 404;
      response.end(JSON.stringify({ error: `Unexpected fixture request: ${path}` }));
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Fixture did not bind a port');
  initBlockscout({ baseUrl: `http://127.0.0.1:${address.port}` });
});

afterAll(async () => {
  await new Promise<void>((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
    server.closeAllConnections();
  });
});

beforeEach(() => {
  localStorage.clear();
  injected.authorised = false;
  tx = transaction();
  transfers = [];
  requests = [];
  current = null;
  client = new QueryClient();
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(async () => {
  await act(async () => root?.unmount());
  root = null;
  client.clear();
  stopAnnouncement?.();
  container.remove();
});

function Probe() {
  const wallet = useWallet();
  useLayoutEffect(() => {
    current = wallet;
  });
  return createElement(TxDetailWidget, { txHash: hash });
}

async function until(check: () => boolean) {
  await expect
    .poll(async () => {
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 10));
      });
      return check();
    })
    .toBe(true);
}

async function mount() {
  const target = new EventTarget();
  stopAnnouncement = announceInjected(target, injected).stop;
  await act(async () => {
    root = createRoot(container);
    root.render(
      createElement(
        QueryClientProvider,
        { client },
        createElement(WalletProvider, {
          config: null,
          identity: null,
          eventTarget: target,
          children: createElement(Probe),
        })
      )
    );
  });
  await until(() => current?.status === 'signed-out' && current.injected.length === 1);
  await act(async () => {
    if (!current) throw new Error('Wallet did not mount');
    await current.connectInjected(INJECTED_UUID);
  });
  await until(
    () =>
      current?.status === 'ready' &&
      client.getQueryState(queryKeys.txDetail(hash))?.status === 'success'
  );
  expect(requests).toEqual(
    expect.arrayContaining([
      `/api/v2/transactions/${hash}`,
      `/api/v2/transactions/${hash}/token-transfers`,
    ])
  );
  expect(current?.address).toBe(owner);
}

function headline() {
  const amount = container.querySelector('p.text-3xl');
  if (!amount) throw new Error('Transaction amount was not rendered');
  return amount.textContent;
}

function detail(label: string) {
  const element = Array.from(container.querySelectorAll('span')).find(
    (span) => span.textContent === label
  );
  if (!element) throw new Error(`Detail was not rendered: ${label}`);
  return element.parentElement?.textContent;
}

async function tx_detail_first_transfer_selects_wallet_endpoints() {
  transfers = [
    transfer(router, pool, ethers.parseEther('900'), 'UNRELATED', 18),
    transfer(pool, owner.toLowerCase(), ethers.parseUnits('2.5', 6), 'OUTPUT', '6'),
  ];
  await mount();
  expect(headline()).toBe('+2.5 OUTPUT');
  expect(container.textContent).toContain('Received');
  expect(detail('From')).toBe(`From${shortenAddress(pool)}`);
  expect(container.textContent).not.toContain('UNRELATED');
  expect(detail('Status')).toBe('StatusSucceeded');
  expect(detail('Network Fee')).toBe('Network Fee-0.000021 PAX');
}

async function tx_detail_multiple_wallet_transfers_keep_each_direction_and_counterparty() {
  tx.from = { hash: sender };
  transfers = [
    transfer(router, pool, 100n, 'UNRELATED', 0),
    transfer(owner.toLowerCase(), pool, 7n, 'WHOLE', 0),
    transfer(router, owner, ethers.parseUnits('1.25', 6), 'OUTPUT', 6),
  ];
  await mount();
  expect(headline()).toBe('2 token transfers');
  const rows = container.querySelector('[aria-label="Token transfers"]');
  expect(rows?.textContent).toContain(`Sent WHOLETo ${shortenAddress(pool)}-7 WHOLE`);
  expect(rows?.textContent).toContain(`Received OUTPUTFrom ${shortenAddress(router)}+1.25 OUTPUT`);
  expect(rows?.textContent).not.toContain('UNRELATED');
  expect(detail('Status')).toBe('StatusSucceeded');
  expect(detail('Network Fee')).toBe('Network Fee-0.000021 PAX');
}

async function tx_detail_zero_decimals_preserves_numeric_and_string_zero() {
  const precision = Number(
    codec.decodeFunctionResult('decimals', codec.encodeFunctionResult('decimals', [0]))[0]
  );
  expect(precision).toBe(0);
  transfers = [transfer(sender, owner, 42n, 'WHOLE', precision)];
  await mount();
  expect(headline()).toBe('+42 WHOLE');
  expect(detail('From')).toBe(`From${shortenAddress(sender)}`);
  await act(async () => {
    transfers = [transfer(owner, sender, 17n, 'WHOLE', '0')];
    await client.refetchQueries({ queryKey: queryKeys.txDetail(hash) });
  });
  await until(() => headline() === '-17 WHOLE');
  expect(detail('To')).toBe(`To${shortenAddress(sender)}`);
}

async function tx_detail_native_transfer_retains_native_scale() {
  tx.value = ethers.parseEther('2').toString();
  await mount();
  expect(headline()).toBe('-2 PAX');
  expect(detail('To')).toBe(`To${shortenAddress(router)}`);
}

async function tx_detail_unrelated_logs_are_not_wallet_movements() {
  tx.from = { hash: sender };
  transfers = [transfer(router, pool, 42n, 'OTHER', 0)];
  await mount();
  expect(headline()).toBe('No wallet transfer');
  expect(container.textContent).not.toContain('OTHER');
  expect(container.textContent).not.toContain('Received');
}

test(
  tx_detail_first_transfer_selects_wallet_endpoints.name,
  tx_detail_first_transfer_selects_wallet_endpoints
);
test(
  tx_detail_multiple_wallet_transfers_keep_each_direction_and_counterparty.name,
  tx_detail_multiple_wallet_transfers_keep_each_direction_and_counterparty
);
test(
  tx_detail_zero_decimals_preserves_numeric_and_string_zero.name,
  tx_detail_zero_decimals_preserves_numeric_and_string_zero
);
test(
  tx_detail_native_transfer_retains_native_scale.name,
  tx_detail_native_transfer_retains_native_scale
);
test(
  tx_detail_unrelated_logs_are_not_wallet_movements.name,
  tx_detail_unrelated_logs_are_not_wallet_movements
);

test.each([null, undefined, '', ' ', false, -1, 1.5, '1.5', '1e2', 81, '81', 255, NaN, Infinity])(
  'tx_detail_unknown_precision_stays_explicit_in_headline_and_rows: %s',
  async function tx_detail_unknown_precision_stays_explicit_in_headline_and_rows(precision) {
    transfers = [transfer(sender, owner, 42n, 'UNKNOWN', precision)];
    await mount();
    expect(headline()).toBe('+Amount unavailable (precision unknown; raw: 42) UNKNOWN');
    await act(async () => {
      transfers.push(transfer(owner, router, 7n, 'WHOLE', 0));
      await client.refetchQueries({ queryKey: queryKeys.txDetail(hash) });
    });
    await until(() => headline() === '2 token transfers');
    const rows = container.querySelector('[aria-label="Token transfers"]');
    expect(rows?.textContent).toContain('Amount unavailable (precision unknown; raw: 42) UNKNOWN');
    expect(rows?.textContent).toContain('-7 WHOLE');
  }
);
