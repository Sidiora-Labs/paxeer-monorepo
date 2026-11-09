import { readFileSync } from 'node:fs';
import { authenticatedReceipt, nodeVector, receiptWireKeys } from './support/receipt-recordings.js';
import { createServer, type IncomingMessage, type Server, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, beforeEach, describe, expect, it, test } from 'vitest';
import {
  decodeJourney,
  decodeIntentPlan,
  EndpointClient,
  HumanClient,
  HumanServiceError,
  JsonRpcError,
  KernelAvailability,
  KernelClient,
  kernelUnavailableFrom,
  type PlanIntentRequest,
  type SubmitPlanRequest,
} from '../src/index.js';

type Exchange = {
  request: { method: string; params: unknown[] };
  response: Record<string, unknown>;
};
type Golden = {
  method?: string;
  path?: string;
  headers?: Record<string, string>;
  body?: unknown;
  status?: number;
};

const fixtureDir = new URL('./fixtures/endpoint/', import.meta.url);
const goldenDir = new URL('../../../services/human/schema/human-api/golden/', import.meta.url);
const scenarios = [
  'available',
  'kernel-unreachable',
  'no-finalised-checkpoint',
  'kernel-not-configured',
] as const;
type Scenario = (typeof scenarios)[number];

function json<T>(dir: URL, name: string): T {
  return JSON.parse(readFileSync(new URL(name, dir), 'utf8')) as T;
}

const recorded = Object.fromEntries(
  scenarios.map((s) => [s, json<Exchange[]>(fixtureDir, `${s}.json`)])
) as Record<Scenario, Exchange[]>;
const golden = (name: string): Golden => json<Golden>(goldenDir, `${name}.json`);

const merged = json<any>(fixtureDir, 'merged-contract.json');
const mergedOwner: string = merged.account.account.evm_address;
const receiptRecording = nodeVector('V1-pax-transfer');
const owner = `0x${'11'.repeat(20)}` as const;
const did = `did:layerx:${'61'.repeat(32)}`;
const mainAccount = '7c'.repeat(32);

type Seen = { scenario: string; path: string; methods: string[]; batch: boolean };
const seen: Seen[] = [];
const unmatched: string[] = [];
let server: Server;
let base: string;

function answer(scenario: Scenario, call: unknown): unknown {
  const request = call as { id: unknown; method: string; params: unknown[] };
  let result: unknown;
  if (request.method === 'px_getNetwork')
    result = scenario === 'available' ? merged.network : merged.unavailable;
  else if (request.method === 'px_getAccount' && request.params[0] === mergedOwner)
    result = merged.account;
  else if (request.method === 'px_getBalances' && request.params[0] === mergedOwner)
    result = merged.balances;
  else if (request.method === 'eth_getTransactionCount' && request.params[0] === mergedOwner)
    result = '0x2a';
  else if (
    request.method === 'pax_sequencerReceipt' &&
    request.params[0] === receiptRecording.receiptJson.txHash
  )
    result = receiptRecording.receiptJson;
  else if (
    request.method === 'eth_getRawTransactionByHash' &&
    request.params[0] === receiptRecording.receiptJson.txHash
  )
    result = receiptRecording.rawTransaction;
  else if (request.method === 'px_getReceiptKeys')
    result = receiptWireKeys(receiptRecording.receiptJson);
  else if (request.method === 'px_getAccount' && request.params[0] === `0x${'7d'.repeat(20)}`)
    return {
      jsonrpc: '2.0',
      id: request.id,
      error: {
        code: -32010,
        message: 'read unavailable',
        data: { code: 'kernel_unavailable', reason: 'unreachable', backend: 'public_core' },
      },
    };
  if (result !== undefined) return { jsonrpc: '2.0', id: request.id, result };
  const exchange = recorded[scenario].find(
    (entry) =>
      entry.request.method === request.method &&
      JSON.stringify(entry.request.params) === JSON.stringify(request.params)
  );
  if (exchange === undefined) {
    unmatched.push(`${scenario} ${request.method} ${JSON.stringify(request.params)}`);
    return { jsonrpc: '2.0', id: request.id, error: { code: -32601, message: 'Method not found' } };
  }
  return { ...exchange.response, jsonrpc: '2.0', id: request.id };
}

function reply(
  res: ServerResponse,
  status: number,
  body: unknown,
  headers: Record<string, string> = {}
): void {
  res.writeHead(status, { 'content-type': 'application/json', ...headers });
  res.end(JSON.stringify(body));
}

function humanRoute(req: IncomingMessage, path: string, body: unknown, res: ServerResponse): void {
  const plan = golden('intent.plan.request');
  const submit = golden('intent.submit.request');
  const journey = golden('journey.get.request');
  if (req.method === 'POST' && path === plan.path) {
    const matches = JSON.stringify(body) === JSON.stringify(plan.body);
    const recordedAnswer = golden(matches ? 'intent.plan.response' : 'intent.plan.failure');
    reply(res, recordedAnswer.status ?? 500, recordedAnswer.body);
    return;
  }
  if (req.method === 'POST' && path === submit.path) {
    const matches =
      JSON.stringify(body) === JSON.stringify(submit.body) &&
      req.headers['idempotency-key'] === submit.headers?.['Idempotency-Key'];
    const recordedAnswer = golden(matches ? 'intent.submit.response' : 'intent.submit.failure');
    reply(res, recordedAnswer.status ?? 500, recordedAnswer.body);
    return;
  }
  if (req.method === 'GET' && path.startsWith('/v1/journeys/')) {
    const recordedAnswer = golden(
      path === journey.path ? 'journey.get.response' : 'journey.get.failure'
    );
    reply(res, recordedAnswer.status ?? 500, recordedAnswer.body);
    return;
  }
  reply(res, 404, {
    ok: false,
    error: { code: 'not-found', copy_key: 'error.request.not-found', retry: 'final' },
    trace: 'trc_route',
  });
}

function handle(req: IncomingMessage, res: ServerResponse): void {
  const chunks: Buffer[] = [];
  req.on('data', (chunk: Buffer) => chunks.push(chunk));
  req.on('end', () => {
    const raw = Buffer.concat(chunks).toString('utf8');
    const body: unknown = raw.length > 0 ? JSON.parse(raw) : undefined;
    const url = req.url ?? '';
    const [, scope, ...rest] = url.split('/');
    const path = `/${rest.join('/')}`;
    if (scope === 'human') {
      seen.push({ scenario: 'human', path, methods: [req.method ?? ''], batch: false });
      humanRoute(req, path, body, res);
      return;
    }
    const scenario = scope as Scenario;
    if (!scenarios.includes(scenario) || path !== '/rpc' || req.method !== 'POST') {
      reply(res, 404, { error: 'unknown route' });
      return;
    }
    const calls = Array.isArray(body) ? body : [body];
    seen.push({
      scenario,
      path,
      methods: calls.map((call) => (call as { method: string }).method),
      batch: Array.isArray(body),
    });
    const answers = calls.map((call) => answer(scenario, call));
    reply(res, 200, Array.isArray(body) ? answers.reverse() : answers[0]);
  });
}

beforeAll(async () => {
  server = createServer(handle);
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});

afterAll(async () => {
  await new Promise<void>((resolve, reject) =>
    server.close((err) => (err ? reject(err) : resolve()))
  );
});

beforeEach(() => {
  seen.length = 0;
  unmatched.length = 0;
});

function endpoint(scenario: Scenario): EndpointClient {
  return new EndpointClient({
    nodeUrl: `${base}/${scenario}/rpc`,
    hostedUrl: `${base}/${scenario}/rpc`,
  });
}

function methodsSeen(scenario: string): string[] {
  return seen.filter((entry) => entry.scenario === scenario).flatMap((entry) => entry.methods);
}

function endpoint_client_exposes_no_joined_read(): void {
  const client = new EndpointClient();
  for (const name of ['getAccount', 'listAssets', 'getBalances', 'getUnifiedHistory'] as const) {
    expect(Reflect.get(client, name)).toBeUndefined();
    expect(name in client).toBe(false);
  }
  for (const name of [
    'getChainAccount',
    'getBankBalances',
    'listBankAssets',
    'getChainHistory',
  ] as const) {
    expect(client[name]).toBeTypeOf('function');
  }
}

describe('EndpointClient', () => {
  test(endpoint_client_exposes_no_joined_read, endpoint_client_exposes_no_joined_read);

  it('endpoint_batches_mixed_calls_in_one_request_with_per_call_errors', async () => {
    const outcomes = await endpoint('available').batch([
      { method: 'eth_blockNumber', params: [] },
      { method: 'px_getCapabilities', params: [] },
      { method: 'lx_getReceipt', params: ['cd'.repeat(32)] },
      { method: 'px_resolveAccount', params: [owner] },
    ]);
    expect(seen).toHaveLength(1);
    expect(seen.every((request) => request.batch)).toBe(true);
    expect(seen.map((request) => request.methods)).toEqual([
      ['eth_blockNumber', 'px_getCapabilities', 'lx_getReceipt', 'px_resolveAccount'],
    ]);
    expect(outcomes[0]).toEqual({ ok: true, result: '0x1a2b3c' });
    expect(outcomes[1]?.ok).toBe(true);
    expect(outcomes[2]).toEqual({
      ok: false,
      error: { code: -32001, message: 'Read unavailable' },
    });
    expect(outcomes[3]).toMatchObject({
      ok: true,
      result: { evm_address: owner, layerx_did: did, bound: true },
    });
    expect(unmatched).toEqual([]);
  });

  it('endpoint_chain_reads_reject_retired_joined_account_and_assets_payloads', async () => {
    const client = endpoint('available');
    const resolved = await client.resolveAccount(owner);
    expect(resolved).toEqual({
      evm_address: owner,
      pax_address: 'pax1zyg3zyg3zyg3zyg3zyg3zyg3zyg3zyg3a8h3w5',
      layerx_did: did,
      layerx_account: mainAccount,
      bound: true,
    });
    await expect(client.getChainAccount(owner)).rejects.toThrow(
      'malformed chain account paxeer height'
    );
    await expect(client.listBankAssets()).rejects.toThrow('malformed bank asset page height');
    const capabilities = await client.getCapabilities();
    expect(capabilities).toEqual({
      exchange: true,
      bridge: false,
      launchpad: true,
      probed_at: 86400,
      rpc_height: '1715004',
    });
    expect(unmatched).toEqual([]);
  });

  it('endpoint_bank_balances_reject_retired_joined_rows_without_completion_reads', async () => {
    await expect(endpoint('available').getBankBalances(owner)).rejects.toThrow(
      'malformed bank balances height'
    );
    expect(seen).toEqual([
      { scenario: 'available', path: '/rpc', methods: ['px_getBalances'], batch: false },
    ]);
    expect(unmatched).toEqual([]);
  });

  it('endpoint_pages_chain_history_by_cursor_to_exhaustion', async () => {
    const client = endpoint('available');
    const ids: string[] = [];
    for await (const item of client.history(owner, { limit: 2 })) {
      expect(item).not.toHaveProperty('chain');
      expect(item).not.toHaveProperty('side');
      ids.push(item.id);
    }
    expect(ids).toEqual(['5', '3', '1']);
    expect(methodsSeen('available')).toEqual([
      'px_getUnifiedHistory',
      'px_getUnifiedHistory',
      'px_getUnifiedHistory',
    ]);
    const pages: (string | null)[] = [];
    for await (const page of client.historyPages(owner, { limit: 2 })) pages.push(page.next_cursor);
    expect(pages).toEqual(['4', '2', null]);
    expect(unmatched).toEqual([]);
  });

  it('endpoint_reports_kernel_availability_and_reason_from_px_getNetwork', async () => {
    for (const scenario of scenarios) {
      const network = await endpoint(scenario).getNetwork();
      expect(network.consensus).toEqual(scenario === 'available' ? merged.network.consensus : null);
      expect(network.consensus_reason).toBe(
        scenario === 'available' ? null : 'consensus_unavailable'
      );
      expect(network).not.toHaveProperty('kernel');
      expect(network.paxeer.chain_id).toBe('0x7d');
    }
    expect(unmatched).toEqual([]);
  });
});

describe('KernelClient', () => {
  it('endpoint_kernel_reads_run_when_the_kernel_is_available', async () => {
    const client = endpoint('available');
    const kernel = new KernelClient(client, new KernelAvailability(client, { ttlMs: 60_000 }));
    expect(await kernel.getAccount(mergedOwner)).toEqual({
      available: true,
      result: { ...merged.account, paxeer: { ...merged.account.paxeer, height: '16' } },
    });
    expect(await kernel.getBalances(mergedOwner)).toEqual({
      available: true,
      result: { ...merged.balances, height: '16' },
    });
    expect(await kernel.getSequence(mergedOwner)).toEqual({ available: true, result: '42' });
    expect(await kernel.getReceipt(receiptRecording.receiptJson.txHash)).toEqual({
      available: true,
      result: authenticatedReceipt(receiptRecording.receiptJson),
    });
    expect(Reflect.get(kernel, 'getActivityStatus')).toBeUndefined();
    expect(methodsSeen('available')).toEqual([
      'px_getNetwork',
      'px_getAccount',
      'px_getBalances',
      'eth_getTransactionCount',
      'pax_sequencerReceipt',
      'px_getReceiptKeys',
      'eth_getRawTransactionByHash',
    ]);
    await expect(client.rpc.call('lx_getReceipt', ['cd'.repeat(32)])).rejects.toMatchObject({
      name: 'JsonRpcError',
      code: -32001,
    });
    expect(unmatched).toEqual([]);
  });

  it('endpoint_unavailable_state_short_circuits_every_kernel_call', async () => {
    const expected: Record<Exclude<Scenario, 'available'>, string> = {
      'kernel-unreachable': 'consensus_unavailable',
      'no-finalised-checkpoint': 'consensus_unavailable',
      'kernel-not-configured': 'consensus_unavailable',
    };
    for (const [scenario, reason] of Object.entries(expected) as [Scenario, string][]) {
      const kernel = new KernelClient(endpoint(scenario));
      const state = { available: false, reason, backend: null };
      expect(await kernel.getAccount(mergedOwner)).toEqual(state);
      expect(await kernel.getBalances(mergedOwner)).toEqual(state);
      expect(await kernel.getSequence(mergedOwner)).toEqual(state);
      expect(await kernel.getReceipt('ab'.repeat(32))).toEqual(state);
      expect(Reflect.get(kernel, 'getActivityStatus')).toBeUndefined();
      expect(methodsSeen(scenario)).toEqual(['px_getNetwork']);
    }
    expect(unmatched).toEqual([]);
  });

  it('endpoint_kernel_unavailable_error_is_recorded_and_stops_later_calls', async () => {
    const client = endpoint('available');
    const kernel = new KernelClient(client, new KernelAvailability(client, { ttlMs: 60_000 }));
    const state = { available: false, reason: 'unreachable', backend: 'public_core' };
    expect(await kernel.getAccount(`0x${'7d'.repeat(20)}`)).toEqual(state);
    expect(await kernel.getSequence(mergedOwner)).toEqual(state);
    expect(methodsSeen('available')).toEqual(['px_getNetwork', 'px_getAccount']);
    expect(unmatched).toEqual([]);
  });

  it('endpoint_availability_cache_expires_after_its_ttl', async () => {
    let clock = 1_000;
    const client = endpoint('available');
    const availability = new KernelAvailability(client, { ttlMs: 500, now: () => clock });
    expect(await availability.current()).toEqual({ available: true, reason: 'available' });
    clock += 499;
    await availability.current();
    expect(methodsSeen('available')).toEqual(['px_getNetwork']);
    clock += 1;
    await availability.current();
    expect(methodsSeen('available')).toEqual(['px_getNetwork', 'px_getNetwork']);
  });

  it('endpoint_bank_balances_surface_the_typed_kernel_unavailable_error', async () => {
    const error = await endpoint('kernel-unreachable')
      .getBankBalances(owner)
      .then(
        () => null,
        (err: unknown) => err
      );
    expect(error).toBeInstanceOf(JsonRpcError);
    expect(kernelUnavailableFrom(error)).toEqual({
      available: false,
      reason: 'unreachable',
      backend: 'public_core',
    });
  });
});

describe('HumanClient', () => {
  function human(scenario: Scenario): HumanClient {
    const client = endpoint(scenario);
    return new HumanClient({
      url: `${base}/human`,
      kernel: new KernelAvailability(client),
      authorization: () => 'session-token',
    });
  }

  it('endpoint_human_client_plans_submits_and_reads_the_journey_from_goldens', async () => {
    const client = human('available');
    const plan = await client.planIntent(golden('intent.plan.request').body as PlanIntentRequest);
    expect(plan).toEqual((golden('intent.plan.response').body as { result: unknown }).result);
    expect(plan.legs.map((leg) => [leg.domain, leg.fee.amount])).toEqual([['layerx', '200000']]);
    for (const [field, invalids] of [
      ['to', [null, '0x12', `0x${'aa'.repeat(20)}\n`]],
      ['calldata', [null, '0x1', '0xgg', '0x12\n']],
      ['value', [null, 0, '-1', '01', '1\n']],
      ['max_fee_per_gas', [null, 0, '-1', '01']],
      ['max_priority_fee_per_gas', [null, 0, '-1', '01']],
      ['nonce', [-1, 0.5, Number.MAX_SAFE_INTEGER + 1]],
      ['chain_id', [-1, 0.5, Number.MAX_SAFE_INTEGER + 1]],
      ['gas_limit', [-1, 0.5, Number.MAX_SAFE_INTEGER + 1]],
    ] as const) {
      for (const value of invalids) {
        expect(() =>
          decodeIntentPlan({ ...plan, legs: [{ ...plan.legs[0], [field]: value }] })
        ).toThrow(`intent leg ${field}`);
      }
    }
    const submitGolden = golden('intent.submit.request');
    const submitted = await client.submitPlan(
      submitGolden.body as SubmitPlanRequest,
      submitGolden.headers?.['Idempotency-Key'] as string
    );
    expect(submitted).toEqual({
      available: true,
      result: (golden('intent.submit.response').body as { result: unknown }).result,
    });
    const journeyId = (golden('journey.get.request').path as string).split('/').pop() as string;
    const journey = await client.getJourney(journeyId);
    expect(journey).toEqual((golden('journey.get.response').body as { result: unknown }).result);
    for (const [field, value] of [
      ['tx_hash', '0x12'],
      ['nonce', -1],
      ['nonce', 1.5],
      ['nonce', Number.MAX_SAFE_INTEGER + 1],
    ] as const) {
      expect(() =>
        decodeJourney({ ...journey, stages: [{ ...journey.stages[0], [field]: value }] })
      ).toThrow(`journey stage ${field}`);
    }
    expect(journey.stages[0]).toMatchObject({ tx_hash: `0x${'ab'.repeat(32)}`, nonce: 19 });
    expect(seen.filter((entry) => entry.scenario === 'human').map((entry) => entry.path)).toEqual([
      '/v1/intents/plan',
      '/v1/intents/submit',
      `/v1/journeys/${journeyId}`,
    ]);
  });

  it('endpoint_human_submit_refuses_with_the_unavailable_state_without_calling', async () => {
    const submitGolden = golden('intent.submit.request');
    const result = await human('kernel-unreachable').submitPlan(
      submitGolden.body as SubmitPlanRequest,
      submitGolden.headers?.['Idempotency-Key'] as string
    );
    expect(result).toEqual({ available: false, reason: 'consensus_unavailable', backend: null });
    expect(seen.filter((entry) => entry.scenario === 'human')).toEqual([]);
  });

  it('endpoint_human_failure_envelope_is_a_typed_error', async () => {
    const error = await human('available')
      .getJourney('jrn_unknown')
      .then(
        () => null,
        (err: unknown) => err
      );
    expect(error).toBeInstanceOf(HumanServiceError);
    const failure = golden('journey.get.failure').body as {
      error: { code: string; copy_key: string };
      trace: string;
    };
    expect(error).toMatchObject({
      status: 404,
      code: failure.error.code,
      copyKey: failure.error.copy_key,
      trace: failure.trace,
    });
  });
});
