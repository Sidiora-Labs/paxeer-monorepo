import { createHash } from 'node:crypto';
import { describe, expect, test } from 'vitest';
import { ed25519 } from '@noble/curves/ed25519.js';
import { privateKeyToAccount } from 'viem/accounts';
import {
  AgentClient,
  GatewayClientError,
  InvalidParamsError,
  agentPublicKey,
  signAgentRequest,
  type AgentSigningOptions,
  type AgentActionResponse,
  type AgentPolicy,
  type AgentTransactionBody,
  type AgentAllowanceAndCallRequest,
  type AgentBroadcastOutcomeUnknown,
} from '../src/index.js';
import {
  verifyEd25519,
  verifyAgentSignature,
} from '../../../services/wallet/gateway/src/auth/did.js';
import construction from './fixtures/gateway/constructions.json';
import sendFixture from './fixtures/gateway/wallet-send.json';
import messageFixture from './fixtures/gateway/wallet-sign-message.json';
import typedDataFixture from './fixtures/gateway/wallet-sign-typed-data.json';
import refusal from './fixtures/gateway/refusal-tx-value-cap.json';

const apiUrl = 'https://wallet.example.invalid';
const privateKey = new Uint8Array(32).fill(7);
const publicKey = Buffer.from(agentPublicKey(privateKey)).toString('hex');
const did = `did:matrix:agentclient:${publicKey.slice(0, 16)}`;
const token = 'agent-read-token';
const signing: AgentSigningOptions = {
  privateKey,
  keyId: did,
  nonce: Uint8Array.from({ length: 16 }, (_, i) => i),
  expiresAt: 1_900_000_060,
};
const address = construction.address as `0x${string}`;
const spender = construction.transaction.to as `0x${string}`;
const tx: AgentTransactionBody = { tx: construction.transaction as AgentTransactionBody['tx'] };
const challenge = {
  did,
  nonce: 'single-use-challenge',
  expires_in: 60,
  message: `paxeer-agent-auth:${did}:single-use-challenge`,
};
const verifyBody = {
  did,
  public_key: publicKey,
  nonce: challenge.nonce,
  signature: Buffer.from(
    ed25519.sign(new TextEncoder().encode(challenge.message), privateKey)
  ).toString('hex'),
};
const tokenResponse = {
  token,
  token_type: 'Bearer',
  expires_in: 900,
  did,
  scope: 'read',
  owner_user_id: null,
  is_frozen: false,
};
const wallet = { id: 'agent-wallet', address, chain_id: 125 };
const policy: AgentPolicy = {
  mode: 'trade_only',
  max_tx_value_wei: '100',
  max_daily_value_wei: '1000',
  max_approve_wei: '200',
  rate_limit_per_min: 10,
  allow_native_transfer: false,
  withdrawal_allowlist_only: true,
  daily_reset_utc_hour: 0,
};
const { daily_reset_utc_hour: _resetHour, ...mePolicy } = policy;
const action: AgentActionResponse = {
  ok: null,
  action_id: `act_${'ab'.repeat(12)}`,
  kind: 'allowance_and_call',
  status: 'received',
  phase: null,
  terminal: false,
  approval: null,
  call: null,
  credit_verified: false,
  credit: null,
  error: null,
  poll: { endpoint: `/v1/agent/actions/act_${'ab'.repeat(12)}`, after_ms: 1000 },
  created_at: '1970-01-01T00:00:00.000Z',
  updated_at: '1970-01-01T00:00:00.000Z',
  terminal_at: null,
};
const allowanceCall: AgentAllowanceAndCallRequest = {
  token: address,
  spender,
  amount: '100',
  contract: spender,
  method: 'deposit(uint256)',
  args: ['100'],
  idempotency_key: 'deposit:client:1',
};
const sent = { ...sendFixture.body, op: 'send', receipt: null, rung: 'submitted' };
const signedTransaction = await privateKeyToAccount(`0x${'01'.repeat(32)}`).signTransaction({
  type: 'eip1559',
  ...construction.transaction,
  to: spender,
  data: '0x',
  value: BigInt(construction.transaction.value),
  gas: BigInt(construction.transaction.gas),
  maxFeePerGas: BigInt(construction.transaction.maxFeePerGas),
  maxPriorityFeePerGas: BigInt(construction.transaction.maxPriorityFeePerGas),
});

/** Gateway digest framing, using Node's SHA-256 and its Ed25519 verifier. */
function gatewayDigest(method: string, url: string, headers: Headers, body: string): Buffer {
  const methodBytes = Buffer.from(`${method.toUpperCase()} ${new URL(url).pathname}`);
  const keyBytes = Buffer.from(headers.get('x-agent-key')!);
  const methodLength = Buffer.alloc(4);
  methodLength.writeUInt32BE(methodBytes.length);
  const keyLength = Buffer.alloc(4);
  keyLength.writeUInt32BE(keyBytes.length);
  const expiry = Buffer.alloc(8);
  expiry.writeBigUInt64BE(BigInt(headers.get('x-agent-expires')!));
  return createHash('sha256')
    .update(
      Buffer.concat([
        Buffer.from('PXW:AGENT-REQUEST:v1'),
        methodLength,
        methodBytes,
        keyLength,
        keyBytes,
        Buffer.from(headers.get('x-agent-nonce')!, 'hex'),
        expiry,
        createHash('sha256').update(body).digest(),
      ])
    )
    .digest();
}

interface AgentCase {
  name: string;
  method: string;
  path: string;
  auth: 'public' | 'token' | 'signed';
  body?: unknown;
  response: unknown;
  status?: number;
  call: (client: AgentClient) => Promise<unknown>;
}
const cases: AgentCase[] = [
  {
    name: 'challenge',
    method: 'POST',
    path: '/v1/agent/auth/challenge',
    auth: 'public',
    body: { did },
    response: challenge,
    call: (c) => c.challenge({ did }),
  },
  {
    name: 'verify',
    method: 'POST',
    path: '/v1/agent/auth/verify',
    auth: 'public',
    body: verifyBody,
    response: { ...tokenResponse, wallet_provisioned: false },
    call: (c) => c.verify(verifyBody),
  },
  {
    name: 'refresh',
    method: 'POST',
    path: '/v1/agent/auth/refresh',
    auth: 'token',
    response: tokenResponse,
    call: (c) => c.refresh(token),
  },
  {
    name: 'whoami',
    method: 'GET',
    path: '/v1/agent/whoami',
    auth: 'token',
    response: {
      did,
      owner_user_id: null,
      label: 'agentclient',
      is_frozen: false,
      wallet: null,
      policy,
      created_at: action.created_at,
      last_seen_at: null,
    },
    call: (c) => c.whoami(token),
  },
  {
    name: 'provision',
    method: 'POST',
    path: '/v1/agent/provision',
    auth: 'signed',
    body: { bind_signature: verifyBody.signature },
    status: 202,
    response: {
      wallet: { ...wallet, kind: 'agent' },
      provision: {
        state: 'identity',
        awaiting: 'agent_signature',
        walletId: wallet.id,
        address,
        did,
        mainAccountId: null,
        bindMessage: '0x1234',
      },
    },
    call: (c) => c.provision({ bind_signature: verifyBody.signature }, signing),
  },
  {
    name: 'me',
    method: 'GET',
    path: '/v1/agent/me',
    auth: 'token',
    response: {
      did,
      owner_user_id: null,
      is_frozen: false,
      wallet,
      policy: mePolicy,
      chain: { id: 125, rpc_url: 'https://rpc.example.invalid' },
    },
    call: (c) => c.me(token),
  },
  {
    name: 'balances',
    method: 'GET',
    path: `/v1/agent/balances?tokens=${address}%2C${spender}`,
    auth: 'token',
    response: {
      address,
      native_wei: null,
      tokens: [{ token: address, symbol: null, decimals: null, balance: null }],
    },
    call: (c) => c.balances(token, [address, spender]),
  },
  {
    name: 'nonce',
    method: 'GET',
    path: '/v1/agent/nonce',
    auth: 'token',
    response: { address, nonce: 11 },
    call: (c) => c.nonce(token),
  },
  {
    name: 'simulate',
    method: 'POST',
    path: '/v1/agent/simulate',
    auth: 'token',
    body: tx,
    response: { ok: false, gas: null, return_data: null, error: 'execution reverted' },
    call: (c) => c.simulate(token, tx),
  },
  {
    name: 'allowance',
    method: 'GET',
    path: `/v1/agent/allowance?token=${address}&spender=${spender}`,
    auth: 'token',
    response: {
      token: address.toLowerCase(),
      spender: spender.toLowerCase(),
      allowance: '90071992547409930000',
    },
    call: (c) => c.allowance(token, { token: address, spender }),
  },
  {
    name: 'transaction',
    method: 'GET',
    path: `/v1/agent/tx/${sendFixture.body.tx_hash}`,
    auth: 'token',
    response: {
      status: 'reverted',
      block_number: '123',
      gas_used: '21000',
      tx_hash: sendFixture.body.tx_hash,
    },
    call: (c) => c.transaction(token, sendFixture.body.tx_hash as `0x${string}`),
  },
  {
    name: 'activity',
    method: 'GET',
    path: '/v1/agent/activity',
    auth: 'token',
    response: {
      did,
      events: [
        {
          kind: 'transaction',
          to_address: address,
          value_wei: '1',
          tx_hash: null,
          created_at: action.created_at,
        },
      ],
    },
    call: (c) => c.activity(token),
  },
  {
    name: 'sign',
    method: 'POST',
    path: '/v1/agent/sign',
    auth: 'signed',
    body: tx,
    response: {
      signed_tx: signedTransaction,
      address,
      chain_id: 125,
      op: 'sign',
      broadcast_deadline: '2030-01-01T00:00:00.000Z',
    },
    call: (c) => c.sign(tx, signing),
  },
  {
    name: 'send',
    method: 'POST',
    path: '/v1/agent/send',
    auth: 'signed',
    body: tx,
    response: sent,
    call: (c) => c.send(tx, signing),
  },
  {
    name: 'signMessage',
    method: 'POST',
    path: '/v1/agent/sign-message',
    auth: 'signed',
    body: { message: construction.message },
    response: messageFixture.body,
    call: (c) => c.signMessage({ message: construction.message }, signing),
  },
  {
    name: 'signTypedData',
    method: 'POST',
    path: '/v1/agent/sign-typed-data',
    auth: 'signed',
    body: { typedData: construction.typedData },
    response: typedDataFixture.body,
    call: (c) => c.signTypedData({ typedData: construction.typedData }, signing),
  },
  {
    name: 'transfer',
    method: 'POST',
    path: '/v1/agent/transfer',
    auth: 'signed',
    body: { token: null, to: spender, amount: '1' },
    response: { ...sent, op: 'native.transfer' },
    call: (c) => c.transfer({ token: null, to: spender, amount: '1' }, signing),
  },
  {
    name: 'approve',
    method: 'POST',
    path: '/v1/agent/approve',
    auth: 'signed',
    body: { token: address, spender, amount: '100' },
    response: { ...sent, op: 'erc20.approve' },
    call: (c) => c.approve({ token: address, spender, amount: '100' }, signing),
  },
  {
    name: 'allowanceAndCall',
    method: 'POST',
    path: '/v1/agent/actions/allowance-and-call',
    auth: 'signed',
    body: allowanceCall,
    response: action,
    status: 202,
    call: (c) => c.allowanceAndCall(allowanceCall, signing),
  },
  {
    name: 'action',
    method: 'GET',
    path: `/v1/agent/actions/${action.action_id}`,
    auth: 'token',
    response: { ...action, replay: true },
    call: (c) => c.action(token, action.action_id),
  },
];

describe.each(cases)('AgentClient.$name', (entry) => {
  async function agent_route_request() {
    let calls = 0;
    const fetchImpl: typeof fetch = async (url, init) => {
      calls++;
      expect(url).toBe(`${apiUrl}${entry.path}`);
      expect(init?.method).toBe(entry.method);
      const serialized = entry.body === undefined ? undefined : JSON.stringify(entry.body);
      expect(init?.body).toBe(serialized);
      const expectedHeaders = {
        ...(entry.body === undefined ? {} : { 'content-type': 'application/json' }),
        ...(entry.auth === 'token' ? { authorization: `Bearer ${token}` } : {}),
        ...(entry.auth === 'signed'
          ? signAgentRequest({
              ...signing,
              method: entry.method,
              url: `${apiUrl}${entry.path}`,
              body: serialized,
            })
          : {}),
      };
      const headers = new Headers(init?.headers);
      expect(Object.fromEntries(headers)).toEqual(expectedHeaders);
      if (entry.auth === 'signed') {
        expect(
          verifyEd25519(
            publicKey,
            gatewayDigest(entry.method, String(url), headers, serialized!),
            headers.get('x-agent-signature')!
          )
        ).toBe(true);
      }
      return Response.json(entry.response, { status: entry.status ?? 200 });
    };
    const client = new AgentClient({ apiUrl: `${apiUrl}/`, fetch: fetchImpl });
    expect(await entry.call(client)).toEqual(entry.response);
    expect(calls).toBe(1);
  }
  async function agent_route_error() {
    let calls = 0;
    const fetchImpl: typeof fetch = async () => {
      calls++;
      return Response.json(refusal.body, { status: refusal.status });
    };
    const client = new AgentClient({ apiUrl, fetch: fetchImpl });
    const error = await entry.call(client).catch((error: unknown) => error);
    expect(error).toBeInstanceOf(GatewayClientError);
    expect(error).toMatchObject({
      status: refusal.status,
      code: refusal.body.error,
      message: refusal.body.message,
      body: refusal.body,
    });
    expect(calls).toBe(1);
  }
  test(agent_route_request.name, agent_route_request);
  test(agent_route_error.name, agent_route_error);
});

async function agent_signature_binds_method_path_body_and_headers() {
  let checked = false;
  const fetchImpl: typeof fetch = async (url, init) => {
    const headers = new Headers(init?.headers);
    const signature = headers.get('x-agent-signature')!;
    const body = String(init?.body);
    const verify = (method: string, route: string, h: Headers, bytes: string) =>
      verifyEd25519(publicKey, gatewayDigest(method, route, h, bytes), signature);
    expect(verify('POST', String(url), headers, body)).toBe(true);
    expect(verify('GET', String(url), headers, body)).toBe(false);
    expect(verify('POST', `${apiUrl}/other`, headers, body)).toBe(false);
    expect(verify('POST', String(url), headers, `${body} `)).toBe(false);
    for (const [key, value] of [
      ['x-agent-key', `${did}x`],
      ['x-agent-nonce', 'ff'.repeat(16)],
      ['x-agent-expires', '1900000061'],
    ]) {
      const changed = new Headers(headers);
      changed.set(key, value);
      expect(verify('POST', String(url), changed, body)).toBe(false);
    }
    expect(headers.get('x-agent-attestor-authorization-id')).toBe('retained-authorization');
    checked = true;
    return Response.json(messageFixture.body);
  };
  const client = new AgentClient({ apiUrl: `${apiUrl}/gateway`, fetch: fetchImpl });
  await client.signMessage(
    { message: 'Unicode message: \u03bb' },
    { ...signing, authorizationId: 'retained-authorization' }
  );
  expect(checked).toBe(true);
  expect(
    verifyAgentSignature({
      did,
      publicKeyHex: publicKey,
      nonce: verifyBody.nonce,
      signatureHex: verifyBody.signature,
    }).ok
  ).toBe(true);
}

async function agent_default_signing_uses_fresh_nonces_and_bounded_expiry() {
  const seen: string[] = [];
  const before = Math.floor(Date.now() / 1000);
  const fetchImpl: typeof fetch = async (url, init) => {
    const headers = new Headers(init?.headers);
    const nonce = headers.get('x-agent-nonce')!;
    expect(nonce).toMatch(/^[0-9a-f]{32}$/);
    seen.push(nonce);
    const expiry = Number(headers.get('x-agent-expires'));
    expect(expiry).toBeGreaterThanOrEqual(before + 60);
    expect(expiry).toBeLessThanOrEqual(Math.floor(Date.now() / 1000) + 60);
    expect(
      verifyEd25519(
        publicKey,
        gatewayDigest('POST', String(url), headers, String(init?.body)),
        headers.get('x-agent-signature')!
      )
    ).toBe(true);
    return Response.json(sent);
  };
  const client = new AgentClient({ apiUrl, fetch: fetchImpl });
  await client.send(tx, { privateKey, keyId: did });
  await client.send(tx, { privateKey, keyId: did });
  expect(seen).toHaveLength(2);
  expect(new Set(seen).size).toBe(2);
}

async function agent_errors_preserve_action_and_broadcast_recovery() {
  const envelope = {
    ok: false,
    action_id: action.action_id,
    phase: 'validation',
    code: 'INVALID_REQUEST',
    message: 'Invalid method',
    cause: {},
    retry: {
      allowed: false,
      strategy: 'NEVER',
      after_ms: null,
      reuse_idempotency_key: true,
      must_not_resubmit: false,
    },
    remedy: { action: 'NONE', endpoint: null },
  } as const;
  const rejected = {
    ...action,
    ok: false,
    terminal: true,
    status: 'invalid_request',
    error: envelope,
  };
  const uncertain: AgentBroadcastOutcomeUnknown = {
    error: 'outcome_unknown',
    tx_hash: sent.tx_hash as `0x${string}`,
    nonce: 11,
    retry: 'poll_receipt',
    poll: 'eth_getTransactionReceipt',
    must_not_resubmit: true,
    member: null,
    members: 2,
  };
  const responses = [
    Response.json(rejected, { status: 400 }),
    Response.json(uncertain, { status: 202 }),
    Response.json({ ...action, error: { ...envelope, code: 'RPC_UNAVAILABLE' } }),
    new Response('unavailable', { status: 503 }),
  ];
  let calls = 0;
  const fetchImpl: typeof fetch = async () => responses[calls++]!;
  const client = new AgentClient({ apiUrl, fetch: fetchImpl });
  await expect(client.allowanceAndCall(allowanceCall, signing)).rejects.toMatchObject({
    status: 400,
    code: 'INVALID_REQUEST',
    message: 'Invalid method',
    body: rejected,
  });
  expect(await client.send(tx, signing)).toEqual(uncertain);
  expect(await client.action(token, action.action_id)).toEqual({
    ...action,
    error: { ...envelope, code: 'RPC_UNAVAILABLE' },
  });
  await expect(client.me(token)).rejects.toMatchObject({
    status: 503,
    code: 'HTTP_503',
    body: null,
  });
  expect(calls).toBe(4);
}

async function agent_broadcast_returns_accepted_outcome_unknown() {
  const body: AgentBroadcastOutcomeUnknown = {
    error: 'outcome_unknown',
    tx_hash: sent.tx_hash as `0x${string}`,
    nonce: 11,
    retry: 'poll_receipt',
    poll: 'eth_getTransactionReceipt',
    must_not_resubmit: true,
    member: 0,
    members: 2,
  };
  const paths: string[] = [];
  const fetchImpl: typeof fetch = async (url) => {
    paths.push(new URL(String(url)).pathname);
    return Response.json(body, { status: 202 });
  };
  const client = new AgentClient({ apiUrl, fetch: fetchImpl });
  for (const result of [
    await client.send(tx, signing),
    await client.transfer({ to: spender, amount: '1' }, signing),
    await client.approve({ token: address, spender, amount: '100' }, signing),
  ]) {
    expect(result).toEqual(body);
    expect('error' in result && result.must_not_resubmit).toBe(true);
  }
  expect(paths).toEqual(['/v1/agent/send', '/v1/agent/transfer', '/v1/agent/approve']);
}

async function agent_action_returns_http_success_with_error_envelope() {
  const body: AgentActionResponse = {
    ...action,
    ok: false,
    status: 'simulation_reverted',
    phase: 'call_simulation',
    terminal: true,
    terminal_at: action.updated_at,
    poll: { endpoint: action.poll.endpoint, after_ms: null },
    error: {
      ok: false,
      action_id: action.action_id,
      phase: 'call_simulation',
      code: 'SIMULATION_REVERTED',
      message: 'execution reverted',
      cause: {},
      retry: {
        allowed: false,
        strategy: 'NEVER',
        after_ms: null,
        reuse_idempotency_key: true,
        must_not_resubmit: false,
      },
      remedy: { action: 'NONE', endpoint: null },
    },
  };
  let calls = 0;
  const fetchImpl: typeof fetch = async (url, init) => {
    calls++;
    expect(url).toBe(`${apiUrl}/v1/agent/actions/${action.action_id}`);
    expect(init?.method).toBe('GET');
    expect(new Headers(init?.headers).get('authorization')).toBe(`Bearer ${token}`);
    return Response.json(body, { status: 200 });
  };
  const client = new AgentClient({ apiUrl, fetch: fetchImpl });
  expect(await client.action(token, action.action_id)).toEqual(body);
  expect(calls).toBe(1);
}

async function agent_simulation_returns_success_and_revert_results() {
  const bodies = [
    { ok: true, gas: '21000', return_data: '0x1234', error: null },
    { ok: false, gas: null, return_data: null, error: 'execution reverted' },
  ] as const;
  let calls = 0;
  const fetchImpl: typeof fetch = async (url, init) => {
    expect(url).toBe(`${apiUrl}/v1/agent/simulate`);
    expect(init?.method).toBe('POST');
    expect(init?.body).toBe(JSON.stringify(tx));
    expect(Object.fromEntries(new Headers(init?.headers))).toEqual({
      authorization: `Bearer ${token}`,
      'content-type': 'application/json',
    });
    return Response.json(bodies[calls++], { status: 200 });
  };
  const client = new AgentClient({ apiUrl, fetch: fetchImpl });
  for (const expected of bodies) {
    const result = await client.simulate(token, tx);
    expect(result).toEqual(expected);
    const ok: boolean = result.ok;
    const gas: string | null = result.gas;
    const returnData: `0x${string}` | null = result.return_data;
    const error: string | null = result.error;
    expect({ ok, gas, return_data: returnData, error }).toEqual(expected);
  }
  expect(calls).toBe(2);
}

async function agent_optional_inputs_and_encoded_action_paths() {
  const seen: { url: string; body: unknown }[] = [];
  const fetchImpl: typeof fetch = async (url, init) => {
    seen.push({
      url: String(url),
      body: init?.body === undefined ? undefined : JSON.parse(String(init.body)),
    });
    return Response.json(action);
  };
  const client = new AgentClient({ apiUrl, fetch: fetchImpl });
  await client.balances(token);
  await client.provision({}, signing);
  await client.transfer({ to: spender, amount: '1' }, signing);
  await client.transfer({ token: address, to: spender, amount: '2' }, signing);
  await client.action(token, 'act_/other?query');
  expect(seen).toEqual([
    { url: `${apiUrl}/v1/agent/balances`, body: undefined },
    { url: `${apiUrl}/v1/agent/provision`, body: {} },
    { url: `${apiUrl}/v1/agent/transfer`, body: { to: spender, amount: '1' } },
    { url: `${apiUrl}/v1/agent/transfer`, body: { token: address, to: spender, amount: '2' } },
    { url: `${apiUrl}/v1/agent/actions/act_%2Fother%3Fquery`, body: undefined },
  ]);
  await expect(client.me('')).rejects.toBeInstanceOf(InvalidParamsError);
  await expect(
    client.send(tx, { ...signing, privateKey: new Uint8Array(31) })
  ).rejects.toBeInstanceOf(InvalidParamsError);
  expect(seen).toHaveLength(5);
}

test(
  agent_signature_binds_method_path_body_and_headers.name,
  agent_signature_binds_method_path_body_and_headers
);
test(
  agent_default_signing_uses_fresh_nonces_and_bounded_expiry.name,
  agent_default_signing_uses_fresh_nonces_and_bounded_expiry
);
test(
  agent_errors_preserve_action_and_broadcast_recovery.name,
  agent_errors_preserve_action_and_broadcast_recovery
);
test(
  agent_optional_inputs_and_encoded_action_paths.name,
  agent_optional_inputs_and_encoded_action_paths
);
test(
  agent_broadcast_returns_accepted_outcome_unknown.name,
  agent_broadcast_returns_accepted_outcome_unknown
);
test(
  agent_action_returns_http_success_with_error_envelope.name,
  agent_action_returns_http_success_with_error_envelope
);
test(
  agent_simulation_returns_success_and_revert_results.name,
  agent_simulation_returns_success_and_revert_results
);
