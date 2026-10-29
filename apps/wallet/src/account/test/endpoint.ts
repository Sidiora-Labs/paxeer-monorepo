import { readFileSync } from 'node:fs';
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import path from 'node:path';
import type { IntentPlan, PlanIntentRequest, SubmitPlanRequest } from '@paxeer/wallet';
import type { AccountConfig } from '../config';

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

const SDK_ENDPOINT_DIR = path.resolve(
  __dirname,
  '../../../../../sdk/wallet/test/fixtures/endpoint'
);
const GOLDEN_DIR = path.resolve(__dirname, '../../../../../services/human/schema/human-api/golden');

export const SCENARIOS = [
  'available',
  'kernel-unreachable',
  'no-finalised-checkpoint',
  'kernel-not-configured',
  'deposit',
  'merged',
] as const;
export type Scenario = (typeof SCENARIOS)[number];

export const OWNER = `0x${'11'.repeat(20)}`;
export const OWNER_DID = `did:layerx:${'61'.repeat(32)}`;
export const OWNER_ACCOUNT = '7c'.repeat(32);

function readJson<T>(file: string): T {
  return JSON.parse(readFileSync(file, 'utf8')) as T;
}

export function golden(name: string): Golden {
  return readJson<Golden>(path.join(GOLDEN_DIR, `${name}.json`));
}

export function recordings(scenario: Scenario): Exchange[] {
  if (scenario === 'merged' || scenario === 'deposit') return mergedRecordings();
  return readJson<Exchange[]>(path.join(SDK_ENDPOINT_DIR, `${scenario}.json`));
}

function mergedRecordings(): Exchange[] {
  const vectors = readJson<
    typeof import('../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json')
  >(
    path.resolve(
      __dirname,
      '../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json'
    )
  );
  const vector = vectors.cases.find((entry) => entry.id === 'V1-erc20-approve');
  if (!vector)
    throw new Error('the sequencer conformance recording carries no ERC-20 approval receipt');
  const merged: Exchange[] = [
    {
      request: { method: 'px_getAccount', params: [OWNER] },
      response: {
        result: {
          account: OWNER,
          paxeer: { address: OWNER, balance: '0xde0b6b3a7640000', nonce: '0x3', height: '1000' },
        },
      },
    },
    {
      request: { method: 'px_getBalances', params: [OWNER] },
      response: {
        result: { account: OWNER, height: '1000', balances: [{ denom: 'ulxp', amount: '125000' }] },
      },
    },
    {
      request: { method: 'px_listAssets', params: [] },
      response: {
        result: {
          height: '1000',
          assets: [
            {
              asset_id: 'aa'.repeat(32),
              denom: 'ulxp',
              kind: 'bank',
              name: 'LayerX Points',
              symbol: 'LXP',
              decimals: 6,
              supply: '1000000000000',
            },
            {
              asset_id: 'bb'.repeat(32),
              denom: 'usid',
              kind: 'bank',
              name: 'Sidiora',
              symbol: 'SID',
              decimals: 6,
              supply: '1000000000000',
            },
          ],
          joined_limit: 16,
        },
      },
    },
    {
      request: { method: 'eth_getTransactionCount', params: [OWNER, 'pending'] },
      response: { result: '0x4' },
    },
    {
      request: { method: 'pax_sequencerReceipt', params: [vector.receiptJson.txHash] },
      response: { result: vector.receiptJson },
    },
  ];
  const replaced = new Set(merged.map((entry) => entry.request.method));
  return [
    ...recordings('available').filter((entry) => !replaced.has(entry.request.method)),
    ...merged,
  ];
}

export function explorerStatuses(): Record<string, unknown> {
  return readJson<Record<string, unknown>>(path.join(SDK_ENDPOINT_DIR, 'explorer-status.json'));
}

export function expectedPendingNonce(): number {
  const read = recordings('merged').find(
    (entry) =>
      entry.request.method === 'eth_getTransactionCount' &&
      normalised(entry.request.params) === normalised([OWNER, 'pending'])
  );
  const result = read?.response.result;
  if (typeof result !== 'string' || !/^0x(?:0|[1-9a-fA-F][0-9a-fA-F]*)$/.test(result))
    throw new Error('the merged recording carries no pending EVM nonce');
  const nonce = Number(BigInt(result));
  if (!Number.isSafeInteger(nonce))
    throw new Error('the merged recording pending EVM nonce exceeds the safe integer range');
  return nonce;
}

export interface SeenRequest {
  readonly scope: string;
  readonly path: string;
  readonly methods: readonly string[];
  readonly authorization: string | null;
  readonly idempotencyKey: string | null;
  readonly body: unknown;
}

export interface AccountEndpoint {
  readonly base: string;
  readonly seen: SeenRequest[];
  readonly unmatched: string[];
  readonly refusals: string[];
  now: number;
  config(scenario: Scenario): AccountConfig;
  methods(scenario: Scenario): string[];
  close(): Promise<void>;
}

function reply(res: ServerResponse, status: number, body: unknown): void {
  res.writeHead(status, { 'content-type': 'application/json', 'access-control-allow-origin': '*' });
  res.end(JSON.stringify(body));
}

function normalised(value: unknown): string {
  return JSON.stringify(value).toLowerCase();
}

function submitRefusal(body: unknown, now: number): string | null {
  const plan = (golden('intent.plan.response').body as { result: IntentPlan }).result;
  const constraints = (golden('intent.plan.request').body as PlanIntentRequest).constraints;
  const deadline = Math.floor(Date.parse(constraints.deadline) / 1000);
  const request = body as Partial<SubmitPlanRequest>;
  if (request.plan_digest !== plan.plan_digest) return 'plan_digest differs from the plan';
  if (request.signed_digest !== plan.plan_digest)
    return 'signed_digest differs from the plan digest';
  if (!Array.isArray(request.bindings) || request.bindings.length !== plan.legs.length)
    return 'one binding per leg is required';
  let sequence = expectedPendingNonce();
  for (const leg of plan.legs) {
    const binding = request.bindings.find((entry) => entry.leg_index === leg.index);
    const requirement = plan.signing_requirements.find((entry) => entry.leg_index === leg.index);
    if (!binding || !requirement) return `leg ${leg.index} is not bound`;
    if (binding.action_key !== requirement.action_key) return `leg ${leg.index} action key differs`;
    if (binding.authority !== requirement.authority) return `leg ${leg.index} authority differs`;
    if (binding.actor !== OWNER) return `leg ${leg.index} actor differs`;
    if (binding.relationship !== 'self') return `leg ${leg.index} relationship differs`;
    if (binding.fee_limit.currency !== constraints.max_fee.currency)
      return `leg ${leg.index} fee currency differs`;
    const limit = BigInt(binding.fee_limit.amount);
    if (limit < BigInt(leg.fee.amount) || limit > BigInt(constraints.max_fee.amount))
      return `leg ${leg.index} fee limit is out of range`;
    if (binding.account_sequence !== sequence) return `leg ${leg.index} account sequence is stale`;
    if (binding.not_before > binding.not_after || binding.not_after > deadline)
      return `leg ${leg.index} window exceeds the deadline`;
    if (now < binding.not_before || now > binding.not_after)
      return `leg ${leg.index} window does not contain the submission time`;
    if (leg.domain === 'layerx') sequence += 1;
  }
  return null;
}

export async function startAccountEndpoint(): Promise<AccountEndpoint> {
  const seen: SeenRequest[] = [];
  const unmatched: string[] = [];
  const refusals: string[] = [];
  const recorded = Object.fromEntries(
    SCENARIOS.map((scenario) => [
      scenario,
      recordings(scenario).filter(
        (entry) =>
          !entry.request.method.startsWith('lx_') &&
          entry.request.method !== 'paxeer_custodyStatus' &&
          entry.request.method !== 'paxeer_prepareCustody'
      ),
    ])
  ) as Record<Scenario, Exchange[]>;
  const statuses = explorerStatuses();
  const state = { now: 0 };

  function answer(
    scenario: Scenario,
    call: { id: unknown; method: string; params: unknown[] }
  ): unknown {
    const exchange = recorded[scenario].find(
      (entry) =>
        entry.request.method === call.method &&
        normalised(entry.request.params) === normalised(call.params)
    );
    if (!exchange) {
      unmatched.push(`${scenario} ${call.method} ${JSON.stringify(call.params)}`);
      return { jsonrpc: '2.0', id: call.id, error: { code: -32601, message: 'Method not found' } };
    }
    return { jsonrpc: '2.0', id: call.id, ...exchange.response };
  }

  function human(req: IncomingMessage, route: string, body: unknown, res: ServerResponse): void {
    const plan = golden('intent.plan.request');
    const submit = golden('intent.submit.request');
    const journey = golden('journey.get.request');
    if (req.method === 'POST' && route === plan.path) {
      const recordedAnswer = golden(
        JSON.stringify(body) === JSON.stringify(plan.body)
          ? 'intent.plan.response'
          : 'intent.plan.failure'
      );
      reply(res, recordedAnswer.status ?? 500, recordedAnswer.body);
      return;
    }
    if (req.method === 'POST' && route === submit.path) {
      const refusal = submitRefusal(body, state.now);
      if (refusal) refusals.push(refusal);
      const recordedAnswer = golden(refusal ? 'intent.submit.failure' : 'intent.submit.response');
      reply(res, recordedAnswer.status ?? 500, recordedAnswer.body);
      return;
    }
    if (req.method === 'GET' && route.startsWith('/v1/journeys/')) {
      const recordedAnswer = golden(
        route === journey.path ? 'journey.get.response' : 'journey.get.failure'
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

  const server = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (chunk: Buffer) => chunks.push(chunk));
    req.on('end', () => {
      if (req.method === 'OPTIONS') {
        res.writeHead(204, {
          'access-control-allow-origin': '*',
          'access-control-allow-headers': '*',
          'access-control-allow-methods': 'GET, POST',
        });
        res.end();
        return;
      }
      const raw = Buffer.concat(chunks).toString('utf8');
      const body: unknown = raw.length > 0 ? JSON.parse(raw) : undefined;
      const url = new URL(req.url ?? '/', 'http://127.0.0.1');
      const [, scope = '', ...rest] = url.pathname.split('/');
      const route = `/${rest.join('/')}`;
      const header = (name: string) => {
        const value = req.headers[name];
        return typeof value === 'string' ? value : null;
      };
      const calls = Array.isArray(body) ? (body as { method: string }[]) : [];
      seen.push({
        scope,
        path: route,
        methods:
          scope === 'human' || scope === 'explorer'
            ? [req.method ?? '']
            : calls.length > 0
              ? calls.map((call) => call.method)
              : [(body as { method?: string } | undefined)?.method ?? ''],
        authorization: header('authorization'),
        idempotencyKey: header('idempotency-key'),
        body,
      });
      if (scope === 'human') {
        human(req, route, body, res);
        return;
      }
      if (scope === 'explorer') {
        const match = /^\/api\/v2\/transactions\/(0x[0-9a-fA-F]{64})\/status$/.exec(route);
        const status = match ? statuses[match[1].toLowerCase()] : undefined;
        if (status === undefined) {
          unmatched.push(`explorer ${route}`);
          reply(res, 404, { message: 'Not found' });
          return;
        }
        reply(res, 200, status);
        return;
      }
      const scenario = scope as Scenario;
      if (!SCENARIOS.includes(scenario) || route !== '/rpc' || req.method !== 'POST') {
        unmatched.push(`${req.method ?? ''} ${url.pathname}`);
        reply(res, 404, { error: 'unknown route' });
        return;
      }
      const batch = Array.isArray(body);
      const answers = (batch ? (body as unknown[]) : [body]).map((call) =>
        answer(scenario, call as { id: unknown; method: string; params: unknown[] })
      );
      reply(res, 200, batch ? answers : answers[0]);
    });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;

  return {
    base,
    seen,
    unmatched,
    refusals,
    get now() {
      return state.now;
    },
    set now(value: number) {
      state.now = value;
    },
    config(scenario: Scenario): AccountConfig {
      return {
        endpointUrl: `${base}/${scenario}/rpc`,
        humanUrl: `${base}/human`,
        explorerUrl: `${base}/explorer`,
      };
    },
    methods(scenario: Scenario): string[] {
      return seen.filter((entry) => entry.scope === scenario).flatMap((entry) => entry.methods);
    },
    close(): Promise<void> {
      return new Promise((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve()))
      );
    },
  };
}
