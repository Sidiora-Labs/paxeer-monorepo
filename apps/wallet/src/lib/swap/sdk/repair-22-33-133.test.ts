// @vitest-environment jsdom
import { createServer, type Server, type ServerResponse } from 'node:http';
import { once } from 'node:events';
import { act, createElement, StrictMode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { ethers } from 'ethers';
import {
  PECOR_QUOTER_ABI,
  PECOR_VAULT_ABI,
  PRICE_ORACLE_ABI,
  SIDIORA_POOL_ABI,
  SIDIORA_QUOTER_ABI,
} from './abis';
import { CONTRACTS, TOKENS, NATIVE_TOKEN } from './addresses';
import {
  useSwapQuote,
  useTokenPrice,
  useVaultReserves,
  useSidioraPool,
  useSidioraBuyQuote,
  useSidioraSellQuote,
} from './hooks';
import { formatAmount, formatPrice, formatToken, formatNative, parseAmount } from './utils';
import type { QuoteResult } from './types';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
const A = '0x1111111111111111111111111111111111111111';
const B = '0x2222222222222222222222222222222222222222';
const OUT = '0x3333333333333333333333333333333333333333';
const oracle = new ethers.Interface(PRICE_ORACLE_ABI);
const vault = new ethers.Interface(PECOR_VAULT_ABI);
const spot = new ethers.Interface(PECOR_QUOTER_ABI);
const pool = new ethers.Interface(SIDIORA_POOL_ABI);
const launch = new ethers.Interface(SIDIORA_QUOTER_ABI);
const revert = new ethers.Interface(['error Error(string)']);

type Kind = 'price' | 'reserves' | 'pool' | 'swap' | 'buy' | 'sell';
type Inputs = { asset: string; amount: bigint; provider: ethers.JsonRpcProvider };
type View = {
  data: bigint | null;
  loading: boolean;
  error: string | null;
  refresh: () => Promise<void>;
  stale?: boolean;
  asset?: string;
};
function useSubject(kind: Kind, inputs: Inputs): View {
  const { asset, amount, provider } = inputs;
  switch (kind) {
    case 'price': {
      const value = useTokenPrice(asset, provider);
      return { ...value, data: value.price };
    }
    case 'reserves': {
      const value = useVaultReserves(asset, provider);
      return { ...value, data: value.reserves };
    }
    case 'pool': {
      const value = useSidioraPool(asset, provider);
      return { ...value, data: value.state?.price ?? null, asset: value.state?.pool };
    }
    case 'swap': {
      const value = useSwapQuote({ tokenIn: asset, tokenOut: OUT, amountIn: amount, provider });
      return { ...value, data: value.quote?.amountOut ?? null };
    }
    case 'buy': {
      const value = useSidioraBuyQuote(asset, amount, provider);
      return { ...value, data: value.quote?.amountOut ?? null };
    }
    case 'sell': {
      const value = useSidioraSellQuote(asset, amount, provider);
      return { ...value, data: value.quote?.amountOut ?? null };
    }
  }
}
function Probe({ kind, inputs }: { kind: Kind; inputs: Inputs }) {
  latest = useSubject(kind, inputs);
  renders.push({ ...latest });
  return createElement(
    'output',
    null,
    latest.error ?? (latest.data === null ? 'empty' : formatAmount(latest.data, 18, 6))
  );
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((accept) => {
    resolve = accept;
  });
  return { promise, resolve };
}
type Rpc = { jsonrpc: string; id: number; method: string; params: { to: string; data: string }[] };
type Exchange = {
  to: string;
  data: string;
  result: string;
  received: ReturnType<typeof deferred<{ response: ServerResponse; rpc: Rpc }>>;
  acknowledged: ReturnType<typeof deferred<void>>;
};
let server: Server;
let baseUrl: string;
let root: Root;
let container: HTMLDivElement;
let latest: View;
let renders: View[];
let pending: Exchange[];
let unexpected: string[];
let openResponses: ServerResponse[];
let providers: ethers.JsonRpcProvider[];
let chains: Map<string, bigint>;
let acknowledgements: Map<string, () => void>;
let calls: number;
let mounted: boolean;

beforeEach(async () => {
  pending = [];
  unexpected = [];
  openResponses = [];
  providers = [];
  chains = new Map();
  acknowledgements = new Map();
  renders = [];
  calls = 0;
  mounted = true;
  server = createServer(async (request, response) => {
    openResponses.push(response);
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const rpc = JSON.parse(Buffer.concat(chunks).toString()) as Rpc;
    response.setHeader('Content-Type', 'application/json');
    if (rpc.method === 'eth_chainId') {
      response.end(
        JSON.stringify({
          jsonrpc: '2.0',
          id: rpc.id,
          result: ethers.toQuantity(chains.get(request.url!)!),
        })
      );
      return;
    }
    calls++;
    const transaction = rpc.params[0];
    const index = pending.findIndex(
      (item) =>
        rpc.method === 'eth_call' &&
        item.to.toLowerCase() === transaction?.to.toLowerCase() &&
        item.data === transaction.data
    );
    if (index < 0) {
      unexpected.push(JSON.stringify(rpc));
      response.end(
        JSON.stringify({
          jsonrpc: '2.0',
          id: rpc.id,
          error: { code: -32601, message: 'Unexpected fixture request' },
        })
      );
      return;
    }
    const [exchange] = pending.splice(index, 1);
    acknowledgements.set(`${request.url}:${rpc.id}`, () => exchange.acknowledged.resolve());
    exchange.received.resolve({ response, rpc });
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Missing loopback address');
  baseUrl = `http://127.0.0.1:${address.port}`;
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});
afterEach(async () => {
  if (mounted) await act(async () => root.unmount());
  for (const provider of providers) provider.destroy();
  for (const response of openResponses) if (!response.writableEnded) response.end();
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve()))
  );
  container.remove();
  expect(unexpected).toEqual([]);
  expect(pending).toEqual([]);
});
async function providerFor(chainId = 125n, dynamic = false) {
  const path = `/rpc/${providers.length}`;
  chains.set(path, chainId);
  const network = ethers.Network.from(chainId);
  const provider = new ethers.JsonRpcProvider(`${baseUrl}${path}`, dynamic ? 'any' : network, {
    batchMaxCount: 1,
    cacheTimeout: -1,
    ...(dynamic ? {} : { staticNetwork: network }),
  });
  providers.push(provider);
  await provider.on('debug', (event) => {
    if (event.action !== 'receiveRpcResult') return;
    for (const result of event.result) acknowledgements.get(`${path}:${result.id}`)?.();
  });
  return provider;
}
function quoteFixture(amount: bigint, value: bigint): QuoteResult {
  return {
    amountIn: amount,
    amountOut: value,
    grossAmountOut: value,
    executionPrice: value,
    spotPriceIn: value,
    spotPriceOut: value,
    feeAmount: 0n,
    feeBps: 0n,
    sufficientLiquidity: true,
    availableLiquidity: value,
    priceTimestampIn: 1n,
    priceTimestampOut: 1n,
    priceStaleIn: false,
    priceStaleOut: false,
    utilizationCapBps: 10000n,
    tradeUtilizationBps: 0n,
    cappedByUtilization: false,
    twapOut: value,
    twapDeviationBps: 0n,
    effectiveFeeBps: 0n,
    tier: 0n,
    priceImpactBps: 0n,
    priceImpactDeduction: 0n,
  };
}
function plan(kind: Kind, inputs: Inputs, value: bigint) {
  const exchanges: Exchange[] = [];
  function add(
    to: string,
    codec: ethers.Interface,
    method: string,
    args: readonly unknown[],
    result: readonly unknown[]
  ) {
    exchanges.push({
      to,
      data: codec.encodeFunctionData(method, args),
      result: codec.encodeFunctionResult(method, result),
      received: deferred(),
      acknowledged: deferred(),
    });
  }
  switch (kind) {
    case 'price':
      add(CONTRACTS.PriceOracle, oracle, 'getPrice', [inputs.asset], [value]);
      add(CONTRACTS.PriceOracle, oracle, 'isPriceStale', [inputs.asset], [true]);
      break;
    case 'reserves':
      add(CONTRACTS.PECORVault, vault, 'getReserves', [inputs.asset], [value]);
      break;
    case 'pool':
      add(inputs.asset, pool, 'getPrice', [], [value]);
      add(inputs.asset, pool, 'getReserves', [], [value, value + 1n, value + 2n]);
      add(inputs.asset, pool, 'tokenAddress', [], [OUT]);
      break;
    case 'swap':
      add(
        CONTRACTS.PECORQuoter,
        spot,
        'quoteExactIn',
        [inputs.asset, OUT, inputs.amount],
        [quoteFixture(inputs.amount, value)]
      );
      break;
    case 'buy':
    case 'sell':
      add(
        CONTRACTS.Quoter,
        launch,
        'quoteExactInput',
        [inputs.asset, inputs.amount, kind === 'buy'],
        [[value, 1n, 2n]]
      );
      break;
  }
  pending.push(...exchanges);
  return {
    received: Promise.all(exchanges.map((exchange) => exchange.received.promise)),
    async finish(failure = false) {
      await act(async () => {
        for (const exchange of exchanges) {
          const { response, rpc } = await exchange.received.promise;
          response.end(
            JSON.stringify({
              jsonrpc: '2.0',
              id: rpc.id,
              ...(failure && exchange === exchanges[0]
                ? {
                    error: {
                      code: -32000,
                      message: 'execution reverted: Recorded upstream failure',
                      data: revert.encodeErrorResult('Error', ['Recorded upstream failure']),
                    },
                  }
                : { result: exchange.result }),
            })
          );
        }
        await Promise.all(exchanges.map((exchange) => exchange.acknowledged.promise));
        await new Promise((resolve) => setTimeout(resolve, 0));
      });
    },
  };
}
async function render(kind: Kind, inputs: Inputs) {
  await act(async () =>
    root.render(createElement(StrictMode, null, createElement(Probe, { kind, inputs })))
  );
}
async function begin(
  kind: Kind,
  inputs: Inputs,
  value: bigint,
  action = () => render(kind, inputs)
) {
  const exchange = plan(kind, inputs, value);
  await action();
  await exchange.received;
  return exchange;
}
function expectPending(kind: Kind) {
  expect(latest.data).toBeNull();
  expect(latest.loading).toBe(true);
  expect(latest.error).toBeNull();
  if (kind === 'price') expect(latest.stale).toBe(false);
  expect(container.textContent).toBe('empty');
}
function expectValue(kind: Kind, inputs: Inputs, value: bigint) {
  expect(latest.data).toBe(value);
  expect(latest.loading).toBe(false);
  expect(latest.error).toBeNull();
  if (kind === 'price') expect(latest.stale).toBe(true);
  if (kind === 'pool') expect(latest.asset).toBe(inputs.asset);
  expect(container.textContent).toBe(formatAmount(value, 18, 6));
}
async function refresh() {
  await act(async () => {
    void latest.refresh();
  });
}
async function exercise(kind: Kind) {
  const provider = await providerFor();
  const a = { asset: A, amount: 100n, provider };
  const b = { ...a, asset: B };
  // StrictMode starts and cancels its first effect; both real RPC reads must finish.
  const strictRead = plan(kind, a, 99n);
  const initial = await begin(kind, a, 1n);
  await strictRead.received;
  await initial.finish();
  await strictRead.finish();
  expectValue(kind, a, 1n);

  const staleRefresh = latest.refresh;
  const renderStart = renders.length;
  const replacement = await begin(kind, b, 2n);
  expectPending(kind);
  for (const view of renders.slice(renderStart)) expect(view.data).toBeNull();
  const count = calls;
  await act(async () => {
    await staleRefresh();
  });
  expect(calls).toBe(count);
  await replacement.finish(true);
  expect(latest.data).toBeNull();
  expect(latest.loading).toBe(false);
  expect(latest.error).toContain('Recorded upstream failure');
  expect(container.textContent).toContain('Recorded upstream failure');

  const recovering = await begin(kind, b, 3n, refresh);
  expectPending(kind);
  await recovering.finish();
  expectValue(kind, b, 3n);

  const oldSuccess = await begin(kind, a, 4n);
  await (await begin(kind, b, 5n)).finish();
  await oldSuccess.finish();
  expectValue(kind, b, 5n);

  const oldFailure = await begin(kind, a, 6n);
  const current = await begin(kind, b, 7n);
  await oldFailure.finish(true);
  expectPending(kind);
  await current.finish();
  expectValue(kind, b, 7n);

  const firstRefresh = await begin(kind, b, 8n, refresh);
  const secondRefresh = await begin(kind, b, 9n, refresh);
  await secondRefresh.finish();
  await firstRefresh.finish(true);
  expectValue(kind, b, 9n);

  const a1 = await begin(kind, a, 10n);
  const middle = await begin(kind, b, 11n);
  const a2 = await begin(kind, a, 12n);
  await a1.finish();
  expectPending(kind);
  await a2.finish();
  await middle.finish(true);
  expectValue(kind, a, 12n);

  const otherProvider = await providerFor(126n);
  const changedProvider = { ...a, provider: otherProvider };
  const priorProviderRead = await begin(kind, a, 13n, refresh);
  const newProviderRead = await begin(kind, changedProvider, 14n);
  expectPending(kind);
  await priorProviderRead.finish(true);
  expectPending(kind);
  await newProviderRead.finish();
  expectValue(kind, changedProvider, 14n);

  if (kind === 'swap' || kind === 'buy' || kind === 'sell') {
    const changedAmount = { ...changedProvider, amount: 200n };
    const amountRead = await begin(kind, changedAmount, 15n);
    expectPending(kind);
    await amountRead.finish(true);
    expect(latest.data).toBeNull();
    expect(latest.error).toContain('Recorded upstream failure');
    const outstanding = await begin(kind, changedAmount, 16n, refresh);
    await render(kind, { ...changedAmount, amount: 0n });
    expect(latest).toMatchObject({ data: null, loading: false, error: null });
    await outstanding.finish();
    expect(latest).toMatchObject({ data: null, loading: false, error: null });
  }

  const outstanding = await begin(kind, a, 17n);
  await render(kind, { ...a, asset: '' });
  expect(latest).toMatchObject({ data: null, loading: false, error: null });
  await outstanding.finish(true);
  expect(latest).toMatchObject({ data: null, loading: false, error: null });
  const beforeDisabledRefresh = calls;
  await refresh();
  expect(calls).toBe(beforeDisabledRefresh);

  const unmounted = await begin(kind, a, 18n);
  await act(async () => root.unmount());
  mounted = false;
  const renderCount = renders.length;
  await unmounted.finish();
  expect(renders).toHaveLength(renderCount);
}
async function finding51_price_binds_data_staleness_errors_and_loading_to_identity() {
  await exercise('price');
}
async function finding51_reserves_reject_late_asset_provider_and_refresh_results() {
  await exercise('reserves');
}
async function finding51_pool_rejects_late_asset_provider_and_refresh_results() {
  await exercise('pool');
}
async function finding51_swap_quote_clears_failed_and_disabled_replacements() {
  await exercise('swap');
}
async function finding51_buy_quote_clears_failed_and_disabled_replacements() {
  await exercise('buy');
}
async function finding51_sell_quote_clears_failed_and_disabled_replacements() {
  await exercise('sell');
}

async function finding51_network_events_invalidate_in_flight_reads_on_the_same_provider() {
  const provider = await providerFor(125n, true);
  const inputs = { asset: A, amount: 100n, provider };
  const strictRead = plan('reserves', inputs, 1n);
  const initial = await begin('reserves', inputs, 2n);
  await strictRead.received;
  await strictRead.finish();
  await initial.finish();
  expectValue('reserves', inputs, 2n);
  const oldChain = await begin('reserves', inputs, 3n, refresh);
  const newChain = plan('reserves', inputs, 4n);
  expect((await provider.getNetwork()).chainId).toBe(125n);
  chains.set('/rpc/0', 126n);
  await act(async () => {
    await provider.getNetwork();
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  await newChain.received;
  expectPending('reserves');
  await oldChain.finish();
  expectPending('reserves');
  await newChain.finish();
  expectValue('reserves', inputs, 4n);

  const previousChain = await begin('reserves', inputs, 5n, refresh);
  const nextChain = plan('reserves', inputs, 6n);
  expect((await provider.getNetwork()).chainId).toBe(126n);
  chains.set('/rpc/0', 127n);
  await act(async () => {
    await provider.getNetwork();
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  await nextChain.received;
  expectPending('reserves');
  await previousChain.finish(true);
  expectPending('reserves');
  await nextChain.finish();
  expectValue('reserves', inputs, 6n);
}

async function finding53_real_quote_and_oracle_values_keep_exact_large_display_digits() {
  const provider = await providerFor();
  const inputs = { asset: A, amount: 100n, provider };
  const raw = parseAmount('9007199254740993.123456789123456789', 18);
  const strictRead = plan('swap', inputs, raw);
  const quote = await begin('swap', inputs, raw);
  await strictRead.received;
  await strictRead.finish();
  await quote.finish();
  expect(latest.data).toBe(raw);
  expect(container.textContent).toBe('9,007,199,254,740,993.123457');
  expect(formatToken(latest.data!, 'WETH')).toBe('9,007,199,254,740,993.1235 WETH');
  expect(formatNative(latest.data!)).toBe(`9,007,199,254,740,993.1235 ${NATIVE_TOKEN.symbol}`);
  const price = await begin('price', inputs, raw);
  await price.finish();
  expect(latest.data).toBe(raw);
  expect(formatPrice(latest.data!)).toBe('$9007199254740993.123457');
}
function finding53_rounds_only_requested_fractional_digits_with_exact_carry_and_sign() {
  expect(formatAmount(9007199254740993n, 0)).toBe('9,007,199,254,740,993');
  expect(formatAmount(parseAmount('9007199254740993.99995', 18), 18)).toBe('9,007,199,254,740,994');
  expect(formatAmount(parseAmount('-9007199254740993.99995', 18), 18)).toBe(
    '-9,007,199,254,740,994'
  );
  expect(formatAmount(parseAmount('9007199254740993.49', 18), 18, 0)).toBe('9,007,199,254,740,993');
  expect(formatAmount(parseAmount('9007199254740993.5', 18), 18, 0)).toBe('9,007,199,254,740,994');
  expect(formatAmount(parseAmount('1.23445', 18), 18, 4)).toBe('1.2345');
  expect(formatAmount(parseAmount('-1.23445', 18), 18, 4)).toBe('-1.2345');
  expect(formatAmount(1n, 18, 18)).toBe('0.000000000000000001');
  expect(formatAmount(-1n, 18, 4)).toBe('-0');
  expect(formatAmount(0n, 18)).toBe('0');
  expect(formatAmount(parseAmount('1.2300', 6), 6, 8)).toBe('1.23');
  expect(formatPrice(0n)).toBe('$0.000000');
  expect(formatPrice(500000000000n)).toBe('$0.000001');
  expect(formatPrice(-500000000000n)).toBe('$-0.000001');
  expect(formatPrice(parseAmount('9007199254740993.9999995', 18))).toBe('$9007199254740994.000000');
  expect(formatToken(parseAmount('9007199254740993.123456', TOKENS.USDC.decimals), 'USDC')).toBe(
    '9,007,199,254,740,993.1235 USDC'
  );
  const max = 2n ** 256n - 1n;
  expect(formatAmount(max, 0, 0).replaceAll(',', '')).toBe(max.toString());
  expect(formatAmount(max, 18, 18).replaceAll(',', '')).toBe(ethers.formatUnits(max, 18));
  expect(() => formatAmount(1n, 18, -1)).toThrow(RangeError);
  expect(() => formatAmount(1n, 18, 101)).toThrow(RangeError);
  expect(() => formatAmount(1n, 18, 1.5)).toThrow(RangeError);
}

for (const proof of [
  finding51_price_binds_data_staleness_errors_and_loading_to_identity,
  finding51_reserves_reject_late_asset_provider_and_refresh_results,
  finding51_pool_rejects_late_asset_provider_and_refresh_results,
  finding51_swap_quote_clears_failed_and_disabled_replacements,
  finding51_buy_quote_clears_failed_and_disabled_replacements,
  finding51_sell_quote_clears_failed_and_disabled_replacements,
  finding51_network_events_invalidate_in_flight_reads_on_the_same_provider,
  finding53_real_quote_and_oracle_values_keep_exact_large_display_digits,
  finding53_rounds_only_requested_fractional_digits_with_exact_carry_and_sign,
])
  test(proof.name, proof);
