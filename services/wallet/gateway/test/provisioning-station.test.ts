import { spawn, execFileSync, type ChildProcess } from 'node:child_process';
import { createHash, generateKeyPairSync, randomBytes, randomUUID, sign, X509Certificate } from 'node:crypto';
import { createServer as createHttpServer, type Server } from 'node:http';
import { request as httpsRequest } from 'node:https';
import { createServer as createNetServer } from 'node:net';
import { mkdtempSync, openSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { afterAll, beforeAll, expect, test } from 'vitest';
import type { FastifyInstance } from 'fastify';
import { decodeFunctionData, recoverAddress, serializeSignature, type Hex } from 'viem';
import Fastify from 'fastify';
import type { WalletInventoryKey } from '../src/agent/authority.js';
import type { SponsoredBindRequest } from '../src/provision/bind.js';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';
import { startIdentityProvider, type IdentityProvider } from './support/identity.js';
import { ADDR, BIND_SELECTOR, TestChain } from './support/chain.js';
import { KERNEL_POLICY } from './e2e/attestors.js';
import type { SequencerReceiptResult } from '../src/rpc/pool.js';

const here = dirname(fileURLToPath(import.meta.url));
const attestorDir = resolve(here, '../../attestor');
const CHAIN_ID = 125;
const PAYMASTER = '0x4444444444444444444444444444444444444444' as const;
const sponsoredHashes = new Set<Hex>();
let approveInventory: (keys: WalletInventoryKey[]) => void;
const stationRpcRequests: Array<{method: string; params: unknown[]}> = [];
const LONG = 900_000;

function freePort(): Promise<number> {
  return new Promise((resolvePort, reject) => {
    const srv = createNetServer();
    srv.once('error', reject);
    srv.listen(0, '127.0.0.1', () => {
      const addr = srv.address();
      if (!addr || typeof addr === 'string') return reject(new Error('no port'));
      srv.close(() => resolvePort(addr.port));
    });
  });
}

let idp: IdentityProvider;
let pg: EphemeralPostgres;
let app: FastifyInstance;
let chain: TestChain;
let sequencer: Server;
const sequencerRequests: Array<{ method: string; params: Hex[] }> = [];
const committedReceipts = new Set<Hex>();
let workDir: string;
let daemons: ChildProcess[] = [];
let sponsorAddress: string;
let poolModule: typeof import('../src/db/pool.js');
let bindModule: typeof import('../src/provision/bind.js');
let stateModule: typeof import('../src/provision/state.js');
let routesModule: typeof import('../src/routes/wallet.js');
let clientTls: { cert: Buffer; key: Buffer; ca: Buffer };
const endpoints: string[] = [];

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
      stationRpcRequests.push(body);
      if (body.method === 'eth_getCode' || (body.method === 'eth_call' && typeof body.params[0] === 'object' && body.params[0] !== null && (body.params[0] as unknown as {data: string}).data === '0xaffed0e0')) {
        const result = body.method === 'eth_getCode' ? (body.params[0] === PAYMASTER ? '0x60005460005260206000f3' : '0x') : `0x${'00'.repeat(32)}`;
        res.writeHead(200, { 'content-type': 'application/json' });
        res.end(JSON.stringify({jsonrpc:'2.0', id:body.id, result}));
        return;
      }
      if (body.method === 'pax_sequencerReceipt') {
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
            sponsor: sponsoredHashes.has(hash) ? sponsorAddress as Hex : `0x${'00'.repeat(20)}`,
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
        caps: { native: { per_transaction: '1000000000000000000', daily: '10000000000000000000' },
          '0x21f7b20a555199fa73A238B1a91FD0f549068fEe': {per_transaction:'1000000', daily:'1000000'} },
        selectors: { [ADDR]: [BIND_SELECTOR] },
      },
    }),
  );
  const kernelPolicyFile = join(workDir, 'kernel-policy.json');
  writeFileSync(kernelPolicyFile, JSON.stringify(KERNEL_POLICY));
  const apiPorts = await Promise.all(nodes.map(() => freePort()));
  const peerPorts = await Promise.all(nodes.map(() => freePort()));
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
  approveInventory = keys => {
    const claims = JSON.parse(Buffer.from(inventoryPayload, 'base64url').toString('utf8'));
    claims.sequence = String(++inventorySequence);
    claims.keys = keys;
    const payload = Buffer.from(JSON.stringify(claims)).toString('base64url');
    const input = `${inventoryHeader}.${payload}`;
    const signature = sign('sha256', Buffer.from(input), { key: authority.privateKey, dsaEncoding: 'ieee-p1363' });
    writeFileSync(inventoryFile, `${input}.${signature.toString('base64url')}`);
  };
  nodes.forEach((node, i) => {
    const others = nodes.map((n, j) => ({ n, j })).filter((x) => x.j !== i);
    const keyFile = join(workDir, `${node}.store`);
    writeFileSync(keyFile, randomBytes(32).toString('hex'));
    const log = openSync(join(workDir, `${node}.log`), 'w');
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
        ATTESTOR_RPC_URL: `http://127.0.0.1:${sequencerAddress.port}`,
        ATTESTOR_ACTIVITY_TYPES: '0x10005',
      },
      stdio: ['ignore', log, log],
    });
    daemons.push(child);
    endpoints.push(`https://127.0.0.1:${apiPorts[i]}`);
  });
  const deadline = Date.now() + 120_000;
  for (;;) {
    const states = await Promise.all(endpoints.map((u) => health(u)));
    if (states.every((s) => s.ready)) break;
    if (Date.now() > deadline) {
      const logs = nodes.map((n) => readFileSync(join(workDir, `${n}.log`), 'utf8')).join('\n');
      throw new Error(`attestors did not become ready: ${logs}`);
    }
    await new Promise((r) => setTimeout(r, 300));
  }

  const sponsorKey = generatePrivateKey();
  sponsorAddress = privateKeyToAccount(sponsorKey).address.toLowerCase();
  chain.balances.set(sponsorAddress, 10n ** 21n);
  const sponsorFile = join(workDir, 'sponsor.key');
  writeFileSync(sponsorFile, sponsorKey.slice(2));

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
  process.env.PAXEER_GAS_STATION_PAYMASTER = PAYMASTER;
  process.env.PAXEER_GAS_STATION_GAS_LIMIT = '200000';
  process.env.PAXEER_GAS_STATION_MAX_TOKEN_AMOUNT = '1000000';
  process.env.ACCOUNT_SETUP_GAS_CAP_WEI = '1000000000000000';
  delete process.env.WALLET_GAS_STATION_URL;
  process.env.WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE = authorityPrivateKeyFile;
  process.env.WALLET_IDENTITY_BINDING_TENANT = 'provisioning-test';
  process.env.WALLET_CUSTODY_INVENTORY_FILE = inventoryFile;
  process.env.WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE = authorityPublicKeyFile;
  pg = await startPostgres();
  const index = await import('../src/index.js');
  poolModule = await import('../src/db/pool.js');
  bindModule = await import('../src/provision/bind.js');
  stateModule = await import('../src/provision/state.js');
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
  await chain?.stop();
  await idp?.stop();
  if (workDir) rmSync(workDir, { recursive: true, force: true });
}, 120_000);


async function provisionBindIsStationSponsored(): Promise<void> {
  const { GasStationClient } = await import('../src/sponsor/station.js');
  const { constructionDigest, sponsorQuoteDigest } = await import('../src/attestor/client.js');
  const { env } = await import('../src/env.js');
  expect(env.WALLET_GAS_STATION_URL).toBeUndefined();
  const base = routesModule.provisionDepsFromEnv();
  if (!base) throw new Error('real attestor fixture is unavailable');
  const sponsor = privateKeyToAccount(`0x${readFileSync(join(workDir, 'sponsor.key'), 'utf8')}`);
  const quoteRequests: Record<string, unknown>[] = [];
  const submitRequests: SponsoredBindRequest[] = [];
  const endpointErrors: unknown[] = [];
  let issued: SponsoredBindRequest['batch']['quote'] | undefined;
  let failAfterSubmit = true;
  let submittedHash: Hex | null = null;
  const server = createHttpServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (chunk: Buffer) => chunks.push(chunk));
    req.on('end', async () => {
      try {
        const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
        expect(req.method).toBe('POST');
        expect(req.headers['content-type']).toBe('application/json');
        if (req.url === '/quote') {
          expect(Object.keys(body).sort()).toEqual(['account', 'nonce', 'calls', 'maxTokenAmount', 'gasCost', 'chainId', 'token', 'decimals'].sort());
          expect(body).toMatchObject({ chainId: '125', nonce: '0', token: bindModule.SIDIORA_TOKEN, decimals: 6, maxTokenAmount: '1000000' });
          expect(body.calls).toHaveLength(1);
          expect(body.calls[0].to).toBe(ADDR);
          expect(body.calls[0].value).toBe('0');
          expect(body.calls[0].data.slice(0,10)).toBe(BIND_SELECTOR);
          quoteRequests.push(body);
          issued = { sponsor: sponsor.address, token: body.token, maxTokenAmount: body.maxTokenAmount,
            tokenAmount: '100', deadline: String(Math.floor(Date.now()/1000)+300), quoteNonce: '7', gasCost: body.gasCost, decimals: 6 };
          const construction = { kind: 'sponsored_batch', chainId: body.chainId, account: body.account, nonce: body.nonce, calls: body.calls, quote: issued };
          const relayerSignature = await sponsor.sign({ hash: sponsorQuoteDigest(construction) });
          res.writeHead(200, { 'content-type': 'application/json' });
          res.end(JSON.stringify({ quote: issued, relayerSignature }));
          return;
        }
        expect(req.url).toBe('/submit');
        const request: SponsoredBindRequest = body;
        submitRequests.push(request);
        expect(Object.keys(request).sort()).toEqual(['call','authorization','batch','accountSignature','relayerSignature'].sort());
        expect(Object.keys(request.batch).sort()).toEqual(['chainId','account','nonce','calls','quote'].sort());
        expect(request.batch.quote).toEqual(issued);
        expect(request.batch.account).toBe(quoteRequests[0]!.account);
        const construction = { kind:'sponsored_batch', ...request.batch };
        expect(await recoverAddress({hash: constructionDigest(construction), signature:request.accountSignature})).toBe(request.batch.account);
        expect(await recoverAddress({hash: sponsorQuoteDigest(construction), signature:request.relayerSignature as Hex})).toBe(sponsor.address);
        const authorization = request.authorization;
        expect(authorization).toMatchObject({chainId:'125', address:PAYMASTER, nonce:'0'});
        expect(await recoverAddress({hash:constructionDigest({kind:'eip7702_authorization', chainId:authorization.chainId, address:authorization.address, nonce:authorization.nonce}),
          signature:serializeSignature(authorization)})).toBe(request.batch.account);
        expect(request.accountSignature).toMatch(/^0x[0-9a-f]{130}$/i);
        const decoded = decodeFunctionData({abi:bindModule.sponsoredBindAbi, data:request.call.data});
        expect(decoded.functionName).toBe('executeSponsored');
        expect(decoded.args![0]).toEqual(request.batch.calls.map(call => ({...call,value:BigInt(call.value)})));
        expect(decoded.args![2]).toBe(request.accountSignature);
        expect(decoded.args![3]).toBe(request.relayerSignature);
        expect(request.call.to).toBe(request.batch.account);
        expect(request.call.value).toBe('0');
        const persisted = await base.pool.query('select station_quote_nonce::text, station_batch_nonce::text, station_request from account_provisioning where station_quote_nonce = 7');
        expect(persisted.rows).toHaveLength(1);
        expect(persisted.rows[0]).toEqual({station_quote_nonce:'7',station_batch_nonce:'0',station_request:request});
        if (!submittedHash) {
          const raw = await sponsor.signTransaction({ type:'eip7702', chainId:CHAIN_ID, nonce:0, to:request.call.to, value:0n,
            data:request.call.data, gas:200000n, maxFeePerGas:BigInt(request.batch.quote.gasCost)/200000n, maxPriorityFeePerGas:chain.tip,
            authorizationList:[{...authorization,chainId:Number(authorization.chainId),nonce:Number(authorization.nonce)}] });
          submittedHash = await base.rpc.sendRawTransaction(raw);
          sponsoredHashes.add(submittedHash);
          expect(chain['applyBind'](request.batch.account.toLowerCase(), request.batch.calls[0]!.data)).toBe(true);
        }
        if (failAfterSubmit) {
          failAfterSubmit = false;
          res.writeHead(503, {'content-type':'application/json'});
          res.end(JSON.stringify({error:'gas_station_unavailable'}));
          return;
        }
        res.writeHead(200, {'content-type':'application/json'});
        res.end(JSON.stringify({transactionHash:submittedHash}));
      } catch (error) {
        endpointErrors.push(error);
        res.writeHead(500, {'content-type':'application/json'});
        res.end(JSON.stringify({error:'station_contract_violation'}));
      }
    });
  });
  const api = Fastify();
  try {
    await new Promise<void>(resolve => server.listen(0,'127.0.0.1',resolve));
    const address = server.address();
    if (!address || typeof address === 'string') throw new Error('station did not bind');
    const deps = {...base, station:new GasStationClient(`http://127.0.0.1:${address.port}`)};
    const userId = randomUUID();
    const evmId = `wallet:${userId}:standard:secp256k1:0`;
    const edId = `account:${userId}:standard:ed25519:0`;
    const keys: WalletInventoryKey[] = [
      {key_id:evmId,owner:userId,curve:'secp256k1',epoch:0,public_key:'',operations:['generate']},
      {key_id:edId,owner:userId,curve:'ed25519',epoch:0,public_key:'',operations:['generate']},
    ];
    approveInventory(keys);
    const evmKey = await deps.attestors!.generate(evmId,'secp256k1',userId);
    await deps.pool.query(`insert into wallets (user_id,address,encrypted_private_key,key_version,chain_id,kind,migrated_at,attestor_key_id)
      values ($1,$2,null,1,$3,'standard',now(),$4)`,[userId,evmKey.address,CHAIN_ID,evmId]);
    const pending = await stateModule.provisionAccount(deps,{kind:'standard',userId,token:null});
    expect(pending).toMatchObject({state:'identity',awaiting:'owner'});
    const rowBefore = (await deps.pool.query('select * from account_provisioning where user_id = $1',[userId])).rows[0]!;
    approveInventory(keys.map(key => ({...key,public_key:key.curve==='secp256k1'?evmKey.publicKey:rowBefore.ed_public_key,operations:['sign']})));
    await api.register(routesModule.walletRoutes,{provision:deps});
    const token = await idp.mintUserToken(userId);
    const first = await api.inject({method:'POST',url:'/v1/wallet/provision',headers:{authorization:`Bearer ${token}`}});
    expect(endpointErrors).toEqual([]);
    expect(first.statusCode,first.body).toBe(503);
    expect(first.json()).toMatchObject({error:'gas_station_unavailable'});
    const response = await api.inject({method:'POST',url:'/v1/wallet/provision',headers:{authorization:`Bearer ${token}`}});
    expect(endpointErrors).toEqual([]);
    expect(response.statusCode,response.body).toBe(200);
    const body = response.json();
    expect(body.provisioning).toEqual({state:'active',awaiting:null});
    expect(body.wallet.binding_state).toBe('bound');
    const row = (await deps.pool.query('select * from account_provisioning where user_id = $1',[userId])).rows[0]!;
    expect(row.bind_tx_hash).toBe(submittedHash);
    expect(row.station_quote_nonce).toBe('7');
    expect(row.station_batch_nonce).toBe('0');
    expect(row.bind_raw_tx).toBeNull();
    expect(row.topup_raw_tx).toBeNull();
    expect(row.topup_tx_hash).toBeNull();
    expect(row.topup_value_wei).toBeNull();
    expect(bindModule.verifyEd25519(row.ed_public_key,bindModule.bindMessage(CHAIN_ID,body.wallet.address,row.bind_nonce),row.bind_signature)).toBe(true);
    expect(chain.bindings.get(body.wallet.address.toLowerCase())).toBe(row.ed_public_key);
    expect(chain.bindNonces.get(body.wallet.address.toLowerCase())).toBe(1n);
    expect(chain.balances.get(body.wallet.address.toLowerCase()) ?? 0n).toBe(0n);
    expect(chain.sent).toHaveLength(1);
    expect(chain.sent[0]!.from).toBe(sponsor.address.toLowerCase());
    expect(chain.sent[0]!.to).toBe(body.wallet.address.toLowerCase());
    expect(chain.sent[0]!.value).toBe(0n);
    expect(chain.sent[0]!.data).toBe(submitRequests[0]!.call.data);
    expect(quoteRequests).toHaveLength(1);
    expect(submitRequests).toHaveLength(2);
    expect(submitRequests[1]).toEqual(submitRequests[0]);
    expect((await deps.pool.query("select * from account_setup_audit where wallet_id = $1 and event = 'topup'",[body.wallet.id])).rows).toHaveLength(0);
    expect(await deps.rpc.sequencerReceipt(row.bind_tx_hash)).toMatchObject({sponsor:sponsor.address.toLowerCase(),status:'0x1'});
    const again = await stateModule.provisionAccount(deps,{kind:'standard',userId,token});
    expect(again.state).toBe('active');
    expect(submitRequests).toHaveLength(2);
    expect(chain.sent).toHaveLength(1);
  } finally {
    await api.close();
    await new Promise<void>((resolve,reject) => server.close(error => error?reject(error):resolve()));
  }
}

async function provisionRefusesWithoutStation(): Promise<void> {
  const { env } = await import('../src/env.js');
  const { GasStationClient } = await import('../src/sponsor/station.js');
  expect(env.WALLET_GAS_STATION_URL).toBeUndefined();
  const base = routesModule.provisionDepsFromEnv();
  if (!base) throw new Error('attestor fixture unavailable');
  const pool = base.pool;
  const before = (await pool.query('select count(*)::int as n from account_provisioning')).rows[0]!.n;
  const requests = stationRpcRequests.length;
  const userId = randomUUID();
  for (const station of [null,new GasStationClient()]) {
    await expect(stateModule.provisionAccount({...base,station},{kind:'standard',userId,token:null})).rejects.toMatchObject({status:503,code:'gas_station_unconfigured'});
  }
  expect(stationRpcRequests).toHaveLength(requests);
  expect((await pool.query('select count(*)::int as n from account_provisioning')).rows[0]!.n).toBe(before);
  const response = await app.inject({method:'POST',url:'/v1/wallet/provision',headers:{authorization:`Bearer ${await idp.mintUserToken(userId)}`}});
  expect(response.statusCode,response.body).toBe(503);
  expect(response.json()).toMatchObject({error:'gas_station_unconfigured'});
  expect((await pool.query('select * from wallets where user_id = $1',[userId])).rows).toHaveLength(0);
}

test(provisionBindIsStationSponsored, provisionBindIsStationSponsored, LONG);
test(provisionRefusesWithoutStation, provisionRefusesWithoutStation);
