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
  verifyStorageAbsence,
} from '../src/index.js';

const vector = JSON.parse(
  readFileSync(
    new URL('../../../tests/vectors/names/storage_proof_v1.json', import.meta.url),
    'utf8'
  )
);

const commitVector = JSON.parse(
  readFileSync(
    new URL('../../../tests/vectors/names/commit_v1.json', import.meta.url),
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
                app_hash: Buffer.from(appHash).toString('hex').toUpperCase(),
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

async function tendermint_headers_decode_the_consensus_hex_app_hash() {
  const requests: string[] = [];
  const header = { ...commitVector.header };
  const server = createServer((request, response) => {
    requests.push(`${request.method} ${request.url}`);
    response.setHeader('content-type', 'application/json');
    response.end(JSON.stringify({ result: { signed_header: { header } } }));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  try {
    const address = server.address();
    if (address === null || typeof address === 'string')
      throw new Error('Missing HTTP address');
    const headers = tendermintHeaders(`http://127.0.0.1:${address.port}`);
    const expected = Uint8Array.from(
      Buffer.from(commitVector.header.app_hash, 'hex')
    );
    const appHash = await headers.appHashAt(7n);
    expect(appHash).toHaveLength(32);
    expect(appHash).toEqual(expected);
    expect(requests).toEqual(['GET /commit?height=7']);

    header.app_hash = commitVector.header.app_hash.toLowerCase();
    await expect(headers.appHashAt(7n)).resolves.toEqual(expected);

    for (const malformed of [
      Buffer.from(expected).toString('base64'),
      commitVector.header.app_hash.slice(2),
      `${commitVector.header.app_hash}00`,
      `G${commitVector.header.app_hash.slice(1)}`,
      `0x${commitVector.header.app_hash}`,
      `${commitVector.header.app_hash}\n`,
      null,
    ]) {
      header.app_hash = malformed;
      await expect(headers.appHashAt(7n)).rejects.toThrow(TypeError);
    }

    header.app_hash = commitVector.header.app_hash;
    header.height = '8';
    await expect(headers.appHashAt(7n)).rejects.toThrow('wrong header height');
    await expect(headers.appHashAt(0n)).rejects.toThrow(RangeError);
    await expect(headers.appHashAt(-1n)).rejects.toThrow(RangeError);
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
test(
  tendermint_headers_decode_the_consensus_hex_app_hash,
  tendermint_headers_decode_the_consensus_hex_app_hash
);

async function resolve_content_returns_null_for_a_proven_absent_contenthash() {
  const absence = JSON.parse(
    readFileSync(
      new URL('../../../tests/vectors/names/storage_absence_v1.json', import.meta.url),
      'utf8'
    )
  );
  expect(absence.value).toBe('0x');
  const input = {
    proofOps: absence.proof_ops,
    appHash: hexToBytes(absence.app_hash.slice(2)),
    store: absence.store,
    key: hexToBytes(absence.key.slice(2)),
  };
  expect(verifyStorageAbsence(input)).toBe(true);
  expect(() =>
    verifyStorageAbsence({
      ...input,
      key: hexToBytes(vector.key.slice(2)),
      proofOps: vector.proof_ops,
    })
  ).toThrow('ics23:iavl: Expected a non-existence proof');
  expect(() => verifyStorageAbsence({ ...input, store: 'bank' })).toThrow('ics23:simple');
  expect(() =>
    verifyStorageAbsence({ ...input, proofOps: input.proofOps.slice(0, 1) })
  ).toThrow('ics23:simple');
  expect(() =>
    verifyStorageAbsence({ ...input, proofOps: [...input.proofOps].reverse() })
  ).toThrow('ics23:iavl');
  const wrongKey = input.key.slice();
  wrongKey[0] ^= 1;
  expect(() => verifyStorageAbsence({ ...input, key: wrongKey })).toThrow('ics23:iavl');
  const malformedOps = input.proofOps.map(
    (op: { type: string; key: string; data: string }) => ({ ...op })
  );
  malformedOps[0].data = '0xff';
  expect(() => verifyStorageAbsence({ ...input, proofOps: malformedOps })).toThrow(
    'ics23:iavl'
  );

  const requests: unknown[] = [];
  const headerHeights: bigint[] = [];
  const blockNumber = `0x${BigInt(absence.height).toString(16)}`;
  let responseOps = absence.proof_ops;
  let responseValue: unknown = '';
  let appHash = input.appHash;
  const server = createServer(async (request, response) => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
    requests.push(body);
    response.setHeader('content-type', 'application/json');
    if (body.method === 'eth_blockNumber') {
      response.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result: blockNumber }));
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
          address: absence.address,
          hexValues: [responseValue],
          storageProof: [
            {
              ops: responseOps.map((op: { type: string; key: string; data: string }) => ({
                type: op.type,
                key: Buffer.from(op.key.slice(2), 'hex').toString('base64'),
                data: Buffer.from(op.data.slice(2), 'hex').toString('base64'),
              })),
            },
          ],
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
    const options = {
      rpcUrl: `http://127.0.0.1:${address.port}`,
      resolver: absence.address,
      headers: {
        async appHashAt(height: bigint) {
          headerHeights.push(height);
          return appHash;
        },
      },
    };
    await expect(resolveContent(absence.name, options)).resolves.toBeNull();
    expect(requests).toEqual([
      { jsonrpc: '2.0', id: 1, method: 'eth_blockNumber', params: [] },
      {
        jsonrpc: '2.0',
        id: 2,
        method: 'eth_getProof',
        params: [absence.address, [absence.slot], blockNumber],
      },
    ]);
    expect(headerHeights).toEqual([BigInt(absence.height) + 1n]);
    responseOps = vector.proof_ops;
    await expect(resolveContent(absence.name, options)).rejects.toThrow(ProofError);
    responseOps = absence.proof_ops;
    appHash = input.appHash.slice();
    appHash[0] ^= 1;
    await expect(resolveContent(absence.name, options)).rejects.toThrow(ProofError);
    await expect(resolveContent(absence.name, options)).rejects.toThrow('ics23:simple');
    appHash = input.appHash;
    for (const malformed of [
      '0x',
      '00',
      '0'.repeat(63),
      'g'.repeat(64),
      `${'0'.repeat(64)}\n`,
      `0x${'0'.repeat(64)}\n`,
      null,
      0,
    ]) {
      responseValue = malformed;
      await expect(resolveContent(absence.name, options)).rejects.toThrow(ProofError);
    }
    responseValue = '00'.repeat(32);
    await expect(resolveContent(absence.name, options)).rejects.toThrow(ProofError);
  } finally {
    const closed = new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve()))
    );
    server.closeAllConnections();
    await closed;
  }
}

test(
  resolve_content_returns_null_for_a_proven_absent_contenthash,
  resolve_content_returns_null_for_a_proven_absent_contenthash
);
