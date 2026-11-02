import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { ed25519 } from '@noble/curves/ed25519.js';
import { sha256 } from '@noble/hashes/sha2.js';
import { bytesToHex, hexToBytes, utf8ToBytes } from '@noble/hashes/utils.js';
import { expect, test } from 'vitest';
import {
  HeaderError,
  headerHash,
  resolveContent,
  validatorSetHash,
  verifiedTendermintHeaders,
  verifyCommit,
  voteSignBytes,
  type CommitSignature,
  type TendermintCommit,
  type TendermintHeader,
  type Validator,
} from '../src/index.js';

type CommitVector = {
  chain_id: string;
  height: number;
  round: number;
  header: TendermintHeader;
  header_hash: string;
  block_id: { hash: string; part_set_header: { total: number; hash: string } };
  signatures: CommitSignature[];
  sign_bytes: string[];
};

const vector: CommitVector = JSON.parse(
  readFileSync(new URL('../../../tests/vectors/names/commit_v1.json', import.meta.url), 'utf8')
);
const validatorVector: { validators: Validator[]; total_power: number } = JSON.parse(
  readFileSync(new URL('../../../tests/vectors/names/validators_v1.json', import.meta.url), 'utf8')
);
const storageVector: {
  height: number;
  app_hash: string;
  name: string;
  address: string;
  value: string;
  proof_ops: { type: string; key: string; data: string }[];
} = JSON.parse(
  readFileSync(
    new URL('../../../tests/vectors/names/storage_proof_v1.json', import.meta.url),
    'utf8'
  )
);

function vectorCommit(): TendermintCommit {
  return structuredClone({
    height: vector.height,
    round: vector.round,
    block_id: vector.block_id,
    signatures: vector.signatures,
  });
}

function signedStorageHeader() {
  const keys = Array.from({ length: 4 }, (_, index) => {
    const seed = sha256(utf8ToBytes(`names-storage-validator-${index}`));
    return { seed, publicKey: ed25519.getPublicKey(seed) };
  });
  const validators: Validator[] = keys.map(({ publicKey }) => ({
    pub_key: Buffer.from(publicKey).toString('base64'),
    voting_power: 10,
  }));
  const validatorsHash = bytesToHex(validatorSetHash(validators)).toUpperCase();
  const header: TendermintHeader = {
    ...structuredClone(vector.header),
    height: String(storageVector.height + 1),
    app_hash: storageVector.app_hash.replace(/^0x/, '').toUpperCase(),
    validators_hash: validatorsHash,
    next_validators_hash: validatorsHash,
  };
  const commit = {
    height: header.height,
    round: vector.round,
    block_id: { ...vector.block_id, hash: bytesToHex(headerHash(header)).toUpperCase() },
    signatures: keys.map(({ publicKey }) => ({
      block_id_flag: 2,
      validator_address: bytesToHex(sha256(publicKey).slice(0, 20)).toUpperCase(),
      timestamp: header.time,
      signature: '',
    })),
  } satisfies TendermintCommit;
  commit.signatures.forEach((signature, index) => {
    signature.signature = Buffer.from(
      ed25519.sign(
        voteSignBytes({ chainId: header.chain_id, commit, signature }),
        keys[index]!.seed
      )
    ).toString('base64');
  });
  return { header, commit, validators };
}

function header_commit_verifies_against_the_consensus_vector_and_refuses_a_forged_app_hash() {
  const commit = vectorCommit();
  const input = {
    chainId: vector.chain_id,
    header: vector.header,
    commit,
    validators: validatorVector.validators,
  };
  expect(vector.sign_bytes).toHaveLength(commit.signatures.length);
  commit.signatures.forEach((signature, index) => {
    expect(bytesToHex(voteSignBytes({ chainId: vector.chain_id, commit, signature }))).toBe(
      vector.sign_bytes[index]
    );
  });
  expect(bytesToHex(headerHash(vector.header)).toUpperCase()).toBe(vector.header_hash);
  const power = BigInt(validatorVector.total_power);
  expect(verifyCommit(input)).toEqual({ verified: true, signedPower: power, totalPower: power });

  const forgedHash = hexToBytes(vector.header.app_hash);
  forgedHash[0] ^= 1;
  const forgedHeader = { ...vector.header, app_hash: bytesToHex(forgedHash) };
  expect(bytesToHex(headerHash(forgedHeader))).not.toBe(vector.header_hash.toLowerCase());
  expect(() => verifyCommit({ ...input, header: forgedHeader })).toThrow(HeaderError);
  expect(() =>
    verifyCommit({
      ...input,
      header: forgedHeader,
      commit: {
        ...commit,
        block_id: { ...vector.block_id, hash: bytesToHex(headerHash(forgedHeader)) },
      },
    })
  ).toThrow('Invalid commit signature');

  const absent: CommitSignature = {
    block_id_flag: 1,
    validator_address: '',
    timestamp: '0001-01-01T00:00:00Z',
    signature: null,
  };
  const threeSigners = { ...commit, signatures: [...commit.signatures.slice(0, 3), absent] };
  expect(verifyCommit({ ...input, commit: threeSigners })).toEqual({
    verified: true,
    signedPower: 30n,
    totalPower: power,
  });
  expect(() =>
    verifyCommit({
      ...input,
      commit: {
        ...commit,
        signatures: [...commit.signatures.slice(0, 2), absent, absent],
      },
    })
  ).toThrow('more than two thirds');
  for (const index of [0, 3]) {
    const tampered = vectorCommit();
    const bytes = Buffer.from(tampered.signatures[index]!.signature!, 'base64');
    bytes[0] ^= 1;
    const signatures = tampered.signatures.map((signature, slot) =>
      slot === index ? { ...signature, signature: bytes.toString('base64') } : signature
    );
    expect(() => verifyCommit({ ...input, commit: { ...tampered, signatures } })).toThrow(
      HeaderError
    );
  }
  expect(() => verifyCommit({ ...input, chainId: 'another-chain' })).toThrow(HeaderError);
  expect(() =>
    verifyCommit({ ...input, commit: { ...commit, height: vector.height + 1 } })
  ).toThrow(HeaderError);
  expect(() => verifyCommit({ ...input, commit: { ...commit, round: 1 } })).toThrow(HeaderError);
  expect(() =>
    verifyCommit({ ...input, commit: { ...commit, signatures: [...commit.signatures].reverse() } })
  ).toThrow(HeaderError);
  expect(() =>
    verifyCommit({
      ...input,
      validators: [input.validators[0]!, input.validators[0]!, ...input.validators.slice(2)],
    })
  ).toThrow(HeaderError);
}

async function resolve_content_uses_only_verified_headers() {
  const requestedHeaders: string[] = [];
  const proofRequests: unknown[] = [];
  const signed = signedStorageHeader();
  const headerHeight = BigInt(storageVector.height) + 1n;
  let servedHeader = structuredClone(signed.header);
  let servedCommit = structuredClone(signed.commit);
  const server = createServer(async (request, response) => {
    response.setHeader('content-type', 'application/json');
    if (request.method === 'GET') {
      requestedHeaders.push(request.url!);
      response.end(
        JSON.stringify({
          result: {
            signed_header: {
              header: servedHeader,
              commit: {
                height: servedCommit.height,
                round: servedCommit.round,
                block_id: {
                  hash: servedCommit.block_id.hash,
                  parts: servedCommit.block_id.part_set_header,
                },
                signatures: servedCommit.signatures,
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
    if (body.method === 'eth_blockNumber') {
      response.end(
        JSON.stringify({
          jsonrpc: '2.0',
          id: body.id,
          result: `0x${storageVector.height.toString(16)}`,
        })
      );
      return;
    }
    proofRequests.push(body);
    response.end(
      JSON.stringify({
        jsonrpc: '2.0',
        id: body.id,
        result: {
          address: storageVector.address,
          height: String(storageVector.height),
          hexValues: [storageVector.value.slice(2)],
          storageProof: [
            {
              ops: storageVector.proof_ops.map((op) => ({
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
    if (address === null || typeof address === 'string') throw new Error('Missing HTTP address');
    const rpcUrl = `http://127.0.0.1:${address.port}`;
    const headers = verifiedTendermintHeaders(rpcUrl, { validators: signed.validators });
    await expect(headers.appHashAt(headerHeight)).resolves.toEqual(
      hexToBytes(signed.header.app_hash)
    );
    const options = { rpcUrl, resolver: storageVector.address, headers };
    await expect(resolveContent(storageVector.name, options)).resolves.toEqual({
      digest: storageVector.value,
      height: BigInt(storageVector.height),
      verified: true,
    });
    expect(proofRequests[0]).toMatchObject({
      method: 'eth_getProof',
      params: [
        storageVector.address,
        expect.any(Array),
        `0x${storageVector.height.toString(16)}`,
      ],
    });
    expect(requestedHeaders).toEqual(Array(2).fill(`/commit?height=${headerHeight}`));

    const signatures = servedCommit.signatures.map((signature, index) => {
      if (index !== 3) return signature;
      const bytes = Buffer.from(signature.signature!, 'base64');
      bytes[0] ^= 1;
      return { ...signature, signature: bytes.toString('base64') };
    });
    servedCommit = { ...servedCommit, signatures };
    let returnedDigest = false;
    const resolution = resolveContent(storageVector.name, options).then((result) => {
      returnedDigest = result !== null;
      return result;
    });
    await expect(resolution).rejects.toThrow(HeaderError);
    await expect(resolution).rejects.toThrow('Invalid commit signature');
    expect(returnedDigest).toBe(false);
    expect(requestedHeaders.at(-1)).toBe(`/commit?height=${headerHeight}`);

    servedCommit = structuredClone(signed.commit);
    servedHeader = { ...signed.header, app_hash: '00'.repeat(32) };
    await expect(resolveContent(storageVector.name, options)).rejects.toThrow(HeaderError);
    servedHeader = { ...signed.header, height: String(headerHeight + 1n) };
    await expect(headers.appHashAt(headerHeight)).rejects.toThrow('wrong header height');
  } finally {
    const closed = new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve()))
    );
    server.closeAllConnections();
    await closed;
  }
}

test(
  header_commit_verifies_against_the_consensus_vector_and_refuses_a_forged_app_hash,
  header_commit_verifies_against_the_consensus_vector_and_refuses_a_forged_app_hash
);
test(resolve_content_uses_only_verified_headers, resolve_content_uses_only_verified_headers);
