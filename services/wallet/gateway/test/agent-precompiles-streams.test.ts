import { randomBytes } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { createServer, type Server } from 'node:http';
import Fastify, { type FastifyInstance } from 'fastify';
import {
  decodeFunctionData,
  decodeFunctionResult,
  getAddress,
  parseTransaction,
  toFunctionSelector,
  type Abi,
  type Hex,
} from 'viem';
import { afterAll, expect, test } from 'vitest';
import { newAgentKey, type AgentKey } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const prefix = '/v1/agent/precompiles/streams/';
const servedAddress = '0x0000000000000000000000000000000000000906';
const payee = '0x00000000000000000000000000000000000000e1';
const token = '0x00000000000000000000000000000000000000e2';
const routes = [
  {
    method: 'POST',
    path: 'open',
    functionName: 'open',
    payload: { payee, token, rate_per_second: '7', start_time: '11', stop_time: '19', cap: '41' },
    args: [getAddress(payee), getAddress(token), 7n, 11n, 19n, 41n],
  },
  {
    method: 'POST',
    path: 'settle',
    functionName: 'settle',
    payload: { stream_id: '0' },
    args: [0n],
  },
  { method: 'POST', path: 'close', functionName: 'close', payload: { stream_id: '0' }, args: [0n] },
  {
    method: 'POST',
    path: 'update-rate',
    functionName: 'updateRate',
    payload: { stream_id: '0', new_rate: '13' },
    args: [0n, 13n],
  },
  { method: 'GET', path: 'stream/:id', functionName: 'getStream', args: [0n] },
  { method: 'GET', path: 'accrued/:id', functionName: 'accrued', args: [0n] },
] as const;

interface RpcRequest {
  method: string;
  params: unknown[];
}

interface TransactionCall {
  to: string;
  data: Hex;
  value?: Hex;
}

interface RpcExchange {
  request: RpcRequest;
  status: number;
  response: {
    result?: Hex;
    error?: { code: number; message: string };
  };
}

const rpcRequests: RpcRequest[] = [];
const rpcExchanges: RpcExchange[] = [];
const registeredRoutes: string[] = [];
const forwardingFailures: string[] = [];
const originalEnvironment = {
  HYPERPAXEER_RPC_URL: process.env.HYPERPAXEER_RPC_URL,
  RPC_URLS: process.env.RPC_URLS,
  SEQUENCER_RPC_URL: process.env.SEQUENCER_RPC_URL,
  HYPERPAXEER_CHAIN_ID: process.env.HYPERPAXEER_CHAIN_ID,
  AGENT_JWT_SECRET: process.env.AGENT_JWT_SECRET,
  DATABASE_URL: process.env.DATABASE_URL,
};
let fixture: Promise<void> | undefined;
let app: FastifyInstance;
let proxy: Server;
let postgres: EphemeralPostgres;
let database: typeof import('../src/db/pool.js');
let rpc: typeof import('../src/rpc/pool.js');
let verifier: typeof import('../src/agent/verify.js');
let precompiles: typeof import('../src/precompiles.js');
let agent: AgentKey;
let bearer: string;
let walletAddress: Hex;

function repositoryFile(path: string): string {
  return readFileSync(new URL(`../../../../${path}`, import.meta.url), 'utf8');
}

function nodeStreamsAbi(): Abi {
  const source = repositoryFile('precompiles/layerxagent/streams/streams.go');
  const registration = repositoryFile('precompiles/layerxagent/streams/merge.go');
  expect(registration).toContain(
    'precompiles.RegisterMergeConstructor(common.HexToAddress(StreamsAddress), NewMergePrecompile)'
  );
  expect(registration).toContain(
    'NewPrecompile(wired.LayerXPayK(), evmKeeper.UpgradeKeeper(), planName)'
  );
  expect(source).toMatch(/StreamsAddress\s*=\s*"0x0000000000000000000000000000000000000906"/);
  expect(source).toContain('abi.JSON(strings.NewReader(abiJSON))');
  expect(source).toContain('agentcommon.NewPrecompile(a,');
  expect(source).toContain('common.HexToAddress(StreamsAddress)');
  expect(repositoryFile('node/precompiles.go')).toContain(
    '_ "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/streams"'
  );
  expect(repositoryFile('precompiles/setup.go')).toContain(
    `{ecommon.HexToAddress("${servedAddress}"), "streams"}`
  );
  const abi = source.match(/\babiJSON\s*=\s*`([\s\S]*?)`/);
  expect(abi, 'the registered constructor must use the repository streams ABI').not.toBeNull();
  return JSON.parse(abi![1]!) as Abi;
}

async function startStreamsFixture(): Promise<void> {
  const upstream = new URL(process.env.HYPERPAXEER_RPC_URL!);
  if (upstream.port === '0') {
    throw new Error(
      'HYPERPAXEER_RPC_URL must name a disposable Paxeer node serving streams at 0x0906; test/setup.ts supplies only localhost:0, not a node fixture'
    );
  }
  proxy = createServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = Buffer.concat(chunks);
      const request = JSON.parse(body.toString('utf8')) as RpcRequest;
      rpcRequests.push(request);
      const response = await fetch(upstream, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body,
        signal: AbortSignal.timeout(10_000),
      });
      const responseBody = Buffer.from(await response.arrayBuffer());
      rpcExchanges.push({
        request,
        status: response.status,
        response: JSON.parse(responseBody.toString('utf8')) as RpcExchange['response'],
      });
      res.writeHead(response.status, { 'content-type': 'application/json' });
      res.end(responseBody);
    } catch (error) {
      forwardingFailures.push(error instanceof Error ? error.message : String(error));
      res.destroy(error instanceof Error ? error : new Error(String(error)));
    }
  });
  await new Promise<void>((resolve, reject) => {
    proxy.once('error', reject);
    proxy.listen(0, '127.0.0.1', resolve);
  });
  const address = proxy.address();
  if (!address || typeof address === 'string') throw new Error('RPC observer did not bind');
  const rpcUrl = `http://127.0.0.1:${address.port}`;
  process.env.HYPERPAXEER_RPC_URL = rpcUrl;
  process.env.RPC_URLS = rpcUrl;
  process.env.SEQUENCER_RPC_URL = rpcUrl;
  process.env.HYPERPAXEER_CHAIN_ID = '125';
  process.env.AGENT_JWT_SECRET = randomBytes(32).toString('hex');
  postgres = await startPostgres();
  database = await import('../src/db/pool.js');
  rpc = await import('../src/rpc/pool.js');
  verifier = await import('../src/agent/verify.js');
  precompiles = await import('../src/precompiles.js');
  const routeModule = await import('../src/routes/agentPrecompiles.js');
  const agentPrecompileRoutes = routeModule.agentPrecompileRoutes;
  app = Fastify();
  verifier.installRawBodyParser(app);
  app.addHook('onRoute', (route) => {
    if (!route.url.startsWith(prefix)) return;
    const methods = Array.isArray(route.method) ? route.method : [route.method];
    for (const method of methods) {
      if (method !== 'HEAD') registeredRoutes.push(`${method} ${route.url}`);
    }
  });
  await app.register(agentPrecompileRoutes);
  await app.ready();

  agent = newAgentKey('streams-conformance');
  const agents = await import('../src/db/agents.js');
  const wallets = await import('../src/db/wallets.js');
  await agents.upsertPrincipalOnVerify({
    did: agent.did,
    label: 'streams conformance',
    keyFingerprint: agent.publicKeyHex.slice(0, 16),
    publicKey: agent.publicKeyHex,
  });
  await agents.setPrincipalFrozen(agent.did, false);
  await database.query(
    `update agent_policies set mode = 'full', withdrawal_allowlist_only = false where did = $1`,
    [agent.did]
  );
  const { wallet } = await wallets.provisionWalletForUser(
    wallets.agentWalletUserId(agent.did),
    'agent'
  );
  walletAddress = wallet.address;
  await agents.setPrincipalWallet(agent.did, wallet.id);
  const { mintAgentToken } = await import('../src/auth/agentToken.js');
  bearer = (await mintAgentToken({ did: agent.did, ownerUserId: null })).token;
}

afterAll(async () => {
  await app?.close();
  rpc?.sharedRpcPool().stop();
  await database?.closePool();
  await postgres?.stop();
  if (proxy) {
    proxy.closeAllConnections();
    await new Promise<void>((resolve, reject) => {
      proxy.close((error) => (error ? reject(error) : resolve()));
    });
  }
  for (const [key, value] of Object.entries(originalEnvironment)) {
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
}, 120_000);

async function driveStreamsRoutes(): Promise<{ functionName: string; call: TransactionCall }[]> {
  expect(registeredRoutes.sort()).toEqual(
    routes.map((route) => `${route.method} ${prefix}${route.path}`).sort()
  );
  const calls: { functionName: string; call: TransactionCall }[] = [];
  for (const route of routes) {
    const url = `${prefix}${route.path.replace(':id', '0')}`;
    const offset = rpcRequests.length;
    let response;
    if (route.method === 'POST') {
      const body = JSON.stringify(route.payload);
      const nonce = randomBytes(16);
      const expiry = BigInt(Math.floor(Date.now() / 1000) + 60);
      const digest = verifier.agentRequestDigest({
        method: `POST ${url}`,
        keyId: agent.did,
        nonce,
        expiry,
        body: Buffer.from(body),
      });
      response = await app.inject({
        method: 'POST',
        url,
        payload: body,
        headers: {
          'content-type': 'application/json',
          'x-agent-key': agent.did,
          'x-agent-nonce': nonce.toString('hex'),
          'x-agent-expires': expiry.toString(),
          'x-agent-signature': agent.sign(digest),
        },
      });
    } else {
      response = await app.inject({
        method: 'GET',
        url,
        headers: { authorization: `Bearer ${bearer}` },
      });
    }
    expect(response.statusCode, `${url}: ${response.body}`).not.toBe(503);
    expect(response.json().error, url).not.toBe('precompile_not_served');
    expect(forwardingFailures, `${url} must reach the serving node`).toEqual([]);
    const requests = rpcRequests.slice(offset);
    const method = route.method === 'POST' ? 'eth_estimateGas' : 'eth_call';
    const encoded = requests.filter(
      (request) =>
        request.method === method &&
        !(
          request.method === 'eth_call' &&
          (request.params[0] as TransactionCall).to === precompiles.PRECOMPILE_ADDRESSES.streams &&
          (request.params[0] as TransactionCall).data === '0xffffffff'
        )
    );
    expect(encoded, `${url} must reach ${method}`).toHaveLength(1);
    const call = encoded[0]!.params[0] as TransactionCall;
    expect(call.to.toLowerCase(), url).toBe(servedAddress);
    expect(call.data, url).toMatch(/^0x[0-9a-f]{8}(?:[0-9a-f]{64})+$/);
    if (route.method === 'GET') {
      const exchanges = rpcExchanges.filter((exchange) => exchange.request === encoded[0]);
      expect(exchanges, `${url} must receive a node RPC response`).toHaveLength(1);
      const exchange = exchanges[0]!;
      expect(exchange.status, url).toBe(200);
      expect(exchange.response.error, `${url} must execute the served method`).toBeUndefined();
      expect(exchange.response.result, url).toMatch(/^0x[0-9a-f]+$/);
      const result = decodeFunctionResult({
        abi: nodeStreamsAbi(),
        functionName: route.functionName,
        data: exchange.response.result!,
      });
      expect(result, `${url} must decode the node ABI result`).toEqual(
        route.functionName === 'accrued'
          ? 0n
          : {
              id: 0n,
              payer: '0x0000000000000000000000000000000000000000',
              payee: '0x0000000000000000000000000000000000000000',
              token: '0x0000000000000000000000000000000000000000',
              ratePerSecond: 0n,
              cap: 0n,
              startTime: 0n,
              stopTime: 0n,
              settled: 0n,
              active: false,
            }
      );
    }
    if (route.method === 'POST') {
      expect(response.statusCode, response.body).toBe(502);
      expect(response.json().error).toBe('send_failed');
      expect(BigInt(call.value ?? '0x0')).toBe(0n);
      for (const request of requests.filter((entry) => entry.method === 'eth_sendRawTransaction')) {
        const transaction = parseTransaction(request.params[0] as Hex);
        expect(transaction.to?.toLowerCase()).toBe(servedAddress);
        expect(transaction.data).toBe(call.data);
        expect(transaction.value ?? 0n).toBe(0n);
      }
    } else if (route.functionName === 'getStream') {
      expect(response.statusCode, response.body).toBe(404);
      expect(response.json()).toEqual({ error: 'not_found' });
    } else {
      expect(response.statusCode, response.body).toBe(200);
      expect(response.json()).toEqual({ stream_id: '0', accrued: '0' });
    }
    calls.push({ functionName: route.functionName, call });
  }
  return calls;
}

async function streamsRoutesTargetServedAddress() {
  await (fixture ??= startStreamsFixture());
  const PRECOMPILE_ADDRESSES = precompiles.PRECOMPILE_ADDRESSES;
  expect(PRECOMPILE_ADDRESSES.streams).toBe(servedAddress);
  expect(() => precompiles.servedAddress('streams')).not.toThrow(
    precompiles.PrecompileNotServedError
  );
  await expect(precompiles.ensureServed('streams')).resolves.toBeUndefined();
  const probes = rpcRequests.filter(
    (request) =>
      request.method === 'eth_call' &&
      (request.params[0] as TransactionCall).to === PRECOMPILE_ADDRESSES.streams &&
      (request.params[0] as TransactionCall).data === '0xffffffff'
  );
  expect(probes).toHaveLength(1);
  expect(probes[0]!.params).toEqual([
    { to: PRECOMPILE_ADDRESSES.streams, data: '0xffffffff' },
    'latest',
  ]);
  expect(
    await (await import('../src/chainReads.js'))
      .publicClient()
      .getBalance({ address: walletAddress })
  ).toBe(0n);
  const calls = await driveStreamsRoutes();
  expect(calls).toHaveLength(6);
  for (const { call } of calls) expect(call.to.toLowerCase()).toBe(PRECOMPILE_ADDRESSES.streams);
}

async function streamsRoutesEncodeNodeAbiSelectors() {
  const abi = nodeStreamsAbi();
  const methods = abi.filter((item) => item.type === 'function');
  expect(methods.map((method) => method.name).sort()).toEqual(
    routes.map((route) => route.functionName).sort()
  );
  await (fixture ??= startStreamsFixture());
  const calls = await driveStreamsRoutes();
  expect(calls).toHaveLength(6);
  for (const { functionName, call } of calls) {
    const method = methods.find((entry) => entry.name === functionName);
    expect(method, functionName).toBeDefined();
    expect(call.data.slice(0, 10), functionName).toBe(toFunctionSelector(method!));
    expect(decodeFunctionData({ abi, data: call.data }), functionName).toEqual({
      functionName,
      args: routes.find((route) => route.functionName === functionName)!.args,
    });
  }
  expect(new Set(calls.map(({ call }) => call.data.slice(0, 10)))).toEqual(
    new Set(methods.map((method) => toFunctionSelector(method)))
  );
}

test(streamsRoutesTargetServedAddress, streamsRoutesTargetServedAddress, 120_000);
test(streamsRoutesEncodeNodeAbiSelectors, streamsRoutesEncodeNodeAbiSelectors, 120_000);
