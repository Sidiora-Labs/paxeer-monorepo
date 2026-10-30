import { afterAll, beforeAll, describe, expect, it, test } from 'vitest';
import { randomBytes, randomUUID, verify } from 'node:crypto';
import { createServer, type Server } from 'node:http';
import type { FastifyInstance } from 'fastify';
import {
  decodeFunctionData,
  getAddress,
  keccak256,
  parseTransaction,
  recoverAddress,
  recoverMessageAddress,
  recoverTransactionAddress,
  recoverTypedDataAddress,
  serializeSignature,
  serializeTransaction,
  toHex,
  type Hex,
  type TypedDataDefinition,
} from 'viem';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import { startPostgres, type EphemeralPostgres } from '../support/postgres.js';
import { ADDR, TestChain, chainMainAccountId } from '../support/chain.js';
import type { SequencerReceiptResult } from '../../src/rpc/pool.js';
import type { WalletInventoryKey } from '../../src/agent/authority.js';
import type { SponsoredBindRequest } from '../../src/provision/bind.js';
import {
  CHAIN_ID,
  NATIVE_PER_TX_CAP_WEI,
  PAYMASTER,
  startAttestorNetwork,
  startIdentity,
  type AttestorNetwork,
  type Identity,
} from './attestors.js';

const RECIPIENT = '0x000000000000000000000000000000000000c0de';

const typedData = {
  domain: { name: 'Ether Mail', version: '1', chainId: CHAIN_ID, verifyingContract: '0xcccccccccccccccccccccccccccccccccccccccc' },
  types: {
    Person: [
      { name: 'name', type: 'string' },
      { name: 'wallet', type: 'address' },
    ],
    Mail: [
      { name: 'from', type: 'Person' },
      { name: 'to', type: 'Person' },
      { name: 'contents', type: 'string' },
    ],
  },
  primaryType: 'Mail',
  message: {
    from: { name: 'Cow', wallet: '0xCD2a3d9F938E13CD947Ec05AbC7FE734Df8DD826' },
    to: { name: 'Bob', wallet: '0xbBbBBBBbbBBBbbbBbbBbbbbBBbBbbbbBbBbbBBbB' },
    contents: 'Hello, Bob!',
  },
} as const;

let identity: Identity;
let network: AttestorNetwork;
let pg: EphemeralPostgres;
let app: FastifyInstance;
let chain: TestChain;
let sequencerProxy: Server;
let station: Server;
const broadcasts: Hex[] = [];
const sequencerReceipts: Array<{ hash: Hex; result: SequencerReceiptResult | null }> = [];
let sponsorAddress: string;
const userId = randomUUID();
let token: string;
let key: { keyId: string; address: `0x${string}` };

const auth = (t: string) => ({ authorization: `Bearer ${t}` });

const txBody = (value: bigint) => ({
  tx: {
    to: RECIPIENT,
    value: value.toString(),
    gas: '21000',
    maxFeePerGas: '2000000000',
    maxPriorityFeePerGas: '1000000000',
    chainId: CHAIN_ID,
  },
});

beforeAll(async () => {
  identity = await startIdentity();
  chain = new TestChain(CHAIN_ID);
  await chain.start();
  const chainUpstream = chain.url;

  sequencerProxy = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (chunk: Buffer) => chunks.push(chunk));
    req.on('end', () => {
      void (async () => {
        const body = Buffer.concat(chunks);
        const rpc = JSON.parse(body.toString('utf8')) as { id: number; method: string; params: unknown[] };
        if (rpc.method === 'eth_getCode' || (rpc.method === 'eth_call' && (rpc.params[0] as { data?: Hex }).data === '0xaffed0e0')) {
          const result = rpc.method === 'eth_getCode'
            ? (rpc.params[0] === PAYMASTER ? '0x60005460005260206000f3' : '0x')
            : `0x${'00'.repeat(32)}`;
          res.writeHead(200, { 'content-type': 'application/json' });
          res.end(JSON.stringify({ jsonrpc: '2.0', id: rpc.id, result }));
          return;
        }
        if (rpc.method === 'eth_sendRawTransaction') broadcasts.push(rpc.params[0] as Hex);
        const upstream = await fetch(chainUpstream, {
          method: 'POST', headers: { 'content-type': 'application/json' }, body,
        });
        const answer = await upstream.text();
        if (rpc.method === 'pax_sequencerReceipt') {
          const envelope = JSON.parse(answer) as { result?: SequencerReceiptResult | null };
          if ('result' in envelope) sequencerReceipts.push({ hash: rpc.params[0] as Hex, result: envelope.result! });
        }
        res.writeHead(upstream.status, { 'content-type': 'application/json' });
        res.end(answer);
      })().catch((error: Error) => {
        res.writeHead(502, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ error: error.message }));
      });
    });
  });
  await new Promise<void>((resolve, reject) => {
    sequencerProxy.once('error', reject);
    sequencerProxy.listen(0, '127.0.0.1', resolve);
  });
  const sequencerAddress = sequencerProxy.address();
  if (!sequencerAddress || typeof sequencerAddress === 'string') throw new Error('sequencer proxy did not bind');
  chain.url = `http://127.0.0.1:${sequencerAddress.port}`;
  network = await startAttestorNetwork({ identity, rpcUrl: chain.url });

  const sponsor = privateKeyToAccount(generatePrivateKey());
  sponsorAddress = sponsor.address.toLowerCase();
  chain.balances.set(sponsorAddress, 10n ** 21n);
  type QuoteRequest = Omit<SponsoredBindRequest['batch'], 'quote'> & {
    token: Hex; maxTokenAmount: string; gasCost: string; decimals: 6;
  };
  let issued: SponsoredBindRequest['batch'] | undefined;
  station = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (chunk: Buffer) => chunks.push(chunk));
    req.on('end', () => {
      void (async () => {
        const { constructionDigest, sponsorQuoteDigest } = await import('../../src/attestor/client.js');
        const { sponsoredBindAbi, SIDIORA_TOKEN } = await import('../../src/provision/bind.js');
        const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
        expect(req.method).toBe('POST');
        expect(req.headers['content-type']).toBe('application/json');
        if (req.url === '/quote') {
          const request: QuoteRequest = body;
          expect(request).toMatchObject({ chainId: String(CHAIN_ID), nonce: '0', token: SIDIORA_TOKEN, decimals: 6, maxTokenAmount: '1000000' });
          expect(request.calls).toHaveLength(1);
          expect(request.calls[0]!.to).toBe(ADDR);
          expect(request.calls[0]!.value).toBe('0');
          const quote: SponsoredBindRequest['batch']['quote'] = {
            sponsor: sponsor.address, token: request.token, maxTokenAmount: request.maxTokenAmount,
            tokenAmount: '100', deadline: String(Math.floor(Date.now() / 1000) + 300),
            quoteNonce: '1', gasCost: request.gasCost, decimals: 6,
          };
          issued = { chainId: request.chainId, account: request.account, nonce: request.nonce, calls: request.calls, quote };
          const relayerSignature = await sponsor.sign({ hash: sponsorQuoteDigest({ kind: 'sponsored_batch', ...issued }) });
          res.writeHead(200, { 'content-type': 'application/json' });
          res.end(JSON.stringify({ quote, relayerSignature }));
          return;
        }
        expect(req.url).toBe('/submit');
        const request: SponsoredBindRequest = body;
        expect(issued).toBeDefined();
        expect(request.batch).toEqual(issued);
        expect(await recoverAddress({ hash: constructionDigest({ kind: 'sponsored_batch', ...request.batch }), signature: request.accountSignature })).toBe(request.batch.account);
        expect(await recoverAddress({ hash: sponsorQuoteDigest({ kind: 'sponsored_batch', ...request.batch }), signature: request.relayerSignature as Hex })).toBe(sponsor.address);
        const authorization = request.authorization;
        expect(authorization).toMatchObject({ chainId: String(CHAIN_ID), address: PAYMASTER, nonce: '0' });
        expect(await recoverAddress({ hash: constructionDigest({ kind: 'eip7702_authorization', chainId: authorization.chainId, address: authorization.address, nonce: authorization.nonce }), signature: serializeSignature(authorization) })).toBe(request.batch.account);
        const decoded = decodeFunctionData({ abi: sponsoredBindAbi, data: request.call.data });
        expect(decoded.functionName).toBe('executeSponsored');
        expect(decoded.args![0]).toEqual(request.batch.calls.map(call => ({ ...call, value: BigInt(call.value) })));
        expect(decoded.args![2]).toBe(request.accountSignature);
        expect(decoded.args![3]).toBe(request.relayerSignature);
        expect(request.call.to).toBe(request.batch.account);
        expect(request.call.value).toBe('0');
        const { sharedRpcPool } = await import('../../src/rpc/pool.js');
        const rpc = sharedRpcPool();
        const raw = await sponsor.signTransaction({
          type: 'eip7702', chainId: CHAIN_ID, nonce: await rpc.getTransactionCount(sponsor.address, 'pending'),
          to: request.call.to, value: 0n, data: request.call.data, gas: 200_000n,
          maxFeePerGas: BigInt(request.batch.quote.gasCost) / 200_000n, maxPriorityFeePerGas: chain.tip,
          authorizationList: [{ ...authorization, chainId: Number(authorization.chainId), nonce: Number(authorization.nonce) }],
        });
        const transactionHash = await rpc.sendRawTransaction(raw);
        expect(chain['applyBind'](request.batch.account.toLowerCase(), request.batch.calls[0]!.data)).toBe(true);
        res.writeHead(200, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ transactionHash }));
      })().catch((error: Error) => {
        res.writeHead(500, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ error: 'station_contract_violation', message: error.message }));
      });
    });
  });
  await new Promise<void>((resolve, reject) => {
    station.once('error', reject);
    station.listen(0, '127.0.0.1', resolve);
  });
  const stationAddress = station.address();
  if (!stationAddress || typeof stationAddress === 'string') throw new Error('station did not bind');

  process.env.SUPABASE_URL = identity.url;
  process.env.HYPERPAXEER_RPC_URL = chain.url;
  process.env.HYPERPAXEER_CHAIN_ID = String(CHAIN_ID);
  process.env.RPC_URLS = chain.url;
  process.env.SEQUENCER_RPC_URL = `http://127.0.0.1:${sequencerAddress.port}`;
  process.env.WALLET_GAS_STATION_URL = `http://127.0.0.1:${stationAddress.port}`;
  process.env.PAXEER_GAS_STATION_PAYMASTER = PAYMASTER;
  process.env.PAXEER_GAS_STATION_GAS_LIMIT = '200000';
  process.env.PAXEER_GAS_STATION_MAX_TOKEN_AMOUNT = '1000000';
  process.env.WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE = network.authority.producerKeyFile;
  process.env.WALLET_IDENTITY_BINDING_TENANT = network.authority.tenant;
  process.env.WALLET_CUSTODY_INVENTORY_FILE = network.authority.inventoryFile;
  process.env.WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE = network.authority.inventoryPublicKeyFile;
  process.env.ATTESTOR_ENDPOINTS = network.nodes.map((n) => n.apiUrl).join(',');
  process.env.ATTESTOR_CLIENT_CERT_FILE = network.pki.clientCert;
  process.env.ATTESTOR_CLIENT_KEY_FILE = network.pki.clientKey;
  process.env.ATTESTOR_CA_FILE = network.pki.caFile;
  process.env.ATTESTOR_QUORUM = '3';
  process.env.ATTESTOR_HEALTH_INTERVAL_MS = '1000';
  process.env.ATTESTOR_TIMEOUT_MS = '300000';
  process.env.POLICY_MAX_TX_VALUE_WEI = (NATIVE_PER_TX_CAP_WEI * 10n).toString();
  process.env.POLICY_MAX_DAILY_VALUE_WEI = (NATIVE_PER_TX_CAP_WEI * 100n).toString();
  process.env.AGENT_JWT_SECRET = randomBytes(48).toString('base64url');
  process.env.LOG_LEVEL = 'warn';

  pg = await startPostgres();
  const { buildApp } = await import('../../src/index.js');
  app = await buildApp();
  await app.ready();

  token = await identity.mint(userId);
  const inventoryKeys: WalletInventoryKey[] = [
    { key_id: `wallet:${userId}:standard:secp256k1:0`, owner: userId, epoch: 0, curve: 'secp256k1', public_key: '', operations: ['generate'] },
    { key_id: `account:${userId}:standard:ed25519:0`, owner: userId, epoch: 0, curve: 'ed25519', public_key: '', operations: ['generate'] },
  ];
  network.authority.approveInventory(inventoryKeys);
  const { provisionAccount } = await import('../../src/provision/state.js');
  const { provisionDepsFromEnv } = await import('../../src/routes/wallet.js');
  const deps = provisionDepsFromEnv();
  if (!deps) throw new Error('provisioning dependencies are unavailable');
  const generated = await provisionAccount(deps, { kind: 'standard', userId, token: null });
  expect(generated.awaiting).toBe('owner');
  network.authority.approveInventory(await Promise.all(inventoryKeys.map(async (entry): Promise<WalletInventoryKey> => ({
    ...entry,
    public_key: await network.publicKey(entry.key_id),
    operations: ['generate', 'sign'],
  }))));
  const provisioned = await app.inject({ method: 'POST', url: '/v1/wallet/provision', headers: auth(token) });
  expect(provisioned.statusCode, provisioned.body).toBe(200);
  const body = provisioned.json() as { wallet: { address: `0x${string}`; did: string; main_account_id: string; binding_state: string }; provisioning: { state: string } };
  expect(body.provisioning.state).toBe('active');
  expect(body.wallet.binding_state).toBe('bound');
  expect(body.wallet.address).toMatch(/^0x[0-9a-fA-F]{40}$/);
  const address = body.wallet.address.toLowerCase();
  const pub = chain.bindings.get(address)!;
  expect(pub).toMatch(/^[0-9a-f]{64}$/);
  expect(body.wallet.did).toBe(`did:layerx:${pub}`);
  expect(body.wallet.main_account_id).toBe(chainMainAccountId(pub));
  expect(chain.bindNonces.get(address)).toBe(1n);
  expect(chain.sent.filter((t) => t.from === address)).toEqual([]);
  const binds = chain.sent.filter((t) => t.from === sponsorAddress && t.to === address);
  expect(binds).toHaveLength(1);
  const { sponsoredBindAbi } = await import('../../src/provision/bind.js');
  const decodedBind = decodeFunctionData({ abi: sponsoredBindAbi, data: binds[0]!.data! });
  expect(decodedBind.functionName).toBe('executeSponsored');
  expect(decodedBind.args![0]).toHaveLength(1);
  expect(decodedBind.args![0][0]!.to).toBe(ADDR);
  const provisioning = await deps.pool.query<{ bind_tx_hash: Hex; station_request: SponsoredBindRequest }>(
    `select bind_tx_hash, station_request from account_provisioning where user_id = $1 and kind = 'standard'`,
    [userId],
  );
  expect(provisioning.rows).toHaveLength(1);
  const binding = provisioning.rows[0]!;
  expect(binding.station_request.batch.account.toLowerCase()).toBe(address);
  expect(binding.station_request.batch.calls[0]!.to).toBe(ADDR);
  expect(decodedBind.args![0]).toEqual(binding.station_request.batch.calls.map((call) => ({ ...call, value: BigInt(call.value) })));
  const rawBind = broadcasts.find((raw) => keccak256(raw) === binds[0]!.hash);
  expect(rawBind).toBeDefined();
  expect(parseTransaction(rawBind!)).toMatchObject({
    type: 'eip7702',
    authorizationList: [{ ...binding.station_request.authorization, chainId: CHAIN_ID, address: PAYMASTER, nonce: 0 }],
  });
  expect(chain.receipts.get(binds[0]!.hash)!.status).toBe('0x1');
  expect(binds[0]!.hash).toBe(binding.bind_tx_hash);

  const { getPool } = await import('../../src/db/pool.js');
  const { rows } = await getPool().query<{ attestor_key_id: string; migrated_at: Date | null; encrypted_private_key: string | null }>(
    `select attestor_key_id, migrated_at, encrypted_private_key from wallets where user_id = $1 and kind = 'standard'`,
    [userId],
  );
  expect(rows).toHaveLength(1);
  expect(rows[0]!.attestor_key_id).toBe(`wallet:${userId}:standard:secp256k1:0`);
  expect(rows[0]!.migrated_at).not.toBeNull();
  expect(rows[0]!.encrypted_private_key).toBeNull();
  key = { keyId: rows[0]!.attestor_key_id, address: body.wallet.address };
  chain.balances.set(address, (chain.balances.get(address) ?? 0n) + NATIVE_PER_TX_CAP_WEI * 2n);
}, 900_000);

afterAll(async () => {
  await app?.close();
  const { provisionDepsFromEnv } = await import('../../src/routes/wallet.js');
  if (app) provisionDepsFromEnv()?.attestors?.close();
  const { closePool } = await import('../../src/db/pool.js');
  await closePool();
  await pg?.stop();
  await network?.stop();
  await identity?.close();
  if (station) await new Promise<void>((resolve, reject) => {
    station.close((error) => error ? reject(error) : resolve());
  });
  if (sequencerProxy) await new Promise<void>((resolve, reject) => {
    sequencerProxy.close((error) => error ? reject(error) : resolve());
  });
  await chain?.stop();
}, 120_000);

async function evmReviewApproveSignReturnsReceipt(): Promise<void> {
  const unknown = await fetch(chain.url, {
    method: 'POST', headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'pax_sequencerReceipt', params: [`0x${'00'.repeat(32)}`] }),
  });
  expect(await unknown.json()).toEqual({ jsonrpc: '2.0', id: 1, result: null });
  const { sharedRpcPool } = await import('../../src/rpc/pool.js');
  const headers = auth(await identity.mint(userId));
  const transfer = {
    type: 'eip1559',
    chainId: CHAIN_ID,
    nonce: await sharedRpcPool().getTransactionCount(key.address, 'pending'),
    to: RECIPIENT,
    value: 100_000_000_000_000_000n,
    gas: 21_000n,
    maxFeePerGas: 2_000_000_000n,
    maxPriorityFeePerGas: 1_000_000_000n,
  } as const;
  const unsigned = serializeTransaction(transfer);
  const reviewed = await app.inject({
    method: 'POST', url: '/v1/wallet/evm/review', headers, payload: { transaction: unsigned },
  });
  expect(reviewed.statusCode, reviewed.body).toBe(200);
  const review = reviewed.json();
  expect(review).toMatchObject({
    state: 'reviewed', chain_id: CHAIN_ID, to: getAddress(transfer.to),
    value: transfer.value.toString(), data: '0x', signing_hash: keccak256(unsigned),
    key_id: key.keyId, principal: userId,
  });
  expect(review.id).toMatch(/^[0-9a-f-]{36}$/);
  expect(review.participants).toHaveLength(3);
  expect(new Set(review.participants).size).toBe(3);

  const approved = await app.inject({
    method: 'POST', url: '/v1/wallet/evm/approve', headers,
    payload: { review_id: review.id, signing_hash: review.signing_hash, decision: 'approve' },
  });
  expect(approved.statusCode, approved.body).toBe(200);
  expect(approved.json()).toMatchObject({
    ...review, state: 'approved',
    approval: {
      version: 1, principal: userId, key_id: key.keyId, session_id: review.session_id,
      signing_hash: keccak256(unsigned), expires_at: review.expires_at,
    },
  });

  broadcasts.length = 0;
  sequencerReceipts.length = 0;
  const signed = await app.inject({
    method: 'POST', url: '/v1/wallet/evm/sign', headers,
    payload: { approval_id: review.id },
  });
  expect(signed.statusCode, signed.body).toBe(200);
  const answer = signed.json();
  expect(answer).toMatchObject({ id: review.id, state: 'signed', signing_hash: review.signing_hash });
  expect(broadcasts).toHaveLength(1);
  const raw = broadcasts[0]!;
  expect(answer.tx_hash).toBe(keccak256(raw));
  expect(parseTransaction(raw)).toMatchObject(transfer);
  expect((await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase()).toBe(key.address.toLowerCase());
  expect(['receipted', 'committed']).toContain(answer.rung);
  expect(answer.receipt).toMatchObject({ txHash: answer.tx_hash, rung: answer.rung, status: '0x1' });
  expect(answer.receipt.receipt).toMatch(/^0x(?:[0-9a-fA-F]{2})+$/);
  expect(sequencerReceipts).toEqual([{ hash: answer.tx_hash, result: answer.receipt }]);
  expect(answer.receipt).toMatchObject({
    rung: 'receipted', nonce: toHex(transfer.nonce), from: key.address.toLowerCase(), to: RECIPIENT,
    keyId: keccak256(toHex(chain.sequencerPublicKey.export({ type: 'spki', format: 'der' }))),
  });
  const { receipt: signature, ...receiptPayload } = answer.receipt;
  expect(verify(
    null, Buffer.from(JSON.stringify(receiptPayload)), chain.sequencerPublicKey,
    Buffer.from(signature.slice(2), 'hex'),
  )).toBe(true);
}

describe('gateway against five real attestor daemons', () => {
  test(evmReviewApproveSignReturnsReceipt, evmReviewApproveSignReturnsReceipt, 300_000);

  it('reports every readiness component up', async () => {
    const res = await app.inject({ method: 'GET', url: '/readyz' });
    const body = res.json();
    expect(res.statusCode, JSON.stringify(body)).toBe(200);
    expect(body.ready).toBe(true);
    expect(body.components.attestors).toMatchObject({ state: 'up', required: 3, healthy: 5 });
    expect(body.components.nonce_store.state).toBe('up');
    expect(body.components.rpc_pool).toMatchObject({ state: 'up', healthy: 1 });
    expect(body.components.rpc_pool.endpoints).toEqual([expect.objectContaining({ url: chain.url, state: 'healthy' })]);
    expect(body.components.identity_provider).toMatchObject({ state: 'up', keys: 1 });
  });

  it('answers the provisioned wallet with the attestor key address', async () => {
    const res = await app.inject({ method: 'GET', url: '/v1/wallet/me', headers: auth(token) });
    expect(res.statusCode).toBe(200);
    expect(res.json().wallet.address.toLowerCase()).toBe(key.address.toLowerCase());
  });

  it('signs a transaction through the quorum and the signature recovers the wallet', async () => {
    const res = await app.inject({ method: 'POST', url: '/v1/wallet/sign', headers: auth(await identity.mint(userId)), payload: txBody(1000n) });
    const body = res.json();
    expect(res.statusCode, JSON.stringify(body)).toBe(200);
    const signed = body.signed_tx as Hex;
    const tx = parseTransaction(signed);
    expect(tx.chainId).toBe(CHAIN_ID);
    expect(tx.to?.toLowerCase()).toBe(RECIPIENT);
    expect(tx.value).toBe(1000n);
    const recovered = await recoverTransactionAddress({ serializedTransaction: signed as never });
    expect(recovered.toLowerCase()).toBe(key.address.toLowerCase());

    const { getPool } = await import('../../src/db/pool.js');
    const { rows } = await getPool().query<{ path: string; decision: string; attestor_audit: Array<{ node_id: string; audit_sequence: number }> }>(
      `select path, decision, attestor_audit from signing_audit where client_subject = $1 and route = '/v1/wallet/sign' order by id desc limit 1`,
      [userId],
    );
    expect(rows[0]).toMatchObject({ path: 'attestor', decision: 'signed' });
    expect(rows[0]!.attestor_audit).toHaveLength(3);
    for (const entry of rows[0]!.attestor_audit) {
      expect(network.nodes.map((n) => n.nodeId)).toContain(entry.node_id);
      expect(entry.audit_sequence).toBeGreaterThan(0);
    }
  }, 300_000);

  it('signs a personal message and the signature recovers the wallet', async () => {
    const message = `sign in to the wallet ${randomUUID()}`;
    const res = await app.inject({ method: 'POST', url: '/v1/wallet/sign-message', headers: auth(await identity.mint(userId)), payload: { message } });
    const body = res.json();
    expect(res.statusCode, JSON.stringify(body)).toBe(200);
    const recovered = await recoverMessageAddress({ message, signature: body.signature as Hex });
    expect(recovered.toLowerCase()).toBe(key.address.toLowerCase());
  }, 300_000);

  it('signs typed data and the signature recovers the wallet', async () => {
    const res = await app.inject({
      method: 'POST',
      url: '/v1/wallet/sign-typed-data',
      headers: auth(await identity.mint(userId)),
      payload: { typed_data: typedData },
    });
    const body = res.json();
    expect(res.statusCode, JSON.stringify(body)).toBe(200);
    const recovered = await recoverTypedDataAddress({
      ...(typedData as unknown as TypedDataDefinition),
      signature: body.signature as Hex,
    });
    expect(recovered.toLowerCase()).toBe(key.address.toLowerCase());
  }, 300_000);

  it('refuses a token signed by a key outside the identity provider', async () => {
    const forged = await identity.mintForeign(userId);
    const res = await app.inject({ method: 'POST', url: '/v1/wallet/sign', headers: auth(forged), payload: txBody(1000n) });
    expect(res.statusCode).toBe(401);
    expect(res.json().error).toBe('unauthorized');
  });

  it('refuses a transaction over the attestor policy cap', async () => {
    const res = await app.inject({
      method: 'POST',
      url: '/v1/wallet/sign',
      headers: auth(await identity.mint(userId)),
      payload: txBody(NATIVE_PER_TX_CAP_WEI + 1n),
    });
    const body = res.json();
    expect(res.statusCode, JSON.stringify(body)).toBe(403);
    expect(body).toMatchObject({ error: 'attestor_policy_refused', category: 'policy', code: 'policy_denied' });
  }, 300_000);

  it('refuses an unsigned agent write', async () => {
    const res = await app.inject({ method: 'POST', url: '/v1/agent/send', payload: txBody(1000n) });
    expect(res.statusCode).toBe(401);
    expect(res.json().error).toBe('agent_signature_required');
  });

  it('mounts custody and constrained digest adapters under real wallet authentication', async () => {
    for (const url of ['/v1/wallet/sign-custody', '/v1/wallet/sign-digest']) {
      const unauthenticated = await app.inject({ method: 'POST', url, payload: {} });
      expect(unauthenticated.statusCode).toBe(401);
    }
  });

  it('refuses opaque custody bytes and bare digest before requesting a signing session', async () => {
    const headers = auth(await identity.mint(userId));
    const custody = await app.inject({ method: 'POST', url: '/v1/wallet/sign-custody', headers,
      payload: { custody: '0x4c583a435553544f44593a7631' + '00'.repeat(32) } });
    expect(custody.statusCode).toBe(400);
    const digest = await app.inject({ method: 'POST', url: '/v1/wallet/sign-digest', headers,
      payload: { digest: '0x' + '01'.repeat(32) } });
    expect(digest.statusCode).toBe(400);
  });

  it('answers not ready with a typed body once the attestor quorum is lost', async () => {
    for (const node of network.nodes.slice(0, 3)) await network.stopNode(node);
    const res = await app.inject({ method: 'GET', url: '/readyz' });
    const body = res.json();
    expect(res.statusCode).toBe(503);
    expect(body.error).toBe('not_ready');
    expect(body.ready).toBe(false);
    expect(body.components.attestors).toMatchObject({ state: 'down', required: 3, reason: 'attestor_quorum_unavailable' });
    expect(body.components.attestors.healthy).toBeLessThan(3);
    expect(body.components.nonce_store.state).toBe('up');
    expect(body.components.rpc_pool.state).toBe('up');
    expect(body.components.identity_provider.state).toBe('up');
  }, 120_000);
});


describe('wallet custody boundary against the gateway database', () => {
  it('refuses a migrated row before touching its retained legacy envelope', async () => {
    const { findWalletByUserId, getSigningAccountForRow, WalletMigratedError } = await import('../../src/db/wallets.js');
    const row = await findWalletByUserId(userId);
    expect(row).not.toBeNull();
    await expect(getSigningAccountForRow(row!)).rejects.toBeInstanceOf(WalletMigratedError);
  });

  it('rechecks custody when an already-created legacy signing handle is used', async () => {
    const { provisionWalletForUser, getSigningAccountForRow, WalletMigratedError } = await import('../../src/db/wallets.js');
    const { getPool } = await import('../../src/db/pool.js');
    const { row } = await provisionWalletForUser(randomUUID(), 'standard');
    const account = await getSigningAccountForRow(row);
    await getPool().query('update wallets set migrated_at = now(), attestor_key_id = $2 where id = $1', [row.id, key.keyId]);
    await expect(account.signMessage({ message: 'migration must revoke this stale handle' })).rejects.toBeInstanceOf(WalletMigratedError);
    const { rows } = await getPool().query('select encrypted_private_key from wallets where id = $1', [row.id]);
    expect(rows[0].encrypted_private_key).not.toBeNull();
  });

  it('refuses null-envelope legacy signing without decrypting', async () => {
    const { provisionWalletForUser, getSigningAccountForRow, WalletMigratedError } = await import('../../src/db/wallets.js');
    const { getPool } = await import('../../src/db/pool.js');
    const { row } = await provisionWalletForUser(randomUUID(), 'standard');
    await getPool().query('update wallets set encrypted_private_key = null where id = $1', [row.id]);
    await expect(getSigningAccountForRow(row)).rejects.toBeInstanceOf(WalletMigratedError);
  });
});
