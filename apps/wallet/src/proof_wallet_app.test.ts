/*
Proves: useSurface.useFeeSelection/readGrantCaps/useModuleSend and KernelPlan.legBindings ->
exported wallet TypeScript modules and production HTTP route -> FeeChoice, WebDataView,
BridgeView, ExchangeView, LaunchpadView, HistoryView, StatusLadder and KernelNotice DOM.

Repairs: useSurface/BridgeView/ExchangeView/LaunchpadView silently rounded excess asset
precision; WebDataView accepted hexadecimal gas/ids; KernelPlan accepted unsafe sequences
and inverted windows; StatusLadder accepted unknown evidence; KernelNotice accepted blank
refusals; HistoryView could publish a prior account's in-flight read after switching accounts.
The tests use SDK builders, decoders, providers, React providers and production route handlers.
No RPC, signer, fetch, hook or consumer is replaced. HTTP is served on a real loopback socket.
A signed-out provider deliberately proves refusal, not a successful on-chain transaction.
The wallet proxy is read-only (app/api/wallet/[...path]/route.ts, POST); it must return 405,
not manufacture sequencer data. These tests preserve that external-interface limitation.
All workspace imports resolve to source via the package's Vitest aliases; none use dist.
The hosted success/reopen case replays bytes captured from the production human
service fixture; record-22-33-152.py regenerates that recording at implementation time.
*/
// @vitest-environment jsdom
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { act, createElement as h, useState } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { decodeFunctionData, getAddress, parseAbi } from 'viem';
import {
  PaxeerProvider,
  bridge,
  decodeAccountDocument,
  decodeIntentPlan,
  exchange,
  launchpad,
  statusLadder,
  webData,
  type IntentPlan,
  type LadderStep,
  type ModuleTransaction,
} from '@paxeer/wallet';
import { WalletProvider } from './wallet/WalletProvider';
import { AccountProvider } from './account/AccountProvider';
import { mount, type Mounted } from './account/test/render';
import { HistoryView } from './account/HistoryView';
import { legBindings } from './account/KernelPlan';
import { KernelNotice } from './account/KernelNotice';
import { StatusLadder } from './account/StatusLadder';
import { kernelGate } from './account/hooks';
import { BridgeView, buildBridgeOut } from './surfaces/BridgeView';
import { ExchangeView, USDL, word } from './surfaces/ExchangeView';
import { LaunchpadView, launchpadOrder, parseDeadlineMinutes } from './surfaces/LaunchpadView';
import { FeeChoice, quantity } from './surfaces/FeeChoice';
import { WebDataView, DrawCaps, buildWebDataRequest } from './surfaces/WebDataView';
import { SentResult, TransactionPreview } from './surfaces/Surface';
import {
  parseSurfaceAmount,
  parseSurfaceId,
  readGrantCaps,
  useFeeSelection,
  useModuleSend,
} from './surfaces/useSurface';
import { POST as walletPost } from './app/api/wallet/[...path]/route';
import { proveHostedIntentReopen } from './account/test/hostedIntentRecording';

const ADDRESS = `0x${'11'.repeat(20)}`;
const ASSET = `0x${'22'.repeat(20)}`;
const ACCOUNT = '33'.repeat(32);
let url: string;
let requests = 0;
const httpRequests: { method: string | undefined; path: string | undefined; body: unknown }[] = [];
let responseBarrier: Promise<void> | null = null;
const views: Mounted[] = [];
// This adapter serves the production handler's response byte for byte. It has no
// response fixtures, application state, JSON-RPC dispatch or chain implementation.
const server = createServer((request, response) => {
  requests += 1;
  const barrier = responseBarrier;
  responseBarrier = null;
  void (async () => {
    let raw = '';
    for await (const chunk of request) raw += String(chunk);
    httpRequests.push({
      method: request.method,
      path: request.url,
      body: raw ? JSON.parse(raw) : null,
    });
    if (barrier) await barrier;
    const answer = walletPost();
    response.writeHead(answer.status, Object.fromEntries(answer.headers.entries()));
    response.end(Buffer.from(await answer.arrayBuffer()));
  })();
});

beforeAll(async () => {
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  url = `http://127.0.0.1:${(server.address() as AddressInfo).port}/api/wallet`;
});

afterEach(async () => {
  for (const view of views.splice(0)) await view.unmount();
});

afterAll(async () => {
  await new Promise<void>((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
    server.closeAllConnections();
  });
});

function provider(): PaxeerProvider {
  return new PaxeerProvider({ gatewayUrl: url, rpcUrl: url, token: () => null });
}

function markup(transaction: ModuleTransaction): string {
  return renderToStaticMarkup(h(TransactionPreview, { transaction }));
}

async function show(node: Parameters<typeof mount>[0]): Promise<Mounted> {
  const view = await mount(node);
  views.push(view);
  return view;
}

function planState(): IntentPlan {
  // Domain input to the real plan decoder; no transport returns this input.
  return decodeIntentPlan({
    plan_digest: '44'.repeat(32),
    journey_kind: 'wallet-to-agent',
    legs: [
      {
        index: 0,
        domain: 'layerx',
        mechanism: 'kernel_send',
        source: { kind: 'paxeer-wallet' },
        destination: { kind: 'agent', account: 'agent:recipient' },
        money: { amount: '1000000', currency: 'SID' },
        fee: { amount: '12', currency: 'SID' },
      },
      {
        index: 1,
        domain: 'layerx',
        mechanism: 'budget_grant',
        source: { kind: 'paxeer-wallet' },
        destination: { kind: 'agent-budget', account: 'budget:recipient' },
        money: { amount: '500000', currency: 'SID' },
        fee: { amount: '3', currency: 'SID' },
      },
    ],
    total_fee: { amount: '15', currency: 'SID' },
    signing_requirements: [0, 1].map((leg_index) => ({
      leg_index,
      action_key: '55'.repeat(32),
      signing_context: '66'.repeat(32),
      authority: 'wallet-owner',
    })),
  });
}

async function wallet_app_sdk_builders_reach_transaction_consumers_without_rounding() {
  const real = provider();
  for (let observation = 0; observation < 2; observation += 1) {
    const bridged = buildBridgeOut(bridge(real), '125', ASSET, '1.000001', '6', ADDRESS);
    expect(
      decodeFunctionData({
        abi: parseAbi(['function bridgeOut(uint64,address,uint256,address)']),
        data: bridged.data as `0x${string}`,
      }).args
    ).toEqual([125n, ASSET, 1000001n, ADDRESS]);
    expect(markup(bridged)).toContain(bridged.to);
    expect(markup(bridged)).toContain(bridged.data.slice(0, 10));
    expect(() => buildBridgeOut(bridge(real), '125', ASSET, '1.0000001', '6', ADDRESS)).toThrow(
      'exact'
    );
    expect(() => buildBridgeOut(bridge(real), '0x7d', ASSET, '1', '6', ADDRESS)).toThrow('decimal');
    expect(() => buildBridgeOut(bridge(real), '125', ASSET, '1', '', ADDRESS)).toThrow('decimal');

    const deposited = exchange(real).deposit(
      USDL.address,
      parseSurfaceAmount('2.000001', USDL.decimals)
    );
    expect(
      decodeFunctionData({
        abi: parseAbi(['function deposit(address,uint256)']),
        data: deposited.data as `0x${string}`,
      }).args
    ).toEqual([getAddress(USDL.address), 2000001n]);
    expect(markup(deposited)).toContain(deposited.to);
    expect(() => word('-1')).toThrow('decimal');
    expect(() => word((1n << 256n).toString())).toThrow('exceeds');
    expect(word('7')).toBe(`0x${'0'.repeat(63)}7`);

    const order = launchpadOrder(ASSET, '1.000001', '7', '10', true, () => 1800000000000);
    const bought = launchpad(real).buy(order);
    expect(order).toEqual({ pool: ASSET, amountIn: 1000001n, minOut: 7n, deadline: 1800000600n });
    expect(markup(bought)).toContain(bought.to);
    expect(() => launchpadOrder(ASSET, '1.0000001', '7', '10', true, Date.now)).toThrow('exact');
    expect(() => parseDeadlineMinutes('9007199254740993')).toThrow('supported range');
    expect(() => parseDeadlineMinutes('1e3')).toThrow('positive whole');

    const requested = buildWebDataRequest(webData(real), 'search', 'network', '200000', '', 1n);
    expect(requested).toEqual(webData(real).search('network', 200000n, 1n));
    if (!requested) throw new Error('the request was not built');
    expect(markup(requested)).toContain('1 wei PAX');
    expect(() => buildWebDataRequest(webData(real), 'search', 'network', '0x10', '', 1n)).toThrow(
      'decimal'
    );
    expect(() =>
      buildWebDataRequest(webData(real), 'refund', '', '', '18446744073709551616', null)
    ).toThrow('64 bits');
  }
  expect(parseSurfaceAmount('0', 0)).toBe(0n);
  expect(() => parseSurfaceAmount('1.1', 0)).toThrow('exact');
  expect(() => parseSurfaceAmount('1', Number.NaN)).toThrow('decimals');
  expect(() => parseSurfaceId('1e3')).toThrow('decimal');
}
test(wallet_app_sdk_builders_reach_transaction_consumers_without_rounding);

function PlanFees({ plan }: { plan: IntentPlan }) {
  const selection = useFeeSelection(null, null);
  return h(FeeChoice, {
    provider: null,
    address: null,
    transaction: null,
    selection,
    sidRate: null,
    legs: plan.legs,
  });
}

async function wallet_app_plan_bindings_and_fees_observe_the_same_producer_twice() {
  const plan = planState();
  const document = decodeAccountDocument({
    evm_address: ADDRESS,
    pax_address: null,
    layerx_did: 'did:layerx:owner',
    layerx_account: ACCOUNT,
    bound: true,
  });
  const window = { notBefore: 1800000000, notAfter: 1800000600 };
  const maximum = { amount: '15', currency: 'SID' };
  for (let observation = 0; observation < 2; observation += 1) {
    const bindings = legBindings(plan, document, 7, window, maximum);
    expect(bindings.map((binding) => binding.account_sequence)).toEqual([7, 8]);
    expect(bindings.map((binding) => binding.actor)).toEqual([ADDRESS, ADDRESS]);
    expect(bindings.map((binding) => binding.action_key)).toEqual(
      plan.signing_requirements.map((entry) => entry.action_key)
    );
    expect(bindings.every((binding) => binding.not_after === window.notAfter)).toBe(true);
    const view = await show(
      h(WalletProvider, { config: null, identity: null, children: h(PlanFees, { plan }) })
    );
    expect(view.all('[data-layerx-leg]').map((node) => node.textContent)).toEqual([
      'Leg 0 · kernel_send12 SID',
      'Leg 1 · budget_grant3 SID',
    ]);
    expect(view.text('[data-role="fee-denom"]')).toBe('reading the fee token');
    await view.unmount();
    views.pop();
  }
  expect(() => legBindings(plan, { ...document, bound: false }, 7, window, maximum)).toThrow(
    'not bound'
  );
  expect(() => legBindings(plan, document, Number.MAX_SAFE_INTEGER, window, maximum)).toThrow(
    'sequence'
  );
  expect(() => legBindings(plan, document, 7, { notBefore: 2, notAfter: 1 }, maximum)).toThrow(
    'window'
  );
  expect(() => legBindings(plan, document, 7, window, { amount: '11', currency: 'SID' })).toThrow(
    'maximum fee'
  );
  expect(() =>
    legBindings({ ...plan, signing_requirements: [] }, document, 7, window, maximum)
  ).toThrow('no signer');
}
test(wallet_app_plan_bindings_and_fees_observe_the_same_producer_twice);

async function wallet_app_hosted_intent_plan_survives_producer_store_reopen() {
  await proveHostedIntentReopen();
}
test(
  wallet_app_hosted_intent_plan_survives_producer_store_reopen.name,
  wallet_app_hosted_intent_plan_survives_producer_store_reopen
);

function wallet_app_ladder_and_notice_refuse_malformed_producer_results() {
  for (let observation = 0; observation < 2; observation += 1) {
    const step = statusLadder.fromJourney('done-finalised');
    const html = renderToStaticMarkup(h(StatusLadder, { steps: [step] }));
    expect(html.match(/data-reached="true"/g) ?? []).toHaveLength(3);
    expect(html).toContain('data-source="journey"');
    const available = kernelGate({
      status: 'ready',
      value: { available: true, reason: 'available' },
    });
    expect(renderToStaticMarkup(h(KernelNotice, { gate: available }))).toContain(
      'data-kernel="available"'
    );
    const refused = kernelGate({
      status: 'ready',
      value: { available: false, reason: 'consensus_unavailable', backend: null },
    });
    expect(renderToStaticMarkup(h(KernelNotice, { gate: refused }))).toContain(
      'catching up or unreachable'
    );
  }
  const step = statusLadder.fromJourney('done-finalised');
  expect(() =>
    StatusLadder({ steps: [{ ...step, rung: 'final' } as unknown as LadderStep] })
  ).toThrow('malformed evidence');
  expect(() => KernelNotice({ gate: { open: false, reason: ' ' } })).toThrow('no refusal reason');
  expect(() => quantity('0x00', 'eth_estimateGas')).toThrow('malformed quantity');
  expect(quantity('0x5208', 'eth_estimateGas')).toBe(21000n);
}
test(wallet_app_ladder_and_notice_refuse_malformed_producer_results);

function SendConsumer({ real }: { real: PaxeerProvider }) {
  const selection = useFeeSelection(null, null);
  const module = exchange(real);
  const sending = useModuleSend(module, real, ADDRESS);
  return h(
    'div',
    null,
    h(FeeChoice, { provider: null, address: null, transaction: null, selection, sidRate: null }),
    h(
      'button',
      { 'data-send': true, onClick: () => void sending.send(module.closePosition(1n)) },
      'Send'
    ),
    h(SentResult, { state: sending })
  );
}

async function wallet_app_fee_and_send_hooks_reach_real_consumers_and_refuse_unsigned_writes() {
  const real = provider();
  const before = requests;
  const view = await show(
    h(WalletProvider, { config: null, identity: null, children: h(SendConsumer, { real }) })
  );
  expect(view.text('[data-fee-choice="pax_gas"] [data-role="label"]')).toBe('PAX gas');
  await view.click('[data-fee-choice="sid_sponsored"]');
  await view.until(() => view.query('[data-role="sponsored-phase"]')?.textContent === 'refused');
  expect(view.text('[data-role="sponsored-reason"]')).toContain(
    'SID sponsored fees need the embedded wallet'
  );
  for (let attempt = 0; attempt < 2; attempt += 1) {
    await view.click('[data-send]');
    await view.until(() => view.query('[data-role="send-error"]') !== null);
    expect(view.text('[data-role="send-error"]')).toBe('call eth_requestAccounts first');
    expect(view.query('[data-role="sent"]')).toBeNull();
  }
  expect(requests).toBe(before);
}
test(wallet_app_fee_and_send_hooks_reach_real_consumers_and_refuse_unsigned_writes);

async function wallet_app_production_http_refusal_reaches_caps_and_history_on_every_read() {
  const real = provider();
  const before = requests;
  // app/api/wallet/[...path]/route.ts POST is not a sequencer RPC server.
  // Expected behaviour: refuse HTTP 405, never decode it as caps or history.
  for (let observation = 0; observation < 2; observation += 1) {
    await expect(readGrantCaps(real, ADDRESS)).rejects.toMatchObject({
      code: -32603,
      message: 'the RPC base failed: 405',
    });
    expect(requests).toBe(before + observation + 1);
    expect(httpRequests.at(-1)).toMatchObject({
      method: 'POST',
      path: '/api/wallet',
      body: { method: 'eth_getBlockByNumber', params: ['finalized', false] },
    });
  }
  const config = { endpointUrl: url, humanUrl: url, explorerUrl: url };
  for (let observation = 0; observation < 2; observation += 1) {
    const at = requests;
    const view = await show(
      h(AccountProvider, { config, children: h(HistoryView, { account: ADDRESS }) })
    );
    await view.until(() => view.query('[role="alert"]') !== null);
    expect(view.text('[role="alert"]')).toContain('response does not answer the request id');
    expect(view.all('[data-item]')).toHaveLength(0);
    expect(requests).toBe(at + 1);
    expect(httpRequests.at(-1)).toMatchObject({
      method: 'POST',
      path: '/api/wallet',
      body: { method: 'px_getUnifiedHistory', params: [ADDRESS, null, 20, null] },
    });
    await view.unmount();
    views.pop();
  }
  const response = await fetch(url, { method: 'POST' });
  expect(response.status).toBe(405);
  expect(await response.json()).toEqual({ error: 'method not allowed' });
}
test(wallet_app_production_http_refusal_reaches_caps_and_history_on_every_read);

async function wallet_app_real_surface_consumers_require_a_connected_wallet() {
  const view = await show(
    h(WalletProvider, {
      config: null,
      identity: null,
      children: h(
        'div',
        null,
        h(BridgeView, { sidRate: null }),
        h(ExchangeView, { sidRate: null }),
        h(LaunchpadView, { sidRate: null }),
        h(WebDataView, {
          sidRate: null,
          caps: { state: 'unavailable', reason: 'consensus_unavailable' },
          refreshCaps: () => undefined,
        })
      ),
    })
  );
  expect(view.all('[data-role="not-connected"]').map((node) => node.textContent)).toEqual([
    'Connect a wallet to use Bridge',
    'Connect a wallet to use Exchange',
    'Connect a wallet to use Launchpad',
    'Connect a wallet to use Web data',
  ]);
  expect(view.query('[data-action="send"]')).toBeNull();
  const refused = renderToStaticMarkup(
    h(DrawCaps, { caps: { state: 'refused', reason: 'The account or its evidence was refused.' } })
  );
  expect(refused).toContain('data-caps-state="refused"');
  expect(refused).not.toContain('data-grant=');
}
test(wallet_app_real_surface_consumers_require_a_connected_wallet);

function HistorySwitch() {
  const [pageSize, setPageSize] = useState(20);
  const config = { endpointUrl: url, humanUrl: url, explorerUrl: url };
  return h(AccountProvider, {
    config,
    children: h(
      'div',
      null,
      h('button', { 'data-switch': true, onClick: () => setPageSize(0) }, 'Change query'),
      h(HistoryView, { account: pageSize === 0 ? ASSET : ADDRESS, pageSize })
    ),
  });
}

async function wallet_app_history_does_not_publish_an_old_account_request_after_switching() {
  let release: () => void = () => {
    throw new Error('the response barrier was not installed');
  };
  responseBarrier = new Promise<void>((resolve) => {
    release = resolve;
  });
  const at = requests;
  try {
    const view = await show(h(HistorySwitch));
    await view.until(() => requests === at + 1, 'initial account request');
    await view.click('[data-switch]');
    await view.until(() => view.query('[role="alert"]') !== null, 'new query refusal');
    expect(view.text('[role="alert"]')).toContain('history limit must be an integer from 1 to 100');
    await act(async () => {
      release();
      await new Promise((resolve) => setTimeout(resolve, 50));
    });
    expect(view.text('[role="alert"]')).toContain('history limit must be an integer from 1 to 100');
    expect(view.all('[data-item]')).toHaveLength(0);
    expect(requests).toBe(at + 1);
  } finally {
    release();
    responseBarrier = null;
  }
}
test(wallet_app_history_does_not_publish_an_old_account_request_after_switching);
