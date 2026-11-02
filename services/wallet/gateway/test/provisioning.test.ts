import { spawn, execFileSync, type ChildProcess } from 'node:child_process';
import { createHash, generateKeyPairSync, randomBytes, randomUUID, sign, X509Certificate } from 'node:crypto';
import { createServer as createHttpServer, type Server } from 'node:http';
import { request as httpsRequest } from 'node:https';
import { createServer as createNetServer } from 'node:net';
import { mkdtempSync, openSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { afterAll, beforeAll, describe, expect, it, test } from 'vitest';
import type { FastifyInstance } from 'fastify';
import { decodeFunctionData, recoverAddress, serializeSignature, type Hex } from 'viem';
import type { SponsoredBindRequest } from '../src/provision/bind.js';
import type { WalletInventoryKey } from '../src/agent/authority.js';
import { generatePrivateKey, privateKeyToAccount, publicKeyToAddress } from 'viem/accounts';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';
import { newAgentKey, startIdentityProvider, type IdentityProvider } from './support/identity.js';
import { ADDR, BIND_SELECTOR, TestChain, addrAbiJson, chainMainAccountId, repoRoot } from './support/chain.js';
import { KERNEL_POLICY } from './e2e/attestors.js';
import type { SequencerReceiptResult } from '../src/rpc/pool.js';

const here = dirname(fileURLToPath(import.meta.url));
const attestorDir = resolve(here, '../../attestor');
const bindVectors = JSON.parse(readFileSync(join(repoRoot, 'layerxproof/testdata/paxeer_bind_vectors.json'), 'utf8')) as {
  public_key: string;
  did: string;
  main_account_name: string;
  main_account_id: string;
  binds: Array<{ name: string; chain_id: string; evm_address: string; nonce: string; message: string; signature: string; valid: boolean }>;
};

const CHAIN_ID = 125;
const PAYMASTER = '0x4444444444444444444444444444444444444444' as const;
const LONG = 900_000;

function reservePort(): Promise<{ port: number; release(): Promise<void> }> {
  return new Promise((resolvePort, reject) => {
    const srv = createNetServer();
    srv.once('error', reject);
    srv.listen(0, '127.0.0.1', () => {
      const addr = srv.address();
      if (!addr || typeof addr === 'string') return reject(new Error('no port'));
      resolvePort({
        port: addr.port,
        release: () => new Promise<void>((resolveRelease, rejectRelease) => {
          srv.close((error) => error ? rejectRelease(error) : resolveRelease());
        }),
      });
    });
  });
}

let idp: IdentityProvider;
let pg: EphemeralPostgres;
let app: FastifyInstance;
let chain: TestChain;
let sequencer: Server;
let station: Server;
const quoteRequests: Array<{ account: Hex }> = [];
const submitRequests: SponsoredBindRequest[] = [];
const submissions = new Map<string, { request: SponsoredBindRequest; hash: Hex }>();
const stationErrors: unknown[] = [];
let failAfterSubmit = false;
let failReceipt = false;
const sequencerRequests: Array<{ method: string; params: Hex[] }> = [];
const committedReceipts = new Set<Hex>();
let workDir: string;
let daemons: ChildProcess[] = [];
let sponsorAddress: string;
let poolModule: typeof import('../src/db/pool.js');
let walletsModule: typeof import('../src/db/wallets.js');
let bindModule: typeof import('../src/provision/bind.js');
let stateModule: typeof import('../src/provision/state.js');
let backfillModule: typeof import('../src/provision/backfill.js');
let routesModule: typeof import('../src/routes/wallet.js');
let clientTls: { cert: Buffer; key: Buffer; ca: Buffer };
const endpoints: string[] = [];
const inventoryKeys = new Map<string, WalletInventoryKey>();
let approveInventory: (keys: WalletInventoryKey[]) => void;

function ossl(args: string[]): void {
  execFileSync('openssl', args, { stdio: 'pipe' });
}

function issueCertificates(dir: string, names: string[]): void {
  const p = (n: string): string => join(dir, n);
  const ec = ['-newkey', 'ec', '-pkeyopt', 'ec_paramgen_curve:prime256v1', '-nodes'];
  ossl(['req', '-x509', ...ec, '-keyout', p('ca.key'), '-out', p('ca.crt'), '-days', '1', '-subj', '/CN=provisioning-test-ca']);
  writeFileSync(p('leaf.ext'), 'subjectAltName=IP:127.0.0.1\nextendedKeyUsage=serverAuth,clientAuth\n');
  for (const name of names) {
    ossl(['req', ...ec, '-keyout', p(`${name}.key`), '-out', p(`${name}.csr`), '-subj', `/CN=${name}`]);
    ossl(['x509', '-req', '-in', p(`${name}.csr`), '-CA', p('ca.crt'), '-CAkey', p('ca.key'), '-CAcreateserial', '-out', p(`${name}.crt`), '-days', '1', '-extfile', p('leaf.ext')]);
  }
}

function spkiPin(certPath: string): string {
  const cert = new X509Certificate(readFileSync(certPath));
  return createHash('sha256').update(cert.publicKey.export({ type: 'spki', format: 'der' })).digest('hex');
}

function health(url: string): Promise<{ ready: boolean }> {
  return new Promise((resolveHealth) => {
    const req = httpsRequest(new URL('/health', url), { method: 'GET', ...clientTls, timeout: 5_000 }, (res) => {
      const chunks: Buffer[] = [];
      res.on('data', (c: Buffer) => chunks.push(c));
      res.on('end', () => {
        try {
          resolveHealth(JSON.parse(Buffer.concat(chunks).toString('utf8')) as { ready: boolean });
        } catch {
          resolveHealth({ ready: false });
        }
      });
    });
    req.on('error', () => resolveHealth({ ready: false }));
    req.on('timeout', () => req.destroy());
    req.end();
  });
}

beforeAll(async () => {
  workDir = mkdtempSync(join(tmpdir(), 'provisioning-'));
  const bin = join(workDir, 'attestor');
  execFileSync('go', ['build', '-o', bin, './cmd/attestor'], { cwd: attestorDir, stdio: 'pipe', timeout: 600_000 });

  idp = await startIdentityProvider();
  chain = new TestChain(CHAIN_ID);
  await chain.start();
  sequencer = createHttpServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (chunk: Buffer) => chunks.push(chunk));
    req.on('end', async () => {
      const body = JSON.parse(Buffer.concat(chunks).toString('utf8')) as { id: number; method: string; params: Hex[] };
      sequencerRequests.push(body);
      if (body.method === 'eth_getCode' || (body.method === 'eth_call' && typeof body.params[0] === 'object' && body.params[0] !== null && (body.params[0] as unknown as { data: string }).data === '0xaffed0e0')) {
        const result = body.method === 'eth_getCode' ? (body.params[0] === PAYMASTER ? '0x60005460005260206000f3' : '0x') : `0x${'00'.repeat(32)}`;
        res.writeHead(200, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result }));
        return;
      }
      if (body.method === 'pax_sequencerReceipt') {
        if (failReceipt) {
          failReceipt = false;
          res.writeHead(200, { 'content-type': 'application/json' });
          res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, error: { code: -32000, message: 'injected pax_sequencerReceipt failure' } }));
          return;
        }
        const hash = body.params[0]!;
        const receipt = chain.receipts.get(hash);
        const tx = chain.sent.find((sent) => sent.hash === hash);
        let result: SequencerReceiptResult | null = null;
        if (receipt && tx) {
          result = {
            txHash: hash,
            nonce: `0x${(chain.nonces.get(tx.from)! - 1).toString(16)}`,
            rung: 'receipted',
            seq: receipt.blockNumber,
            height: receipt.blockNumber,
            index: '0x0',
            timestamp: '0x0',
            status: receipt.status,
            returnData: '0x',
            logs: [],
            gasUsed: receipt.gasUsed,
            cumulativeGasUsed: receipt.gasUsed,
            effectiveGasPrice: `0x${(chain.baseFee + chain.tip).toString(16)}`,
            feeDenom: 'pax',
            feeAmount: `0x${(BigInt(receipt.gasUsed) * (chain.baseFee + chain.tip)).toString(16)}`,
            sponsor: [...submissions.values()].some((submission) => submission.hash === hash) ? sponsorAddress as Hex : `0x${'00'.repeat(20)}`,
            from: tx.from as Hex,
            to: tx.to as Hex,
            contractAddress: `0x${'00'.repeat(20)}`,
            vmError: receipt.status === '0x1' ? '' : 'execution reverted',
            keyId: '0x',
            receipt: '0x',
          };
          if (committedReceipts.has(hash)) {
            result = { ...result, rung: 'committed', blockNumber: receipt.blockNumber, blockHash: `0x${'01'.repeat(32)}` };
          }
        }
        res.writeHead(200, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result }));
        return;
      }
      try {
        const response = await fetch(chain.url, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body) });
        res.writeHead(response.status, { 'content-type': 'application/json' });
        res.end(await response.text());
      } catch (err) {
        res.writeHead(500);
        res.end(String(err));
      }
    });
  });
  await new Promise<void>((resolveSequencer) => sequencer.listen(0, '127.0.0.1', resolveSequencer));
  const sequencerAddress = sequencer.address();
  if (!sequencerAddress || typeof sequencerAddress === 'string') throw new Error('sequencer did not bind');

  const authority = generateKeyPairSync('ec', { namedCurve: 'prime256v1' });
  const authorityPrivateKeyFile = join(workDir, 'authority-private.pem');
  const authorityPublicKeyFile = join(workDir, 'authority-public.pem');
  writeFileSync(authorityPrivateKeyFile, authority.privateKey.export({ type: 'pkcs8', format: 'pem' }), { mode: 0o600 });
  writeFileSync(authorityPublicKeyFile, authority.publicKey.export({ type: 'spki', format: 'pem' }));

  const nodes = ['node-1', 'node-2', 'node-3', 'node-4', 'node-5'];
  issueCertificates(workDir, [...nodes, 'gateway']);
  clientTls = {
    cert: readFileSync(join(workDir, 'gateway.crt')),
    key: readFileSync(join(workDir, 'gateway.key')),
    ca: readFileSync(join(workDir, 'ca.crt')),
  };
  const policyFile = join(workDir, 'policy.json');
  writeFileSync(
    policyFile,
    JSON.stringify({
      version: 1,
      defaults: {
        chain_id: CHAIN_ID,
        kinds: ['evm_tx', 'lx_bind', 'sponsored_batch', 'eip7702_authorization'],
        authorization_delegates: [PAYMASTER],
        rate_per_minute: 1000,
        caps: {
          native: { per_transaction: '1000000000000000000', daily: '10000000000000000000' },
          '0x21f7b20a555199fa73A238B1a91FD0f549068fEe': { per_transaction: '1000000', daily: '1000000' },
        },
        selectors: { [ADDR]: [BIND_SELECTOR] },
      },
    }),
  );
  const kernelPolicyFile = join(workDir, 'kernel-policy.json');
  writeFileSync(kernelPolicyFile, JSON.stringify(KERNEL_POLICY));
  const reservedPorts = await Promise.all(Array.from({ length: nodes.length * 2 }, () => reservePort()));
  const apiPorts = reservedPorts.slice(0, nodes.length).map(({ port }) => port);
  const peerPorts = reservedPorts.slice(nodes.length).map(({ port }) => port);
  const pins = nodes.map((n) => spkiPin(join(workDir, `${n}.crt`)));
  const inventoryFile = join(workDir, 'inventory.jwt');
  const issuedAt = Math.floor(Date.now() / 1000);
  const inventoryHeader = Buffer.from(JSON.stringify({ alg: 'ES256', typ: 'wallet-custody-inventory+jwt' })).toString('base64url');
  const inventoryPayload = Buffer.from(JSON.stringify({
    version: 1,
    iss: `${idp.url}/auth/v1`,
    aud: 'wallet-custody-inventory',
    tenant: 'provisioning-test',
    sequence: '1',
    iat: issuedAt,
    exp: issuedAt + 3600,
    protocol: 'wallet',
    threshold: 3,
    members: nodes.map((id, i) => ({ id, spki_sha256: pins[i]! })),
    keys: [],
  })).toString('base64url');
  const inventoryInput = `${inventoryHeader}.${inventoryPayload}`;
  const inventorySignature = sign('sha256', Buffer.from(inventoryInput), { key: authority.privateKey, dsaEncoding: 'ieee-p1363' });
  writeFileSync(inventoryFile, `${inventoryInput}.${inventorySignature.toString('base64url')}`);
  let inventorySequence = 1;
  approveInventory = (keys) => {
    const claims = JSON.parse(Buffer.from(inventoryPayload, 'base64url').toString('utf8'));
    claims.sequence = String(++inventorySequence);
    claims.keys = keys;
    const payload = Buffer.from(JSON.stringify(claims)).toString('base64url');
    const input = `${inventoryHeader}.${payload}`;
    const signature = sign('sha256', Buffer.from(input), { key: authority.privateKey, dsaEncoding: 'ieee-p1363' });
    writeFileSync(inventoryFile, `${input}.${signature.toString('base64url')}`);
  };
  for (const [i, node] of nodes.entries()) {
    const others = nodes.map((n, j) => ({ n, j })).filter((x) => x.j !== i);
    const keyFile = join(workDir, `${node}.store`);
    writeFileSync(keyFile, randomBytes(32).toString('hex'));
    const log = openSync(join(workDir, `${node}.log`), 'w');
    await Promise.all([reservedPorts[i]!.release(), reservedPorts[i + nodes.length]!.release()]);
    const child = spawn(bin, [], {
      env: {
        PATH: process.env.PATH ?? '',
        ATTESTOR_NODE_ID: node,
        ATTESTOR_REGION: `region-${i + 1}`,
        ATTESTOR_LISTEN_ADDR: `127.0.0.1:${apiPorts[i]}`,
        ATTESTOR_PEER_LISTEN_ADDR: `127.0.0.1:${peerPorts[i]}`,
        ATTESTOR_PEERS: others.map((x) => `${x.n}=127.0.0.1:${peerPorts[x.j]}`).join(','),
        ATTESTOR_PEER_PINS: others.map((x) => `${x.n}=${pins[x.j]}`).join(','),
        ATTESTOR_NODE_KEY_FILE: keyFile,
        ATTESTOR_DATA_DIR: join(workDir, `${node}-data`),
        ATTESTOR_CHAIN_ID: String(CHAIN_ID),
        ATTESTOR_AGENT_PAYMASTER: PAYMASTER,
        ATTESTOR_JWKS_URL: `${idp.url}/auth/v1/.well-known/jwks.json`,
        ATTESTOR_JWT_ISSUER: `${idp.url}/auth/v1`,
        ATTESTOR_JWT_AUDIENCE: 'authenticated',
        ATTESTOR_AUTHORITY_PUBLIC_KEY_FILE: authorityPublicKeyFile,
        ATTESTOR_AUTHORITY_ISSUER: `${idp.url}/auth/v1`,
        ATTESTOR_AUTHORITY_TENANT: 'provisioning-test',
        ATTESTOR_INVENTORY_PUBLIC_KEY_FILE: authorityPublicKeyFile,
        ATTESTOR_INVENTORY_FILE: inventoryFile,
        ATTESTOR_POLICY_FILE: policyFile,
        ATTESTOR_TLS_CERT_FILE: join(workDir, `${node}.crt`),
        ATTESTOR_TLS_KEY_FILE: join(workDir, `${node}.key`),
        ATTESTOR_TLS_CA_FILE: join(workDir, 'ca.crt'),
        ATTESTOR_OPERATOR_CA_FILE: join(workDir, 'ca.crt'),
        ATTESTOR_KERNEL_POLICY_FILE: kernelPolicyFile,
        ATTESTOR_RPC_URL: chain.url,
        ATTESTOR_ACTIVITY_TYPES: '0x10005',
      },
      stdio: ['ignore', log, log],
    });
    daemons.push(child);
    endpoints.push(`https://127.0.0.1:${apiPorts[i]}`);
  }
  const deadline = Date.now() + 120_000;
  for (;;) {
    const states = await Promise.all(endpoints.map((u) => health(u)));
    if (states.every((s) => s.ready)) break;
    if (daemons.some((child) => child.exitCode !== null || child.signalCode !== null) || Date.now() > deadline) {
      const logs = nodes.map((n) => readFileSync(join(workDir, `${n}.log`), 'utf8')).join('\n');
      throw new Error(`attestors did not become ready: ${logs}`);
    }
    await new Promise((r) => setTimeout(r, 300));
  }

  const relayer = privateKeyToAccount(generatePrivateKey());
  sponsorAddress = relayer.address.toLowerCase();
  chain.balances.set(sponsorAddress, 10n ** 21n);
  const issuedQuotes = new Map<string, SponsoredBindRequest['batch']['quote']>();
  station = createHttpServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (chunk: Buffer) => chunks.push(chunk));
    req.on('end', async () => {
      try {
        const { constructionDigest, sponsorQuoteDigest } = await import('../src/attestor/client.js');
        expect(req.method).toBe('POST');
        expect(req.headers['content-type']).toBe('application/json');
        if (req.url === '/quote') {
          const body = JSON.parse(Buffer.concat(chunks).toString('utf8')) as Omit<SponsoredBindRequest['batch'], 'quote'> & {
            token: Hex; maxTokenAmount: string; gasCost: string; decimals: number;
          };
          expect(body).toMatchObject({ chainId: String(CHAIN_ID), nonce: '0', token: bindModule.SIDIORA_TOKEN, decimals: 6, maxTokenAmount: '1000000' });
          expect(body.calls).toHaveLength(1);
          expect(body.calls[0]!.to).toBe(ADDR);
          expect(body.calls[0]!.value).toBe('0');
          expect(body.calls[0]!.data.slice(0, 10)).toBe(BIND_SELECTOR);
          try {
            await deps().rpc.request('eth_estimateGas', [{ from: body.account, to: body.calls[0]!.to, data: body.calls[0]!.data }]);
          } catch (error) {
            res.writeHead(503, { 'content-type': 'application/json' });
            res.end(JSON.stringify({ error: String(error) }));
            return;
          }
          quoteRequests.push(body);
          const quote: SponsoredBindRequest['batch']['quote'] = {
            sponsor: relayer.address, token: body.token, maxTokenAmount: body.maxTokenAmount,
            tokenAmount: '100', deadline: String(Math.floor(Date.now() / 1000) + 300),
            quoteNonce: String(quoteRequests.length), gasCost: body.gasCost, decimals: 6,
          };
          issuedQuotes.set(body.account.toLowerCase(), quote);
          const construction = { kind: 'sponsored_batch', chainId: body.chainId, account: body.account, nonce: body.nonce, calls: body.calls, quote };
          const relayerSignature = await relayer.sign({ hash: sponsorQuoteDigest(construction) });
          res.writeHead(200, { 'content-type': 'application/json' });
          res.end(JSON.stringify({ quote, relayerSignature }));
          return;
        }
        expect(req.url).toBe('/submit');
        const request = JSON.parse(Buffer.concat(chunks).toString('utf8')) as SponsoredBindRequest;
        submitRequests.push(request);
        const account = request.batch.account.toLowerCase();
        expect(request.batch.quote).toEqual(issuedQuotes.get(account));
        const construction = { kind: 'sponsored_batch', ...request.batch };
        expect(await recoverAddress({ hash: constructionDigest(construction), signature: request.accountSignature })).toBe(request.batch.account);
        expect(await recoverAddress({ hash: sponsorQuoteDigest(construction), signature: request.relayerSignature as Hex })).toBe(relayer.address);
        const authorization = request.authorization;
        expect(authorization).toMatchObject({ chainId: String(CHAIN_ID), address: PAYMASTER, nonce: '0' });
        expect(await recoverAddress({ hash: constructionDigest({ kind: 'eip7702_authorization', chainId: authorization.chainId, address: authorization.address, nonce: authorization.nonce }), signature: serializeSignature(authorization) })).toBe(request.batch.account);
        const decoded = decodeFunctionData({ abi: bindModule.sponsoredBindAbi, data: request.call.data });
        expect(decoded.functionName).toBe('executeSponsored');
        expect(decoded.args![0]).toEqual(request.batch.calls.map((call) => ({ ...call, value: BigInt(call.value) })));
        expect(decoded.args![2]).toBe(request.accountSignature);
        expect(decoded.args![3]).toBe(request.relayerSignature);
        expect(request.call.to).toBe(request.batch.account);
        expect(request.call.value).toBe('0');
        const persisted = await poolModule.getPool().query('select p.station_request from account_provisioning p join wallets w on w.id = p.wallet_id where lower(w.address) = $1', [account]);
        expect(persisted.rows).toEqual([{ station_request: request }]);
        let submitted = submissions.get(account);
        if (submitted) {
          expect(request).toEqual(submitted.request);
        } else {
          const raw = await relayer.signTransaction({
            type: 'eip7702', chainId: CHAIN_ID, nonce: chain.nonces.get(sponsorAddress) ?? 0,
            to: request.call.to, value: 0n, data: request.call.data, gas: 200000n,
            maxFeePerGas: BigInt(request.batch.quote.gasCost) / 200000n, maxPriorityFeePerGas: chain.tip,
            authorizationList: [{ ...authorization, chainId: Number(authorization.chainId), nonce: Number(authorization.nonce) }],
          });
          const hash = await deps().rpc.sendRawTransaction(raw);
          expect(chain['applyBind'](account, request.batch.calls[0]!.data)).toBe(true);
          submitted = { request, hash };
          submissions.set(account, submitted);
        }
        if (failAfterSubmit) {
          failAfterSubmit = false;
          res.writeHead(503, { 'content-type': 'application/json' });
          res.end(JSON.stringify({ error: 'gas_station_unavailable' }));
          return;
        }
        res.writeHead(200, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ transactionHash: submitted.hash }));
      } catch (error) {
        stationErrors.push(error);
        res.writeHead(500, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ error: 'station_contract_violation' }));
      }
    });
  });
  await new Promise<void>((resolveStation) => station.listen(0, '127.0.0.1', resolveStation));
  const stationAddress = station.address();
  if (!stationAddress || typeof stationAddress === 'string') throw new Error('station did not bind');

  process.env.SUPABASE_URL = idp.url;
  process.env.LOG_LEVEL = 'error';
  process.env.RPC_URLS = `http://127.0.0.1:${sequencerAddress.port}`;
  process.env.SEQUENCER_RPC_URL = `http://127.0.0.1:${sequencerAddress.port}`;
  process.env.HYPERPAXEER_CHAIN_ID = String(CHAIN_ID);
  process.env.ATTESTOR_ENDPOINTS = endpoints.join(',');
  process.env.ATTESTOR_CLIENT_CERT_FILE = join(workDir, 'gateway.crt');
  process.env.ATTESTOR_CLIENT_KEY_FILE = join(workDir, 'gateway.key');
  process.env.ATTESTOR_CA_FILE = join(workDir, 'ca.crt');
  process.env.ATTESTOR_QUORUM = '3';
  process.env.ATTESTOR_TIMEOUT_MS = String(LONG);
  process.env.WALLET_GAS_STATION_URL = `http://127.0.0.1:${stationAddress.port}`;
  process.env.PAXEER_GAS_STATION_PAYMASTER = PAYMASTER;
  process.env.PAXEER_GAS_STATION_GAS_LIMIT = '200000';
  process.env.PAXEER_GAS_STATION_MAX_TOKEN_AMOUNT = '1000000';
  process.env.ACCOUNT_SETUP_GAS_CAP_WEI = '1000000000000000';
  process.env.WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE = authorityPrivateKeyFile;
  process.env.WALLET_IDENTITY_BINDING_TENANT = 'provisioning-test';
  process.env.WALLET_CUSTODY_INVENTORY_FILE = inventoryFile;
  process.env.WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE = authorityPublicKeyFile;
  pg = await startPostgres();
  const index = await import('../src/index.js');
  poolModule = await import('../src/db/pool.js');
  walletsModule = await import('../src/db/wallets.js');
  bindModule = await import('../src/provision/bind.js');
  stateModule = await import('../src/provision/state.js');
  backfillModule = await import('../src/provision/backfill.js');
  routesModule = await import('../src/routes/wallet.js');
  app = await index.buildApp();
  await app.ready();
}, LONG);

afterAll(async () => {
  await app?.close();
  routesModule?.provisionDepsFromEnv()?.attestors?.close();
  await poolModule?.closePool();
  await pg?.stop();
  for (const d of daemons) d.kill('SIGTERM');
  if (sequencer) await new Promise<void>((resolveSequencer, reject) => sequencer.close((err) => err ? reject(err) : resolveSequencer()));
  if (station) await new Promise<void>((resolveStation, reject) => station.close((err) => err ? reject(err) : resolveStation()));
  await chain?.stop();
  await idp?.stop();
  if (workDir) rmSync(workDir, { recursive: true, force: true });
}, 120_000);

async function asUser(userId: string, method: 'GET' | 'POST', url: string) {
  return app.inject({ method, url, headers: { authorization: `Bearer ${await idp.mintUserToken(userId)}` } });
}

function deps() {
  const d = routesModule.provisionDepsFromEnv();
  if (!d) throw new Error('provisioning is not configured');
  return d;
}

function admitStandardKeys(userId: string): void {
  for (const curve of ['secp256k1', 'ed25519'] as const) {
    const keyId = `${curve === 'secp256k1' ? 'wallet' : 'account'}:${userId}:standard:${curve}:0`;
    inventoryKeys.set(keyId, { key_id: keyId, owner: userId, curve, epoch: 0, public_key: '', operations: ['generate'] });
  }
  approveInventory([...inventoryKeys.values()]);
}

async function approveStandardSigning(userId: string, kind = 'standard'): Promise<void> {
  const row = await provisionRow(userId, kind);
  const wallet = (await poolModule.getPool().query(
    'select address, attestor_key_id, layerx_key_id from wallets where id = $1',
    [row.wallet_id],
  )).rows[0];
  expect(wallet.attestor_key_id).toBe(row.evm_key_id);
  expect(wallet.layerx_key_id).toBe(row.ed_key_id);
  for (const admitted of inventoryKeys.values()) {
    if (admitted.key_id !== row.evm_key_id && admitted.key_id !== row.ed_key_id) continue;
    if (admitted.curve === 'ed25519') {
      expect(row.ed_public_key).toMatch(/^[0-9a-f]{64}$/);
      inventoryKeys.set(admitted.key_id, { ...admitted, public_key: row.ed_public_key as string, operations: ['sign'] });
      continue;
    }
    const held = await Promise.all(endpoints.map((endpoint) => new Promise<WalletInventoryKey>((resolveKey, reject) => {
      const request = httpsRequest(new URL('/v1/keys/describe', endpoint), {
        method: 'POST', ...clientTls, headers: { 'content-type': 'application/json' }, timeout: 5_000,
      }, (response) => {
        const chunks: Buffer[] = [];
        response.on('data', (chunk: Buffer) => chunks.push(chunk));
        response.on('end', () => {
          try {
            expect(response.statusCode).toBe(200);
            const key = JSON.parse(Buffer.concat(chunks).toString('utf8')) as WalletInventoryKey;
            expect(key).toMatchObject({ key_id: admitted.key_id, owner: admitted.owner, curve: admitted.curve, epoch: 0 });
            resolveKey(key);
          } catch (error) { reject(error); }
        });
      });
      request.on('error', reject);
      request.on('timeout', () => request.destroy(new Error('key description timed out')));
      request.end(JSON.stringify({ session_id: randomUUID(), key_id: admitted.key_id }));
    })));
    expect(new Set(held.map((key) => key.public_key)).size).toBe(1);
    expect(publicKeyToAddress(`0x${held[0]!.public_key}`).toLowerCase()).toBe(wallet.address.toLowerCase());
    inventoryKeys.set(admitted.key_id, { ...admitted, public_key: held[0]!.public_key, operations: ['sign'] });
  }
  approveInventory([...inventoryKeys.values()]);
}

async function provisionRow(userId: string, kind = 'standard') {
  const { rows } = await poolModule.getPool().query(
    `select * from account_provisioning where user_id = $1 and kind = $2`,
    [userId, kind],
  );
  return rows[0] as Record<string, unknown>;
}

function sentTo(address: string) {
  return chain.sent.filter((t) => t.to === address.toLowerCase());
}

function sentFrom(address: string) {
  return chain.sent.filter((t) => t.from === address.toLowerCase());
}

async function signedBindTransaction() {
  const account = privateKeyToAccount(generatePrivateKey());
  const identity = newAgentKey('bind-receipt');
  chain.balances.set(account.address.toLowerCase(), 10n ** 18n);
  const message = bindModule.bindMessage(BigInt(CHAIN_ID), account.address, 0n);
  const raw = await account.signTransaction(bindModule.bindLayerX({
    chainId: CHAIN_ID,
    address: account.address,
    publicKey: identity.publicKeyHex,
    signature: identity.sign(message),
    nonce: 0,
    gas: 100_000n,
    maxFeePerGas: 2_000_000_000n,
    maxPriorityFeePerGas: chain.tip,
  }));
  return { raw, hash: bindModule.transactionHash(raw), address: account.address.toLowerCase(), identity };
}

async function bindReadsSequencerReceipt(): Promise<void> {
  const rpc = deps().rpc;
  const { raw, hash, address, identity } = await signedBindTransaction();
  chain.fail('eth_getTransactionReceipt');
  const legacyRead = chain.faults[chain.faults.length - 1]!;
  try {
    expect(await bindModule.readReceipt(rpc, hash)).toBeNull();
    await expect(bindModule.waitForReceipt(rpc, hash, 30, 5)).rejects.toThrow(`transaction ${hash} has no receipt after 30 ms`);
    const [receipt, sentHash] = await Promise.all([
      bindModule.waitForReceipt(rpc, hash, 5_000, 10),
      (async () => {
        await new Promise((resolveSend) => setTimeout(resolveSend, 30));
        return rpc.sendRawTransaction(raw);
      })(),
    ]);
    expect(sentHash).toBe(hash);
    const executed = chain.receipts.get(hash)!;
    expect(receipt).toEqual({ status: 'success', gasUsed: BigInt(executed.gasUsed), blockNumber: BigInt(executed.blockNumber) });
    expect(await bindModule.readReceipt(rpc, hash)).toEqual(receipt);
    committedReceipts.add(hash);
    expect(await bindModule.waitForReceipt(rpc, hash, 5_000, 10)).toEqual(receipt);
    expect(sequencerRequests.filter((request) => request.method === 'pax_sequencerReceipt' && request.params[0] === hash).length).toBeGreaterThan(2);
    expect(chain.bindings.get(address)).toBe(identity.publicKeyHex);
    expect(chain.bindNonces.get(address)).toBe(1n);
    expect(legacyRead.remaining).toBe(1);
  } finally {
    chain.faults.splice(chain.faults.indexOf(legacyRead), 1);
  }
}

async function bindBroadcastIsIdempotent(): Promise<void> {
  const rpc = deps().rpc;
  const { raw, hash, address, identity } = await signedBindTransaction();
  expect(await bindModule.broadcastOnce(rpc, raw)).toBe(hash);
  expect(await bindModule.readReceipt(rpc, hash)).toMatchObject({ status: 'success' });
  expect(sentFrom(address)).toHaveLength(1);
  expect(sequencerRequests.filter((request) => request.method === 'eth_sendRawTransaction' && request.params[0] === raw)).toHaveLength(1);
  expect(chain.bindings.get(address)).toBe(identity.publicKeyHex);
  chain.fail('eth_sendRawTransaction');
  const duplicateSend = chain.faults[chain.faults.length - 1]!;
  try {
    expect(await bindModule.broadcastOnce(rpc, raw)).toBe(hash);
    expect(await bindModule.broadcastOnce(rpc, raw)).toBe(hash);
    expect(duplicateSend.remaining).toBe(1);
    expect(sentFrom(address)).toHaveLength(1);
    expect(sequencerRequests.filter((request) => request.method === 'eth_sendRawTransaction' && request.params[0] === raw)).toHaveLength(1);
    expect(chain.bindNonces.get(address)).toBe(1n);
  } finally {
    chain.faults.splice(chain.faults.indexOf(duplicateSend), 1);
  }
}

test(bindReadsSequencerReceipt);
test(bindBroadcastIsIdempotent);

describe('provisioning derivations', () => {
  it('provisioning derives the DID, main account and bind message of the published vectors', () => {
    expect(bindModule.didFromPublicKey(bindVectors.public_key)).toBe(bindVectors.did);
    expect(bindModule.mainAccountName(bindVectors.did)).toBe(bindVectors.main_account_name);
    expect(bindModule.mainAccountId(bindVectors.did)).toBe(bindVectors.main_account_id);
    for (const v of bindVectors.binds) {
      const message = bindModule.bindMessage(BigInt(v.chain_id), `0x${v.evm_address}`, BigInt(v.nonce));
      expect(bindModule.verifyEd25519(bindVectors.public_key, message, v.signature)).toBe(v.valid);
      if (v.valid) expect(message.toString('hex')).toBe(v.message);
    }
    const calldata = bindModule.bindLayerXCalldata(bindVectors.public_key, bindVectors.binds[1]!.signature);
    const decoded = decodeFunctionData({ abi: addrAbiJson, data: calldata });
    expect(decoded.functionName).toBe('bindLayerX');
    expect(decoded.args).toEqual([`0x${bindVectors.public_key}`, `0x${bindVectors.binds[1]!.signature}`]);
    expect(calldata.slice(0, 10)).toBe('0xdd9aa628');
  });
});

const provisioningFlowTests = {
  async 'provisioning creates both keys, has the gas station sponsor the binding and confirms it before active'() {
    const user = randomUUID();
    admitStandardKeys(user);
    const first = await asUser(user, 'POST', '/v1/wallet/provision');
    expect(first.statusCode, first.body).toBe(500);
    expect(first.json()).toEqual({
      error: 'wallet_provision_failed',
      detail: `owner-approved wallet inventory does not admit sign for account:${user}:standard:ed25519:0`,
    });
    const pending = await provisionRow(user);
    expect(pending.state).toBe('identity');
    expect(pending.station_request).toBeNull();
    expect(pending.bind_tx_hash).toBeNull();
    await approveStandardSigning(user);
    const res = await asUser(user, 'POST', '/v1/wallet/provision');
    expect(res.statusCode, res.body).toBe(200);
    const body = res.json() as { wallet: { id: string; address: `0x${string}`; did: string; main_account_id: string; binding_state: string }; provisioning: { state: string } };
    expect(body.provisioning.state).toBe('active');
    expect(body.wallet.binding_state).toBe('bound');
    expect(body.wallet.id).toBe(pending.wallet_id);
    const address = body.wallet.address.toLowerCase();
    const pub = chain.bindings.get(address)!;
    expect(pub).toMatch(/^[0-9a-f]{64}$/);
    expect(body.wallet.did).toBe(`did:layerx:${pub}`);
    expect(body.wallet.main_account_id).toBe(chainMainAccountId(pub));
    expect(chain.bindNonces.get(address)).toBe(1n);

    const row = await provisionRow(user);
    expect(row.state).toBe('active');
    expect(row.evm_key_id).toBe(`wallet:${user}:standard:secp256k1:0`);
    expect(row.ed_key_id).toBe(`account:${user}:standard:ed25519:0`);
    expect(stationErrors).toEqual([]);
    const submitted = submissions.get(address)!;
    expect(quoteRequests.filter((request) => request.account.toLowerCase() === address)).toHaveLength(1);
    expect(submitRequests.filter((request) => request.batch.account.toLowerCase() === address)).toEqual([submitted.request]);
    expect(row.station_request).toEqual(submitted.request);
    expect(row.bind_tx_hash).toBe(submitted.hash);
    expect(row.topup_tx_hash).toBeNull();
    expect(row.topup_raw_tx).toBeNull();
    expect(row.topup_value_wei).toBeNull();
    expect(sentTo(address).filter((tx) => tx.value > 0n)).toHaveLength(0);
    const binds = sentTo(address);
    expect(binds).toHaveLength(1);
    expect(binds[0]!.from).toBe(sponsorAddress);
    expect(binds[0]!.value).toBe(0n);
    expect(binds[0]!.data).toBe(submitted.request.call.data);
    expect(binds[0]!.hash).toBe(submitted.hash);
    expect(sentFrom(address)).toHaveLength(0);
    expect(chain.receipts.get(submitted.hash)!.status).toBe('0x1');
    expect(chain.balances.get(address)).toBe(0n);

    const wallet = await poolModule.getPool().query(
      `select migrated_at, attestor_key_id, encrypted_private_key, did, main_account_id, layerx_key_id, binding_state from wallets where id = $1`,
      [body.wallet.id],
    );
    expect(wallet.rows[0]).toMatchObject({
      attestor_key_id: row.evm_key_id,
      encrypted_private_key: null,
      did: body.wallet.did,
      main_account_id: body.wallet.main_account_id,
      layerx_key_id: row.ed_key_id,
      binding_state: 'bound',
    });
    expect(wallet.rows[0].migrated_at).not.toBeNull();
    const audit = await poolModule.getPool().query(
      `select event, tx_hash, value_wei::text from account_setup_audit where address = $1 order by id`,
      [address],
    );
    expect(audit.rows.map((r) => r.event)).toEqual(['bind']);
    expect(audit.rows[0].tx_hash).toBe(submitted.hash);

    const me = await asUser(user, 'GET', '/v1/wallet/me');
    expect(me.statusCode).toBe(200);
    expect(me.json()).toMatchObject({
      wallet: { id: body.wallet.id, address: body.wallet.address, chain_id: CHAIN_ID, did: body.wallet.did, main_account_id: body.wallet.main_account_id, binding_state: 'bound' },
      chain: { id: CHAIN_ID },
      kernel: { state: 'available' },
    });
    expect((me.json() as { wallet: Record<string, unknown> }).wallet.created_at).toBeTruthy();

    const sentBefore = chain.sent.length;
    const again = await asUser(user, 'POST', '/v1/wallet/provision');
    expect(again.statusCode).toBe(200);
    expect(again.json()).toMatchObject({ wallet: { id: body.wallet.id, did: body.wallet.did, binding_state: 'bound' } });
    expect(chain.sent.length).toBe(sentBefore);
    expect(submitRequests.filter((request) => request.batch.account.toLowerCase() === address)).toHaveLength(1);
  },

  async 'provisioning resumes after a failure at each step without a second key, station request or binding'() {
    const user = randomUUID();
    admitStandardKeys(user);
    const d = deps();
    const token = async () => idp.mintUserToken(user);

    chain.fail('getUnifiedAccount');
    await expect(stateModule.provisionAccount(d, { kind: 'standard', userId: user, token: await token() })).rejects.toThrow(/getUnifiedAccount/);
    let row = await provisionRow(user);
    expect(row.state).toBe('identity');
    const walletId = row.wallet_id as string;
    const did = row.did as string;
    await approveStandardSigning(user);

    chain.fail('eth_estimateGas');
    await expect(stateModule.provisionAccount(d, { kind: 'standard', userId: user, token: await token() })).rejects.toThrow(/eth_estimateGas/);
    row = await provisionRow(user);
    expect(row.state).toBe('bind_signed');
    expect(row.bind_signature).toMatch(/^[0-9a-f]{128}$/);
    expect(row.station_request).toBeNull();
    expect(row.bind_tx_hash).toBeNull();

    failAfterSubmit = true;
    await expect(stateModule.provisionAccount(d, { kind: 'standard', userId: user, token: await token() })).rejects.toMatchObject({ code: 'gas_station_unavailable' });
    row = await provisionRow(user);
    expect(stationErrors).toEqual([]);
    expect(row.state).toBe('bind_signed');
    expect(row.bind_tx_hash).toBeNull();
    const address = (await poolModule.getPool().query(`select address from wallets where id = $1`, [walletId])).rows[0].address.toLowerCase();
    const submitted = submissions.get(address)!;
    const persistedRequest = row.station_request;
    expect(persistedRequest).toEqual(submitted.request);
    expect(String(row.station_quote_nonce)).toBe(submitted.request.batch.quote.quoteNonce);
    expect(String(row.station_batch_nonce)).toBe(submitted.request.batch.nonce);
    expect(submitRequests.filter((request) => request.batch.account.toLowerCase() === address)).toEqual([persistedRequest]);
    expect(sentTo(address)).toHaveLength(1);
    expect(chain.bindNonces.get(address)).toBe(1n);

    failReceipt = true;
    await expect(stateModule.provisionAccount(d, { kind: 'standard', userId: user, token: await token() })).rejects.toThrow(/pax_sequencerReceipt/);
    row = await provisionRow(user);
    expect(row.state).toBe('bind_sent');
    expect(row.station_request).toEqual(persistedRequest);
    expect(row.bind_tx_hash).toBe(submitted.hash);
    expect(submitRequests.filter((request) => request.batch.account.toLowerCase() === address)).toEqual([persistedRequest, persistedRequest]);
    expect(quoteRequests.filter((request) => request.account.toLowerCase() === address)).toHaveLength(1);
    expect(sentTo(address)).toHaveLength(1);
    expect(sentFrom(address)).toHaveLength(0);

    const done = await stateModule.provisionAccount(d, { kind: 'standard', userId: user, token: await token() });
    expect(done.state).toBe('active');
    expect(done.walletId).toBe(walletId);
    expect(done.did).toBe(did);

    row = await provisionRow(user);
    expect(row.evm_key_generation).toBe(0);
    expect(row.ed_key_generation).toBe(0);
    expect(row.evm_key_id).toBe(`wallet:${user}:standard:secp256k1:0`);
    expect(row.ed_key_id).toBe(`account:${user}:standard:ed25519:0`);
    expect(row.station_request).toEqual(persistedRequest);
    expect(row.bind_tx_hash).toBe(submitted.hash);
    expect(sentTo(address)).toHaveLength(1);
    expect(sentFrom(address)).toHaveLength(0);
    expect(sentTo(address).filter((tx) => tx.value > 0n)).toHaveLength(0);
    expect(chain.bindings.get(address)).toBe(did.slice('did:layerx:'.length));
    expect(chain.bindNonces.get(address)).toBe(1n);
    const topups = await poolModule.getPool().query(`select count(*)::int as n from account_setup_audit where address = $1 and event = 'topup'`, [address]);
    expect(topups.rows[0].n).toBe(0);
    const binds = await poolModule.getPool().query(`select tx_hash from account_setup_audit where address = $1 and event = 'bind'`, [address]);
    expect(binds.rows).toEqual([{ tx_hash: submitted.hash }]);
    const wallets = await poolModule.getPool().query(`select count(*)::int as n from wallets where user_id = $1`, [user]);
    expect(wallets.rows[0].n).toBe(1);

    const again = await stateModule.provisionAccount(d, { kind: 'standard', userId: user, token: await token() });
    expect(again.state).toBe('active');
    expect(stationErrors).toEqual([]);
    expect(submitRequests.filter((request) => request.batch.account.toLowerCase() === address)).toEqual([persistedRequest, persistedRequest]);
    expect(quoteRequests.filter((request) => request.account.toLowerCase() === address)).toHaveLength(1);
    expect(sentTo(address)).toHaveLength(1);
    expect(chain.bindNonces.get(address)).toBe(1n);
    expect(sentFrom(address)).toHaveLength(0);
  },
};

describe('provisioning flow', () => {
  test(
    provisioningFlowTests['provisioning creates both keys, has the gas station sponsor the binding and confirms it before active'],
    provisioningFlowTests['provisioning creates both keys, has the gas station sponsor the binding and confirms it before active'],
    LONG,
  );
  test(
    provisioningFlowTests['provisioning resumes after a failure at each step without a second key, station request or binding'],
    provisioningFlowTests['provisioning resumes after a failure at each step without a second key, station request or binding'],
    LONG,
  );

  it('provisioning refuses and records an address already bound to a different DID', async () => {
    const user = randomUUID();
    admitStandardKeys(user);
    chain.fail('getUnifiedAccount');
    const first = await asUser(user, 'POST', '/v1/wallet/provision');
    expect(first.statusCode).toBe(500);
    const row = await provisionRow(user);
    expect(row.state).toBe('identity');
    await approveStandardSigning(user);
    const address = (await poolModule.getPool().query(`select address from wallets where id = $1`, [row.wallet_id])).rows[0].address.toLowerCase();
    const foreign = randomBytes(32).toString('hex');
    chain.bindings.set(address, foreign);
    chain.bindingsByDid.set(foreign, address);

    const refused = await asUser(user, 'POST', '/v1/wallet/provision');
    expect(refused.statusCode).toBe(409);
    expect(refused.json()).toMatchObject({ error: 'binding_refused', bound_did: `did:layerx:${foreign}` });
    const after = await provisionRow(user);
    expect(after.state).toBe('refused');
    expect(after.refusal_reason).toMatch(/different DID/);
    const audit = await poolModule.getPool().query(`select event, outcome from account_setup_audit where address = $1`, [address]);
    expect(audit.rows).toEqual([{ event: 'refusal', outcome: `did:layerx:${foreign}` }]);
    expect(sentTo(address)).toHaveLength(0);
    expect(sentFrom(address)).toHaveLength(0);
    const me = await asUser(user, 'GET', '/v1/wallet/me');
    expect(me.json()).toMatchObject({ wallet: { binding_state: 'refused' } });

    const again = await asUser(user, 'POST', '/v1/wallet/provision');
    expect(again.statusCode).toBe(409);
    expect(chain.bindings.get(address)).toBe(foreign);
  }, LONG);

  async function provisioningSponsorsAgentBindThroughTwoReauthorizationChallengesAndConfirmsItBeforeActive() {
    const d = deps();
    const pool = poolModule.getPool();

    const owner = randomUUID();
    admitStandardKeys(owner);
    const evmKeyId = `wallet:${owner}:standard:secp256k1:0`;
    const key = await d.attestors!.generate(evmKeyId, 'secp256k1', owner);
    const migrated = await pool.query(
      `insert into wallets (user_id, address, encrypted_private_key, key_version, chain_id, kind, migrated_at, attestor_key_id)
       values ($1, $2, null, 1, $3, 'standard', now(), $4) returning id::text`,
      [owner, key.address, CHAIN_ID, evmKeyId],
    );
    const migratedId = migrated.rows[0].id as string;

    const legacyUser = randomUUID();
    const legacy = await walletsModule.provisionWalletForUser(legacyUser, 'standard');

    const agent = newAgentKey('provisioning');
    const agentUserId = walletsModule.agentWalletUserId(agent.did);
    const agentEvmKeyId = `wallet:${agentUserId}:agent:secp256k1:0`;
    const agentOwner = `agent:${agent.publicKeyHex}`;
    inventoryKeys.set(agentEvmKeyId, {
      key_id: agentEvmKeyId, owner: agentOwner, curve: 'secp256k1', epoch: 0, public_key: '', operations: ['generate'],
    });
    approveInventory([...inventoryKeys.values()]);
    const agentKey = await d.attestors!.generate(agentEvmKeyId, 'secp256k1', agentOwner);
    const agentWallet = { row: (await pool.query<{
      id: string; address: string; encrypted_private_key: null;
    }>(
      `insert into wallets (user_id, address, encrypted_private_key, key_version, chain_id, kind, migrated_at, attestor_key_id)
       values ($1, $2, null, 1, $3, 'agent', now(), $4) returning id::text, address, encrypted_private_key`,
      [agentUserId, agentKey.address, CHAIN_ID, agentEvmKeyId],
    )).rows[0]! };
    const agents = await import('../src/db/agents.js');
    await agents.upsertPrincipalOnVerify({
      did: agent.did, label: 'provisioning', keyFingerprint: agent.publicKeyHex.slice(0, 16), publicKey: agent.publicKeyHex,
    });
    await agents.setPrincipalWallet(agent.did, agentWallet.row.id);
    await agents.setPrincipalFrozen(agent.did, false);
    const agentPolicy = await agents.upsertPolicy(agent.did, {
      mode: 'full', max_tx_value_wei: 1_000_000n, max_daily_value_wei: 100n,
    }, null);
    expect(agentPolicy.max_tx_value_wei).toBe(1_000_000n);
    expect(agentPolicy.max_daily_value_wei).toBe(100n);

    const first = await backfillModule.backfillAccounts(d, { batchSize: 1, maxBatches: 1 });
    expect(first.outcomes).toHaveLength(1);
    expect(first.done).toBe(false);
    const rest = await backfillModule.backfillAccounts(d, { batchSize: 1, maxBatches: 10 });
    expect(rest.done).toBe(true);
    const outcomes = [...first.outcomes, ...rest.outcomes];
    expect(outcomes.map((o) => o.wallet_id).sort()).toEqual([migratedId, agentWallet.row.id].sort());
    const byId = new Map(outcomes.map((o) => [o.wallet_id, o]));
    expect(byId.get(migratedId), JSON.stringify(byId.get(migratedId))).toMatchObject({ outcome: 'awaiting_owner', kind: 'standard' });
    expect(byId.get(migratedId)!.did).toMatch(/^did:layerx:[0-9a-f]{64}$/);
    const agentOutcome = byId.get(agentWallet.row.id);
    expect(agentOutcome, JSON.stringify(agentOutcome)).toMatchObject({
      outcome: 'awaiting_agent_signature',
      kind: 'agent',
      did: `did:layerx:${agent.publicKeyHex}`,
      main_account_id: chainMainAccountId(agent.publicKeyHex),
    });
    const serialised = JSON.stringify(outcomes);
    expect(agentWallet.row.encrypted_private_key).toBeNull();
    expect(serialised).not.toContain(legacy.row.encrypted_private_key);
    for (const o of outcomes) {
      expect(Object.keys(o).sort()).toEqual(['address', 'did', 'kind', 'main_account_id', 'outcome', 'reason', 'wallet_id']);
    }
    const legacyRow = await pool.query(`select did from wallets where id = $1`, [legacy.row.id]);
    expect(legacyRow.rows[0].did).toBeNull();
    const cursor = await pool.query(`select last_wallet_id::text from account_backfill_cursor where name = 'unified_account'`);
    expect(cursor.rows[0].last_wallet_id).toBe(rest.cursor);
    const sentBefore = chain.sent.length;
    const empty = await backfillModule.backfillAccounts(d, { batchSize: 5, maxBatches: 1 });
    expect(empty.outcomes).toHaveLength(0);
    expect(chain.sent.length).toBe(sentBefore);

    const layerxKey = (await pool.query(`select layerx_key_id from wallets where id = $1`, [migratedId])).rows[0].layerx_key_id;
    await approveStandardSigning(owner);
    const completed = await asUser(owner, 'POST', '/v1/wallet/provision');
    expect(completed.statusCode, completed.body).toBe(200);
    expect(completed.json()).toMatchObject({ wallet: { id: migratedId, did: byId.get(migratedId)!.did, binding_state: 'bound' } });
    expect((await pool.query(`select layerx_key_id from wallets where id = $1`, [migratedId])).rows[0].layerx_key_id).toBe(layerxKey);
    expect(chain.bindings.get(key.address!.toLowerCase())).toBe(byId.get(migratedId)!.did!.slice('did:layerx:'.length));

    const pending = await stateModule.provisionAccount(d, { kind: 'agent', did: agent.did });
    expect(pending.awaiting).toBe('agent_signature');
    const message = Buffer.from(pending.bindMessage!.slice(2), 'hex');
    const impostor = newAgentKey('impostor');
    await expect(
      stateModule.provisionAccount(d, { kind: 'agent', did: agent.did, agentSignature: impostor.sign(message) }),
    ).rejects.toMatchObject({ code: 'agent_signature_invalid' });
    const agentAddress = agentWallet.row.address.toLowerCase();
    expect(chain.bindings.has(agentAddress)).toBe(false);
    expect(sentTo(agentAddress)).toHaveLength(0);

    await approveStandardSigning(agentUserId, 'agent');
    const { agentRequestDigest } = await import('../src/agent/verify.js');
    const requestBody = Buffer.from(JSON.stringify({ bind_signature: agent.sign(message) }));
    const nonce = randomBytes(16);
    const expiry = Math.floor(Date.now() / 1000) + 300;
    const method = 'POST /v1/agent/provision';
    const origin = {
      method, did: agent.did, body: requestBody.toString('base64'), nonce: nonce.toString('hex'), expiry,
      signature: agent.sign(agentRequestDigest({ method, keyId: agent.did, nonce, expiry: BigInt(expiry), body: requestBody })),
    };
    const { AttestorReauthorizationRequired } = await import('../src/attestor/client.js');
    const challenge = await stateModule.provisionAccount(d, {
      kind: 'agent', did: agent.did, agentSignature: agent.sign(message), custody: { origin },
    }).then(
      () => { throw new Error('agent provisioning must require exact-envelope reauthorization'); },
      (error: unknown) => {
        expect(error, error instanceof Error ? error.message : String(error)).toBeInstanceOf(AttestorReauthorizationRequired);
        if (!(error instanceof AttestorReauthorizationRequired)) throw error;
        return error;
      },
    );
    expect(challenge.code).toBe('agent_reauthorization_required');
    expect(challenge.keyId).toBe(agentEvmKeyId);
    expect(challenge.kind).toBe('eth_sign_digest');
    expect(JSON.parse(challenge.request)).toMatchObject({
      kind: 'eth_sign_digest',
      key_id: agentEvmKeyId,
      origin,
      construction: { kind: 'sponsored_batch', account: agentKey.address },
    });
    const agentProvision = await provisionRow(agentUserId, 'agent');
    expect(agentProvision).toMatchObject({
      wallet_id: agentWallet.row.id,
      state: 'bind_signed',
      bind_signature: agent.sign(message).replace(/^0x/, ''),
      station_request: {
        batch: { account: agentKey.address, nonce: '0' },
        authorization: { chainId: String(CHAIN_ID), address: PAYMASTER, nonce: '0' },
      },
    });
    const retained = agentProvision.station_request as import('../src/provision/bind.js').SponsoredBindConstruction;
    const batchEnvelope = JSON.parse(challenge.request);
    expect(batchEnvelope.construction).toEqual({
      kind: 'sponsored_batch', ...retained.batch,
      quote: Object.fromEntries(Object.entries(retained.batch.quote).filter(([key]) => key !== 'decimals')),
    });
    expect(retained.accountSignature).toBeUndefined();
    expect(String(agentProvision.station_quote_nonce)).toBe(retained.batch.quote.quoteNonce);
    expect(String(agentProvision.station_batch_nonce)).toBe(retained.batch.nonce);
    expect(stationErrors).toEqual([]);
    expect(quoteRequests.filter((request) => request.account.toLowerCase() === agentAddress)).toHaveLength(1);
    expect(submitRequests.filter((request) => request.batch.account.toLowerCase() === agentAddress)).toHaveLength(0);
    expect(chain.bindings.has(agentAddress)).toBe(false);
    expect(sentTo(agentAddress)).toHaveLength(0);
    expect(sentFrom(agentAddress)).toHaveLength(0);
    const agentRow = await pool.query(`select binding_state, layerx_key_id, did from wallets where id = $1`, [agentWallet.row.id]);
    expect(agentRow.rows[0]).toEqual({ binding_state: 'pending', layerx_key_id: agent.did, did: `did:layerx:${agent.publicKeyHex}` });

    const reauthorize = (body: string) => {
      const wire = JSON.parse(body);
      const nonce = randomBytes(16);
      const expiry = Math.floor(Date.now() / 1000) + 300;
      return { body, nonce: nonce.toString('hex'), expiry,
        signature: agent.sign(agentRequestDigest({ method: '/v1/sign', keyId: wire.key_id,
          nonce, expiry: BigInt(expiry), body: Buffer.from(body) })) };
    };
    const batchApproval = reauthorize(challenge.request);
    const authorizationChallenge = await stateModule.provisionAccount(d, {
      kind: 'agent', did: agent.did, agentSignature: agent.sign(message),
      custody: { origin, reauthorization: batchApproval },
    }).then(
      () => { throw new Error('the EIP-7702 authorization must require its own reauthorization'); },
      (error: unknown) => {
        expect(error, error instanceof Error ? error.message : String(error)).toBeInstanceOf(AttestorReauthorizationRequired);
        if (!(error instanceof AttestorReauthorizationRequired)) throw error;
        return error;
      },
    );
    expect(authorizationChallenge.code).toBe('agent_reauthorization_required');
    expect(authorizationChallenge.keyId).toBe(agentEvmKeyId);
    expect(authorizationChallenge.kind).toBe('eip7702_authorization');
    expect(JSON.parse(authorizationChallenge.request)).toMatchObject({
      key_id: agentEvmKeyId, kind: 'eth_sign_digest', origin,
      construction: { kind: 'eip7702_authorization', ...retained.authorization },
    });
    const afterBatch = await provisionRow(agentUserId, 'agent');
    const signedBatch = afterBatch.station_request as import('../src/provision/bind.js').SponsoredBindConstruction;
    expect(afterBatch.state).toBe('bind_signed');
    expect(signedBatch.batch).toEqual(retained.batch);
    expect(signedBatch.authorization).toEqual(retained.authorization);
    expect(signedBatch.relayerSignature).toBe(retained.relayerSignature);
    expect(signedBatch.accountSignature).toMatch(/^0x[0-9a-f]{130}$/);
    expect(quoteRequests.filter(request => request.account.toLowerCase() === agentAddress)).toHaveLength(1);
    expect(submitRequests.filter(request => request.batch.account.toLowerCase() === agentAddress)).toHaveLength(0);
    expect(chain.bindings.has(agentAddress)).toBe(false);
    expect((await pool.query('select binding_state from wallets where id = $1', [agentWallet.row.id])).rows[0].binding_state).toBe('pending');

    const active = await stateModule.provisionAccount(d, {
      kind: 'agent', did: agent.did, agentSignature: agent.sign(message),
      custody: { origin, reauthorization: reauthorize(authorizationChallenge.request) },
    });
    expect(active).toMatchObject({ state: 'active', awaiting: null, walletId: agentWallet.row.id, did: `did:layerx:${agent.publicKeyHex}` });
    const boundWallet = (await pool.query(
      'select binding_state, layerx_key_id, attestor_key_id, did from wallets where id = $1', [agentWallet.row.id],
    )).rows[0];
    expect(boundWallet).toEqual({ binding_state: 'bound', layerx_key_id: agent.did, attestor_key_id: agentEvmKeyId, did: `did:layerx:${agent.publicKeyHex}` });
    const finalRow = await provisionRow(agentUserId, 'agent');
    const submitted = submitRequests.filter(request => request.batch.account.toLowerCase() === agentAddress);
    expect(submitted).toHaveLength(1);
    expect(finalRow.station_request).toEqual(submitted[0]);
    expect(submitted[0]!.batch).toEqual(retained.batch);
    expect(submitted[0]!.accountSignature).toBe(signedBatch.accountSignature);
    expect(submitted[0]!.relayerSignature).toBe(retained.relayerSignature);
    expect(quoteRequests.filter(request => request.account.toLowerCase() === agentAddress)).toHaveLength(1);
    expect(chain.bindings.has(agentAddress)).toBe(true);
    expect(chain.bindings.get(agentAddress)).toBe(agent.publicKeyHex);
    expect(chain.bindNonces.get(agentAddress)).toBe(1n);
    expect(stationErrors).toEqual([]);
    expect(sentFrom(agentAddress)).toHaveLength(0);
  }

  test('provisioning sponsors the agent bind through two reauthorization challenges and confirms it before active',
    provisioningSponsorsAgentBindThroughTwoReauthorizationChallengesAndConfirmsItBeforeActive, LONG);
});
