import { describe, expect, test } from 'vitest';
import {
  OwnerClient,
  GatewayClientError,
  InvalidParamsError,
  buildClaim,
  agentPublicKey,
  type AgentPolicy,
  type AgentRule,
  type AgentBudget,
} from '../src/index.js';
import construction from './fixtures/gateway/constructions.json';
import refusal from './fixtures/gateway/refusal-unauthorized.json';

const apiUrl = 'https://wallet.example.invalid';
const privateKey = new Uint8Array(32).fill(7);
const did = `did:matrix:ownerclient:${Buffer.from(agentPublicKey(privateKey)).toString('hex').slice(0, 16)}`;
const ownerUserId = '5d0c7a4e-8f5b-4c71-9a3e-2b6f1d9e4a10';
const path = `/v1/agents/${encodeURIComponent(did)}`;
const policy: AgentPolicy = {
  mode: 'trade_only',
  max_tx_value_wei: '100',
  max_daily_value_wei: '1000',
  rate_limit_per_min: 10,
  max_approve_wei: '200',
  allow_native_transfer: false,
  withdrawal_allowlist_only: true,
  daily_reset_utc_hour: 0,
};
const rule: AgentRule = {
  id: '9007199254740993',
  effect: 'allow',
  subject: 'contract',
  value: construction.address,
  max_value_wei: '100',
  note: null,
};
const budget: AgentBudget = {
  id: '9007199254740994',
  target_contract: null,
  token: null,
  cap_wei: '1000',
  spent_wei: '100',
  remaining_wei: '900',
  expires_at: '2030-01-01T00:00:00.000Z',
};
const summary = {
  did,
  label: 'ownerclient',
  is_frozen: false,
  mode: policy.mode,
  address: construction.address,
  created_at: '1970-01-01T00:00:00.000Z',
  last_seen_at: null,
};
const claim = buildClaim({ privateKey, did, ownerUserId, expiresAt: 1_900_000_060 });
const patch = {
  ...policy,
  max_tx_value_wei: null,
  max_approve_wei: null,
  rate_limit_per_min: null,
};
const ruleInput = {
  effect: 'allow',
  subject: 'selector',
  value: '0x095ea7b3',
  max_value_wei: null,
  note: 'Approve',
} as const;
const budgetInput = {
  target_contract: null,
  token: null,
  cap_wei: '1000',
  expires_in_seconds: 3600,
};
const treasury = {
  tx_hash: `0x${'ab'.repeat(32)}`,
  from: construction.address,
  to: construction.transaction.to,
};
const address = construction.address as `0x${string}`;

interface OwnerCase {
  name: string;
  method: string;
  path: string;
  body?: unknown;
  response: unknown;
  status?: number;
  call: (client: OwnerClient) => Promise<unknown>;
}
const cases: OwnerCase[] = [
  {
    name: 'list',
    method: 'GET',
    path: '/v1/agents',
    response: { agents: [summary] },
    call: (c) => c.list(),
  },
  {
    name: 'read',
    method: 'GET',
    path,
    response: {
      did,
      label: summary.label,
      owner_user_id: ownerUserId,
      is_frozen: false,
      created_at: summary.created_at,
      last_seen_at: null,
      wallet: { id: ownerUserId, address, native_wei: null },
      policy,
      rules: [rule],
      budgets: [budget],
    },
    call: (c) => c.read(did),
  },
  {
    name: 'claim',
    method: 'POST',
    path: `${path}/claim`,
    body: claim,
    response: { did, owner_user_id: ownerUserId },
    call: (c) => c.claim(did, claim),
  },
  {
    name: 'updatePolicy',
    method: 'PUT',
    path: `${path}/policy`,
    body: patch,
    response: { did, policy },
    call: (c) => c.updatePolicy(did, patch),
  },
  {
    name: 'freeze',
    method: 'POST',
    path: `${path}/freeze`,
    response: { did, is_frozen: true },
    call: (c) => c.freeze(did),
  },
  {
    name: 'unfreeze',
    method: 'POST',
    path: `${path}/unfreeze`,
    response: { did, is_frozen: false },
    call: (c) => c.unfreeze(did),
  },
  {
    name: 'listRules',
    method: 'GET',
    path: `${path}/rules`,
    response: { did, rules: [rule] },
    call: (c) => c.listRules(did),
  },
  {
    name: 'addRule',
    method: 'POST',
    path: `${path}/rules`,
    body: ruleInput,
    response: { rule },
    status: 201,
    call: (c) => c.addRule(did, ruleInput),
  },
  {
    name: 'removeRule',
    method: 'DELETE',
    path: `${path}/rules/${rule.id}`,
    response: { deleted: true },
    call: (c) => c.removeRule(did, rule.id),
  },
  {
    name: 'listBudgets',
    method: 'GET',
    path: `${path}/budgets`,
    response: { did, budgets: [budget] },
    call: (c) => c.listBudgets(did),
  },
  {
    name: 'addBudget',
    method: 'POST',
    path: `${path}/budgets`,
    body: budgetInput,
    response: {
      budget: {
        id: budget.id,
        target_contract: null,
        token: null,
        cap_wei: budget.cap_wei,
        expires_at: budget.expires_at,
      },
    },
    status: 201,
    call: (c) => c.addBudget(did, budgetInput),
  },
  {
    name: 'removeBudget',
    method: 'DELETE',
    path: `${path}/budgets/${budget.id}`,
    response: { deactivated: true },
    call: (c) => c.removeBudget(did, budget.id),
  },
  {
    name: 'fund',
    method: 'POST',
    path: `${path}/fund`,
    body: { token: null, amount: '10' },
    response: treasury,
    call: (c) => c.fund(did, { token: null, amount: '10' }),
  },
  {
    name: 'sweep',
    method: 'POST',
    path: `${path}/sweep`,
    body: { token: address, amount: '10', to: address },
    response: treasury,
    call: (c) => c.sweep(did, { token: address, amount: '10', to: address }),
  },
  {
    name: 'activity',
    method: 'GET',
    path: `${path}/activity`,
    response: {
      did,
      events: [
        {
          kind: 'transaction',
          to_address: null,
          value_wei: '100',
          tx_hash: null,
          created_at: summary.created_at,
        },
      ],
    },
    call: (c) => c.activity(did),
  },
];

describe.each(cases)('OwnerClient.$name', (entry) => {
  async function owner_route_request() {
    let calls = 0;
    const fetchImpl: typeof fetch = async (url, init) => {
      calls++;
      expect(url).toBe(`${apiUrl}${entry.path}`);
      expect(init?.method).toBe(entry.method);
      expect(Object.fromEntries(new Headers(init?.headers))).toEqual({
        authorization: 'Bearer owner-session',
        ...(entry.body === undefined ? {} : { 'content-type': 'application/json' }),
      });
      expect(init?.body).toBe(entry.body === undefined ? undefined : JSON.stringify(entry.body));
      return Response.json(entry.response, { status: entry.status ?? 200 });
    };
    const client = new OwnerClient({
      apiUrl: `${apiUrl}///`,
      token: 'owner-session',
      fetch: fetchImpl,
    });
    expect(await entry.call(client)).toEqual(entry.response);
    expect(calls).toBe(1);
  }
  async function owner_route_error() {
    let calls = 0;
    const fetchImpl: typeof fetch = async () => {
      calls++;
      return Response.json(refusal.body, { status: refusal.status });
    };
    const client = new OwnerClient({ apiUrl, token: 'owner-session', fetch: fetchImpl });
    const error = await entry.call(client).catch((error: unknown) => error);
    expect(error).toBeInstanceOf(GatewayClientError);
    expect(error).toMatchObject({
      status: 401,
      code: 'unauthorized',
      message: refusal.body.detail,
      body: refusal.body,
    });
    expect(calls).toBe(1);
  }
  test(owner_route_request.name, owner_route_request);
  test(owner_route_error.name, owner_route_error);
});

async function owner_token_is_read_for_each_request() {
  let token = 'first-session';
  const seen: (string | null)[] = [];
  const fetchImpl: typeof fetch = async (_url, init) => {
    seen.push(new Headers(init?.headers).get('authorization'));
    return Response.json({ agents: [] });
  };
  const client = new OwnerClient({ apiUrl, token: async () => token, fetch: fetchImpl });
  await client.list();
  token = 'renewed-session';
  await client.list();
  token = '';
  await expect(client.list()).rejects.toBeInstanceOf(InvalidParamsError);
  expect(seen).toEqual(['Bearer first-session', 'Bearer renewed-session']);
}

async function owner_treasury_omits_optional_fields() {
  const seen: unknown[] = [];
  const fetchImpl: typeof fetch = async (_url, init) => {
    seen.push(JSON.parse(String(init?.body)));
    return Response.json(treasury);
  };
  const client = new OwnerClient({ apiUrl, token: 'owner-session', fetch: fetchImpl });
  await client.fund(did, { token: address, amount: '900719925474099300000' });
  await client.fund(did, { amount: '1' });
  await client.sweep(did, { amount: '2' });
  expect(seen).toEqual([
    { token: address, amount: '900719925474099300000' },
    { amount: '1' },
    { amount: '2' },
  ]);
}

async function owner_errors_preserve_mutation_and_deletion_details() {
  const responses = [
    Response.json(
      {
        error: 'custody_authority_unavailable',
        replication_pending: true,
        mutation_recorded: true,
      },
      { status: 503 }
    ),
    Response.json({ deleted: false }, { status: 404 }),
    Response.json({ deactivated: false }, { status: 404 }),
    new Response('unavailable', { status: 502 }),
    new Response('invalid json', { status: 200 }),
  ];
  let calls = 0;
  const fetchImpl: typeof fetch = async () => responses[calls++]!;
  const client = new OwnerClient({ apiUrl, token: 'owner-session', fetch: fetchImpl });
  await expect(client.freeze(did)).rejects.toMatchObject({
    status: 503,
    code: 'custody_authority_unavailable',
    body: { replication_pending: true, mutation_recorded: true },
  });
  await expect(client.removeRule(did, rule.id)).rejects.toMatchObject({
    status: 404,
    code: 'HTTP_404',
    body: { deleted: false },
  });
  await expect(client.removeBudget(did, budget.id)).rejects.toMatchObject({
    status: 404,
    code: 'HTTP_404',
    body: { deactivated: false },
  });
  await expect(client.list()).rejects.toMatchObject({ status: 502, code: 'HTTP_502', body: null });
  await expect(client.list()).rejects.toMatchObject({
    status: 200,
    code: 'invalid_gateway_response',
  });
  expect(calls).toBe(5);
}

test(owner_token_is_read_for_each_request.name, owner_token_is_read_for_each_request);
test(owner_treasury_omits_optional_fields.name, owner_treasury_omits_optional_fields);
test(
  owner_errors_preserve_mutation_and_deletion_details.name,
  owner_errors_preserve_mutation_and_deletion_details
);
