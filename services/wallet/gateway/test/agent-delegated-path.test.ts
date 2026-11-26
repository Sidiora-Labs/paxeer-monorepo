import { randomBytes, randomUUID } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import type { FastifyInstance } from 'fastify';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';
import {
  encodeFunctionData,
  erc20Abi,
  parseAbi,
  parseTransaction,
  toFunctionSelector,
  type Hex,
} from 'viem';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import { SIDIORA_TOKEN } from '../src/provision/protocol.js';
import type { WalletInventoryKey } from '../src/agent/authority.js';
import type { AgentPolicyRuleRow } from '../src/db/agents.js';
import { TestChain } from './support/chain.js';
import { startBindingStation } from './support/binding-station.js';
import { newAgentKey } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';
import {
  startAttestorNetwork,
  startIdentity,
  PAYMASTER,
  type AttestorNetwork,
  type Identity,
} from './e2e/attestors.js';

let agent: ReturnType<typeof newAgentKey>;
let ownerId: string;
const application = privateKeyToAccount(generatePrivateKey()).address.toLowerCase() as Hex;
const token = SIDIORA_TOKEN.toLowerCase() as Hex;
const outsider = privateKeyToAccount(generatePrivateKey()).address.toLowerCase() as Hex;
const tradeAbi = parseAbi(['function trade(uint256 amount) payable']);
const tradeSelector = toFunctionSelector('trade(uint256)');
const tradeData = encodeFunctionData({ abi: tradeAbi, functionName: 'trade', args: [1n] });
const chain = new TestChain(125);
let identity: Identity;
let network: AttestorNetwork;
let pg: EphemeralPostgres;
let proxy: Server;
let station: Awaited<ReturnType<typeof startBindingStation>>;
let app: FastifyInstance;
let worker: { stop(): Promise<void> };
let database: typeof import('../src/db/pool.js');
let signing: typeof import('../src/agent/verify.js');
let claims: typeof import('../src/routes/agents.js');
let baseUrl: string;
let ownerToken: string;
let readToken: string;
let ownerAddress: Hex;
let agentAddress: Hex;

const rpcFailures: unknown[] = [];
const inventory: WalletInventoryKey[] = [];

type Response = { status: number; body: Record<string, any>; diagnostics: string };

function attestorDiagnostics(offsets: number[]): string {
  return network.nodes
    .map((node, i) => `${node.nodeId}: ${node.log.slice(offsets[i]).join('')}`)
    .join('\n');
}

async function request(
  method: string,
  path: string,
  payload?: unknown,
  headers: Record<string, string> = {}
): Promise<Response> {
  const offsets = network.nodes.map((node) => node.log.length);
  const response = await fetch(baseUrl + path, {
    method,
    headers: {
      ...(payload === undefined ? {} : { 'content-type': 'application/json' }),
      ...headers,
    },
    ...(payload === undefined
      ? {}
      : { body: typeof payload === 'string' ? payload : JSON.stringify(payload) }),
    signal: AbortSignal.timeout(120_000),
  });
  const body = (await response.json()) as Record<string, any>;
  const diagnostics = `${method} ${path}\n` + attestorDiagnostics(offsets);
  return { status: response.status, body, diagnostics };
}

function owner(method: string, suffix: string, payload?: unknown): Promise<Response> {
  return request(method, `/v1/agents/${encodeURIComponent(agent.did)}${suffix}`, payload, {
    authorization: `Bearer ${ownerToken}`,
  });
}

function reader(path: string): Promise<Response> {
  return request('GET', path, undefined, { authorization: `Bearer ${readToken}` });
}

function authorization(body: string) {
  const nonce = randomBytes(16);
  const expiry = Math.floor(Date.now() / 1000) + 300;
  return {
    body,
    nonce: nonce.toString('hex'),
    expiry,
    signature: agent.sign(
      signing.agentRequestDigest({
        method: '/v1/sign',
        keyId: JSON.parse(body).key_id,
        nonce,
        expiry: BigInt(expiry),
        body: Buffer.from(body),
      })
    ),
  };
}

function signed(
  path: string,
  payload: unknown,
  approved?: ReturnType<typeof authorization>
): Promise<Response> {
  const body = JSON.stringify(payload);
  const nonce = randomBytes(16);
  const expiry = BigInt(Math.floor(Date.now() / 1000) + 300);
  const headers: Record<string, string> = {
    'x-agent-key': agent.did,
    'x-agent-nonce': nonce.toString('hex'),
    'x-agent-expires': expiry.toString(),
    'x-agent-signature': agent.sign(
      signing.agentRequestDigest({
        method: `POST ${path}`,
        keyId: agent.did,
        nonce,
        expiry,
        body: Buffer.from(body),
      })
    ),
  };
  if (approved) headers['x-agent-attestor-authorization'] = JSON.stringify(approved);
  return request('POST', path, body, headers);
}

async function authorizedPost(path: string, payload: unknown): Promise<Response> {
  let response = await signed(path, payload);
  for (
    let leg = 0;
    leg < 2 && response.status === 409 && response.body.error === 'agent_reauthorization_required';
    leg++
  ) {
    expect(response.body.signing_request).toBeTypeOf('string');
    response = await signed(path, payload, authorization(response.body.signing_request));
  }
  return response;
}

function accepted(response: Response, status = 200): void {
  expect(response.status, `${JSON.stringify(response.body)}\n${response.diagnostics}`).toBe(status);
  expect(response.body.error).toBeUndefined();
}

async function publishInventory(): Promise<void> {
  network.authority.approveInventory(inventory);
  const response = await request(
    'POST',
    '/v1/admin/custody-inventory',
    {
      token: readFileSync(network.authority.inventoryFile, 'utf8'),
    },
    { authorization: `Bearer ${process.env.GATEWAY_ADMIN_TOKEN}` }
  );
  accepted(response);
}

async function admitGeneratedKeys(owner: string): Promise<void> {
  for (const entry of inventory) {
    if (entry.owner === owner && entry.public_key === '') {
      entry.public_key = await network.publicKey(entry.key_id);
      entry.operations = ['generate', 'sign'];
    }
  }
  await publishInventory();
}

beforeAll(async () => {
  identity = await startIdentity();
  await chain.start();
  const upstreamUrl = chain.url;
  proxy = createServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = Buffer.concat(chunks);
      const rpc = JSON.parse(body.toString('utf8'));
      if (
        rpc.method === 'eth_getCode' ||
        (rpc.method === 'eth_call' && rpc.params[0].data === '0xaffed0e0')
      ) {
        res.setHeader('content-type', 'application/json');
        res.end(
          JSON.stringify({
            jsonrpc: '2.0',
            id: rpc.id,
            result:
              rpc.method === 'eth_getCode'
                ? rpc.params[0] === PAYMASTER
                  ? '0x60005460005260206000f3'
                  : '0x'
                : `0x${'00'.repeat(32)}`,
          })
        );
        return;
      }
      const response = await fetch(upstreamUrl, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body,
      });
      res.writeHead(response.status, { 'content-type': 'application/json' });
      res.end(await response.text());
    } catch (error) {
      rpcFailures.push(error);
      res.writeHead(500).end();
    }
  });
  await new Promise<void>((resolve) => proxy.listen(0, '127.0.0.1', resolve));
  chain.url = `http://127.0.0.1:${(proxy.address() as AddressInfo).port}`;
  network = await startAttestorNetwork({ identity, rpcUrl: chain.url });
  station = await startBindingStation(chain, { tokenAmount: '1' });
  Object.assign(process.env, {
    SUPABASE_URL: identity.url,
    AGENT_JWT_SECRET: randomBytes(32).toString('hex'),
    GATEWAY_ADMIN_TOKEN: randomBytes(32).toString('hex'),
    HYPERPAXEER_RPC_URL: chain.url,
    RPC_URLS: chain.url,
    SEQUENCER_RPC_URL: chain.url,
    WALLET_GAS_STATION_URL: station.url,
    PAXEER_GAS_STATION_PAYMASTER: PAYMASTER,
    PAXEER_GAS_STATION_GAS_LIMIT: '200000',
    PAXEER_GAS_STATION_MAX_TOKEN_AMOUNT: '1000000',
    WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE: network.authority.producerKeyFile,
    WALLET_IDENTITY_BINDING_TENANT: network.authority.tenant,
    WALLET_CUSTODY_INVENTORY_FILE: network.authority.inventoryFile,
    WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE: network.authority.inventoryPublicKeyFile,
    ATTESTOR_ENDPOINTS: network.nodes.map((n) => n.apiUrl).join(','),
    ATTESTOR_CLIENT_CERT_FILE: network.pki.clientCert,
    ATTESTOR_CLIENT_KEY_FILE: network.pki.clientKey,
    ATTESTOR_CA_FILE: network.pki.caFile,
    ATTESTOR_QUORUM: '3',
    ATTESTOR_TIMEOUT_MS: '120000',
    ACTION_WORKER_INTERVAL_MS: '100',
    ACTION_CONFIRMATIONS: '1',
    LOG_LEVEL: 'error',
  });
  pg = await startPostgres();
  database = await import('../src/db/pool.js');
  signing = await import('../src/agent/verify.js');
  claims = await import('../src/routes/agents.js');
  app = await (await import('../src/index.js')).buildApp();
  baseUrl = await app.listen({ host: '127.0.0.1', port: 0 });
}, 900_000);

afterEach(async () => {
  await worker?.stop();
}, 120_000);

afterAll(async () => {
  await worker?.stop();
  await app?.close();
  await database?.closePool();
  await pg?.stop();
  await network?.stop();
  if (station)
    await new Promise<void>((resolve, reject) =>
      station.server.close((err) => (err ? reject(err) : resolve()))
    );
  if (proxy)
    await new Promise<void>((resolve, reject) =>
      proxy.close((err) => (err ? reject(err) : resolve()))
    );
  await chain.stop();
  await identity?.close();
}, 120_000);

async function refuse(
  path: string,
  payload: unknown,
  code: string,
  approved?: ReturnType<typeof authorization>
): Promise<void> {
  const before = chain.requests.length;
  const sent = chain.sent.length;
  const response = await signed(path, payload, approved);
  expect(response.status, `${JSON.stringify(response.body)}\n${response.diagnostics}`).toBe(403);
  expect(response.body.error, `${JSON.stringify(response.body)}\n${response.diagnostics}`).toBe(
    code
  );
  expect(chain.sent).toHaveLength(sent);
  expect(
    chain.requests
      .slice(before)
      .filter((entry) =>
        [
          'eth_call',
          'eth_estimateGas',
          'eth_getTransactionCount',
          'eth_sendRawTransaction',
        ].includes(entry.method)
      )
  ).toEqual([]);
}

async function provisionStandardWallet(): Promise<void> {
  for (const [key_id, curve] of [
    [`wallet:${ownerId}:standard:secp256k1:0`, 'secp256k1'],
    [`account:${ownerId}:standard:ed25519:0`, 'ed25519'],
  ] as const) {
    inventory.push({
      key_id,
      curve,
      owner: ownerId,
      public_key: '',
      epoch: 0,
      operations: ['generate'],
    });
  }
  await publishInventory();
  const pendingOwner = await request(
    'POST',
    '/v1/wallet/provision',
    {},
    { authorization: `Bearer ${ownerToken}` }
  );
  expect(pendingOwner.status, JSON.stringify(pendingOwner.body)).toBe(503);
  expect(pendingOwner.body.error).toBe('custody_authority_unavailable');
  await admitGeneratedKeys(ownerId);
  const provisionedOwner = await request(
    'POST',
    '/v1/wallet/provision',
    {},
    { authorization: `Bearer ${ownerToken}` }
  );
  accepted(provisionedOwner);
  ownerAddress = provisionedOwner.body.wallet.address.toLowerCase();
  expect(provisionedOwner.body.wallet.binding_state).toBe('bound');
}

async function prepareDelegatedAgent(label: string, walletOnly = false): Promise<void> {
  agent = newAgentKey(label);
  ownerId = randomUUID();
  ownerToken = await identity.mint(ownerId);
  const challenge = await request('POST', '/v1/agent/auth/challenge', { did: agent.did });
  accepted(challenge);
  const verify = await request('POST', '/v1/agent/auth/verify', {
    did: agent.did,
    public_key: agent.publicKeyHex,
    nonce: challenge.body.nonce,
    signature: agent.sign(Buffer.from(challenge.body.message)),
  });
  accepted(verify);
  expect(verify.body).toMatchObject({
    owner_user_id: null,
    is_frozen: true,
    wallet_provisioned: false,
    scope: 'read',
  });
  readToken = verify.body.token;

  const expires = BigInt(Math.floor(Date.now() / 1000) + 300);
  accepted(
    await owner('POST', '/claim', {
      expires: expires.toString(),
      signature: agent.sign(claims.agentClaimDigest(agent.did, ownerId, expires)),
    })
  );
  accepted(await owner('POST', '/unfreeze'));

  if (!walletOnly) await provisionStandardWallet();

  const { agentWalletUserId } = await import('../src/db/wallets.js');
  const agentKeyId = `wallet:${agentWalletUserId(agent.did)}:agent:secp256k1:0`;
  inventory.push({
    key_id: agentKeyId,
    curve: 'secp256k1',
    owner: `agent:${agent.publicKeyHex}`,
    public_key: '',
    epoch: 0,
    operations: ['generate'],
  });
  await publishInventory();
  const pending = await signed('/v1/agent/provision', walletOnly ? { stage: 'wallet' } : {});
  accepted(pending, 202);
  if (walletOnly) {
    expect(pending.body.provision).toMatchObject({
      state: 'evm_key',
      did: null,
      bindMessage: null,
    });
    agentAddress = pending.body.wallet.address.toLowerCase();
    await admitGeneratedKeys(`agent:${agent.publicKeyHex}`);
    return;
  }
  expect(pending.body.provision).toMatchObject({
    state: 'identity',
    awaiting: 'agent_signature',
    did: `did:layerx:${agent.publicKeyHex}`,
  });
  agentAddress = pending.body.wallet.address.toLowerCase();
  await admitGeneratedKeys(`agent:${agent.publicKeyHex}`);
  accepted(
    await owner('PUT', '/policy', {
      mode: 'full',
      max_tx_value_wei: '1000000',
      max_daily_value_wei: '1000000',
    })
  );
  const provisioned = await authorizedPost('/v1/agent/provision', {
    bind_signature: agent.sign(Buffer.from(pending.body.provision.bindMessage.slice(2), 'hex')),
  });
  accepted(provisioned);
  expect(provisioned.body.provision.state).toBe('active');
  expect(chain.bindings.get(agentAddress)).toBe(agent.publicKeyHex);
}

async function saveDelegatedPolicy(): Promise<void> {
  const preset = {
    mode: 'trade_only',
    max_tx_value_wei: '100',
    max_daily_value_wei: '100',
    max_approve_wei: '50',
    rate_limit_per_min: 100,
    allow_native_transfer: false,
    withdrawal_allowlist_only: true,
  };
  const policy = await owner('PUT', '/policy', preset);
  accepted(policy);
  expect(policy.body.policy).toMatchObject(preset);
  for (const [subject, value] of [
    ['contract', application],
    ['contract', token],
    ['selector', tradeSelector],
    ['selector', '0x095ea7b3'],
    ['address', application],
    ['withdrawal', ownerAddress],
  ]) {
    const rule = await owner('POST', '/rules', { effect: 'allow', subject, value });
    accepted(rule, 201);
    expect(rule.body.rule).toMatchObject({ effect: 'allow', subject, value });
  }
  const budget = await owner('POST', '/budgets', {
    target_contract: application,
    cap_wei: '25',
    expires_in_seconds: 600,
  });
  accepted(budget, 201);
  expect(budget.body.budget).toMatchObject({ target_contract: application, cap_wei: '25' });
  expect(new Date(budget.body.budget.expires_at).getTime()).toBeGreaterThan(Date.now());
  expect(new Date(budget.body.budget.expires_at).getTime()).toBeLessThanOrEqual(
    Date.now() + 600_000
  );
}

async function performAllowanceAndCall(): Promise<Response> {
  const offsets = network.nodes.map((node) => node.log.length);
  const actionPath = '/v1/agent/actions/allowance-and-call';
  const actionRequest = {
    token,
    amount: '20',
    spender: application,
    contract: application,
    method: 'trade(uint256)',
    args: ['20'],
    idempotency_key: randomUUID(),
  };
  const submitted = await signed(actionPath, actionRequest);
  expect(submitted.status, JSON.stringify(submitted.body)).toBe(202);
  expect(submitted.body.error).toBeNull();
  worker = (await import('../src/agent/actions/worker.js')).startActionWorker(app.log);
  let action = await reader(submitted.body.poll.endpoint);
  const deadline = Date.now() + 60_000;
  const approvedRequests = new Set<string>();
  while (!action.body.terminal && Date.now() < deadline) {
    const signingRequest = action.body.error?.cause?.signing_request;
    if (
      action.body.error?.code === 'AGENT_REAUTHORIZATION_REQUIRED' &&
      typeof signingRequest === 'string' &&
      !approvedRequests.has(signingRequest)
    ) {
      approvedRequests.add(signingRequest);
      const replay = await signed(actionPath, actionRequest, authorization(signingRequest));
      expect(replay.status, `${JSON.stringify(replay.body)}\n${attestorDiagnostics(offsets)}`).toBe(
        200
      );
      expect(replay.body.error).toBeNull();
      expect(replay.body.action_id).toBe(submitted.body.action_id);
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
    action = await reader(submitted.body.poll.endpoint);
    expect(action.status, JSON.stringify(action.body)).toBe(200);
    if (action.body.error !== null) {
      expect(
        action.body.error.code,
        `${JSON.stringify(action.body)}\n${attestorDiagnostics(offsets)}`
      ).toBe('AGENT_REAUTHORIZATION_REQUIRED');
    }
  }
  expect(
    action.body,
    `${JSON.stringify(action.body)}\n${attestorDiagnostics(offsets)}`
  ).toMatchObject({
    ok: true,
    status: 'confirmed',
    terminal: true,
    error: null,
  });
  expect(chain.sent.find((tx) => tx.hash === action.body.approval.tx_hash)).toMatchObject({
    from: agentAddress,
    to: token,
    data: encodeFunctionData({ abi: erc20Abi, functionName: 'approve', args: [application, 20n] }),
  });
  expect(chain.sent.find((tx) => tx.hash === action.body.call.tx_hash)).toMatchObject({
    from: agentAddress,
    to: application,
    value: 0n,
    data: encodeFunctionData({ abi: tradeAbi, functionName: 'trade', args: [20n] }),
  });
  expect(chain.sent.filter((tx) => tx.hash === action.body.approval.tx_hash)).toHaveLength(1);
  expect(chain.sent.filter((tx) => tx.hash === action.body.call.tx_hash)).toHaveLength(1);
  await worker.stop();

  return action;
}

async function delegatedAgentPath(): Promise<void> {
  await prepareDelegatedAgent('delegated');
  await saveDelegatedPolicy();
  chain.balances.set(ownerAddress, 10n ** 18n);
  chain.tokens.set(token, { balances: new Map([[ownerAddress, 1000n]]), allowances: new Map() });
  const agentAccounting = await import('../src/db/agents.js');
  const spentBeforeFunding = await agentAccounting.sumAgentDailyValue(
    agent.did,
    new Date(0).toISOString()
  );
  const requestsBeforeFunding = await agentAccounting.countAgentSignaturesLastMinute(agent.did);
  const nativeFund = await owner('POST', '/fund', { amount: '500000000000000000' });
  accepted(nativeFund);
  expect(nativeFund.body).toMatchObject({
    from: expect.any(String),
    to: expect.any(String),
    tx_hash: expect.stringMatching(/^0x[0-9a-f]{64}$/),
  });
  expect(chain.sent.find((tx) => tx.hash === nativeFund.body.tx_hash)).toMatchObject({
    from: ownerAddress,
    to: agentAddress,
    value: 500000000000000000n,
  });
  const tokenFund = await owner('POST', '/fund', { token, amount: '200' });
  accepted(tokenFund);
  expect(chain.sent.find((tx) => tx.hash === tokenFund.body.tx_hash)).toMatchObject({
    from: ownerAddress,
    to: token,
    data: encodeFunctionData({
      abi: erc20Abi,
      functionName: 'transfer',
      args: [agentAddress, 200n],
    }),
  });
  expect(await agentAccounting.sumAgentDailyValue(agent.did, new Date(0).toISOString())).toBe(
    spentBeforeFunding
  );
  expect(await agentAccounting.countAgentSignaturesLastMinute(agent.did)).toBe(
    requestsBeforeFunding
  );
  const balances = await reader(`/v1/agent/balances?tokens=${token}`);
  accepted(balances);
  expect(balances.body).toMatchObject({
    native_wei: '500000000000000000',
    tokens: [{ token, balance: '200' }],
  });

  const sent = await authorizedPost('/v1/agent/send', {
    tx: { to: application, data: tradeData, value: '60' },
  });
  accepted(sent);
  expect(chain.sent.find((tx) => tx.hash === sent.body.tx_hash)).toMatchObject({
    from: agentAddress,
    to: application,
    data: tradeData,
    value: 60n,
  });

  await refuse(
    '/v1/agent/send',
    { tx: { to: application, data: tradeData, value: '101' } },
    'TX_VALUE_CAP'
  );
  await refuse(
    '/v1/agent/send',
    { tx: { to: application, data: tradeData, value: '60' } },
    'DAILY_VALUE_CAP'
  );
  await refuse(
    '/v1/agent/send',
    { tx: { to: outsider, data: tradeData, value: '0' } },
    'CONTRACT_NOT_ALLOWLISTED'
  );
  await refuse(
    '/v1/agent/approve',
    { token, spender: outsider, amount: '20' },
    'SPENDER_NOT_ALLOWLISTED'
  );
  await refuse('/v1/agent/transfer', { to: ownerAddress, amount: '1' }, 'NATIVE_TRANSFER_BLOCKED');

  const frozen = await owner('POST', '/freeze');
  accepted(frozen);
  expect(frozen.body).toMatchObject({ did: agent.did, is_frozen: true });
  await refuse(
    '/v1/agent/send',
    { tx: { to: application, data: tradeData, value: '0' } },
    'agent_frozen'
  );
  const unfreeze = await owner('POST', '/unfreeze');
  accepted(unfreeze);
  expect(unfreeze.body).toMatchObject({ did: agent.did, is_frozen: false });
  const unfrozen = await authorizedPost('/v1/agent/send', {
    tx: { to: application, data: tradeData, value: '30' },
  });
  accepted(unfrozen);

  const action = await performAllowanceAndCall();

  const beforeSweep = chain.tokens.get(token)!.balances.get(ownerAddress)!;
  const sweptToken = await owner('POST', '/sweep', { token, amount: '180' });
  accepted(sweptToken);
  expect(sweptToken.body.to.toLowerCase()).toBe(ownerAddress);
  expect(chain.tokens.get(token)!.balances.get(ownerAddress)).toBe(beforeSweep + 180n);
  const sweptNative = await owner('POST', '/sweep', { amount: '1000' });
  accepted(sweptNative);
  expect(chain.sent.find((tx) => tx.hash === sweptNative.body.tx_hash)).toMatchObject({
    from: agentAddress,
    to: ownerAddress,
    value: 1000n,
  });
  const activity = await owner('GET', '/activity');
  accepted(activity);
  const hashes = activity.body.events.map((event: { tx_hash: string }) => event.tx_hash);
  expect(hashes).toEqual(
    expect.arrayContaining([
      nativeFund.body.tx_hash,
      tokenFund.body.tx_hash,
      sent.body.tx_hash,
      unfrozen.body.tx_hash,
      action.body.approval.tx_hash,
      action.body.call.tx_hash,
      sweptToken.body.tx_hash,
      sweptNative.body.tx_hash,
    ])
  );
  expect(new Set(hashes).size).toBe(hashes.length);
  const agentActivity = await reader('/v1/agent/activity');
  accepted(agentActivity);
  expect(agentActivity.body.events).toEqual(activity.body.events);
  expect(rpcFailures).toEqual([]);
}

async function selectorAllowRulesRestrictContractFunctions(): Promise<void> {
  await prepareDelegatedAgent('selectors');
  await saveDelegatedPolicy();
  await refuse(
    '/v1/agent/send',
    { tx: { to: application, data: toFunctionSelector('withdraw(uint256)'), value: '0' } },
    'SELECTOR_NOT_ALLOWLISTED'
  );
}

async function approvalSpendersDoNotBecomeWithdrawalDestinations(): Promise<void> {
  await prepareDelegatedAgent('spenders');
  await saveDelegatedPolicy();
  await refuse(
    '/v1/agent/approve',
    { token, spender: outsider, amount: '20' },
    'SPENDER_NOT_ALLOWLISTED'
  );
  const transfer = encodeFunctionData({
    abi: erc20Abi,
    functionName: 'transfer',
    args: [application, 1n],
  });
  const rule = await owner('POST', '/rules', {
    effect: 'allow',
    subject: 'selector',
    value: transfer.slice(0, 10),
  });
  accepted(rule, 201);
  await refuse(
    '/v1/agent/send',
    { tx: { to: token, data: transfer, value: '0' } },
    'WITHDRAWAL_NOT_ALLOWLISTED'
  );
  accepted(await owner('DELETE', `/rules/${rule.body.rule.id}`));
}

async function agentReauthorizationCountsTransactionOnce(): Promise<void> {
  await prepareDelegatedAgent('reauthorization');
  await saveDelegatedPolicy();
  chain.balances.set(agentAddress, 10n ** 18n);
  const path = '/v1/agent/send';
  const payload = { tx: { to: application, data: tradeData, value: '60' } };
  const sentBefore = chain.sent.length;
  const challenge = await signed(path, payload);
  expect(challenge.status, JSON.stringify(challenge.body)).toBe(409);
  expect(challenge.body.error).toBe('agent_reauthorization_required');
  expect(chain.sent).toHaveLength(sentBefore);
  const agents = await import('../src/db/agents.js');
  expect(await agents.sumAgentDailyValue(agent.did, new Date(0).toISOString())).toBe(60n);
  expect(await agents.countAgentSignaturesLastMinute(agent.did)).toBe(1);
  await refuse(path, { tx: { to: application, data: tradeData, value: '41' } }, 'DAILY_VALUE_CAP');
  const wire = JSON.parse(challenge.body.signing_request);
  const offered = parseTransaction(`0x${wire.transaction}`);
  const alreadySigned = await privateKeyToAccount(generatePrivateKey()).signTransaction({
    ...offered,
    r: undefined,
    s: undefined,
    v: undefined,
    yParity: undefined,
  });
  const invalid = await signed(
    path,
    payload,
    authorization(
      JSON.stringify({
        ...wire,
        transaction: alreadySigned.slice(2),
      })
    )
  );
  expect(invalid.status, JSON.stringify(invalid.body)).toBe(401);
  expect(invalid.body).toMatchObject({
    error: 'attestor_token_refused',
    code: 'agent_authorization_mismatch',
  });
  expect(chain.sent).toHaveLength(sentBefore);
  const approved = authorization(challenge.body.signing_request);
  await refuse(
    path,
    { tx: { to: application, data: tradeData, value: '41' } },
    'DAILY_VALUE_CAP',
    approved
  );
  const sent = await signed(path, payload, approved);
  accepted(sent);
  expect(chain.sent).toHaveLength(sentBefore + 1);
  expect(await agents.sumAgentDailyValue(agent.did, new Date(0).toISOString())).toBe(60n);
  expect(await agents.countAgentSignaturesLastMinute(agent.did)).toBe(1);
  expect(chain.sent.at(-1)).toMatchObject({ hash: sent.body.tx_hash, value: 60n });
}

async function standardProvisioningRequiresSigningInventory(): Promise<void> {
  ownerId = randomUUID();
  ownerToken = await identity.mint(ownerId);
  await provisionStandardWallet();
}

async function nativeTransferSwitchRestrictsBareTransfers(): Promise<void> {
  await prepareDelegatedAgent('native-transfer');
  await saveDelegatedPolicy();
  await refuse('/v1/agent/transfer', { to: ownerAddress, amount: '1' }, 'NATIVE_TRANSFER_BLOCKED');
}

async function allowanceAndCallAccountsForEachLegOnce(): Promise<void> {
  await prepareDelegatedAgent('action-accounting');
  await saveDelegatedPolicy();
  chain.balances.set(agentAddress, 10n ** 18n);
  chain.tokens.set(token, { balances: new Map([[agentAddress, 200n]]), allowances: new Map() });
  const action = await performAllowanceAndCall();
  const activity = await owner('GET', '/activity');
  accepted(activity);
  const hashes = activity.body.events.map((event: { tx_hash: string }) => event.tx_hash);
  expect(hashes).toHaveLength(2);
  expect(hashes).toEqual(
    expect.arrayContaining([action.body.approval.tx_hash, action.body.call.tx_hash])
  );
  expect(new Set(hashes).size).toBe(2);
}

async function approvalSpenderPolicyMatchesAttestor(): Promise<void> {
  await prepareDelegatedAgent('policy-parity');
  const agents = await import('../src/db/agents.js');
  const { evaluateAgent } = await import('../src/policy/agent.js');
  const principal = await agents.findPrincipal(agent.did);
  const savedPolicy = await agents.getPolicy(agent.did);
  expect(principal).not.toBeNull();
  expect(savedPolicy).not.toBeNull();
  if (!principal || !savedPolicy) throw new Error('claimed agent has no policy');
  const ownerWallet = '0x5555555555555555555555555555555555555555';
  const spender = '0x2222222222222222222222222222222222222222';
  const other = '0x4444444444444444444444444444444444444444';
  const asset = '0x3333333333333333333333333333333333333333';
  for (const mode of ['trade_only', 'full'] as const) {
    const policy = {
      ...savedPolicy,
      mode,
      max_tx_value_wei: 100n,
      max_daily_value_wei: 100n,
      max_approve_wei: 50n,
      rate_limit_per_min: 100,
      allow_native_transfer: false,
      withdrawal_allowlist_only: true,
    };
    for (const scenario of [
      { name: 'listed spender', method: 'approve', recipient: spender, amount: 20n, code: null },
      {
        name: 'other spender',
        method: 'approve',
        recipient: other,
        amount: 20n,
        code: 'SPENDER_NOT_ALLOWLISTED',
      },
      {
        name: 'other spender on withdrawal list',
        method: 'approve',
        recipient: other,
        amount: 20n,
        withdrawal: other,
        code: 'SPENDER_NOT_ALLOWLISTED',
      },
      {
        name: 'no address rule outside withdrawal list',
        method: 'approve',
        recipient: spender,
        amount: 20n,
        noAddressRule: true,
        code: 'WITHDRAWAL_NOT_ALLOWLISTED',
      },
      {
        name: 'no address rule on withdrawal list',
        method: 'approve',
        recipient: spender,
        amount: 20n,
        noAddressRule: true,
        withdrawal: spender,
        code: null,
      },
      {
        name: 'transfer to spender',
        method: 'transfer',
        recipient: spender,
        amount: 20n,
        code: 'WITHDRAWAL_NOT_ALLOWLISTED',
      },
      {
        name: 'transferFrom to spender',
        method: 'transferFrom',
        recipient: spender,
        amount: 20n,
        code: 'WITHDRAWAL_NOT_ALLOWLISTED',
      },
      {
        name: 'denied spender',
        method: 'approve',
        recipient: spender,
        amount: 20n,
        denySpender: true,
        code: 'ADDRESS_DENYLISTED',
      },
      {
        name: 'approve cap',
        method: 'approve',
        recipient: spender,
        amount: 51n,
        code: 'APPROVE_CAP',
      },
      {
        name: 'token cap',
        method: 'approve',
        recipient: spender,
        amount: 20n,
        tokenCap: 19n,
        code: 'TOKEN_VALUE_CAP',
      },
      {
        name: 'token contract rule',
        method: 'approve',
        recipient: spender,
        amount: 20n,
        omitToken: true,
        code: mode === 'full' ? null : 'CONTRACT_NOT_ALLOWLISTED',
      },
    ] as const) {
      const rules: AgentPolicyRuleRow[] = [];
      function rule(
        effect: AgentPolicyRuleRow['effect'],
        subject: AgentPolicyRuleRow['subject'],
        value: string,
        cap: bigint | null = null
      ): void {
        rules.push({
          id: String(rules.length + 1),
          did: principal!.did,
          effect,
          subject,
          value,
          max_value_wei: cap,
          note: null,
          created_at: principal!.created_at,
          created_by: ownerId,
        });
      }
      rule('allow', 'contract', spender);
      rule('allow', 'withdrawal', ownerWallet);
      if (!('omitToken' in scenario)) rule('allow', 'contract', asset);
      if (!('noAddressRule' in scenario)) rule('allow', 'address', spender);
      if ('withdrawal' in scenario) rule('allow', 'withdrawal', scenario.withdrawal);
      if ('denySpender' in scenario) rule('deny', 'address', spender);
      if ('tokenCap' in scenario) rule('allow', 'token', asset, scenario.tokenCap);
      const recipient = scenario.recipient;
      const data =
        scenario.method === 'transferFrom'
          ? encodeFunctionData({
              abi: erc20Abi,
              functionName: 'transferFrom',
              args: [ownerWallet, recipient, scenario.amount],
            })
          : encodeFunctionData({
              abi: erc20Abi,
              functionName: scenario.method,
              args: [recipient, scenario.amount],
            });
      const decision = await evaluateAgent({
        principal,
        policy,
        rules,
        intent: { kind: 'transaction', to: asset, data, value: 0n },
      });
      expect(decision.allow, `${mode}/${scenario.name}`).toBe(scenario.code === null);
      if (scenario.code !== null)
        expect(decision).toMatchObject({ allow: false, code: scenario.code });
    }
  }
}

test(delegatedAgentPath, delegatedAgentPath, 900_000);
test(approvalSpenderPolicyMatchesAttestor, approvalSpenderPolicyMatchesAttestor, 120_000);
test(
  selectorAllowRulesRestrictContractFunctions,
  selectorAllowRulesRestrictContractFunctions,
  60_000
);
test(
  approvalSpendersDoNotBecomeWithdrawalDestinations,
  approvalSpendersDoNotBecomeWithdrawalDestinations,
  60_000
);

test(agentReauthorizationCountsTransactionOnce, agentReauthorizationCountsTransactionOnce, 60_000);
test(
  standardProvisioningRequiresSigningInventory,
  standardProvisioningRequiresSigningInventory,
  60_000
);
test(
  nativeTransferSwitchRestrictsBareTransfers,
  nativeTransferSwitchRestrictsBareTransfers,
  60_000
);

test(allowanceAndCallAccountsForEachLegOnce, allowanceAndCallAccountsForEachLegOnce, 120_000);

async function walletOnlyProvisioningLeavesIdentityPending(): Promise<void> {
  const before = chain.sent.length;
  await prepareDelegatedAgent('wallet-only', true);
  expect(chain.sent).toHaveLength(before);
  expect(chain.bindings.has(agentAddress)).toBe(false);
  const again = await signed('/v1/agent/provision', { stage: 'wallet' });
  accepted(again, 202);
  expect(again.body.wallet.address.toLowerCase()).toBe(agentAddress);
  expect(again.body.provision).toMatchObject({ state: 'evm_key', did: null, bindMessage: null });
  const identity = await reader('/v1/agent/me');
  accepted(identity);
  expect(identity.body.wallet.address.toLowerCase()).toBe(agentAddress);
  expect(chain.sent).toHaveLength(before);
  const full = await signed('/v1/agent/provision', { stage: 'wallet', unexpected: true });
  expect(full.status).toBe(400);
  expect(full.body.error).toBe('invalid_body');
}

async function launchpadBuyDecodesOnlyCanonicalSignedOperands(): Promise<void> {
  const { decodeLaunchpadBuy, LAUNCHPAD_BUY_ABI } = await import('../src/policy/launchpad.js');
  const encoded = encodeFunctionData({
    abi: LAUNCHPAD_BUY_ABI,
    functionName: 'buy',
    args: [application, 10000001n, 1n, 2000000000n],
  });
  expect(decodeLaunchpadBuy(encoded)).toEqual({ pool: application, amount: 10000001n });
  expect(() => decodeLaunchpadBuy(`${encoded}00`)).toThrow();
  expect(() => decodeLaunchpadBuy(encoded.slice(0, -2) as Hex)).toThrow();
  expect(() => decodeLaunchpadBuy(tradeData)).toThrow();
}

async function receiptSourceSelectionIsExplicit(): Promise<void> {
  const { EnvSchema } = await import('../src/env.js');
  expect(
    EnvSchema.parse({ ...process.env, CHAIN_RECEIPT_SOURCE: undefined }).CHAIN_RECEIPT_SOURCE
  ).toBe('sequencer');
  expect(
    EnvSchema.parse({ ...process.env, CHAIN_RECEIPT_SOURCE: 'evm' }).CHAIN_RECEIPT_SOURCE
  ).toBe('evm');
  expect(() => EnvSchema.parse({ ...process.env, CHAIN_RECEIPT_SOURCE: 'automatic' })).toThrow();
}

test(
  walletOnlyProvisioningLeavesIdentityPending,
  walletOnlyProvisioningLeavesIdentityPending,
  120_000
);
test(
  launchpadBuyDecodesOnlyCanonicalSignedOperands,
  launchpadBuyDecodesOnlyCanonicalSignedOperands
);
test(receiptSourceSelectionIsExplicit, receiptSourceSelectionIsExplicit);
