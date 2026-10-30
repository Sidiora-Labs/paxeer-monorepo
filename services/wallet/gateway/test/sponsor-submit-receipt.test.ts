import Fastify, { type FastifyInstance } from 'fastify';
import { randomUUID, verify } from 'node:crypto';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { encodeFunctionData, parseAbi, parseSignature, toHex, type Hex } from 'viem';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import type { RpcPool, SequencerReceiptResult } from '../src/rpc/pool.js';
import { TestChain } from './support/chain.js';
import { startIdentityProvider, type IdentityProvider } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const chain = new TestChain(125);
const sponsor = privateKeyToAccount(generatePrivateKey());
const executeAbi = parseAbi([
  'function executeSponsored((address to,uint256 value,bytes data)[] calls,(address sponsor,address token,uint256 maxTokenAmount,uint256 tokenAmount,uint256 deadline,uint256 quoteNonce,uint256 gasCost) quote,bytes accountSignature,bytes relayerSignature)',
]);
type SubmitBody = import('zod').infer<typeof import('../src/routes/wallet.js').SponsoredSubmitBody>;
type StationSubmission = {
  call: { to: Hex; value: string; data: Hex };
  authorization: SubmitBody['authorization'];
  batch: Omit<SubmitBody['construction'], 'kind'> & {
    quote: SubmitBody['construction']['quote'] & { decimals: 6 };
  };
  accountSignature: Hex;
  relayerSignature: Hex;
};
type Mode = 'normal' | 'status-disconnected' | 'submit-disconnected' | 'sequencer-disconnected';
const submissions = new Map<string, { hash: Hex; nonce: number; deadline: string }>();
const calls: { path: string; body: unknown }[] = [];
const endpointErrors: unknown[] = [];
let mode: Mode = 'normal';
let expectedSubmission: StationSubmission;
let identity: IdentityProvider;
let postgres: EphemeralPostgres;
let server: Server;
let app: FastifyInstance;
let rpc: RpcPool;
let relayRpc: RpcPool;
let database: typeof import('../src/db/pool.js');
let wallets: typeof import('../src/db/wallets.js');

beforeAll(async () => {
  identity = await startIdentityProvider();
  await chain.start();
  chain.balances.set(sponsor.address.toLowerCase(), 10n ** 20n);
  server = createServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
      expect(req.method).toBe('POST');
      expect(req.headers['content-type']).toBe('application/json');
      res.setHeader('content-type', 'application/json');
      calls.push({ path: req.url!, body });
      if (req.url === '/rpc' || req.url === '/sequencer') {
        if (req.url === '/sequencer') {
          expect(body.method).toBe('pax_sequencerReceipt');
          expect(chain.receipts.has(body.params[0])).toBe(true);
          if (mode === 'sequencer-disconnected') {
            res.destroy();
            return;
          }
        } else {
          expect(['eth_blockNumber', 'eth_call']).toContain(body.method);
        }
        const upstream = await fetch(chain.url, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify(body),
        });
        res.end(await upstream.text());
        return;
      }
      if (req.url === '/station/submit') {
        expect(body).toStrictEqual(expectedSubmission);
        const submission = body as StationSubmission;
        const nonce = await relayRpc.getTransactionCount(sponsor.address);
        const raw = await sponsor.signTransaction({
          type: 'eip1559',
          chainId: chain.chainId,
          nonce,
          to: submission.call.to,
          value: BigInt(submission.call.value),
          data: submission.call.data,
          gas: TestChain.gasFor(submission.call.to, submission.call.data),
          maxFeePerGas: chain.baseFee + chain.tip,
          maxPriorityFeePerGas: chain.tip,
        });
        const hash = await relayRpc.sendRawTransaction(raw);
        submissions.set(submission.batch.quote.quoteNonce, {
          hash,
          nonce,
          deadline: submission.batch.quote.deadline,
        });
        if (mode === 'submit-disconnected') res.destroy();
        else res.end(JSON.stringify({ transactionHash: hash }));
        return;
      }
      expect(req.url).toBe('/station/status');
      expect(body).toStrictEqual({
        account: expectedSubmission.batch.account,
        sponsor: sponsor.address,
        quoteNonce: expectedSubmission.batch.quote.quoteNonce,
        relayerSignature: expectedSubmission.relayerSignature,
      });
      const submission = submissions.get(body.quoteNonce);
      if (!submission) throw new Error('status requested before relay');
      const receipt = chain.receipts.get(submission.hash);
      if (!receipt) throw new Error('relayed transaction has no applied receipt');
      if (mode === 'status-disconnected') {
        res.destroy();
        return;
      }
      res.end(
        JSON.stringify({
          state: 'completed',
          deadline: submission.deadline,
          submission: { sponsorNonce: String(submission.nonce), transactionHash: submission.hash },
          replacement: null,
          completion: {
            outcome: 'included',
            transactionHash: submission.hash,
            blockNumber: BigInt(receipt.blockNumber).toString(),
            sidCollected: expectedSubmission.batch.quote.tokenAmount,
            paxSpent: (BigInt(receipt.gasUsed) * (chain.baseFee + chain.tip)).toString(),
          },
        })
      );
    } catch (error) {
      endpointErrors.push(error);
      res.writeHead(500);
      res.end(JSON.stringify({ error: 'relay_contract_violation' }));
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const url = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.SUPABASE_URL = identity.url;
  process.env.HYPERPAXEER_CHAIN_ID = String(chain.chainId);
  process.env.HYPERPAXEER_RPC_URL = `${url}/rpc`;
  process.env.RPC_URLS = `${url}/rpc`;
  process.env.SEQUENCER_RPC_URL = `${url}/sequencer`;
  process.env.LOG_LEVEL = 'error';
  postgres = await startPostgres();
  database = await import('../src/db/pool.js');
  wallets = await import('../src/db/wallets.js');
  const { RpcPool, sharedRpcPool } = await import('../src/rpc/pool.js');
  const { walletSponsorRoutes } = await import('../src/routes/wallet.js');
  relayRpc = new RpcPool({
    urls: [chain.url],
    sequencerUrl: chain.url,
    chainId: chain.chainId,
    timeoutMs: 2_000,
    healthIntervalMs: 60_000,
    lagThresholdBlocks: 2,
  });
  rpc = sharedRpcPool();
  app = Fastify();
  await app.register(walletSponsorRoutes, { stationUrl: `${url}/station` });
  await app.ready();
}, 120_000);

afterAll(async () => {
  await app?.close();
  rpc?.stop();
  relayRpc?.stop();
  if (server) {
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve()))
    );
  }
  await database?.closePool();
  await postgres?.stop();
  await chain.stop();
  await identity?.stop();
}, 120_000);

async function sponsoredSubmitReturnsSequencerReceipt(): Promise<void> {
  const { constructionDigest, sponsorQuoteDigest } = await import('../src/attestor/client.js');
  const { decrypt, loadMasterKey } = await import('../src/crypto.js');
  const { env } = await import('../src/env.js');
  const { wallet, row } = await wallets.provisionWalletForUser(randomUUID());
  if (!row.encrypted_private_key) throw new Error('wallet is missing its signing envelope');
  const key = decrypt(row.encrypted_private_key, loadMasterKey(env.WALLET_MASTER_KEY));
  const owner = privateKeyToAccount(toHex(key));
  key.fill(0);
  expect(owner.address).toBe(wallet.address);
  const token = await identity.mintUserToken(row.user_id);
  let quoteNonce = 0;
  for (const scenario of [
    'normal',
    'status-disconnected',
    'submit-disconnected',
    'sequencer-disconnected',
  ] as const) {
    mode = scenario;
    const construction: SubmitBody['construction'] = {
      kind: 'sponsored_batch',
      chainId: String(chain.chainId),
      account: wallet.address,
      nonce: '0',
      calls: [{ to: sponsor.address, value: '0', data: '0x' }],
      quote: {
        sponsor: sponsor.address,
        token: '0x000000000000000000000000000000000000c0de',
        maxTokenAmount: '1000',
        tokenAmount: '10',
        deadline: String(Math.floor(Date.now() / 1_000) + 600),
        quoteNonce: String(quoteNonce++),
        gasCost: '1000000000000000',
      },
    };
    const accountSignature = await owner.sign({ hash: constructionDigest(construction) });
    const relayerSignature = await sponsor.sign({ hash: sponsorQuoteDigest(construction) });
    const authorizationConstruction = {
      kind: 'eip7702_authorization' as const,
      chainId: construction.chainId,
      address: sponsor.address,
      nonce: '0',
    };
    const auth = parseSignature(
      await owner.sign({ hash: constructionDigest(authorizationConstruction) })
    );
    const authorization: SubmitBody['authorization'] = {
      chainId: authorizationConstruction.chainId,
      address: authorizationConstruction.address,
      nonce: authorizationConstruction.nonce,
      r: auth.r,
      s: auth.s,
      yParity: auth.yParity!,
    };
    const q = construction.quote;
    const data = encodeFunctionData({
      abi: executeAbi,
      functionName: 'executeSponsored',
      args: [
        construction.calls.map((call) => ({
          ...call,
          to: call.to as Hex,
          data: call.data as Hex,
          value: BigInt(call.value),
        })),
        {
          sponsor: q.sponsor as Hex,
          token: q.token as Hex,
          maxTokenAmount: BigInt(q.maxTokenAmount),
          tokenAmount: BigInt(q.tokenAmount),
          deadline: BigInt(q.deadline),
          quoteNonce: BigInt(q.quoteNonce),
          gasCost: BigInt(q.gasCost),
        },
        accountSignature,
        relayerSignature,
      ],
    });
    expectedSubmission = {
      call: { to: wallet.address, value: '0', data },
      authorization,
      batch: {
        chainId: construction.chainId,
        account: construction.account,
        nonce: construction.nonce,
        calls: construction.calls,
        quote: { ...q, decimals: 6 },
      },
      accountSignature,
      relayerSignature,
    };
    const payload: SubmitBody = {
      chain_id: construction.chainId,
      account: wallet.address,
      to: wallet.address,
      value: '0',
      data,
      construction,
      account_signature: accountSignature,
      relayer_signature: relayerSignature,
      authorization,
      quote_decimals: 6,
    };
    const start = calls.length;
    const sentBefore = chain.sent.length;
    const response = await app.inject({
      method: 'POST',
      url: '/v1/wallet/sponsored/submit',
      headers: { authorization: `Bearer ${token}` },
      payload,
    });
    expect(chain.sent).toHaveLength(sentBefore + 1);
    const sent = chain.sent[sentBefore]!;
    expect(sent.from).toBe(sponsor.address.toLowerCase());
    expect(sent.to).toBe(wallet.address.toLowerCase());
    expect(sent.data).toBe(data);
    const requests = calls.slice(start).filter((call) => call.path !== '/rpc');
    expect(requests.map((call) => call.path)).toStrictEqual([
      '/station/submit',
      '/station/status',
      '/sequencer',
    ]);
    expect(requests[2]!.body).toMatchObject({
      method: 'pax_sequencerReceipt',
      params: [sent.hash],
    });
    if (scenario === 'sequencer-disconnected') {
      expect(response.statusCode, response.body).toBe(503);
      expect(response.json()).toStrictEqual({
        error: 'sequencer_receipt_unavailable',
        status: 'pending',
      });
    } else {
      expect(response.statusCode, response.body).toBe(200);
      expect(response.headers['cache-control']).toBe('no-store');
      const body = response.json<{
        tx_hash: Hex;
        status: string;
        did: string;
        station: Record<string, unknown> | null;
        rung: SequencerReceiptResult['rung'];
        receipt: SequencerReceiptResult;
      }>();
      const receipt = await relayRpc.sequencerReceipt(sent.hash);
      expect(body.tx_hash).toBe(sent.hash);
      expect(body.did).toBe(wallet.address.toLowerCase());
      expect(body.status).toBe(scenario === 'status-disconnected' ? 'pending' : 'confirmed');
      if (scenario === 'status-disconnected') expect(body.station).toBeNull();
      else
        expect(body.station).toMatchObject({
          state: 'completed',
          completion: { transactionHash: sent.hash },
        });
      expect(body.rung).toBe('receipted');
      expect(body.receipt).toStrictEqual(receipt);
      expect(body.receipt.txHash).toBe(sent.hash);
      expect(body.receipt.rung).toBe(body.rung);
      if (body.receipt.rung === 'submitted')
        throw new Error('relayed transaction must be receipted');
      const { receipt: signature, ...signed } = body.receipt;
      expect(
        verify(
          null,
          Buffer.from(JSON.stringify(signed)),
          chain.sequencerPublicKey,
          Buffer.from(signature.slice(2), 'hex')
        )
      ).toBe(true);
    }
    expect(endpointErrors).toStrictEqual([]);
  }
}

test(sponsoredSubmitReturnsSequencerReceipt);
