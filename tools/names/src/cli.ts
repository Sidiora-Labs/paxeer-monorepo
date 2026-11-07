#!/usr/bin/env node
import { constants } from 'node:fs';
import { open } from 'node:fs/promises';
import { parseArgs } from 'node:util';
import { getAddress, type Hex } from 'viem';
import { publish } from './publish.ts';
import { verify } from './verify.ts';

async function main(): Promise<void> {
  const [command, ...args] = process.argv.slice(2);
  if (command !== 'publish' && command !== 'verify') {
    throw new Error(
      'Usage: paxnames <publish|verify> --name NAME --bucket-url URL --rpc URL --resolver ADDRESS'
    );
  }
  const { values } = parseArgs({
    args,
    strict: true,
    allowPositionals: false,
    options: {
      dir: { type: 'string' },
      name: { type: 'string' },
      'bucket-url': { type: 'string' },
      's3-endpoint': { type: 'string' },
      's3-bucket': { type: 'string' },
      region: { type: 'string' },
      rpc: { type: 'string' },
      resolver: { type: 'string' },
      'key-file': { type: 'string' },
    },
  });
  function required(key: keyof typeof values): string {
    const value = values[key];
    if (!value) throw new Error(`Missing --${key}`);
    return value;
  }
  const shared = {
    name: required('name'),
    rpcUrl: required('rpc'),
    resolver: getAddress(required('resolver')),
    publicBaseUrl: required('bucket-url'),
  };
  if (command === 'verify') {
    const mismatches = await verify(shared);
    console.log(JSON.stringify(mismatches));
    if (mismatches.length) process.exitCode = 1;
    return;
  }
  const keyFile = await open(required('key-file'), constants.O_RDONLY | constants.O_NOFOLLOW);
  let privateKey: Hex;
  try {
    const stat = await keyFile.stat();
    if (!stat.isFile() || (stat.mode & 0o7777) !== 0o600) {
      throw new Error('Private key file must be a regular file with mode 0600');
    }
    const key = (await keyFile.readFile('utf8')).trim();
    if (!/^0x[0-9a-fA-F]{64}$/.test(key)) throw new Error('Invalid private key file');
    privateKey = key as Hex;
  } finally {
    await keyFile.close();
  }
  const accessKeyId = process.env.AWS_ACCESS_KEY_ID;
  const secretAccessKey = process.env.AWS_SECRET_ACCESS_KEY;
  if (!accessKeyId || !secretAccessKey)
    throw new Error('AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required');
  const result = await publish({
    dir: required('dir'),
    name: shared.name,
    rpcUrl: shared.rpcUrl,
    resolver: shared.resolver,
    privateKey,
    bucket: {
      endpoint: required('s3-endpoint'),
      region: required('region'),
      bucket: required('s3-bucket'),
      publicBaseUrl: shared.publicBaseUrl,
      credentials: { accessKeyId, secretAccessKey, sessionToken: process.env.AWS_SESSION_TOKEN },
    },
  });
  console.log(JSON.stringify(result));
}

main().catch(() => {
  console.error(
    'paxnames failed: check command options, key file permissions, credentials and remote services'
  );
  process.exitCode = 1;
});
