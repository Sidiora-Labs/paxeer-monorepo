import React, { act, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { createServer, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { inspect } from 'node:util';
import { expect, test } from 'vitest';
import {
  BaseError,
  UserRejectedRequestError,
  createPublicClient,
  createWalletClient,
  erc20Abi,
  getAddress,
  http,
} from 'viem';
import { paxeerWalletChain } from '@/core/wallet-sdk/client';
import { WalletSessionController } from '@/core/wallet-sdk/session';
import { useTransactionAttempt } from '@/hooks/tx/use-transaction-attempt';
import { ActionButton, TradeQuoteNotices, quoteActionState } from '../trade-panel-parts';
import { currentTradeQuote } from '../use-trade-panel';
import { isWalletRefusal, plainTradeError } from '../trade-copy';
import { executableQuote, quoteFingerprint, type TradeContext } from './domain';
import { ChainTradeProvider, type TradeDeployment } from './providers';
import { TradeJourney, failureReason } from './runtime';

const pause = (ms: number) => new Promise<void>(resolve => setTimeout(resolve, ms));
async function until(check: () => boolean, timeout = 5000, report?: () => Promise<string>) {
  const end = Date.now() + timeout;
  while (!check()) {
    if (Date.now() >= end) throw new Error(report ? await report() : 'Trade state did not settle');
    await act(async () => { await pause(20); });
  }
}

function walletRefusalUsesCodesCausesAndPlainCopy() {
  const refusal = new UserRejectedRequestError(new Error('Declined'));
  const wrapped = new BaseError('Wallet request failed', { cause: refusal });
  const coded = Object.assign(new Error('Declined'), { code: 4001 });
  for (const error of [refusal, wrapped, coded, new Error('User denied transaction'), 'Request canceled']) {
    expect(isWalletRefusal(error)).toBe(true);
    expect(plainTradeError(failureReason(error))).toBe('You cancelled in your wallet');
  }
  expect(plainTradeError(failureReason(wrapped))).toBe('You cancelled in your wallet');
  expect(failureReason(wrapped)).toBe('User rejected the request.');
  expect(plainTradeError('User rejected the request')).toBe('You cancelled in your wallet');
  for (const error of [new Error('RPC unavailable'), new Error('Simulation expired'), { code: -32603 }, null]) {
    expect(isWalletRefusal(error)).toBe(false);
  }
  const cycle = new Error('RPC unavailable');
  cycle.cause = cycle;
  expect(isWalletRefusal(cycle)).toBe(false);
  expect(plainTradeError('insufficient funds')).toBe('Not enough balance for this trade');
}
test(walletRefusalUsesCodesCausesAndPlainCopy.name, walletRefusalUsesCodesCausesAndPlainCopy);

async function walletRefusalPreservesTradeAndExpiredQuoteRequestsOnePrice() {
  const bytes = readFileSync(process.env.L26_FIXTURE_JSON!);
  expect(createHash('sha256').update(bytes).digest('hex')).toBe(process.env.L26_FIXTURE_SHA256);
  const data = JSON.parse(bytes.toString());
  const rpc = process.env.L26_RPC_URL!;
  expect(rpc).toMatch(/^http:\/\/127\.0\.0\.1:\d+$/);
  const deployment: TradeDeployment = data.deployment.tradeDeployment;
  const account = getAddress(data.account);
  const admin = createWalletClient({ chain: paxeerWalletChain, transport: http(rpc, { retryCount: 0 }) });
  let pendingWallet: { response: ServerResponse; id: number } | undefined;
  let heldSimulation: (() => void) | undefined;
  let holdSimulation = false;
  let walletPrompts = 0;
  const relayErrors: unknown[] = [];
  // Reads use the deployed chain. Signing waits for the wallet's consent response.
  const relay = createServer((request, response) => {
    const chunks: Buffer[] = [];
    request.on('data', (chunk: Buffer) => chunks.push(chunk));
    request.on('end', () => {
      void (async () => {
        const body = Buffer.concat(chunks);
        const wire = JSON.parse(body.toString());
        if (wire.method === 'eth_sendTransaction') {
          expect(wire.params[0].from.toLowerCase()).toBe(account.toLowerCase());
          expect(wire.params[0].to.toLowerCase()).toBe(deployment.router.toLowerCase());
          walletPrompts++;
          pendingWallet = { response, id: wire.id };
          return;
        }
        const upstream = await fetch(rpc, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body });
        const text = await upstream.text();
        if (holdSimulation && wire.method === 'eth_call' && wire.params[0].to?.toLowerCase() === deployment.router.toLowerCase()) {
          holdSimulation = false;
          await new Promise<void>(resolve => { heldSimulation = resolve; });
        }
        response.writeHead(upstream.status, { 'Content-Type': 'application/json' });
        response.end(text);
      })().catch(error => { relayErrors.push(error); response.destroy(); });
    });
  });
  await new Promise<void>(resolve => relay.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${(relay.address() as AddressInfo).port}`;
  const client = createPublicClient({ chain: paxeerWalletChain, cacheTime: 0, transport: http(origin, { retryCount: 0, timeout: 90000 }) });
  const wallet = createWalletClient({ chain: paxeerWalletChain, transport: http(origin, { retryCount: 0, timeout: 90000 }) });
  const session = new WalletSessionController(() => {});
  session.select('trade-wallet', 'injected', account, 125, ['trade-wallet']);
  const provider = new ChainTradeProvider(client, wallet, deployment, session);
  const { JSDOM } = createRequire(import.meta.url)('jsdom');
  const dom = new JSDOM('<div id="root"></div>', { url: 'https://trade.local' });
  const keys = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT'] as const;
  const previous = new Map(keys.map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)]));
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true });
  const host = dom.window.document.getElementById('root')!;
  const root = createRoot(host);
  let quoteRequests = 0;
  const journey = new TradeJourney(provider, account, dom.window.localStorage, undefined, event => {
    if (event.kind === 'quote_start') quoteRequests++;
  });
  let transactionError: string | undefined;
  let balancesFresh = true;
  let refreshes = 0;
  let settled: Promise<unknown> | undefined;
  try {
    const info = await provider.inspectPool(getAddress(data.pool));
    const context: TradeContext = {
      chainId: 125, manifestRevision: deployment.revision, pool: data.pool,
      token: deployment.usdl, router: deployment.router, sender: account, recipient: account,
      inputRaw: 1000000n, isBuy: true, opticalRevision: info.opticalRevision, opticalData: '0x',
    };
    const oldAllowance = await provider.readAllowance(context);
    const approve = async (amount: bigint) => {
      const hash = await admin.writeContract({ account, address: deployment.usdl, abi: erc20Abi, functionName: 'approve', args: [deployment.spender, amount] });
      expect((await client.waitForTransactionReceipt({ hash })).status).toBe('success');
    };
    await approve(context.inputRaw);
    function Surface() {
      const state = useTransactionAttempt(journey);
      const [, redraw] = useState(0);
      const current = currentTradeQuote(state, context, Date.now());
      const action = state.signing ? { kind: 'confirming' as const } : quoteActionState({
        quoteUnavailable: !current.quote, quoteRequesting: state.quoteStatus === 'requesting', balancesFresh, side: 'buy', tokenSymbol: 'Coin',
      });
      const refresh = () => { refreshes++; transactionError = undefined; journey.requote(); };
      const submit = () => {
        if (action.kind === 'quote-refresh') { refresh(); return; }
        transactionError = undefined;
        const quote = state.quote!;
        settled = journey.swap(session.capture(), 100, { quoteFingerprint: quoteFingerprint(quote), slippageBps: 100, slippage: true, impact: true }).catch(error => {
          transactionError = isWalletRefusal(error) ? failureReason(error) : error instanceof Error ? error.message : String(error);
          redraw(value => value + 1);
          return error;
        });
      };
      return <>
        <div role="status"><TradeQuoteNotices priceMoved={current.priceMoved} staleQuoteCause={current.staleQuoteCause} error={transactionError ?? state.quoteError} onRefresh={refresh} /></div>
        <ActionButton state={action} side="buy" onClick={submit} />
      </>;
    }
    const button = (text: string) => [...host.querySelectorAll('button')].find(node => node.textContent === text)!;
    const refuse = () => {
      const prompt = pendingWallet!;
      pendingWallet = undefined;
      prompt.response.writeHead(200, { 'Content-Type': 'application/json' });
      prompt.response.end(JSON.stringify({ jsonrpc: '2.0', id: prompt.id, error: { code: 4001, message: 'User rejected the request.' } }));
    };
    const start = async () => {
      await act(async () => { button('Place trade').click(); });
      await until(() => !!pendingWallet, 5000, async () => {
        const state = journey.state;
        const block = await client.getBlock({ blockTag: 'latest' });
        const chainAheadSeconds = block.timestamp - BigInt(Math.floor(Date.now() / 1000));
        return `No wallet request arrived: ${inspect({
          transactionError,
          relayErrors,
          journey: state,
          chainAheadSeconds: chainAheadSeconds.toString(),
        }, { depth: null })}`;
      });
      expect(journey.state.signing).toBe(true);
    };
    await act(async () => { root.render(<Surface />); journey.setContext(context); });
    expect(button('Getting price...').disabled).toBe(true);
    await until(() => !!journey.state.quote);
    const reviewed = journey.state.quote!;
    const nonce = await client.getTransactionCount({ address: account });
    await start();
    await act(async () => { refuse(); await settled; });
    expect(isWalletRefusal(await settled)).toBe(true);
    expect(journey.state.quote).toBe(reviewed);
    expect(executableQuote(reviewed, context, Date.now())).toBe(true);
    expect(journey.state).toMatchObject({ quoteStatus: 'executable', signing: false });
    expect(journey.state.staleQuote).toBeUndefined();
    expect(button('Place trade').disabled).toBe(false);
    expect(host.querySelector('[role="status"]')!.children).toHaveLength(1);
    expect(host.textContent).toContain(plainTradeError(transactionError));
    expect(host.textContent).not.toContain('The price moved.');
    expect(button('Getting price...')).toBeUndefined();
    await act(async () => { await pause(550); });
    expect(quoteRequests).toBe(1);

    // A transport disconnect after signing was requested requires an explicit new price.
    await start();
    await act(async () => { pendingWallet!.response.destroy(); pendingWallet = undefined; await settled; });
    expect(isWalletRefusal(await settled)).toBe(false);
    expect(journey.state.quote).toBeUndefined();
    expect(journey.state.staleQuote).toBe(reviewed);
    expect(journey.state.quoteStatus).toBe('stale');
    expect(button('Refresh price').disabled).toBe(false);
    expect(button('Getting price...')).toBeUndefined();
    expect(host.textContent).not.toContain('The price moved.');
    await act(async () => { button('Refresh price').click(); });
    expect(refreshes).toBe(1);
    expect(button('Getting price...').disabled).toBe(true);
    await until(() => !!journey.state.quote);
    expect(quoteRequests).toBe(2);

    await start();
    const expiring = journey.state.quote!;
    await until(() => Date.now() >= expiring.expiresAt, 35000);
    await act(async () => { refuse(); await settled; });
    expect(isWalletRefusal(await settled)).toBe(true);
    expect(journey.state.quote).toBeUndefined();
    expect(journey.state.quoteStatus).toBe('requesting');
    expect(button('Getting price...').disabled).toBe(true);
    expect(host.textContent).toContain('The price is out of date. Refresh to see the current price.');
    await until(() => !!journey.state.quote);
    await act(async () => { await pause(550); });
    expect(quoteRequests).toBe(3);
    expect(button('Place trade').disabled).toBe(false);

    const fresh = journey.state.quote!;
    expect(currentTradeQuote({ quote: fresh }, { ...context, inputRaw: context.inputRaw + 1n }, Date.now()).priceMoved).toBe(true);
    holdSimulation = true;
    await act(async () => { button('Place trade').click(); });
    await until(() => !!heldSimulation);
    await until(() => Date.now() >= fresh.expiresAt, 35000);
    await act(async () => { heldSimulation!(); heldSimulation = undefined; await settled; });
    expect((await settled as Error).message).toBe('Simulation expired; review again');
    expect(journey.state).toMatchObject({ quoteStatus: 'stale', staleQuoteReason: 'simulation' });
    expect(host.textContent).toContain('The price moved. Refresh to see the new price.');
    expect(button('Refresh price').disabled).toBe(false);

    await act(async () => { button('Refresh price').click(); });
    await until(() => !!journey.state.quote);
    await approve(0n);
    await act(async () => { button('Place trade').click(); await settled; });
    expect(journey.state).toMatchObject({ quoteStatus: 'stale', staleQuoteReason: 'simulation' });
    expect(host.textContent).toContain('The price moved. Refresh to see the new price.');
    expect(button('Refresh price').disabled).toBe(false);
    expect(walletPrompts).toBe(3);
    expect(await client.getTransactionCount({ address: account })).toBe(nonce + 1);
    expect(journey.state.attempts).toHaveLength(0);
    balancesFresh = false;
    await act(async () => { root.render(<Surface />); });
    expect(button('Getting price...').disabled).toBe(true);
    balancesFresh = true;
    await act(async () => { root.render(<Surface />); });
    expect(button('Getting price...')).toBeUndefined();
    expect(button('Refresh price').disabled).toBe(false);
    await approve(oldAllowance);
    expect(relayErrors).toEqual([]);
  } finally {
    heldSimulation?.();
    pendingWallet?.response.destroy();
    await act(async () => { root.unmount(); journey.close(); });
    session.dispose();
    relay.closeAllConnections();
    await new Promise<void>(resolve => relay.close(() => resolve()));
    dom.window.close();
    for (const key of keys) {
      const descriptor = previous.get(key);
      if (descriptor) Object.defineProperty(globalThis, key, descriptor);
      else Reflect.deleteProperty(globalThis, key);
    }
  }
}
test(walletRefusalPreservesTradeAndExpiredQuoteRequestsOnePrice.name, walletRefusalPreservesTradeAndExpiredQuoteRequestsOnePrice, 120000);
