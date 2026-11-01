import { randomBytes } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { createServer, type Server } from 'node:http';
import Fastify, { type FastifyInstance } from 'fastify';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { decodeFunctionData, toFunctionSelector, type Abi, type Hex } from 'viem';
import { newAgentKey } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const teeAddress = '0x0000000000000000000000000000000000000907';
const expectedReportData = `0x${'42'.repeat(32)}` as Hex;
const rootIndex = 1n << 255n;
const cases = [
  {
    path: 'verify',
    method: 'verify',
    payload: { family: 'intel_tdx', quote: '0x0102' },
    args: [0, '0x0102'],
    status: 422,
  },
  {
    path: 'verify',
    method: 'verifyAndExpect',
    payload: { family: 1, quote: '0x0102', expected_report_data: expectedReportData },
    args: [1, '0x0102', expectedReportData],
    status: 422,
  },
  {
    path: 'root-count',
    method: 'rootCount',
    payload: { family: 'nvidia_h100' },
    args: [2],
    status: 200,
  },
  {
    path: 'root-of',
    method: 'rootOf',
    payload: { family: 'intel_sgx', index: rootIndex.toString() },
    args: [3, rootIndex],
    status: 200,
  },
] as const;

interface RpcRequest {
  method: string;
  params: unknown[];
}

let app: FastifyInstance;
let proxy: Server;
let postgres: EphemeralPostgres;
let pool: typeof import('../src/db/pool.js');
let precompiles: typeof import('../src/precompiles.js');
let authorization: string;
const requests: RpcRequest[] = [];
const forwardingFailures: string[] = [];
const registeredRoutes = new Set<string>();
const originalRpcUrl = process.env.HYPERPAXEER_RPC_URL!;
const originalJwtSecret = process.env.AGENT_JWT_SECRET;

function repositoryFile(path: string): string {
  return readFileSync(new URL(`../../../../${path}`, import.meta.url), 'utf8');
}

function registeredNodeAbi(): Abi {
  const source = repositoryFile('precompiles/layerxagent/teeattestor/teeattestor.go');
  expect(source).toContain(`TeeAttestorAddress            = "${teeAddress}"`);
  expect(source).toMatch(
    /RegisterMergeConstructor\(ethcommon\.HexToAddress\(TeeAttestorAddress\), NewMergePrecompile\)/,
  );
  expect(source).toContain(
    'NewPrecompile(wired.XWebK(), evmKeeper, evmKeeper.UpgradeKeeper(), planName)',
  );
  expect(source).toContain('abi.JSON(strings.NewReader(abiJSON))');
  expect(repositoryFile('precompiles/setup.go')).toContain(
    `{ecommon.HexToAddress("${teeAddress}"), "tee attestor"}`,
  );
  expect(repositoryFile('node/precompiles.go')).toMatch(
    /_ "[^"\n]+\/precompiles\/layerxagent\/teeattestor"/,
  );
  const embeddedAbi = source.match(/const abiJSON = `([\s\S]*?)`/);
  expect(embeddedAbi).not.toBeNull();
  return JSON.parse(embeddedAbi![1]!) as Abi;
}

beforeAll(async () => {
  proxy = createServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = Buffer.concat(chunks).toString('utf8');
      const rpc = JSON.parse(body) as RpcRequest;
      requests.push(rpc);
      const upstream = await fetch(originalRpcUrl, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body,
        signal: AbortSignal.timeout(5_000),
      });
      res.writeHead(upstream.status, {
        'content-type': upstream.headers.get('content-type') ?? 'application/octet-stream',
      });
      res.end(Buffer.from(await upstream.arrayBuffer()));
    } catch (err) {
      forwardingFailures.push((err as Error).message);
      res.writeHead(502, { 'content-type': 'text/plain' });
      res.end(`Node RPC forwarding failed: ${(err as Error).message}`);
    }
  });
  await new Promise<void>((resolve, reject) => {
    proxy.once('error', reject);
    proxy.listen(0, '127.0.0.1', resolve);
  });
  const address = proxy.address();
  if (!address || typeof address === 'string') throw new Error('RPC proxy did not bind');
  process.env.HYPERPAXEER_RPC_URL = `http://127.0.0.1:${address.port}`;
  process.env.AGENT_JWT_SECRET = randomBytes(32).toString('hex');
  postgres = await startPostgres();
  pool = await import('../src/db/pool.js');
  precompiles = await import('../src/precompiles.js');
  const agent = newAgentKey('tee-conformance');
  const { upsertPrincipalOnVerify } = await import('../src/db/agents.js');
  await upsertPrincipalOnVerify({
    did: agent.did,
    label: 'TEE conformance',
    keyFingerprint: agent.did,
    publicKey: agent.publicKeyHex,
  });
  const { mintAgentToken } = await import('../src/auth/agentToken.js');
  authorization = `Bearer ${(await mintAgentToken({ did: agent.did, ownerUserId: null })).token}`;
  app = Fastify();
  app.addHook('onRoute', (route) => {
    if (route.url.startsWith('/v1/agent/precompiles/tee/')) {
      for (const method of [route.method].flat()) registeredRoutes.add(`${method} ${route.url}`);
    }
  });
  const agentPrecompileRoutes = (await import('../src/routes/agentPrecompiles.js'))
    .agentPrecompileRoutes;
  await app.register(agentPrecompileRoutes);
  await app.ready();
}, 60_000);

afterAll(async () => {
  await app?.close();
  await pool?.closePool();
  await postgres?.stop();
  if (proxy) {
    proxy.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      proxy.close((err) => (err ? reject(err) : resolve())),
    );
  }
  process.env.HYPERPAXEER_RPC_URL = originalRpcUrl;
  if (originalJwtSecret === undefined) delete process.env.AGENT_JWT_SECRET;
  else process.env.AGENT_JWT_SECRET = originalJwtSecret;
}, 60_000);

async function driveRoute(example: (typeof cases)[number]) {
  const start = requests.length;
  const response = await app.inject({
    method: 'POST',
    url: `/v1/agent/precompiles/tee/${example.path}`,
    headers: { authorization },
    payload: example.payload,
  });
  return { response, outgoing: requests.slice(start) };
}

async function teeRoutesTargetServedAddress() {
  const PRECOMPILE_ADDRESSES = precompiles.PRECOMPILE_ADDRESSES;
  expect(PRECOMPILE_ADDRESSES.teeAttestor).toBe(teeAddress);
  expect(() => precompiles.servedAddress('teeAttestor')).not.toThrow(
    precompiles.PrecompileNotServedError,
  );
  expect(precompiles.servedAddress('teeAttestor')).toBe(teeAddress);
  expect([...registeredRoutes].sort()).toEqual(
    [...new Set(cases.map((example) => `POST /v1/agent/precompiles/tee/${example.path}`))].sort(),
  );
  for (const example of cases) {
    const { response, outgoing } = await driveRoute(example);
    expect.soft(response.statusCode, example.method).not.toBe(503);
    expect.soft(response.json().error, example.method).not.toBe('precompile_not_served');
    expect.soft(response.statusCode, example.method).toBe(example.status);
    const calls = outgoing.filter((rpc) => rpc.method === 'eth_call');
    expect
      .soft(
        calls.length,
        `${example.method} must issue eth_call to the serving node; RPC forwarding errors: ${forwardingFailures.join('; ')}`,
      )
      .toBeGreaterThan(0);
    for (const rpc of outgoing) {
      expect.soft(['eth_call', 'eth_getCode'], example.method).toContain(rpc.method);
      const target =
        rpc.method === 'eth_getCode' ? rpc.params[0] : (rpc.params[0] as { to: string }).to;
      expect.soft(String(target).toLowerCase(), example.method).toBe(teeAddress);
    }
    const body = response.json();
    if (example.path === 'verify') {
      expect.soft(body.error, example.method).toBe('attestation_failed');
      expect.soft(body.detail, example.method).toContain('LXP_AGENT_TEE_INVALID_QUOTE');
    } else if (example.path === 'root-count') {
      expect.soft(body.family).toBe(2);
      expect.soft(body.count).toMatch(/^\d+$/);
    } else {
      expect.soft(body).toEqual({ family: 3, index: rootIndex.toString(), root: '0x' });
    }
  }
}

async function teeRoutesEncodeNodeAbiSelectors() {
  const abi = registeredNodeAbi();
  const methods = abi.filter((item) => item.type === 'function');
  expect(methods.map((method) => method.name).sort()).toEqual(
    cases.map((example) => example.method).sort(),
  );
  const selectors = new Set(methods.map((method) => toFunctionSelector(method)));
  const observed = new Set<string>();
  for (const example of cases) {
    const { response, outgoing } = await driveRoute(example);
    expect.soft(response.statusCode, example.method).toBe(example.status);
    const calls = outgoing.filter((rpc) => rpc.method === 'eth_call');
    expect.soft(calls.length, `${example.method} must encode a node ABI call`).toBeGreaterThan(0);
    for (const rpc of calls) {
      const call = rpc.params[0] as { to: string; data: Hex };
      expect(call.to.toLowerCase()).toBe(teeAddress);
      expect(selectors.has(call.data.slice(0, 10)), example.method).toBe(true);
      const decoded = decodeFunctionData({ abi, data: call.data });
      expect(decoded.functionName).toBe(example.method);
      expect(decoded.args).toEqual(example.args);
      observed.add(call.data.slice(0, 10));
    }
  }
  expect(observed).toEqual(selectors);
}

test(teeRoutesTargetServedAddress, 60_000);
test(teeRoutesEncodeNodeAbiSelectors, 60_000);
