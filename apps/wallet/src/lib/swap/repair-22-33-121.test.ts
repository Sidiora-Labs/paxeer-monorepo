import { createServer, type Server } from 'node:http';
import { once } from 'node:events';
import { ethers } from 'ethers';
import { afterAll, beforeAll, beforeEach, expect, test } from 'vitest';
import { createSwapEvm } from './evm-22-33-121.fixture';
import { executeSwap, prepareSidioraMultihop } from './executor';
import { fetchLaunchpadTokens, getCachedLaunchpadTokens } from './hlpmm-v2';
import { getSwapQuotes, getBestQuote } from './quoter';
import { isSidioraToken, type SwapToken } from './constants';
import {
  assertQuoteNetwork,
  quoteNetwork,
  resolveTokenCapability,
  setSwapNetwork,
  tokenCapability,
} from './network';
import { SwapRecovery } from './recovery';
import { CONTRACTS, TOKENS, SIDIORA_ROUTER_ABI, quoteBuy, quoteMultihop, quoteSell } from './sdk';
import type { SwapParams, SwapQuote } from './types';
import type { SwapStorage } from './storage';

class MemorySwapStorage implements SwapStorage {
  readonly records = new Map<string, string>();
  getItem(key: string) {
    return this.records.get(key) ?? null;
  }
  setItem(key: string, value: string) {
    this.records.set(key, value);
  }
  removeItem(key: string) {
    this.records.delete(key);
  }
}

let evm: Awaited<ReturnType<typeof createSwapEvm>>;
let stack: Awaited<ReturnType<Awaited<ReturnType<typeof createSwapEvm>>['markets']>>;
let server: Server;
let base: string;
let storage: MemorySwapStorage;
let indexMode: 'complete' | 'failed' | 'bounded' = 'complete';
let requests: string[] = [];
let selected = 0;
let holdPage: (() => void) | undefined;
let pageStarted: (() => void) | undefined;

function selectNetwork(
  endpoint = `memory://swap/${++selected}`,
  chainId = 125,
  provider: ethers.Provider = evm.provider
) {
  setSwapNetwork({
    chainId,
    endpoint,
    provider,
    indexerUrl: base,
    metadataUrl: `${base}/metadata`,
  });
}

function parameters(
  tokenIn = stack.markets[0].token,
  tokenOut = stack.markets[1].token
): SwapParams {
  return {
    tokenIn,
    tokenOut,
    amountIn: ethers.parseUnits('1000', 6).toString(),
    slippageBps: 213,
    recipient: evm.owner,
  };
}

async function confirmed(): Promise<{ quote: SwapQuote; params: SwapParams }> {
  const params = parameters();
  const quotes = await getSwapQuotes(params);
  expect(quotes).toHaveLength(1);
  expect(quotes[0].protocol).toBe('sidiora-multihop');
  return { quote: quotes[0], params };
}

beforeAll(async () => {
  evm = await createSwapEvm();
  stack = await evm.markets();
  // The indexer peer derives its catalogue from tokens produced by the real factory.
  const catalogue = await Promise.all(
    stack.markets.map(async (market) => ({
      address_hash: market.token.address,
      name: await market.contract.name(),
      symbol: await market.contract.symbol(),
      decimals: String(await market.contract.decimals()),
      type: 'ERC-20',
      icon_url: null,
    }))
  );
  const unknown = new ethers.Contract(
    await stack.ordinary.getAddress(),
    [
      'function name() view returns (string)',
      'function symbol() view returns (string)',
      'function decimals() view returns (uint8)',
    ],
    evm.provider
  );
  const ordinary = {
    address_hash: await stack.ordinary.getAddress(),
    name: await unknown.name(),
    symbol: await unknown.symbol(),
    decimals: String(await unknown.decimals()),
    type: 'ERC-20',
    icon_url: null,
  };
  server = createServer(async (request, response) => {
    const url = new URL(request.url!, 'http://localhost');
    requests.push(`${url.pathname}${url.search}`);
    if (url.pathname === '/metadata') {
      response.setHeader('Content-Type', 'application/json');
      response.end('{}');
      return;
    }
    if (url.pathname !== '/api/v2/tokens') {
      response.writeHead(404).end();
      return;
    }
    if (pageStarted) {
      const notify = pageStarted;
      pageStarted = undefined;
      notify();
      await new Promise<void>((resolve) => {
        holdPage = resolve;
      });
    }
    const page = Number(url.searchParams.get('page') ?? 0);
    if (indexMode === 'failed' && page === 1) {
      response.writeHead(503).end();
      return;
    }
    const items =
      indexMode === 'bounded'
        ? Array.from({ length: 50 }, (_, i) => catalogue[i % 2])
        : page === 0
          ? [catalogue[0], ordinary]
          : [catalogue[1]];
    const next_page_params =
      indexMode === 'bounded'
        ? page < 10
          ? { page: page + 1 }
          : null
        : page === 0
          ? { page: 1 }
          : null;
    response.setHeader('Content-Type', 'application/json');
    response.end(JSON.stringify({ items, next_page_params }));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Missing loopback server address');
  base = `http://127.0.0.1:${address.port}`;
}, 180_000);

beforeEach(() => {
  storage = new MemorySwapStorage();
  requests = [];
  indexMode = 'complete';
  selectNetwork();
});

afterAll(async () => {
  holdPage?.();
  if (server) {
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve()))
    );
  }
  evm?.provider.destroy();
});

async function finding_29_registry_capability_controls_route_selection() {
  const launchpad = { ...stack.markets[0].token, isLaunchpad: false };
  expect(tokenCapability(launchpad).state).toBe('undiscovered');
  expect(isSidioraToken(launchpad)).toBe(false);
  const member = await resolveTokenCapability(launchpad);
  expect(member).toMatchObject({
    chainId: 125,
    state: 'launchpad',
    poolAddress: stack.markets[0].pool,
  });
  expect(isSidioraToken(launchpad)).toBe(true);
  // A real ERC-20 produced by the factory is deliberately not registered in this second namespace.
  const foreign = await createSwapEvm(126);
  try {
    selectNetwork('memory://other-chain', 126, foreign.provider);
    expect(tokenCapability(launchpad).state).toBe('undiscovered');
    expect(isSidioraToken(launchpad)).toBe(false);
    // No registry deployed on this namespace: failed reads must stay undiscovered, never unsupported.
    await expect(resolveTokenCapability(launchpad)).rejects.toThrow();
    expect(tokenCapability(launchpad).state).toBe('undiscovered');
  } finally {
    foreign.provider.destroy();
  }
  selectNetwork();
  const unsupported: SwapToken = { ...launchpad, address: evm.owner, isLaunchpad: true };
  expect((await resolveTokenCapability(unsupported)).state).toBe('unsupported');
  expect(isSidioraToken(unsupported)).toBe(false);
  expect(await getSwapQuotes({ ...parameters(), tokenIn: unsupported })).toEqual([]);
}

async function finding_30_indexed_tokens_require_selected_chain_registry_membership() {
  // Add a real ERC-20 address with no registry membership and no PECOR core classification.
  const unsupported: SwapToken = {
    address: await stack.ordinary.getAddress(),
    name: 'No market',
    symbol: 'NONE',
    decimals: 18,
  };
  expect((await resolveTokenCapability(unsupported)).state).toBe('unsupported');
  const found = await fetchLaunchpadTokens({ storage });
  expect(found.map((token) => token.address.toLowerCase())).toEqual(
    stack.markets.map((m) => m.token.address.toLowerCase())
  );
  expect(found.every((token) => token.isLaunchpad)).toBe(true);
  expect(found.discovery.unsupported.map((token) => token.address)).toEqual([
    (await stack.ordinary.getAddress()).toLowerCase(),
  ]);
  expect(found.discovery).toMatchObject({
    complete: true,
    status: 'complete',
    pages: 2,
    scanned: 3,
    continuation: null,
  });
  expect(found.some((token) => token.address === TOKENS.USDL.address)).toBe(false);
  expect(getCachedLaunchpadTokens(storage)?.tokens).toEqual([...found]);
  expect(requests.filter((path) => path.startsWith('/api/v2/tokens'))).toHaveLength(2);
  // Cache and discovered memberships never carry across endpoint changes, including on the same chain.
  selectNetwork('memory://same-chain-different-tenant');
  expect(getCachedLaunchpadTokens(storage)).toBeNull();
  expect(tokenCapability(stack.markets[0].token).state).toBe('undiscovered');
}

async function finding_31_failed_pages_preserve_good_inventory_and_bound_exposes_continuation() {
  const good = await fetchLaunchpadTokens({ storage });
  const persisted = [...storage.records.entries()];
  indexMode = 'failed';
  const failed = await fetchLaunchpadTokens({ storage });
  expect(failed.discovery).toMatchObject({
    status: 'failed',
    complete: false,
    pages: 1,
    continuation: { page: '1' },
    updatedAt: good.discovery.updatedAt,
  });
  expect(failed.discovery.error).toBe('Token page failed: HTTP 503');
  expect([...failed]).toEqual([...good]);
  expect([...storage.records.entries()]).toEqual(persisted);
  indexMode = 'bounded';
  requests = [];
  const bounded = await fetchLaunchpadTokens({ storage });
  expect(bounded.discovery).toMatchObject({
    status: 'truncated',
    complete: false,
    pages: 10,
    scanned: 500,
    continuation: { page: '10' },
  });
  expect(requests.filter((path) => path.startsWith('/api/v2/tokens'))).toHaveLength(10);
  expect([...storage.records.entries()]).toEqual(persisted);
  const last = await fetchLaunchpadTokens({
    storage,
    continuation: bounded.discovery.continuation!,
  });
  expect(last.discovery).toMatchObject({
    complete: true,
    pages: 1,
    scanned: 50,
    continuation: null,
  });
  expect([...storage.records.entries()]).toEqual(persisted);
}

async function finding_8_active_network_quotes_expire_and_switch_invalidates_inflight_work() {
  const { quote, params } = await confirmed();
  expect(quote.network).toMatchObject({ chainId: 125 });
  expect(quote.network!.endpoint).toMatch(/^memory:\/\/swap\//);
  expect(quote.network!.expiresAt).toBeGreaterThan(Date.now());
  expect(quote.network!.expiresAt).toBeLessThanOrEqual(Date.now() + 30_000);
  expect(() => assertQuoteNetwork({ ...quote.network!, expiresAt: Date.now() - 1 })).toThrow(
    'quote expired'
  );
  const originalNonce = await evm.provider.getTransactionCount(evm.owner);
  selectNetwork('memory://new-namespace');
  expect(() => getBestQuote([quote])).toThrow('network changed');
  await expect(executeSwap(evm.signer, quote, params, { storage })).rejects.toThrow(
    'network changed'
  );
  expect(await evm.provider.getTransactionCount(evm.owner)).toBe(originalNonce);
  // A real provider with the wrong chain cannot produce a quote, even if its registry has matching addresses.
  selectNetwork('memory://wrong-chain', 126);
  await expect(getSwapQuotes(params)).rejects.toThrow('provider chain mismatch');
  selectNetwork();
  let quoteStarted!: () => void;
  let releaseQuote!: () => void;
  const entered = new Promise<void>((resolve) => {
    quoteStarted = resolve;
  });
  const released = new Promise<void>((resolve) => {
    releaseQuote = resolve;
  });
  const delayed = new ethers.BrowserProvider(
    {
      request: async (args) => {
        const values = args.params as readonly unknown[] | undefined;
        const input = values?.[0] as { data?: string } | undefined;
        if (
          args.method === 'eth_call' &&
          input?.data?.startsWith(ethers.id('quoteMultihop(address,address,uint256)').slice(0, 10))
        ) {
          quoteStarted();
          await released;
        }
        return evm.request({ method: args.method, params: values });
      },
    },
    undefined,
    { cacheTimeout: -1 }
  );
  try {
    selectNetwork('memory://delayed', 125, delayed);
    const pendingQuote = getSwapQuotes(params);
    const quoteRejected = expect(pendingQuote).rejects.toThrow('network changed');
    await entered;
    selectNetwork('memory://after-quote-start');
    releaseQuote();
    await quoteRejected;
  } finally {
    delayed.destroy();
  }
  const started = new Promise<void>((resolve) => {
    pageStarted = resolve;
  });
  const inFlight = fetchLaunchpadTokens({ storage });
  const rejected = expect(inFlight).rejects.toThrow('network changed');
  await started;
  selectNetwork('memory://switch-during-page');
  holdPage!();
  holdPage = undefined;
  await rejected;
  expect(getCachedLaunchpadTokens(storage)).toBeNull();
  expect(storage.records.size).toBe(0);
}

async function finding_7_funded_generation_2_multihop_receipt_and_minimum_revert() {
  const { quote, params } = await confirmed();
  const [first, second] = stack.markets;
  const beforeIn: bigint = await first.contract.balanceOf(evm.owner);
  const beforeOut: bigint = await second.contract.balanceOf(evm.owner);
  const depositorRole: string = await stack.treasury.DEPOSITOR_ROLE();
  const accumulatorAddress = await stack.accumulator.getAddress();
  const preparedWithoutDepositor = prepareSidioraMultihop(evm.signer, quote, params);
  await stack.send(stack.treasury, 'revokeRole', depositorRole, accumulatorAddress);
  try {
    let refusal: unknown;
    try {
      await preparedWithoutDepositor.method.staticCall(...preparedWithoutDepositor.args);
    } catch (error) {
      refusal = error;
    }
    expect(ethers.isError(refusal, 'CALL_EXCEPTION')).toBe(true);
    if (!ethers.isError(refusal, 'CALL_EXCEPTION')) throw refusal;
    expect(refusal.data?.slice(0, 10)).toBe('0x0161a64a');
    const decoded = stack.treasury.interface.parseError(refusal.data!);
    expect(decoded?.name).toBe('MissingRole');
    expect(decoded?.args.toArray()).toEqual([accumulatorAddress, depositorRole]);
    expect(depositorRole).toBe(ethers.id('DEPOSITOR_ROLE'));
    expect(await first.contract.balanceOf(evm.owner)).toBe(beforeIn);
    expect(await second.contract.balanceOf(evm.owner)).toBe(beforeOut);
  } finally {
    await stack.send(stack.treasury, 'grantRole', depositorRole, accumulatorAddress);
  }
  const result = await executeSwap(evm.signer, quote, params, { storage });
  expect(result.receipt).toBeInstanceOf(ethers.TransactionReceipt);
  expect(result.receipt.status).toBe(1);
  expect(result.receipt.to).toBe(CONTRACTS.Router);
  const tx = await evm.provider.getTransaction(result.hash);
  const call = new ethers.Interface(SIDIORA_ROUTER_ABI).parseTransaction({ data: tx!.data });
  expect(call?.name).toBe('swapTokenForToken');
  const minimum = (BigInt(quote.amountOut) * 9787n) / 10000n;
  expect(call?.args.toArray().slice(0, 4)).toEqual([
    first.token.address,
    second.token.address,
    BigInt(quote.amountIn),
    minimum,
  ]);
  expect(await first.contract.balanceOf(evm.owner)).toBe(beforeIn - BigInt(quote.amountIn));
  expect(await second.contract.balanceOf(evm.owner)).toBe(beforeOut + BigInt(quote.amountOut));
  expect(new SwapRecovery(evm.owner, quote.network!, storage).read()?.state).toBe('confirmed');
  const again = await confirmed();
  const prepared = prepareSidioraMultihop(evm.signer, again.quote, again.params);
  const protectedMinimum = (BigInt(again.quote.amountOut) * 9787n) / 10000n;
  // An independently funded purchase on the destination pool worsens the output price.
  await stack.send(
    stack.router,
    'buy',
    second.pool,
    ethers.parseUnits('10000', 6),
    0,
    ethers.MaxUint256
  );
  const moved = await quoteMultihop(
    first.token.address,
    second.token.address,
    BigInt(again.quote.amountIn),
    evm.provider
  );
  expect(moved.amountOut).toBeLessThan(protectedMinimum);
  const unchangedIn = await first.contract.balanceOf(evm.owner);
  const unchangedOut = await second.contract.balanceOf(evm.owner);
  let failure: unknown;
  try {
    await prepared.method.staticCall(...prepared.args);
  } catch (error) {
    failure = error;
  }
  expect(ethers.isError(failure, 'CALL_EXCEPTION')).toBe(true);
  if (!ethers.isError(failure, 'CALL_EXCEPTION')) throw failure;
  // Custom error comes from the real pool's minimum-output check, not selector/allowance/balance failure.
  expect(failure.data).toBe(ethers.id('SlippageExceeded()').slice(0, 10));
  await expect(
    executeSwap(evm.signer, again.quote, again.params, { storage })
  ).rejects.toMatchObject({
    code: 'CALL_EXCEPTION',
    data: ethers.id('SlippageExceeded()').slice(0, 10),
  });
  expect(await first.contract.balanceOf(evm.owner)).toBe(unchangedIn);
  expect(await second.contract.balanceOf(evm.owner)).toBe(unchangedOut);
  expect(prepared.args[3]).toBe(protectedMinimum);
}

async function finding_5_restart_reconciles_bridge_receipt_and_resumes_exact_settlement() {
  const [first, second] = stack.markets;
  const amount = ethers.parseUnits('1000', 6);
  const sold = await quoteSell(first.pool, amount, evm.provider);
  const bought = await quoteBuy(second.pool, sold.amountOut, evm.provider);
  const source = quoteNetwork(Date.now() + 120_000);
  const params = parameters();
  const quote: SwapQuote = {
    protocol: 'sidiora-multihop',
    protocolLabel: 'Historical bridge route',
    tokenIn: first.token,
    tokenOut: second.token,
    amountIn: amount.toString(),
    amountOut: bought.amountOut.toString(),
    intermediateAmount: sold.amountOut.toString(),
    amountOutDisplay: '0',
    fee: '0',
    feeBps: 0,
    priceImpact: 0,
    sufficient: true,
    network: source,
  };
  let component: SwapRecovery | undefined = new SwapRecovery(evm.owner, source, storage);
  component.begin(quote, params);
  const beforeIn = await first.contract.balanceOf(evm.owner);
  const firstReceipt = await stack.send(
    stack.router,
    'sell',
    first.pool,
    amount,
    0,
    ethers.MaxUint256
  );
  await component.recordBridgeLeg(firstReceipt.hash);
  const saved = component.read()!;
  expect(saved).toMatchObject({
    wallet: evm.owner,
    firstLegHash: firstReceipt.hash,
    receivedUsdl: sold.amountOut.toString(),
    minimumOut: ((bought.amountOut * 9787n) / 10000n).toString(),
    state: 'bridge',
  });
  expect(saved.network.chainId).toBe(125);
  expect(saved.quote.tokenOut).toEqual(second.token);
  component = undefined; // stop the consumer; only the serialized store survives
  component = new SwapRecovery(evm.owner, source, storage);
  expect(await component.reconcile()).toEqual(saved);
  expect(await component.actions()).toEqual(['resume', 'retain-bridge-assets']);
  const beforeUsdl: bigint = await stack.usdl.balanceOf(evm.owner);
  const beforeOut: bigint = await second.contract.balanceOf(evm.owner);
  const receipt = await component.resume(evm.signer);
  expect(receipt.status).toBe(1);
  expect(await stack.usdl.balanceOf(evm.owner)).toBe(beforeUsdl - sold.amountOut);
  expect(await second.contract.balanceOf(evm.owner)).toBe(beforeOut + bought.amountOut);
  expect(await first.contract.balanceOf(evm.owner)).toBe(beforeIn - amount);
  expect((await component.reconcile()).state).toBe('confirmed');
  await expect(component.resume(evm.signer)).rejects.toThrow('No settled bridge leg');
  // A distinct operation offers retain without sending a second transaction.
  const retained = new SwapRecovery(evm.owner, quoteNetwork(Date.now() + 120_000), storage);
  retained.begin(quote, params);
  const next = await stack.send(stack.router, 'sell', first.pool, amount, 0, ethers.MaxUint256);
  await retained.recordBridgeLeg(next.hash);
  const nonce = await evm.provider.getTransactionCount(evm.owner);
  const retainedState = await new SwapRecovery(evm.owner, source, storage).retainBridgeAssets();
  expect(retainedState.state).toBe('retained');
  expect(await evm.provider.getTransactionCount(evm.owner)).toBe(nonce);
  expect(await retained.actions()).toEqual([]);
  await expect(retained.resume(evm.signer)).rejects.toThrow('No settled bridge leg');
  const shortSource = quoteNetwork(Date.now() + 1500);
  const expiring = new SwapRecovery(evm.owner, shortSource, storage);
  expiring.begin({ ...quote, network: shortSource }, params);
  const expiringReceipt = await stack.send(
    stack.router,
    'sell',
    first.pool,
    amount,
    0,
    ethers.MaxUint256
  );
  await expiring.recordBridgeLeg(expiringReceipt.hash);
  await new Promise((resolve) =>
    setTimeout(resolve, Math.max(0, expiring.read()!.expiresAt - Date.now() + 2))
  );
  const expired = new SwapRecovery(evm.owner, shortSource, storage);
  expect(await expired.actions()).toEqual(['retain-bridge-assets']);
  await expect(expired.resume(evm.signer)).rejects.toThrow('Bridge confirmation expired');
  await expired.retainBridgeAssets();

  // Simulate stopping the observer after an atomic transaction hash is durably recorded.
  const fresh = await confirmed();
  let observer: SwapRecovery | undefined = new SwapRecovery(
    evm.owner,
    fresh.quote.network!,
    storage
  );
  observer.begin(fresh.quote, fresh.params);
  const prepared = prepareSidioraMultihop(evm.signer, fresh.quote, fresh.params);
  observer.submitting();
  const pending = await prepared.method.send(...prepared.args);
  observer.submitted(pending.hash);
  observer = undefined;
  const reopened = new SwapRecovery(evm.owner, fresh.quote.network!, storage);
  expect(reopened.read()).toMatchObject({ state: 'submitted', hash: pending.hash });
  expect((await reopened.reconcile()).state).toBe('confirmed');
  expect(await reopened.actions()).toEqual([]);
}

for (const proof of [
  finding_29_registry_capability_controls_route_selection,
  finding_30_indexed_tokens_require_selected_chain_registry_membership,
  finding_31_failed_pages_preserve_good_inventory_and_bound_exposes_continuation,
  finding_8_active_network_quotes_expire_and_switch_invalidates_inflight_work,
  finding_7_funded_generation_2_multihop_receipt_and_minimum_revert,
  finding_5_restart_reconciles_bridge_receipt_and_resumes_exact_settlement,
])
  test(proof.name, proof, 60_000);
