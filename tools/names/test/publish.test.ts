import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { once } from 'node:events';
import { mkdtemp, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { createServer, type IncomingMessage, type Server } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test, { type TestContext } from 'node:test';
import {
  canonicalManifest,
  encodeContenthash,
  manifestDigest,
  namehash,
  parseManifest,
} from '@paxeer/names';
import {
  decodeFunctionData,
  encodeAbiParameters,
  encodeFunctionData,
  keccak256,
  parseTransaction,
  type Hex,
} from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { immutableCacheControl, publish } from '../src/publish.ts';
import { verify } from '../src/verify.ts';

const privateKey = `0x${'11'.repeat(32)}` as const;
const resolver = '0x0000000000000000000000000000000000001234' as const;
const blockHash = `0x${'22'.repeat(32)}` as const;
const account = privateKeyToAccount(privateKey);
const abi = [
  {
    type: 'function',
    name: 'setContenthash',
    stateMutability: 'nonpayable',
    inputs: [
      { name: 'node', type: 'bytes32' },
      { name: 'hash', type: 'bytes' },
    ],
    outputs: [],
  },
  {
    type: 'function',
    name: 'contenthash',
    stateMutability: 'view',
    inputs: [{ name: 'node', type: 'bytes32' }],
    outputs: [{ name: '', type: 'bytes' }],
  },
] as const;

async function body(request: IncomingMessage): Promise<Buffer> {
  const chunks: Buffer[] = [];
  for await (const chunk of request) chunks.push(Buffer.from(chunk));
  return Buffer.concat(chunks);
}

async function listen(server: Server): Promise<string> {
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  return `http://127.0.0.1:${address.port}`;
}

async function close(server: Server): Promise<void> {
  await new Promise<void>((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
    server.closeAllConnections();
  });
}

async function fixture(t: TestContext) {
  const dir = await mkdtemp(join(tmpdir(), 'paxnames-'));
  t.after(() => rm(dir, { recursive: true, force: true }));
  await mkdir(join(dir, 'assets'));
  await writeFile(join(dir, 'index.html'), '<!doctype html><h1>Paxeer</h1>');
  await writeFile(join(dir, 'assets/site.css'), 'body{color:blue}');
  const stored = new Map<string, Buffer>();
  const requests: {
    method: string;
    key: string;
    conditional?: string;
    cache?: string;
    type?: string;
  }[] = [];
  const raceKeys = new Set<string>();
  let headStatus: number | undefined;
  const s3 = createServer((request, response) => {
    void (async () => {
      const url = new URL(request.url!, 'http://localhost');
      if (!url.pathname.startsWith('/sites/')) {
        response.writeHead(404).end();
        return;
      }
      const key = decodeURIComponent(url.pathname.slice('/sites/'.length));
      requests.push({
        method: request.method!,
        key,
        conditional: request.headers['if-none-match'],
        cache: request.headers['cache-control'],
        type: request.headers['content-type'],
      });
      if (request.method === 'HEAD' || request.method === 'GET') {
        const bytes = stored.get(key);
        if (request.method === 'HEAD' && headStatus !== undefined) {
          response.writeHead(headStatus).end();
        } else if (!bytes) {
          response.writeHead(404).end();
        } else {
          response
            .writeHead(200, { 'content-length': bytes.length })
            .end(request.method === 'HEAD' ? undefined : bytes);
        }
      } else if (request.method === 'PUT') {
        const bytes = await body(request);
        if (raceKeys.delete(key)) stored.set(key, bytes);
        if (request.headers['if-none-match'] === '*' && stored.has(key)) {
          response
            .writeHead(412, { 'content-type': 'application/xml' })
            .end('<Error><Code>PreconditionFailed</Code></Error>');
          return;
        }
        if (request.headers['if-none-match'] !== '*') {
          response.writeHead(400).end();
          return;
        }
        stored.set(key, bytes);
        response.writeHead(200, { etag: '"stored"' }).end();
      } else {
        response.writeHead(405).end();
      }
    })().catch((error: unknown) => {
      response.writeHead(500).end(String(error));
    });
  });
  t.after(() => close(s3));
  const endpoint = await listen(s3);
  const transactions: { to: string; data: Hex; hash: Hex; raw: Hex }[] = [];
  const records = new Map<string, Hex>();
  const calls: string[] = [];
  let reverted = false;
  const rpc = createServer((request, response) => {
    void (async () => {
      const payload = JSON.parse((await body(request)).toString()) as {
        id: number;
        method: string;
        params?: unknown[];
      };
      const params = payload.params ?? [];
      calls.push(payload.method);
      let result: unknown;
      switch (payload.method) {
        case 'eth_chainId':
          result = '0x7d';
          break;
        case 'eth_getTransactionCount':
          result = `0x${transactions.length.toString(16)}`;
          break;
        case 'eth_gasPrice':
          result = '0x3b9aca00';
          break;
        case 'eth_estimateGas':
          result = '0x186a0';
          break;
        case 'eth_maxPriorityFeePerGas':
          result = '0x3b9aca00';
          break;
        case 'eth_blockNumber':
          result = '0x1';
          break;
        case 'eth_getBlockByNumber':
          result = {
            number: '0x1',
            hash: blockHash,
            parentHash: blockHash,
            timestamp: '0x1',
            gasLimit: '0x1c9c380',
            gasUsed: '0x186a0',
            baseFeePerGas: '0x3b9aca00',
            transactions: [],
          };
          break;
        case 'eth_sendRawTransaction': {
          const raw = params[0] as Hex;
          const transaction = parseTransaction(raw);
          assert.equal(transaction.chainId, 125);
          assert.ok(transaction.to);
          assert.ok(transaction.data);
          const decoded = decodeFunctionData({ abi, data: transaction.data });
          assert.equal(decoded.functionName, 'setContenthash');
          if (decoded.functionName !== 'setContenthash') throw new Error('Unexpected transaction');
          const [node, contenthash] = decoded.args;
          if (!reverted) records.set(node, contenthash);
          const hash = keccak256(raw);
          transactions.push({ to: transaction.to, data: transaction.data, hash, raw });
          result = hash;
          break;
        }
        case 'eth_getTransactionReceipt': {
          const transaction = transactions.find((item) => item.hash === params[0]);
          result = transaction
            ? {
                transactionHash: transaction.hash,
                transactionIndex: '0x0',
                blockHash,
                blockNumber: '0x1',
                from: account.address,
                to: transaction.to,
                cumulativeGasUsed: '0x186a0',
                gasUsed: '0x186a0',
                contractAddress: null,
                logs: [],
                logsBloom: `0x${'00'.repeat(256)}`,
                status: reverted ? '0x0' : '0x1',
                effectiveGasPrice: '0x3b9aca00',
                type: '0x2',
              }
            : null;
          break;
        }
        case 'eth_call': {
          const call = params[0] as { to: string; data: Hex };
          assert.equal(call.to.toLowerCase(), resolver.toLowerCase());
          const decoded = decodeFunctionData({ abi, data: call.data });
          assert.equal(decoded.functionName, 'contenthash');
          result = encodeAbiParameters([{ type: 'bytes' }], [records.get(decoded.args[0]) ?? '0x']);
          break;
        }
        default:
          response.writeHead(200, { 'content-type': 'application/json' }).end(
            JSON.stringify({
              jsonrpc: '2.0',
              id: payload.id,
              error: { code: -32601, message: `Unexpected method: ${payload.method}` },
            })
          );
          return;
      }
      response
        .writeHead(200, { 'content-type': 'application/json' })
        .end(JSON.stringify({ jsonrpc: '2.0', id: payload.id, result }));
    })().catch((error: unknown) => {
      response.writeHead(500).end(String(error));
    });
  });
  t.after(() => close(rpc));
  const rpcUrl = await listen(rpc);
  const options = {
    dir,
    name: 'example.pax',
    bucket: {
      endpoint,
      region: 'us-east-1',
      bucket: 'sites',
      publicBaseUrl: `${endpoint}/sites`,
      credentials: { accessKeyId: 'test-access', secretAccessKey: 'test-secret' },
    },
    rpcUrl,
    resolver,
    privateKey,
  };
  const verifyOptions = {
    name: options.name,
    rpcUrl,
    resolver,
    publicBaseUrl: options.bucket.publicBaseUrl,
  };
  return {
    dir,
    stored,
    requests,
    transactions,
    records,
    calls,
    options,
    verifyOptions,
    raceKeys,
    setHeadStatus(status: number) {
      headStatus = status;
    },
    setReverted() {
      reverted = true;
    },
  };
}

async function publish_uploads_only_missing_objects_and_writes_the_manifest(t: TestContext) {
  const f = await fixture(t);
  const result = await publish(f.options);
  assert.equal(result.uploaded, 2);
  assert.equal(result.skipped, 0);
  const manifestKey = `manifests/${result.digest.slice(2)}.json`;
  const manifestBytes = f.stored.get(manifestKey);
  assert.ok(manifestBytes);
  assert.equal(createHash('sha256').update(manifestBytes).digest('hex'), result.digest.slice(2));
  const manifest = parseManifest(manifestBytes);
  assert.equal(manifest.name, 'example.pax');
  assert.equal(manifest.version, 1);
  assert.equal(manifestDigest(manifest), result.digest);
  assert.equal(canonicalManifest(manifest), manifestBytes.toString());
  assert.deepEqual(Object.keys(manifest.entries).sort(), ['/assets/site.css', '/index.html']);
  for (const [path, type] of [
    ['/assets/site.css', 'text/css'],
    ['/index.html', 'text/html'],
  ]) {
    const bytes = await readFile(join(f.dir, path!));
    const sha256 = createHash('sha256').update(bytes).digest('hex');
    assert.deepEqual(manifest.entries[path!], { sha256, size: bytes.length, type });
    assert.deepEqual(f.stored.get(`objects/${sha256}`), bytes);
  }
  assert.equal(f.stored.size, 3);
  for (const [index, request] of f.requests.entries()) {
    if (request.method !== 'PUT') continue;
    assert.equal(request.conditional, '*');
    assert.equal(request.cache, immutableCacheControl);
    assert.ok(
      f.requests
        .slice(0, index)
        .some((prior) => prior.key === request.key && prior.method === 'HEAD')
    );
    const expectedType =
      request.key === manifestKey
        ? 'application/json'
        : Object.values(manifest.entries).find(
            (entry) => request.key === `objects/${entry.sha256}`
          )!.type;
    assert.equal(request.type, expectedType);
  }
  f.requests.length = 0;
  const second = await publish(f.options);
  assert.equal(second.digest, result.digest);
  assert.equal(second.uploaded, 0);
  assert.equal(second.skipped, 2);
  assert.equal(f.requests.filter((request) => request.method === 'HEAD').length, 3);
  assert.equal(f.requests.filter((request) => request.method === 'PUT').length, 0);
  assert.equal(f.transactions.length, 2);

  const html = manifest.entries['/index.html']!;
  f.stored.delete(`objects/${html.sha256}`);
  f.raceKeys.add(`objects/${html.sha256}`);
  const concurrent = await publish(f.options);
  assert.equal(concurrent.uploaded, 0);
  assert.equal(concurrent.skipped, 2);
  assert.deepEqual(await verify(f.verifyOptions), []);
  f.setHeadStatus(403);
  await assert.rejects(publish(f.options));
  assert.equal(f.transactions.length, 3);
}

async function publish_sets_the_contenthash_to_the_manifest_digest(t: TestContext) {
  const f = await fixture(t);
  const result = await publish(f.options);
  assert.equal(f.transactions.length, 1);
  const transaction = f.transactions[0]!;
  assert.equal(transaction.to.toLowerCase(), resolver.toLowerCase());
  assert.equal(transaction.hash, result.txHash);
  assert.equal(
    transaction.data,
    encodeFunctionData({
      abi,
      functionName: 'setContenthash',
      args: [namehash('example.pax'), `0x70617831${result.digest.slice(2)}`],
    })
  );
  assert.equal(f.records.get(namehash('example.pax')), encodeContenthash(result.digest));
  assert.ok(f.calls.includes('eth_getTransactionReceipt'));
  f.setReverted();
  await assert.rejects(publish(f.options), /Contenthash transaction reverted/);
}

async function verify_refuses_an_object_whose_hash_differs(t: TestContext) {
  const f = await fixture(t);
  await assert.rejects(verify(f.verifyOptions), /no supported contenthash/);
  const result = await publish(f.options);
  assert.deepEqual(await verify(f.verifyOptions), []);
  const manifestKey = `manifests/${result.digest.slice(2)}.json`;
  const originalManifest = f.stored.get(manifestKey)!;
  const manifest = parseManifest(originalManifest);
  const entry = manifest.entries['/index.html']!;
  const corrupt = Buffer.alloc(entry.size, 0);
  assert.equal(corrupt.length, entry.size);
  f.stored.set(`objects/${entry.sha256}`, corrupt);
  assert.deepEqual(await verify(f.verifyOptions), [
    {
      path: '/index.html',
      expected: entry.sha256,
      actual: createHash('sha256').update(corrupt).digest('hex'),
    },
  ]);
  f.stored.delete(`objects/${entry.sha256}`);
  assert.deepEqual(await verify(f.verifyOptions), [
    { path: '/index.html', expected: entry.sha256, actual: null },
  ]);
  f.stored.set(manifestKey, Buffer.from('invalid JSON'));
  const start = f.requests.length;
  assert.deepEqual(await verify(f.verifyOptions), [
    {
      path: manifestKey,
      expected: result.digest.slice(2),
      actual: createHash('sha256').update('invalid JSON').digest('hex'),
    },
  ]);
  assert.deepEqual(
    f.requests.slice(start).map((request) => request.key),
    [manifestKey]
  );
}

test(publish_uploads_only_missing_objects_and_writes_the_manifest);
test(publish_sets_the_contenthash_to_the_manifest_digest);
test(verify_refuses_an_object_whose_hash_differs);
