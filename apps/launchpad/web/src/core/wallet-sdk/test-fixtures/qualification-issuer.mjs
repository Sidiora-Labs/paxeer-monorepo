import { createServer } from 'node:https';
import { request } from 'node:http';
import { readFileSync, writeFileSync, appendFileSync } from 'node:fs';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import {
  createPublicClient,
  createWalletClient,
  defineChain,
  getAddress,
  http,
  keccak256,
} from 'viem';
import { createSiweMessage } from 'viem/siwe';
import { decodeV2Contract } from '@kindle/sdk';

const config = JSON.parse(readFileSync(process.argv[2], 'utf8'));
const sha = (bytes) => createHash('sha256').update(bytes).digest('hex');
const pinnedClient = readFileSync(new URL('../client.ts', import.meta.url), 'utf8');
if (!pinnedClient.includes('export const PAXEER_CHAIN_ID = 125;'))
  throw new Error('Wallet SDK chain constant changed; review the private issuer');
const chain = defineChain({
  id: 125,
  name: 'Owned wallet qualification',
  nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
  rpcUrls: { default: { http: [config.rpc] } },
});
const client = createPublicClient({ chain, transport: http(config.rpc), cacheTime: 0 });
const wallet = createWalletClient({ chain, transport: http(config.rpc) });
if ((await client.getChainId()) !== chain.id) throw new Error('Owned wallet chain mismatch');
const accounts = await wallet.getAddresses();
const token = JSON.parse(readFileSync(config.artifact, 'utf8'));
const deployment = await wallet.deployContract({
  account: accounts[0],
  abi: token.abi,
  bytecode: token.bytecode.object,
  args: [6],
});
const deployed = await client.waitForTransactionReceipt({ hash: deployment });
if (deployed.status !== 'success' || !deployed.contractAddress)
  throw new Error('Wallet token deployment failed');
const address = deployed.contractAddress;
const transfer = async (recipient) => {
  const hash = await wallet.writeContract({
    account: accounts[0],
    address,
    abi: token.abi,
    functionName: 'transfer',
    args: [recipient, 1000000n],
  });
  const receipt = await client.waitForTransactionReceipt({ hash });
  if (receipt.status !== 'success') throw new Error('Wallet deposit failed');
  appendFileSync(
    config.receipts,
    JSON.stringify({ transactionHash: hash, recipient, receipt }, (_, value) =>
      typeof value === 'bigint' ? value.toString() : value
    ) + '\n',
    { mode: 0o600 }
  );
  return hash;
};
const fundingTransactionHash = await transfer(accounts[1]);
const unrelatedTransactionHash = await transfer(accounts[2]);
const head = await client.getBlock();
const runtime = await client.getCode({ address, blockNumber: head.number });
if (!runtime || runtime === '0x') throw new Error('Wallet activation code is absent');
const sourceSha256 = sha(
  Buffer.concat([
    Buffer.from(pinnedClient),
    readFileSync(new URL('./src/FundingToken.sol', import.meta.url)),
    readFileSync(new URL('./qualification-issuer.mjs', import.meta.url)),
  ])
);
const challenges = new Map();
let origin;
const server = createServer(
  { key: readFileSync(config.key), cert: readFileSync(config.cert), minVersion: 'TLSv1.3' },
  async (req, res) => {
    try {
      if (req.method === 'POST' && req.url === '/rpc') {
        const upstream = request(
          config.rpc,
          { method: 'POST', headers: { 'Content-Type': 'application/json' }, timeout: 15000 },
          (reply) => {
            res.writeHead(reply.statusCode, reply.headers);
            reply.pipe(res);
          }
        );
        upstream.on('error', (error) => res.destroy(error));
        upstream.on('timeout', () => upstream.destroy(new Error('Owned RPC timed out')));
        req.pipe(upstream);
        return;
      }
      if (req.method !== 'POST' || req.url !== '/v2/auth/challenges') {
        res.writeHead(404).end();
        return;
      }
      let raw = '';
      for await (const part of req) {
        raw += part;
        if (Buffer.byteLength(raw) > 4096) throw new Error('Challenge input exceeds bound');
      }
      const body = JSON.parse(raw),
        account = getAddress(body.wallet);
      if (
        req.headers.origin !== origin ||
        body.origin !== origin ||
        (await client.getChainId()) !== 125
      )
        throw new Error('Challenge origin or chain mismatch');
      const issuedAt = new Date(),
        expiresAt = new Date(issuedAt.getTime() + 300000);
      const challengeId = randomUUID(),
        nonce = randomBytes(32).toString('hex');
      const challenge = {
        chainId: '125',
        challengeId,
        domain: new URL(origin).host,
        expiresAt: expiresAt.toISOString(),
        issuedAt: issuedAt.toISOString(),
        nonce,
        purpose: 'login',
        uri: origin,
        wallet: account,
        message: createSiweMessage({
          address: account,
          chainId: 125,
          domain: new URL(origin).host,
          uri: origin,
          version: '1',
          nonce,
          issuedAt,
          expirationTime: expiresAt,
          requestId: challengeId,
          statement: 'Sign in to Sidiora.',
        }),
      };
      for (const [id, record] of challenges)
        if (Date.parse(record.expiresAt) <= Date.now()) challenges.delete(id);
      if (challenges.size >= 1000) {
        res.writeHead(429).end();
        return;
      }
      challenges.set(challengeId, challenge);
      const envelope = {
        data: { challenge },
        generatedAt: new Date().toISOString(),
        nextCursor: null,
        quality: 'complete',
        requestId: randomUUID(),
        revision: null,
        watermark: null,
      };
      decodeV2Contract('ChallengeResponse', envelope);
      appendFileSync(config.challenges, JSON.stringify({ status: 200, body: envelope }) + '\n', { mode: 0o600 });
      res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
      res.end(JSON.stringify(envelope));
    } catch (error) {
      console.error(error);
      const body = { code: 'INVALID_CHALLENGE', message: error.message };
      appendFileSync(config.challenges, JSON.stringify({ status: 400, body }) + '\n', { mode: 0o600 });
      if (!res.headersSent) res.writeHead(400, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(body));
    }
  }
);
await new Promise((resolve) => server.listen(443, '127.0.0.1', resolve));
origin = 'https://127.0.0.1';
const network = {
  schemaVersion: 1,
  deploymentVersion: 'private-wallet125',
  manifestHash: sha(readFileSync(config.artifact)),
  sourceManifestSha256: sourceSha256,
  chainId: 125,
  publicRPC: origin + '/rpc',
  explorerBase: origin,
  native: { symbol: 'PAX', decimals: 18 },
  settlement: { symbol: 'USDL', decimals: 6 },
  addresses: { USDL: address },
  creationFeeRaw: '1000000',
};
writeFileSync(
  config.output,
  JSON.stringify({
    schemaVersion: 2,
    sourceSha256,
    network,
    usdlRuntimeHash: keccak256(runtime),
    committedHead: { number: head.number.toString(), hash: head.hash },
    wallet: accounts[1],
    identityOrigin: origin,
    fundingTransactionHash,
    unrelatedTransactionHash,
  }) + '\n',
  { mode: 0o600, flag: 'wx' }
);
process.on('SIGTERM', () => {
  server.closeAllConnections();
  server.close(() => process.exit(0));
});
