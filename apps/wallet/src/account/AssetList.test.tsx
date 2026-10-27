// @vitest-environment jsdom
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, test } from 'vitest';
import type { AssetPage, Balances } from '@paxeer/wallet';
import { AccountProvider } from './AccountProvider';
import { AssetList } from './AssetList';
import { OWNER, startAccountEndpoint, type AccountEndpoint, type Scenario } from './test/endpoint';
import { mount, type Mounted } from './test/render';

let endpoint: AccountEndpoint;
let view: Mounted | null = null;

beforeAll(async () => {
  endpoint = await startAccountEndpoint();
});

afterAll(async () => {
  await endpoint.close();
});

beforeEach(() => {
  endpoint.seen.length = 0;
  endpoint.unmatched.length = 0;
});

afterEach(async () => {
  await view?.unmount();
  view = null;
});

async function render(scenario: Scenario): Promise<Mounted> {
  view = await mount(
    <AccountProvider config={endpoint.config(scenario)}>
      <AssetList account={OWNER} />
    </AccountProvider>
  );
  return view;
}

async function asset_list_shows_one_bank_balance_per_denom(): Promise<void> {
  const balances: Balances = {
    account: OWNER,
    height: '1000',
    balances: [
      { denom: 'apax', amount: '1234567890123456789' },
      { denom: 'usdl', amount: '2345678' },
    ],
  };
  const assets: AssetPage = {
    height: '1000',
    joined_limit: 16,
    assets: [
      {
        asset_id: 'dollar',
        denom: 'usdl',
        kind: 'bank',
        name: 'Dollar',
        symbol: 'USDL',
        decimals: 6,
        supply: '1000000000000',
      },
      {
        asset_id: 'native',
        denom: 'apax',
        kind: 'native',
        name: 'Paxeer',
        symbol: 'PAX',
        decimals: 18,
        supply: '100000000000000000000',
      },
      {
        asset_id: 'points',
        denom: 'ulxp',
        kind: 'bank',
        name: 'Points',
        symbol: 'LXP',
        decimals: 6,
        supply: '1000000',
      },
    ],
  };
  const requests: { method: string; params: unknown[] }[] = [];
  const server = createServer(async (req, res) => {
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString('utf8')) as {
      id: number;
      method: string;
      params: unknown[];
    };
    requests.push({ method: body.method, params: body.params });
    res.writeHead(200, { 'content-type': 'application/json' });
    const response =
      body.method === 'px_getBalances'
        ? { result: balances }
        : body.method === 'px_listAssets'
          ? { result: assets }
          : { error: { code: -32601, message: 'Method not found' } };
    res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, ...response }));
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    const config = {
      ...endpoint.config('merged'),
      endpointUrl: `http://127.0.0.1:${(server.address() as AddressInfo).port}`,
    };
    view = await mount(
      <AccountProvider config={config}>
        <AssetList account={OWNER} />
      </AccountProvider>
    );
    const mounted = view;
    await mounted.until(
      () => mounted.query('[aria-label="Bank balances"]') !== null,
      'bank balances'
    );
    expect(mounted.all('[aria-label="Bank balances"] > li')).toHaveLength(2);
    expect(mounted.all('[data-denom]').map((row) => row.getAttribute('data-denom'))).toEqual([
      'apax',
      'usdl',
    ]);
    expect(mounted.text('[data-denom="apax"] .font-semibold')).toBe('PAX');
    expect(mounted.text('[data-denom="apax"] [aria-label="PAX balance"]')).toBe(
      '1.234567890123456789PAX'
    );
    expect(mounted.text('[data-denom="usdl"] .font-semibold')).toBe('USDL');
    expect(mounted.text('[data-denom="usdl"] [aria-label="USDL balance"]')).toBe('2.345678USDL');
    expect(mounted.query('[data-denom="ulxp"]')).toBeNull();
    expect(mounted.query('[data-completed]')).toBeNull();
    expect(mounted.query('[data-side]')).toBeNull();
    expect(mounted.query('[role="alert"]')).toBeNull();
    expect(requests).toHaveLength(2);
    expect(requests).toEqual(
      expect.arrayContaining([
        { method: 'px_getBalances', params: [OWNER] },
        { method: 'px_listAssets', params: [] },
      ])
    );
  } finally {
    await view?.unmount();
    view = null;
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve()))
    );
  }
}

describe('AssetList', () => {
  it('shows joined balances on both sides and completes the rest from the chain', async () => {
    const mounted = await render('available');
    await mounted.until(() => mounted.query('[data-join-limit]') !== null, 'balances');
    const lxp = mounted.query(`[data-asset="${'aa'.repeat(32)}"]`);
    expect(lxp?.textContent).toContain('LXP');
    expect(lxp?.querySelector('[data-side="paxeer"]')?.textContent).toBe('Paxeer 0.125LXP');
    expect(lxp?.querySelector('[data-side="layerx"]')?.textContent).toBe('LayerX 0.75LXP');
    const sid = mounted.query(`[data-asset="${'bb'.repeat(32)}"]`);
    expect(sid?.querySelector('[data-side="paxeer"]')?.textContent).toBe('Paxeer no balance');
    expect(sid?.querySelector('[data-side="layerx"]')?.textContent).toBe('LayerX no balance');

    expect(mounted.text('[data-completed="native:PAX"]')).toContain('Chain balance');
    expect(mounted.text('[data-completed="native:PAX"]')).toContain('1.0PAX');
    expect(mounted.text(`[data-completed="0x${'33'.repeat(20)}"]`)).toContain(
      'Token contract balance'
    );
    expect(mounted.text(`[data-completed="0x${'33'.repeat(20)}"]`)).toContain('1.0USDL');
    expect(mounted.query(`[data-completed="0x${'22'.repeat(20)}"]`)).toBeNull();
    expect(mounted.text(`[data-completed="0x${'44'.repeat(20)}"]`)).toContain('execution reverted');
    expect(mounted.query('[data-join-limit]')?.getAttribute('data-join-limit')).toBe('16');
    expect(mounted.text('[data-join-limit]')).toBe('2 of at most 16 joined assets.');
    expect(endpoint.unmatched).toEqual([]);
  });

  test(asset_list_shows_one_bank_balance_per_denom, asset_list_shows_one_bank_balance_per_denom);

  it('shows the unavailable kernel state instead of balances when the kernel is unreachable', async () => {
    const mounted = await render('kernel-unreachable');
    await mounted.until(
      () => mounted.query('[data-kernel="unavailable"]') !== null,
      'kernel notice'
    );
    expect(mounted.text('[data-kernel="unavailable"]')).toBe(
      'The LayerX kernel is unreachable (public core)'
    );
    expect(mounted.query('[data-asset]')).toBeNull();
    expect(mounted.query('[role="alert"]')).toBeNull();
  });
});
