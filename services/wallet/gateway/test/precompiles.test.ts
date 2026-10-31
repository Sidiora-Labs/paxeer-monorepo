import { readFileSync, readdirSync } from 'node:fs';
import { createServer } from 'node:http';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it, test, vi } from 'vitest';
import { toFunctionSelector, toFunctionSignature, type Abi } from 'viem';
import {
  EIP712_ABI,
  PRECOMPILE_ADDRESSES,
  PrecompileNotServedError,
  SCHEDULER_ABI,
  STREAMS_ABI,
  TEE_ATTESTOR_ABI,
  TEE_FAMILIES,
  UNSERVED_PRECOMPILES,
  precompileAddressBook,
  scheduler,
  servedAddress,
  streams,
  unservedPrecompileAt,
} from '../src/precompiles.js';
import { closePool, getPool } from '../src/db/pool.js';
import { executeAgentTransaction, type AgentCtx } from '../src/routes/agentExec.js';

function readRepositoryFile(path: string): string {
  return readFileSync(new URL(`../../../../${path}`, import.meta.url), 'utf8');
}

const addressTable = readRepositoryFile('spec/layerx-merge/contracts/address-table.md');

function tableRows(start: string, end: string): string[][] {
  const startOffset = addressTable.indexOf(start);
  const endOffset = addressTable.indexOf(end, startOffset + start.length);
  if (startOffset < 0 || endOffset < 0) {
    throw new Error(`Missing address-table section: ${start}`);
  }
  return addressTable
    .slice(startOffset + start.length, endOffset)
    .split('\n')
    .filter((line) => line.startsWith('| 0x'))
    .map((line) =>
      line
        .split('|')
        .slice(1, -1)
        .map((cell) => cell.trim()),
    );
}

const addressRows = tableRows('## (a) Address table', '### (a2) Method selectors');
const selectorRows = tableRows('### (a2) Method selectors', '### 0x101A resolution');
const expectedRoutes = {
  orob: '0x0000000000000000000000000000000000000901',
  clearing: '0x0000000000000000000000000000000000000902',
  oracle: '0x0000000000000000000000000000000000000903',
  pofq: '0x0000000000000000000000000000000000000904',
  scheduler: '0x0000000000000000000000000000000000000905',
  streams: '0x0000000000000000000000000000000000000906',
  teeAttestor: '0x0000000000000000000000000000000000000907',
  eip712: '0x0000000000000000000000000000000000000908',
  staking: '0x0000000000000000000000000000000000001005',
  bech32: '0x0000000000000000000000000000000000001004',
  p256: '0x0000000000000000000000000000000000001011',
} as const;
const routeKeys = Object.keys(expectedRoutes) as (keyof typeof expectedRoutes)[];
const registrationSources = [
  { key: 'staking', package: 'staking', symbol: 'StakingAddress', file: 'staking.go' },
  { key: 'bech32', package: 'addr', symbol: 'AddrAddress', file: 'addr.go' },
  { key: 'p256', package: 'p256', symbol: 'P256VerifyAddress', file: 'p256.go' },
] as const;
const setup = readRepositoryFile('precompiles/setup.go')
  .split('func GetCustomPrecompiles(')[1]
  ?.split('\nfunc InitializePrecompiles(')[0];
const abiRoutes = [
  { key: 'scheduler', abi: SCHEDULER_ABI, count: 5 },
  { key: 'streams', abi: STREAMS_ABI, count: 6 },
  { key: 'eip712', abi: EIP712_ABI, count: 3 },
  { key: 'teeAttestor', abi: TEE_ATTESTOR_ABI, count: 4 },
] as const;

function registeredAddress(source: (typeof registrationSources)[number]): string {
  const file = `precompiles/${source.package}/${source.file}`;
  const match = readRepositoryFile(file).match(
    new RegExp(`\\b${source.symbol}\\s*=\\s*"(0x[0-9a-fA-F]{40})"`),
  );
  if (!match?.[1]) throw new Error(`Missing ${source.key} address constant in ${file}`);
  return match[1].toLowerCase();
}

describe('route map', () => {
  it('retains exactly the eleven distinct route keys', () => {
    expect(Object.keys(PRECOMPILE_ADDRESSES).sort()).toEqual([...routeKeys].sort());
    expect(new Set(Object.values(PRECOMPILE_ADDRESSES)).size).toBe(11);
  });

  it.each(routeKeys)('%s resolves to its exact address in contract section (a)', (key) => {
    const address = PRECOMPILE_ADDRESSES[key];
    expect(address, key).toBe(expectedRoutes[key]);
    expect(
      addressRows.some((row) => row[0]?.toLowerCase() === address),
      key,
    ).toBe(true);
  });

  it.each(registrationSources)(
    '$key matches its Go constant and GetCustomPrecompiles entry',
    (source) => {
      expect(PRECOMPILE_ADDRESSES[source.key], source.key).toBe(registeredAddress(source));
      expect(setup, source.key).toMatch(
        new RegExp(`ecommon\\.HexToAddress\\(\\s*${source.package}\\.${source.symbol}\\s*\\)`),
      );
    },
  );

  it.each(routeKeys)('%s resolves to a Go registration or a new merge entry', (key) => {
    const source = registrationSources.find((entry) => entry.key === key);
    if (source) {
      expect(PRECOMPILE_ADDRESSES[key], key).toBe(registeredAddress(source));
      expect(setup, key).toMatch(
        new RegExp(`ecommon\\.HexToAddress\\(\\s*${source.package}\\.${source.symbol}\\s*\\)`),
      );
    } else {
      const entry = addressRows.find((row) => row[0]?.toLowerCase() === PRECOMPILE_ADDRESSES[key]);
      expect(entry?.[3], key).toBe('new');
      expect(entry?.[2], key).toContain('precompiles/layerxagent/');
      expect(entry?.[5], key).toBe('merge plan');
    }
  });

  it('bech32 targets the addr ABI getPaxAddr and getEvmAddr views', () => {
    const abi = JSON.parse(readRepositoryFile('precompiles/addr/abi.json')) as Abi;
    const methods = abi.filter(
      (item) => item.type === 'function' && ['getPaxAddr', 'getEvmAddr'].includes(item.name),
    );
    expect(PRECOMPILE_ADDRESSES.bech32).toBe(registeredAddress(registrationSources[1]));
    expect(
      methods
        .map((item) => {
          if (item.type !== 'function') throw new Error('Expected addr function');
          return [
            toFunctionSignature(item),
            item.stateMutability,
            item.outputs.map((output) => output.type),
          ];
        })
        .sort(),
    ).toEqual([
      ['getEvmAddr(string)', 'view', ['address']],
      ['getPaxAddr(address)', 'view', ['string']],
    ]);
  });
});

describe('agent ABI selectors', () => {
  it.each(abiRoutes)(
    '$key exactly matches every section (a2) signature and selector',
    ({ key, abi, count }) => {
      const address = `0x${PRECOMPILE_ADDRESSES[key].slice(-4)}`;
      const expected = selectorRows
        .filter((row) => row[0]?.toLowerCase() === address)
        .map((row) => [row[1], row[2]])
        .sort();
      const actual = abi
        .map((method) => [toFunctionSignature(method), toFunctionSelector(method)])
        .sort();
      expect(expected, key).toHaveLength(count);
      expect(actual, key).toEqual(expected);
      expect(new Set(actual.map(([, selector]) => selector)).size, key).toBe(count);
    },
  );
});

function goSources(): string[] {
  const root = fileURLToPath(new URL('../../../../', import.meta.url));
  function walk(directory: string): string[] {
    return readdirSync(join(root, directory), { withFileTypes: true }).flatMap((entry) => {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) {
        return ['.git', 'node_modules', 'target'].includes(entry.name) ? [] : walk(path);
      }
      return entry.isFile() && path.endsWith('.go') && !path.endsWith('_test.go') ? [path] : [];
    });
  }
  return walk('precompiles');
}

function goImports(source: string): string[] {
  const imports: string[] = [];
  const tokens = /\/\/|\/\*|["'`]|[A-Za-z_]\w*|[()]/g;
  let importing = false;
  let block = false;
  let match: RegExpExecArray | null;
  while ((match = tokens.exec(source)) !== null) {
    const token = match[0];
    if (token === '//' || token === '/*') {
      const end = source.indexOf(token === '//' ? '\n' : '*/', tokens.lastIndex);
      tokens.lastIndex = end < 0 ? source.length : end + (token === '//' ? 1 : 2);
    } else if (token === '"' || token === "'" || token === '`') {
      const start = tokens.lastIndex;
      let end = start;
      while (end < source.length && source[end] !== token) {
        end += source[end] === '\\' && token !== '`' ? 2 : 1;
      }
      if (importing && token !== "'") {
        imports.push(source.slice(start, end));
        if (!block) importing = false;
      }
      tokens.lastIndex = end + 1;
    } else if (token === 'import') {
      importing = true;
    } else if (importing && token === '(') {
      block = true;
    } else if (token === ')') {
      importing = false;
      block = false;
    }
  }
  return imports;
}

function unservedSetMatchesNodeRegistrations() {
  const sources = goSources().map((file) => ({ file, source: readRepositoryFile(file) }));
  const registeredAddresses = new Set<string>();
  for (const { file, source } of sources) {
    const registrations = source.matchAll(
      /\bprecompiles\.RegisterMergeConstructor\(\s*\w+\.HexToAddress\(\s*(\w+)\s*\)\s*,/g,
    );
    for (const registration of registrations) {
      const symbol = registration[1];
      const packageSource = sources
        .filter((entry) => dirname(entry.file) === dirname(file))
        .map((entry) => entry.source)
        .join('\n');
      const address = packageSource.match(
        new RegExp(`\\b${symbol}\\s*=\\s*"(0x[0-9a-fA-F]{40})"`),
      );
      expect(address, `${file}: ${symbol}`).not.toBeNull();
      registeredAddresses.add(address![1]!.toLowerCase());
    }
  }
  const mergeTable = readRepositoryFile('precompiles/setup.go')
    .split('var MergeAddressTable = []MergeAddress{')[1]
    ?.split('\n}')[0];
  expect(mergeTable).toBeDefined();
  const agentKeys = routeKeys.filter((key) =>
    PRECOMPILE_ADDRESSES[key].startsWith('0x00000000000000000000000000000000000009'),
  );
  expect(agentKeys).toHaveLength(8);
  for (const key of agentKeys) {
    expect(mergeTable, key).toContain(`HexToAddress("${PRECOMPILE_ADDRESSES[key]}")`);
  }
  const unregisteredKeys = agentKeys.filter(
    (key) => !registeredAddresses.has(PRECOMPILE_ADDRESSES[key]),
  );
  expect([...UNSERVED_PRECOMPILES].sort()).toEqual(unregisteredKeys.sort());
}

function temporarilyUnserve(...keys: (keyof typeof PRECOMPILE_ADDRESSES)[]): () => void {
  const unserved = UNSERVED_PRECOMPILES as Set<keyof typeof PRECOMPILE_ADDRESSES>;
  const previous = new Set(unserved);
  for (const key of keys) unserved.add(key);
  return () => {
    unserved.clear();
    for (const key of previous) unserved.add(key);
  };
}

async function servedAddressReturnsAgentAddresses() {
  for (const key of routeKeys) {
    expect(servedAddress(key), key).toBe(expectedRoutes[key]);
  }
  expect(servedAddress('scheduler')).toBe('0x0000000000000000000000000000000000000905');
  const restore = temporarilyUnserve('scheduler', 'streams');
  try {
    expect(() => servedAddress('scheduler')).toThrowError(PrecompileNotServedError);
    expect(() => servedAddress('streams')).toThrowError(PrecompileNotServedError);
    await expect(scheduler.pending(PRECOMPILE_ADDRESSES.orob)).rejects.toBeInstanceOf(
      PrecompileNotServedError,
    );
    await expect(streams.accrued(1n)).rejects.toBeInstanceOf(PrecompileNotServedError);
  } finally {
    restore();
  }
  expect(servedAddress('scheduler')).toBe(expectedRoutes.scheduler);
  expect(servedAddress('streams')).toBe(expectedRoutes.streams);
}

test(unservedSetMatchesNodeRegistrations, unservedSetMatchesNodeRegistrations);
test(servedAddressReturnsAgentAddresses, servedAddressReturnsAgentAddresses);

const unservedPrecompileTests = {
  async 'refuses a raw send to an unserved precompile address before any wallet read'() {
    const did = 'did:key:z6MkhMVmHTTrRzNBoFKtGAgJcmbopvJ8t83aF2j6cm7uqh5V';
    const ctx: AgentCtx = {
      did,
      ownerUserId: null,
      principal: {
        did,
        owner_user_id: null,
        label: 'Unserved precompile regression',
        key_fingerprint: 'unserved-precompile',
        public_key: '11'.repeat(32),
        wallet_id: null,
        is_frozen: false,
        created_at: new Date(0).toISOString(),
        last_seen_at: null,
      },
    };
    const pool = getPool();
    expect(pool.totalCount).toBe(0);
    const restore = temporarilyUnserve('scheduler', 'streams');
    try {
      const to = '0x0000000000000000000000000000000000000905';
      const result = await executeAgentTransaction({
        ctx,
        tx: { to, value: 1n },
        intent: { kind: 'transaction', to, value: 1n },
        broadcast: true,
        audit: { request_hash: 'unserved-precompile', ip: null, user_agent: null },
      });
      expect(result).toEqual({
        status: 503,
        body: { error: 'precompile_not_served', precompile: 'scheduler', address: to },
      });
      for (const key of UNSERVED_PRECOMPILES) {
        const address = PRECOMPILE_ADDRESSES[key];
        expect(unservedPrecompileAt(address.toUpperCase())).toBe(key);
        for (const broadcast of [false, true]) {
          await expect(
            executeAgentTransaction({
              ctx,
              tx: { to: address, value: 1n },
              intent: { kind: 'transaction', to: address, value: 1n },
              broadcast,
              audit: { request_hash: 'unserved-precompile', ip: null, user_agent: null },
            }),
          ).resolves.toEqual({
            status: 503,
            body: { error: 'precompile_not_served', precompile: key, address },
          });
        }
      }
      expect(pool.totalCount).toBe(0);
      expect(pool.waitingCount).toBe(0);
      expect(unservedPrecompileAt(undefined)).toBeNull();
      expect(unservedPrecompileAt('0x000000000000000000000000000000000000dEaD')).toBeNull();
    } finally {
      restore();
      await closePool();
    }
  },
  'address book marks each unserved precompile as not served'() {
    const book = precompileAddressBook();
    expect(book.addresses).toEqual(PRECOMPILE_ADDRESSES);
    expect(book.tee_families).toEqual(TEE_FAMILIES);
    expect(Object.keys(book.served).sort()).toEqual(Object.keys(PRECOMPILE_ADDRESSES).sort());
    for (const key of Object.keys(PRECOMPILE_ADDRESSES) as (keyof typeof PRECOMPILE_ADDRESSES)[]) {
      expect(book.served[key], key).toBe(!UNSERVED_PRECOMPILES.has(key));
      expect(unservedPrecompileAt(PRECOMPILE_ADDRESSES[key]), key).toBe(
        UNSERVED_PRECOMPILES.has(key) ? key : null,
      );
    }
  },
};

describe('unserved agent precompiles', () => {
  test(
    unservedPrecompileTests['refuses a raw send to an unserved precompile address before any wallet read'],
    unservedPrecompileTests['refuses a raw send to an unserved precompile address before any wallet read'],
  );

  test(
    unservedPrecompileTests['address book marks each unserved precompile as not served'],
    unservedPrecompileTests['address book marks each unserved precompile as not served'],
  );

  it('lists the agent routes whose constructor package is not linked by the node', () => {
    const agentKeys = routeKeys.filter(
      (key) => !registrationSources.some((source) => source.key === key),
    );
    const imports = goImports(readRepositoryFile('node/precompiles.go'));
    const unlinkedKeys = agentKeys.filter(
      (key) =>
        !imports.includes(
          `github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/${key.toLowerCase()}`,
        ),
    );
    expect([...UNSERVED_PRECOMPILES].sort()).toEqual(unlinkedKeys.sort());
  });

  it('refuses to build a scheduler deposit or a stream to an unserved address', () => {
    const restore = temporarilyUnserve('scheduler', 'streams');
    try {
      expect(() =>
        scheduler.schedule({
          target: '0x000000000000000000000000000000000000dEaD',
          executeAtBlock: 1n,
          gasLimit: 1n,
          depositWei: 1n,
        }),
      ).toThrowError(PrecompileNotServedError);
      expect(() => scheduler.cancel(1n)).toThrowError(PrecompileNotServedError);
      expect(() =>
        streams.open({
          payee: '0x000000000000000000000000000000000000dEaD',
          token: '0x000000000000000000000000000000000000dEaD',
          ratePerSecond: 1n,
          cap: 1n,
        }),
      ).toThrowError(PrecompileNotServedError);
    } finally {
      restore();
    }
  });

  it('refuses reads of an unserved address before any RPC', async () => {
    const restore = temporarilyUnserve('scheduler', 'streams');
    try {
      await expect(
        scheduler.pending('0x000000000000000000000000000000000000dEaD'),
      ).rejects.toBeInstanceOf(PrecompileNotServedError);
      await expect(streams.accrued(1n)).rejects.toBeInstanceOf(PrecompileNotServedError);
      await expect(streams.getStream(1n)).rejects.toBeInstanceOf(PrecompileNotServedError);
    } finally {
      restore();
    }
  });
});

async function ensureServedTreatsUnknownMethodRejectionAsServed() {
  const requests: { method: string; params: unknown[] }[] = [];
  const server = createServer(async (req, res) => {
    let body = '';
    for await (const chunk of req) body += chunk.toString();
    const request = JSON.parse(body) as {
      id: number;
      method: string;
      params: [{ to: string; data: string }, string];
    };
    requests.push({ method: request.method, params: request.params });
    const to = request.params[0]?.to?.toLowerCase();
    const response =
      request.method !== 'eth_call'
        ? { error: { code: -32601, message: 'Method not found' } }
        : to === PRECOMPILE_ADDRESSES.scheduler
          ? {
              error: {
                code: -32000,
                message: 'LXP_PRECOMPILE_UNKNOWN_METHOD: no method with id: 0xffffffff',
              },
            }
          : to === PRECOMPILE_ADDRESSES.streams
            ? { result: '0x' }
            : { error: { code: 3, message: 'LXP_MODULE_DISABLED: precompile is disabled' } };
    res.setHeader('content-type', 'application/json');
    res.end(JSON.stringify({ jsonrpc: '2.0', id: request.id, ...response }));
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const previousRpcUrl = process.env.HYPERPAXEER_RPC_URL;
  try {
    const address = server.address();
    if (!address || typeof address === 'string') throw new Error('Expected a TCP listener');
    process.env.HYPERPAXEER_RPC_URL = `http://127.0.0.1:${address.port}`;
    vi.resetModules();
    const { ensureServed, PrecompileNotServedError } = await import('../src/precompiles.js');

    await expect(ensureServed('scheduler')).resolves.toBeUndefined();
    expect(requests).toEqual([
      {
        method: 'eth_call',
        params: [{ to: PRECOMPILE_ADDRESSES.scheduler, data: '0xffffffff' }, 'latest'],
      },
    ]);
    await expect(ensureServed('scheduler')).resolves.toBeUndefined();
    expect(requests).toHaveLength(1);
    await expect(ensureServed('streams')).rejects.toBeInstanceOf(PrecompileNotServedError);
    await expect(ensureServed('streams')).rejects.toBeInstanceOf(PrecompileNotServedError);
    await expect(ensureServed('oracle')).rejects.toThrow('LXP_MODULE_DISABLED');
    expect(requests).toEqual(
      ['scheduler', 'streams', 'streams', 'oracle'].map((key) => ({
        method: 'eth_call',
        params: [
          { to: PRECOMPILE_ADDRESSES[key as keyof typeof PRECOMPILE_ADDRESSES], data: '0xffffffff' },
          'latest',
        ],
      })),
    );
    expect(requests.some(({ method }) => method === 'eth_getCode')).toBe(false);
  } finally {
    if (previousRpcUrl === undefined) delete process.env.HYPERPAXEER_RPC_URL;
    else process.env.HYPERPAXEER_RPC_URL = previousRpcUrl;
    vi.resetModules();
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      server.close((err) => (err ? reject(err) : resolve())),
    );
  }
}

test(
  ensureServedTreatsUnknownMethodRejectionAsServed,
  ensureServedTreatsUnknownMethodRejectionAsServed,
);

const chainProbeTests = {
  async 'probes the chain before serving a linked agent precompile'() {
    let served = false;
    const requests: { method: string; params: unknown[] }[] = [];
    const server = createServer(async (req, res) => {
      let body = '';
      for await (const chunk of req) body += chunk.toString();
      const request = JSON.parse(body) as { id: number; method: string; params: unknown[] };
      requests.push({ method: request.method, params: request.params });
      res.setHeader('content-type', 'application/json');
      res.end(
        JSON.stringify(
          request.method === 'eth_call'
            ? {
                jsonrpc: '2.0',
                id: request.id,
                ...(served
                  ? { error: { code: -32000, message: 'LXP_PRECOMPILE_UNKNOWN_METHOD' } }
                  : { result: '0x' }),
              }
            : {
                jsonrpc: '2.0',
                id: request.id,
                error: { code: -32601, message: 'Method not found' },
              },
        ),
      );
    });
    await new Promise<void>((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const previousRpcUrl = process.env.HYPERPAXEER_RPC_URL;
    try {
      const address = server.address();
      if (!address || typeof address === 'string') throw new Error('Expected a TCP listener');
      process.env.HYPERPAXEER_RPC_URL = `http://127.0.0.1:${address.port}`;
      vi.resetModules();
      const { ensureServed, PrecompileNotServedError, eip712, teeAttestor } = await import(
        '../src/precompiles.js'
      );

      await expect(ensureServed('clearing')).rejects.toBeInstanceOf(PrecompileNotServedError);
      expect(requests).toEqual([
        {
          method: 'eth_call',
          params: [{ to: PRECOMPILE_ADDRESSES.clearing, data: '0xffffffff' }, 'latest'],
        },
      ]);
      const domainSeparator = `0x${'11'.repeat(32)}` as const;
      const structHash = `0x${'22'.repeat(32)}` as const;
      const reads = [
        { key: 'eip712', read: () => eip712.hashTypedData(domainSeparator, structHash) },
        {
          key: 'eip712',
          read: () =>
            eip712.domainSeparator({
              name: 'Agent',
              version: '1',
              chainId: 125n,
              verifyingContract: PRECOMPILE_ADDRESSES.clearing,
            }),
        },
        {
          key: 'eip712',
          read: () => eip712.recoverTypedSigner(domainSeparator, structHash, '0x'),
        },
        { key: 'teeAttestor', read: () => teeAttestor.verify(0, '0x') },
        {
          key: 'teeAttestor',
          read: () => teeAttestor.verifyAndExpect(0, '0x', domainSeparator),
        },
        { key: 'teeAttestor', read: () => teeAttestor.rootOf(0, 0n) },
        { key: 'teeAttestor', read: () => teeAttestor.rootCount(0) },
      ] as const;
      for (const { read } of reads) {
        await expect(read()).rejects.toBeInstanceOf(PrecompileNotServedError);
      }
      expect(requests.slice(1)).toEqual(
        reads.map(({ key }) => ({
          method: 'eth_call',
          params: [{ to: PRECOMPILE_ADDRESSES[key], data: '0xffffffff' }, 'latest'],
        })),
      );

      served = true;
      await expect(ensureServed('clearing')).resolves.toBeUndefined();
      expect(requests).toHaveLength(reads.length + 2);
      expect(requests[reads.length + 1]).toEqual({
        method: 'eth_call',
        params: [{ to: PRECOMPILE_ADDRESSES.clearing, data: '0xffffffff' }, 'latest'],
      });
      served = false;
      await expect(ensureServed('clearing')).resolves.toBeUndefined();
      expect(requests).toHaveLength(reads.length + 2);
      await expect(ensureServed('orob')).rejects.toBeInstanceOf(PrecompileNotServedError);
      expect(requests.at(-1)).toEqual({
        method: 'eth_call',
        params: [{ to: PRECOMPILE_ADDRESSES.orob, data: '0xffffffff' }, 'latest'],
      });
    } finally {
      if (previousRpcUrl === undefined) delete process.env.HYPERPAXEER_RPC_URL;
      else process.env.HYPERPAXEER_RPC_URL = previousRpcUrl;
      vi.resetModules();
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) =>
        server.close((err) => (err ? reject(err) : resolve())),
      );
    }
  },
};

test(
  chainProbeTests['probes the chain before serving a linked agent precompile'],
  chainProbeTests['probes the chain before serving a linked agent precompile'],
);
