import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { hexToBytes } from '@noble/hashes/utils.js';
import { expect, test } from 'vitest';
import {
  ProofError,
  resolveContent,
  tendermintHeaders,
  verifyStorageProof,
} from '../src/index.js';

const vector = JSON.parse(
  readFileSync(
    new URL('../../../tests/vectors/names/storage_proof_v1.json', import.meta.url),
    'utf8'
  )
);

function storage_proof_vector_verifies_and_refuses_a_flipped_value() {
  const input = {
    proofOps: vector.proof_ops,
    appHash: hexToBytes(vector.app_hash.slice(2)),
    store: vector.store,
    key: hexToBytes(vector.key.slice(2)),
    value: hexToBytes(vector.value.slice(2)),
  };
  expect(verifyStorageProof(input)).toBe(true);
  const value = input.value.slice();
  value[0] ^= 1;
  expect(() => verifyStorageProof({ ...input, value })).toThrow(ProofError);
  expect(() => verifyStorageProof({ ...input, value })).toThrow('ics23:iavl');
  const appHash = input.appHash.slice();
  appHash[0] ^= 1;
  expect(() => verifyStorageProof({ ...input, appHash })).toThrow(ProofError);
  expect(() => verifyStorageProof({ ...input, appHash })).toThrow('ics23:simple');
  expect(() => verifyStorageProof({ ...input, store: 'bank' })).toThrow(ProofError);
  expect(() =>
    verifyStorageProof({ ...input, proofOps: input.proofOps.slice(0, 1) })
  ).toThrow(ProofError);
  expect(() =>
    verifyStorageProof({ ...input, proofOps: [...input.proofOps].reverse() })
  ).toThrow(ProofError);
  const malformed = input.proofOps.map(
    (op: { type: string; key: string; data: string }) => ({
      ...op,
    })
  );
  malformed[0].data = '0xff';
  expect(() => verifyStorageProof({ ...input, proofOps: malformed })).toThrow(ProofError);
}

async function resolve_content_returns_the_digest_and_the_proven_height() {
  const requests: unknown[] = [];
  const headerHeights: bigint[] = [];
  const appHash = hexToBytes(vector.app_hash.slice(2));
  const encodedOps = vector.proof_ops.map(
    (op: { type: string; key: string; data: string }) => ({
      type: op.type,
      key: Buffer.from(op.key.slice(2), 'hex').toString('base64'),
      data: Buffer.from(op.data.slice(2), 'hex').toString('base64'),
    })
  );
  const blockNumber = `0x${BigInt(vector.height).toString(16)}`;
  let responseHeight: string | undefined = blockNumber;
  let responseValue: string = vector.value.slice(2);
  let consensusHeight = String(BigInt(vector.height) + 1n);
  const server = createServer(async (request, response) => {
    response.setHeader('content-type', 'application/json');
    if (request.method === 'GET') {
      requests.push(request.url);
      response.end(
        JSON.stringify({
          result: {
            signed_header: {
              header: {
                height: consensusHeight,
                app_hash: Buffer.from(appHash).toString('base64'),
              },
            },
          },
        })
      );
      return;
    }
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
    requests.push(body);
    if (body.method === 'eth_blockNumber') {
      response.end(
        JSON.stringify({ jsonrpc: '2.0', id: body.id, result: responseHeight })
      );
      return;
    }
    if (body.method !== 'eth_getProof') {
      response.end(
        JSON.stringify({
          jsonrpc: '2.0',
          id: body.id,
          error: { code: -32601, message: 'Method not found' },
        })
      );
      return;
    }
    response.end(
      JSON.stringify({
        jsonrpc: '2.0',
        id: body.id,
        result: {
          address: vector.address,
          hexValues: [responseValue],
          storageProof: [{ ops: encodedOps }],
        },
      })
    );
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  try {
    const address = server.address();
    if (address === null || typeof address === 'string')
      throw new Error('Missing HTTP address');
    const rpcUrl = `http://127.0.0.1:${address.port}`;
    const headers = {
      async appHashAt(height: bigint) {
        headerHeights.push(height);
        return appHash;
      },
    };
    const options = { rpcUrl, resolver: vector.address, headers };
    expect(await resolveContent(vector.name, options)).toEqual({
      digest: vector.value,
      height: BigInt(vector.height),
      verified: true,
    });
    expect(requests[0]).toEqual({
      jsonrpc: '2.0',
      id: 1,
      method: 'eth_blockNumber',
      params: [],
    });
    expect(requests[1]).toEqual({
      jsonrpc: '2.0',
      id: 2,
      method: 'eth_getProof',
      params: [vector.address, [vector.slot], blockNumber],
    });
    expect(headerHeights).toEqual([BigInt(vector.height) + 1n]);
    const wrongHash = appHash.slice();
    wrongHash[0] ^= 1;
    await expect(
      resolveContent(vector.name, {
        ...options,
        headers: {
          async appHashAt() {
            return wrongHash;
          },
        },
      })
    ).rejects.toThrow(ProofError);
    await expect(
      resolveContent(vector.name, { ...options, headers: tendermintHeaders(rpcUrl) })
    ).resolves.toEqual({
      digest: vector.value,
      height: BigInt(vector.height),
      verified: true,
    });
    expect(requests).toContain(`/commit?height=${BigInt(vector.height) + 1n}`);
    consensusHeight = String(vector.height);
    await expect(
      tendermintHeaders(rpcUrl).appHashAt(BigInt(vector.height) + 1n)
    ).rejects.toThrow('wrong header height');
    responseValue = '00'.repeat(32);
    await expect(resolveContent(vector.name, options)).rejects.toThrow(ProofError);
    responseValue = vector.value.slice(2);
    for (const malformed of [
      undefined,
      'invalid',
      String(vector.height),
      '0x0',
      '0x00',
      '0x08',
      '0x-1',
      '0x1.5',
      '0x',
    ]) {
      responseHeight = malformed;
      const requestCount = requests.length;
      const headerCount = headerHeights.length;
      await expect(resolveContent(vector.name, options)).rejects.toThrow(TypeError);
      expect(requests.slice(requestCount)).toEqual([
        { jsonrpc: '2.0', id: 1, method: 'eth_blockNumber', params: [] },
      ]);
      expect(headerHeights).toHaveLength(headerCount);
    }
  } finally {
    const closed = new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve()))
    );
    server.closeAllConnections();
    await closed;
  }
}

test(
  storage_proof_vector_verifies_and_refuses_a_flipped_value,
  storage_proof_vector_verifies_and_refuses_a_flipped_value
);
test(
  resolve_content_returns_the_digest_and_the_proven_height,
  resolve_content_returns_the_digest_and_the_proven_height
);
