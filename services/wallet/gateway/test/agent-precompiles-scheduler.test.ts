import { randomBytes } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { createServer, type Server } from 'node:http';
import Fastify, { type FastifyInstance } from 'fastify';
import ts from 'typescript';
import {
  decodeFunctionData,
  decodeFunctionResult,
  encodeFunctionData,
  getAddress,
  toFunctionSelector,
  type Abi,
  type AbiFunction,
  type Hex,
} from 'viem';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { newAgentKey, type AgentKey } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const prefix = '/v1/agent/precompiles/scheduler';
const servedAddress = '0x0000000000000000000000000000000000000905';
const target = '0x000000000000000000000000000000000000c0de';
const missingJobId = ((1n << 256n) - 1n).toString();
const writes = [
  {
    name: 'schedule',
    body: {
      target,
      call_data: '0x12345678',
      execute_at_block: '1000000',
      gas_limit: '100000',
      deposit_wei: '1',
    },
  },
  { name: 'cancel', body: { job_id: '1' } },
  { name: 'reschedule', body: { job_id: '1', new_block: '1000001' } },
] as const;
const reads = [
  { path: `/job/${missingJobId}`, name: 'getJob', status: 404 },
  { path: '/pending', name: 'pending', status: 200 },
] as const;
const registrations: string[] = [];
const rpcCalls: { method: string; params: unknown[]; answer: unknown }[] = [];
const proxyErrors: unknown[] = [];
const environmentKeys = [
  'HYPERPAXEER_RPC_URL',
  'RPC_URLS',
  'SEQUENCER_RPC_URL',
  'AGENT_JWT_SECRET',
  'LOG_LEVEL',
  'DATABASE_URL',
] as const;
const originalEnvironment = new Map(environmentKeys.map((key) => [key, process.env[key]]));
let app: FastifyInstance;
let proxy: Server;
let pg: EphemeralPostgres;
let database: typeof import('../src/db/pool.js');
let precompiles: typeof import('../src/precompiles.js');
let PRECOMPILE_ADDRESSES: typeof import('../src/precompiles.js').PRECOMPILE_ADDRESSES;
let nodeAbi: Abi;

async function agentPrecompileRoutes(instance: FastifyInstance): Promise<void> {
  const routes = await import('../src/routes/agentPrecompiles.js');
  await routes.agentPrecompileRoutes(instance);
}

function repositorySource(path: string): string {
  return readFileSync(new URL(`../../../../${path}`, import.meta.url), 'utf8');
}

beforeAll(async () => {
  const schedulerSource = repositorySource('precompiles/layerxagent/scheduler/scheduler.go');
  const abiMatch = /\babiJSON\s*=\s*`([^`]+)`/u.exec(schedulerSource);
  if (!abiMatch) throw new Error('registered scheduler abiJSON was not found');
  nodeAbi = JSON.parse(abiMatch[1]!) as Abi;

  const upstreamUrl = process.env.HYPERPAXEER_RPC_URL!;
  proxy = createServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = Buffer.concat(chunks).toString('utf8');
      const request = JSON.parse(body) as { method: string; params: unknown[] };
      const call = { ...request, answer: undefined as unknown };
      rpcCalls.push(call);
      const upstreamAddress = new URL(upstreamUrl);
      if (upstreamAddress.port === '0') {
        throw new Error(
          'HYPERPAXEER_RPC_URL must identify a running node serving scheduler at 0x0905; test/setup.ts supplies port 0 only for environment validation'
        );
      }
      const upstream = await fetch(upstreamUrl, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body,
        signal: AbortSignal.timeout(5000),
      });
      const answer = await upstream.text();
      call.answer = JSON.parse(answer);
      res.writeHead(upstream.status, { 'content-type': 'application/json' });
      res.end(answer);
    } catch (error) {
      proxyErrors.push(error);
      res.writeHead(502);
      res.end('scheduler RPC upstream unavailable');
    }
  });
  await new Promise<void>((resolve, reject) => {
    proxy.once('error', reject);
    proxy.listen(0, '127.0.0.1', resolve);
  });
  const address = proxy.address();
  if (!address || typeof address === 'string') throw new Error('RPC proxy did not bind');
  const proxyUrl = `http://127.0.0.1:${address.port}`;
  process.env.HYPERPAXEER_RPC_URL = proxyUrl;
  process.env.RPC_URLS = proxyUrl;
  process.env.SEQUENCER_RPC_URL = proxyUrl;
  process.env.AGENT_JWT_SECRET = randomBytes(32).toString('hex');
  process.env.LOG_LEVEL = 'error';
  pg = await startPostgres();
  database = await import('../src/db/pool.js');
  precompiles = await import('../src/precompiles.js');
  PRECOMPILE_ADDRESSES = precompiles.PRECOMPILE_ADDRESSES;
  const { installRawBodyParser } = await import('../src/agent/verify.js');
  app = Fastify();
  installRawBodyParser(app);
  app.addHook('onRoute', (route) => {
    if (route.url.startsWith(`${prefix}/`) && route.method !== 'HEAD') {
      registrations.push(`${route.method} ${route.url}`);
    }
  });
  await app.register(agentPrecompileRoutes);
  await app.ready();
}, 120_000);

afterAll(async () => {
  await app?.close();
  await database?.closePool();
  await pg?.stop();
  if (proxy) {
    await new Promise<void>((resolve, reject) => {
      proxy.close((error) => (error ? reject(error) : resolve()));
      proxy.closeAllConnections();
    });
  }
  for (const [key, value] of originalEnvironment) {
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
}, 120_000);

async function provisionAgent(
  label: string
): Promise<{ key: AgentKey; token: string; address: Hex }> {
  const key = newAgentKey(label);
  const { upsertPrincipalOnVerify, upsertPolicy, setPrincipalFrozen } = await import(
    '../src/db/agents.js'
  );
  await upsertPrincipalOnVerify({
    did: key.did,
    label,
    keyFingerprint: key.publicKeyHex.slice(0, 16),
    publicKey: key.publicKeyHex,
  });
  await setPrincipalFrozen(key.did, false);
  await upsertPolicy(key.did, { mode: 'full' }, null);
  const { agentWalletUserId, provisionWalletForUser } = await import('../src/db/wallets.js');
  const { wallet } = await provisionWalletForUser(agentWalletUserId(key.did), 'agent');
  const { mintAgentToken } = await import('../src/auth/agentToken.js');
  const { token } = await mintAgentToken({ did: key.did, ownerUserId: null });
  return { key, token, address: wallet.address };
}

async function signedPost(key: AgentKey, name: string, payload: unknown) {
  const url = `${prefix}/${name}`;
  const body = JSON.stringify(payload);
  const nonce = randomBytes(16);
  const expiry = BigInt(Math.floor(Date.now() / 1000) + 60);
  const { agentRequestDigest } = await import('../src/agent/verify.js');
  const signature = key.sign(
    agentRequestDigest({
      method: `POST ${url}`,
      keyId: key.did,
      nonce,
      expiry,
      body: Buffer.from(body),
    })
  );
  return app.inject({
    method: 'POST',
    url,
    payload: body,
    headers: {
      'content-type': 'application/json',
      'x-agent-key': key.did,
      'x-agent-nonce': nonce.toString('hex'),
      'x-agent-expires': expiry.toString(),
      'x-agent-signature': signature,
    },
  });
}

function nodeMethod(name: string): AbiFunction {
  const method = nodeAbi.find(
    (item): item is AbiFunction => item.type === 'function' && item.name === name
  );
  if (!method) throw new Error(`scheduler node ABI has no ${name} method`);
  return method;
}

function builtWrite(route: (typeof writes)[number]) {
  switch (route.name) {
    case 'schedule':
      return precompiles.scheduler.schedule({
        target: route.body.target,
        callData: route.body.call_data,
        executeAtBlock: BigInt(route.body.execute_at_block),
        gasLimit: BigInt(route.body.gas_limit),
        depositWei: BigInt(route.body.deposit_wei),
      });
    case 'cancel':
      return precompiles.scheduler.cancel(BigInt(route.body.job_id));
    case 'reschedule':
      return precompiles.scheduler.reschedule(
        BigInt(route.body.job_id),
        BigInt(route.body.new_block)
      );
  }
}

function writeArguments(route: (typeof writes)[number]): readonly unknown[] {
  switch (route.name) {
    case 'schedule':
      return [
        getAddress(route.body.target),
        route.body.call_data,
        BigInt(route.body.execute_at_block),
        BigInt(route.body.gas_limit),
      ];
    case 'cancel':
      return [BigInt(route.body.job_id)];
    case 'reschedule':
      return [BigInt(route.body.job_id), BigInt(route.body.new_block)];
  }
}

function assertTransactionUsesBuiltCall(name: string): void {
  const source = ts.createSourceFile(
    'agentPrecompiles.ts',
    readFileSync(new URL('../src/routes/agentPrecompiles.ts', import.meta.url), 'utf8'),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TS
  );
  let handler: ts.ArrowFunction | undefined;
  function visit(node: ts.Node): void {
    if (
      ts.isCallExpression(node) &&
      node.arguments[0] &&
      ts.isStringLiteral(node.arguments[0]) &&
      node.arguments[0].text === `${prefix}/${name}`
    ) {
      const candidate = node.arguments[2];
      if (candidate && ts.isArrowFunction(candidate)) handler = candidate;
    }
    ts.forEachChild(node, visit);
  }
  visit(source);
  expect(handler).toBeDefined();
  if (!handler || !ts.isBlock(handler.body)) throw new Error(`missing ${name} route handler`);
  const variables = new Map<string, ts.Expression>();
  for (const statement of handler.body.statements) {
    if (!ts.isVariableStatement(statement)) continue;
    for (const declaration of statement.declarationList.declarations) {
      if (ts.isIdentifier(declaration.name) && declaration.initializer) {
        variables.set(declaration.name.text, declaration.initializer);
      }
    }
  }
  expect(variables.get('call')?.getText(source)).toMatch(new RegExp(`^scheduler\\.${name}\\(`));
  const result = variables.get('res');
  if (!result || !ts.isAwaitExpression(result) || !ts.isCallExpression(result.expression)) {
    throw new Error(`missing ${name} executor call`);
  }
  expect(result.expression.expression.getText(source)).toBe('executeAgentTransaction');
  const args = result.expression.arguments[0];
  if (!args || !ts.isObjectLiteralExpression(args)) throw new Error('missing executor arguments');
  const txProperty = args.properties.find((property) => property.name?.getText(source) === 'tx');
  let tx =
    txProperty && ts.isPropertyAssignment(txProperty)
      ? txProperty.initializer
      : variables.get('tx');
  if (tx && ts.isIdentifier(tx)) tx = variables.get(tx.text);
  if (!tx || !ts.isObjectLiteralExpression(tx)) throw new Error(`missing ${name} transaction`);
  for (const field of ['to', 'data']) {
    const property = tx.properties.find((item) => item.name?.getText(source) === field);
    expect(
      property && ts.isPropertyAssignment(property) && property.initializer.getText(source)
    ).toBe(`call.${field}`);
  }
  const intentProperty = args.properties.find(
    (property) => property.name?.getText(source) === 'intent'
  );
  let intent =
    intentProperty && ts.isPropertyAssignment(intentProperty)
      ? intentProperty.initializer
      : variables.get('intent');
  if (intent && ts.isIdentifier(intent)) intent = variables.get(intent.text);
  if (!intent || !ts.isObjectLiteralExpression(intent)) throw new Error(`missing ${name} intent`);
  for (const field of ['to', 'data']) {
    const property = intent.properties.find((item) => item.name?.getText(source) === field);
    expect(
      property && ts.isPropertyAssignment(property) && property.initializer.getText(source)
    ).toBe(`call.${field}`);
  }
}

async function schedulerRoutesTargetServedAddress(): Promise<void> {
  expect(PRECOMPILE_ADDRESSES.scheduler).toBe(servedAddress);
  expect(() => precompiles.servedAddress('scheduler')).not.toThrow(
    precompiles.PrecompileNotServedError
  );
  expect(precompiles.servedAddress('scheduler')).toBe(servedAddress);
  expect(registrations.sort()).toEqual(
    [
      `GET ${prefix}/job/:id`,
      `GET ${prefix}/pending`,
      ...writes.map(({ name }) => `POST ${prefix}/${name}`),
    ].sort()
  );
  const agent = await provisionAgent('scheduler-address');
  const { addRule } = await import('../src/db/agents.js');
  await addRule({ did: agent.key.did, effect: 'deny', subject: 'contract', value: servedAddress });
  for (const route of writes) {
    assertTransactionUsesBuiltCall(route.name);
    expect(builtWrite(route).to).toBe(PRECOMPILE_ADDRESSES.scheduler);
    const response = await signedPost(agent.key, route.name, route.body);
    expect.soft(response.statusCode, response.body).not.toBe(503);
    expect.soft(response.statusCode, response.body).toBe(403);
    expect.soft(response.json()).toMatchObject({
      error: 'CONTRACT_DENYLISTED',
      contract: servedAddress,
    });
    expect.soft(response.json().error).not.toBe('precompile_not_served');
  }
  for (const route of reads) {
    const start = rpcCalls.length;
    const response = await app.inject({
      method: 'GET',
      url: `${prefix}${route.path}`,
      headers: { authorization: `Bearer ${agent.token}` },
    });
    expect.soft(response.statusCode, response.body).toBe(route.status);
    expect.soft(response.statusCode, response.body).not.toBe(503);
    expect.soft(response.json().error).not.toBe('precompile_not_served');
    const calls = rpcCalls.slice(start).filter(
      (call) =>
        call.method === 'eth_call' &&
        (call.params[0] as { data?: Hex }).data !== '0xffffffff'
    );
    expect.soft(calls).toHaveLength(1);
    for (const call of calls) {
      expect.soft(call.params[0]).toMatchObject({ to: PRECOMPILE_ADDRESSES.scheduler });
      expect.soft(call.answer).toMatchObject({ result: expect.stringMatching(/^0x[0-9a-f]+$/iu) });
      expect.soft(call.answer).not.toHaveProperty('error');
    }
    if (route.name === 'pending') {
      expect.soft(response.json()).toEqual({ creator: agent.address, job_ids: [] });
    }
  }
  const probes = rpcCalls.filter(
    (call) =>
      call.method === 'eth_call' &&
      (call.params[0] as { to?: Hex }).to === servedAddress &&
      (call.params[0] as { data?: Hex }).data === '0xffffffff'
  );
  expect.soft(probes).toHaveLength(1);
  for (const probe of probes) {
    const answer = probe.answer as { result?: Hex; error?: { message?: string } };
    expect
      .soft(
        (typeof answer.result === 'string' && /^0x[0-9a-f]+$/iu.test(answer.result)) ||
          (typeof answer.error?.message === 'string' &&
            answer.error.message.includes('LXP_PRECOMPILE_UNKNOWN_METHOD'))
      )
      .toBe(true);
  }
  expect.soft(proxyErrors, 'a running node serving scheduler at 0x0905 is required').toEqual([]);
}

async function schedulerRoutesEncodeNodeAbiSelectors(): Promise<void> {
  const schedulerSource = repositorySource('precompiles/layerxagent/scheduler/scheduler.go');
  const mergeSource = repositorySource('precompiles/layerxagent/scheduler/merge.go');
  expect(schedulerSource).toMatch(new RegExp(`SchedulerAddress\\s*=\\s*"${servedAddress}"`));
  expect(schedulerSource).toMatch(/abi\.JSON\(strings\.NewReader\(abiJSON\)\)/u);
  expect(mergeSource).toMatch(
    /RegisterMergeConstructor\(common\.HexToAddress\(SchedulerAddress\), NewMergePrecompile\)/u
  );
  expect(mergeSource).toMatch(/NewPrecompile\(\*keeper, evm, evm\.UpgradeKeeper\(\), planName\)/u);
  expect(repositorySource('precompiles/setup.go')).toContain(
    `{ecommon.HexToAddress("${servedAddress}"), "scheduler"}`
  );
  expect(repositorySource('node/precompiles.go')).toContain(
    '_ "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/scheduler"'
  );
  const selectors = new Set(
    nodeAbi.filter((item): item is AbiFunction => item.type === 'function').map(toFunctionSelector)
  );
  expect(selectors.size).toBe(5);
  const agent = await provisionAgent('scheduler-selectors');
  const { addRule } = await import('../src/db/agents.js');
  for (const selector of selectors) {
    await addRule({ did: agent.key.did, effect: 'deny', subject: 'selector', value: selector });
  }
  const observed: string[] = [];
  for (const route of writes) {
    assertTransactionUsesBuiltCall(route.name);
    const call = builtWrite(route);
    const args = writeArguments(route);
    expect(call.to).toBe(servedAddress);
    expect(call.data).toBe(encodeFunctionData({ abi: nodeAbi, functionName: route.name, args }));
    expect(decodeFunctionData({ abi: nodeAbi, data: call.data })).toEqual({
      functionName: route.name,
      args,
    });
    expect(call.value).toBe(route.name === 'schedule' ? route.body.deposit_wei : undefined);
    const response = await signedPost(agent.key, route.name, route.body);
    expect.soft(response.statusCode, response.body).not.toBe(503);
    expect.soft(response.json().error).not.toBe('precompile_not_served');
    const selector = toFunctionSelector(nodeMethod(route.name));
    expect(call.data.slice(0, 10)).toBe(selector);
    expect.soft(response.statusCode, response.body).toBe(403);
    expect.soft(response.json()).toMatchObject({ error: 'SELECTOR_DENYLISTED', selector });
    expect.soft(selectors.has(response.json().selector)).toBe(true);
    observed.push(response.json().selector);
  }
  for (const route of reads) {
    const start = rpcCalls.length;
    const response = await app.inject({
      method: 'GET',
      url: `${prefix}${route.path}`,
      headers: { authorization: `Bearer ${agent.token}` },
    });
    expect.soft(response.statusCode, response.body).toBe(route.status);
    expect.soft(response.statusCode, response.body).not.toBe(503);
    expect.soft(response.json().error).not.toBe('precompile_not_served');
    const calls = rpcCalls.slice(start).filter((call) => call.method === 'eth_call');
    expect.soft(calls).toHaveLength(1);
    for (const call of calls) {
      const tx = call.params[0] as { to: Hex; data: Hex };
      const selector = tx.data.slice(0, 10);
      expect.soft(tx.to).toBe(servedAddress);
      expect.soft(selectors.has(selector)).toBe(true);
      expect.soft(selector).toBe(toFunctionSelector(nodeMethod(route.name)));
      expect.soft(tx.data).toBe(
        encodeFunctionData({
          abi: nodeAbi,
          functionName: route.name,
          args: route.name === 'getJob' ? [BigInt(missingJobId)] : [getAddress(agent.address)],
        })
      );
      const decoded = decodeFunctionData({ abi: nodeAbi, data: tx.data });
      expect.soft(decoded.functionName).toBe(route.name);
      expect
        .soft(decoded.args)
        .toEqual(route.name === 'getJob' ? [BigInt(missingJobId)] : [getAddress(agent.address)]);
      expect.soft(call.answer).not.toHaveProperty('error');
      expect.soft(call.answer).toMatchObject({ result: expect.stringMatching(/^0x[0-9a-f]+$/iu) });
      const answer = call.answer as { result?: Hex };
      if (answer.result) {
        const result = decodeFunctionResult({
          abi: nodeAbi,
          functionName: route.name,
          data: answer.result,
        });
        if (route.name === 'pending') expect.soft(result).toEqual([]);
        else {
          expect
            .soft(result)
            .toEqual([
              0n,
              '0x0000000000000000000000000000000000000000',
              '0x0000000000000000000000000000000000000000',
              '0x',
              0n,
              0n,
              0n,
              false,
            ]);
        }
      }
      observed.push(selector);
    }
  }
  expect.soft(new Set(observed)).toEqual(selectors);
  expect.soft(rpcCalls.filter((call) => call.method === 'eth_sendRawTransaction')).toEqual([]);
  expect.soft(proxyErrors, 'a running node serving scheduler at 0x0905 is required').toEqual([]);
}

test(schedulerRoutesTargetServedAddress, schedulerRoutesTargetServedAddress, 120_000);
test(schedulerRoutesEncodeNodeAbiSelectors, schedulerRoutesEncodeNodeAbiSelectors, 120_000);
