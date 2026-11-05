import Fastify, { type FastifyInstance } from 'fastify';
import { randomBytes } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { createServer, type Server } from 'node:http';
import {
  concatHex,
  decodeFunctionData,
  encodeAbiParameters,
  keccak256,
  stringToHex,
  toFunctionSelector,
  type Abi,
  type Hex,
} from 'viem';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const upstreamUrl = process.env.EIP712_NODE_RPC_URL ?? process.env.HYPERPAXEER_RPC_URL;
const originalEnvironment = {
  HYPERPAXEER_RPC_URL: process.env.HYPERPAXEER_RPC_URL,
  RPC_URLS: process.env.RPC_URLS,
  AGENT_JWT_SECRET: process.env.AGENT_JWT_SECRET,
  DATABASE_URL: process.env.DATABASE_URL,
};
const address = '0x0000000000000000000000000000000000000908';
const prefix = '/v1/agent/precompiles/eip712/';
const domain = {
  name: 'Gateway EIP712 conformance',
  version: '1',
  chainId: 125n,
  verifyingContract: '0x0000000000000000000000000000000000000123',
} as const;
const separator = keccak256(
  encodeAbiParameters(
    [
      { type: 'bytes32' },
      { type: 'bytes32' },
      { type: 'bytes32' },
      { type: 'uint256' },
      { type: 'address' },
    ],
    [
      keccak256(
        stringToHex('EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)'),
      ),
      keccak256(stringToHex(domain.name)),
      keccak256(stringToHex(domain.version)),
      domain.chainId,
      domain.verifyingContract,
    ],
  ),
);
const structHash = keccak256(stringToHex('agent route conformance'));
const digest = keccak256(concatHex(['0x1901', separator, structHash]));
const signer = privateKeyToAccount(generatePrivateKey());

interface RpcRequest {
  method: string;
  params: unknown[];
}

interface RouteCase {
  path: string;
  functionName: string;
  payload: Record<string, string>;
  args: readonly unknown[];
  response: Record<string, string>;
}

const requests: RpcRequest[] = [];
const registeredRoutes: string[] = [];
const forwardingErrors: Error[] = [];
let app: FastifyInstance;
let proxy: Server;
let postgres: EphemeralPostgres;
let database: typeof import('../src/db/pool.js');
let precompiles: typeof import('../src/precompiles.js');
let authorization: string;
let cases: RouteCase[];

function readRepositoryFile(path: string): string {
  return readFileSync(new URL(`../../../../${path}`, import.meta.url), 'utf8');
}

function registeredNodeAbi(): Abi {
  const source = readRepositoryFile('precompiles/layerxagent/eip712/eip712.go');
  const merge = readRepositoryFile('precompiles/layerxagent/eip712/merge.go');
  const setup = readRepositoryFile('precompiles/setup.go');
  expect(source).toMatch(new RegExp(`\\bEip712Address\\s*=\\s*"${address}"`));
  expect(merge).toMatch(
    /func init\(\)\s*\{\s*if err := precompiles\.RegisterMergeConstructor\(common\.HexToAddress\(Eip712Address\), NewMergePrecompile\)/,
  );
  expect(merge).toMatch(/p, err := NewPrecompile\(evmKeeper\.UpgradeKeeper\(\), planName,/);
  expect(source).toMatch(/abi\.JSON\(strings\.NewReader\(abiJSON\)\)/);
  expect(source).toMatch(
    /agentcommon\.NewPrecompile\(a, p, common\.HexToAddress\(Eip712Address\), PrecompileName, upgrade, planName\)/,
  );
  expect(setup).toContain(`{ecommon.HexToAddress("${address}"), "eip712"}`);
  expect(readRepositoryFile('node/precompiles.go')).toMatch(
    /_ "[^"\n]+\/precompiles\/layerxagent\/eip712"/,
  );
  const match = /\babiJSON\s*=\s*`([\s\S]*?)`/.exec(source);
  if (!match?.[1]) throw new Error('Registered EIP712 precompile has no embedded ABI');
  return JSON.parse(match[1]) as Abi;
}

beforeAll(async () => {
  proxy = createServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = Buffer.concat(chunks).toString('utf8');
      const request = JSON.parse(body) as RpcRequest;
      requests.push(request);
      if (!upstreamUrl) throw new Error('EIP712 node RPC URL is required');
      const upstream = await fetch(upstreamUrl, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body,
        signal: AbortSignal.timeout(5_000),
      });
      res.writeHead(upstream.status, { 'content-type': 'application/json' });
      res.end(await upstream.text());
    } catch (error) {
      const failure = error instanceof Error ? error : new Error(String(error));
      forwardingErrors.push(failure);
      res.destroy(failure);
    }
  });
  await new Promise<void>((resolve, reject) => {
    proxy.once('error', reject);
    proxy.listen(0, '127.0.0.1', resolve);
  });
  const listener = proxy.address();
  if (!listener || typeof listener === 'string') throw new Error('RPC proxy did not bind');
  const proxyUrl = `http://127.0.0.1:${listener.port}`;
  process.env.HYPERPAXEER_RPC_URL = proxyUrl;
  process.env.RPC_URLS = proxyUrl;
  process.env.AGENT_JWT_SECRET = randomBytes(48).toString('base64url');
  postgres = await startPostgres();
  database = await import('../src/db/pool.js');
  precompiles = await import('../src/precompiles.js');
  const PRECOMPILE_ADDRESSES = precompiles.PRECOMPILE_ADDRESSES;
  const agentPrecompileRoutes = (await import('../src/routes/agentPrecompiles.js'))
    .agentPrecompileRoutes;
  expect(PRECOMPILE_ADDRESSES.eip712).toBe(address);

  const publicKey = randomBytes(32).toString('hex');
  const did = `did:matrix:eip712:${publicKey}`;
  const { upsertPrincipalOnVerify } = await import('../src/db/agents.js');
  await upsertPrincipalOnVerify({
    did,
    label: 'EIP712 route conformance',
    keyFingerprint: publicKey.slice(0, 16),
    publicKey,
  });
  const { mintAgentToken } = await import('../src/auth/agentToken.js');
  authorization = `Bearer ${(await mintAgentToken({ did, ownerUserId: null })).token}`;
  const signature = await signer.sign({ hash: digest });
  cases = [
    {
      path: 'domain-separator',
      functionName: 'domainSeparator',
      payload: {
        name: domain.name,
        version: domain.version,
        chain_id: domain.chainId.toString(),
        verifying_contract: domain.verifyingContract,
      },
      args: [domain.name, domain.version, domain.chainId, domain.verifyingContract],
      response: { separator },
    },
    {
      path: 'hash-typed-data',
      functionName: 'hashTypedData',
      payload: { domain_separator: separator, struct_hash: structHash },
      args: [separator, structHash],
      response: { digest },
    },
    {
      path: 'recover',
      functionName: 'recoverTypedSigner',
      payload: { domain_separator: separator, struct_hash: structHash, signature },
      args: [separator, structHash, signature],
      response: { signer: signer.address },
    },
  ];
  app = Fastify();
  app.addHook('onRoute', (route) => {
    if (route.url.startsWith(prefix)) {
      for (const method of [route.method].flat()) {
        registeredRoutes.push(`${method} ${route.url}`);
      }
    }
  });
  await app.register(agentPrecompileRoutes);
  await app.ready();
}, 120_000);

afterAll(async () => {
  try {
    await app?.close();
    await database?.closePool();
    await postgres?.stop();
    if (proxy) {
      await new Promise<void>((resolve, reject) => {
        proxy.close((error) => (error ? reject(error) : resolve()));
        proxy.closeAllConnections();
      });
    }
  } finally {
    for (const [key, value] of Object.entries(originalEnvironment)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  }
}, 120_000);

async function exerciseEip712Routes(): Promise<{ route: RouteCase; call: { to: string; data: Hex } }[]> {
  expect(
    upstreamUrl && new URL(upstreamUrl).port !== '0',
    'Set EIP712_NODE_RPC_URL or HYPERPAXEER_RPC_URL to a real node serving the merge precompile at 0x0908',
  ).toBe(true);
  expect(registeredRoutes.slice().sort()).toEqual(
    cases.map((route) => `POST ${prefix}${route.path}`).sort(),
  );
  const observed: { route: RouteCase; call: { to: string; data: Hex } }[] = [];
  for (const route of cases) {
    const start = requests.length;
    const response = await app.inject({
      method: 'POST',
      url: `${prefix}${route.path}`,
      headers: { authorization },
      payload: route.payload,
    });
    expect(forwardingErrors, route.path).toEqual([]);
    expect(response.statusCode, `${route.path}: ${response.body}`).not.toBe(503);
    expect(response.json(), route.path).not.toHaveProperty('error', 'precompile_not_served');
    expect(response.statusCode, `${route.path}: ${response.body}`).toBe(200);
    const expected = Object.fromEntries(
      Object.entries(route.response).map(([key, value]) => [key, value.toLowerCase()]),
    );
    const actual = Object.fromEntries(
      Object.entries(response.json<Record<string, string>>()).map(([key, value]) => [
        key,
        value.toLowerCase(),
      ]),
    );
    expect(actual, route.path).toEqual(expected);
    const emitted = requests.slice(start);
    expect(emitted.every((request) => request.method === 'eth_call')).toBe(true);
    const calls = emitted.filter(
      (request) =>
        request.method === 'eth_call' &&
        (request.params[0] as { data: Hex }).data !== '0xffffffff',
    );
    expect(calls, route.path).toHaveLength(1);
    expect(calls[0]!.params, route.path).toHaveLength(2);
    expect(calls[0]!.params[1], route.path).toBe('latest');
    const call = calls[0]!.params[0] as { to: string; data: Hex };
    expect(call.to, route.path).toBe(address);
    expect(call.data, route.path).toMatch(/^0x[0-9a-f]{8,}$/);
    observed.push({ route, call });
  }
  return observed;
}

async function eip712RoutesTargetServedAddress(): Promise<void> {
  registeredNodeAbi();
  expect(precompiles.PRECOMPILE_ADDRESSES.eip712).toBe(address);
  expect(precompiles.UNSERVED_PRECOMPILES.has('eip712')).toBe(false);
  expect(() => precompiles.servedAddress('eip712')).not.toThrow(precompiles.PrecompileNotServedError);
  expect(precompiles.servedAddress('eip712')).toBe(address);
  const observed = await exerciseEip712Routes();
  expect(observed).toHaveLength(3);
  await expect(precompiles.ensureServed('eip712')).resolves.toBeUndefined();
  expect(
    requests.filter(
      (request) =>
        request.method === 'eth_call' &&
        (request.params[0] as { to: string; data: Hex }).to === address &&
        (request.params[0] as { to: string; data: Hex }).data === '0xffffffff',
    ),
  ).toHaveLength(1);
}

async function eip712RoutesEncodeNodeAbiSelectors(): Promise<void> {
  const abi = registeredNodeAbi();
  const methods = abi.filter((entry) => entry.type === 'function');
  expect(methods).toHaveLength(3);
  const selectors = new Set(methods.map((method) => toFunctionSelector(method)));
  expect(selectors.size).toBe(3);
  const observed = await exerciseEip712Routes();
  for (const { route, call } of observed) {
    const method = methods.find((entry) => entry.name === route.functionName);
    expect(method, route.path).toBeDefined();
    if (!method) throw new Error(`Node ABI is missing ${route.functionName}`);
    expect(['pure', 'view'], route.path).toContain(method.stateMutability);
    expect(selectors.has(call.data.slice(0, 10) as Hex), route.path).toBe(true);
    expect(call.data.slice(0, 10), route.path).toBe(toFunctionSelector(method));
    const decoded = decodeFunctionData({ abi, data: call.data });
    expect(decoded.functionName, route.path).toBe(route.functionName);
    expect(decoded.args, route.path).toEqual(route.args);
  }
  expect(new Set(observed.map(({ call }) => call.data.slice(0, 10)))).toEqual(selectors);
}

test(eip712RoutesTargetServedAddress, { timeout: 60_000 }, eip712RoutesTargetServedAddress);
test(eip712RoutesEncodeNodeAbiSelectors, { timeout: 60_000 }, eip712RoutesEncodeNodeAbiSelectors);
